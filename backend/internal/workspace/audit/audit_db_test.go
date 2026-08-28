package audit_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/jackc/pgx/v5/pgxpool"
)

// dsn is where the test database is, or "" when there is none. The whole file
// skips rather than fails in that case, the way the rest of this repository's
// database tests do.
func dsn() string {
	if url := os.Getenv("AUDIT_TEST_DATABASE_URL"); url != "" {
		return url
	}
	return os.Getenv("DATABASE_URL")
}

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := dsn()
	if url == "" {
		t.Skip("neither AUDIT_TEST_DATABASE_URL nor DATABASE_URL is set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("database unreachable: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// The event has to survive the process. Before audit_events existed it went to
// stdout and nowhere else, which meant "who read my data" had no answer at all.
func TestRecordPersistsToDatabase(t *testing.T) {
	pool := openPool(t)
	audit.UseDatabase(pool)
	t.Cleanup(func() { audit.UseDatabase(nil) })

	ctx := context.Background()
	const action = "test.audit_persisted"
	_, err := pool.Exec(ctx, `DELETE FROM workspace.audit_events WHERE action = $1`, action)
	if err != nil {
		t.Fatalf("clean up before: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM workspace.audit_events WHERE action = $1`, action)
	})

	audit.Record(ctx, "", "device:abc", action, "unit_test", map[string]any{"reason": "coverage"})

	var count int
	var resource, userID, reason string
	err = pool.QueryRow(ctx,
		`SELECT count(*) OVER (), resource, user_id, details->>'reason'
		   FROM workspace.audit_events WHERE action = $1`, action).
		Scan(&count, &resource, &userID, &reason)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one row, got %d", count)
	}
	if resource != "unit_test" || userID != "device:abc" || reason != "coverage" {
		t.Fatalf("row does not match what was recorded: resource=%q user=%q reason=%q",
			resource, userID, reason)
	}
}

// A user id that is not a UUID has to be storable: the device handlers record
// "device:<id>" for an act nobody signed in for.
func TestRecordAcceptsNonUUIDActor(t *testing.T) {
	pool := openPool(t)
	audit.UseDatabase(pool)
	t.Cleanup(func() { audit.UseDatabase(nil) })

	ctx := context.Background()
	const action = "test.audit_non_uuid_actor"
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM workspace.audit_events WHERE action = $1`, action)
	})

	audit.Record(ctx, "", "device:kiosk-7", action, "device", nil)

	var stored string
	if err := pool.QueryRow(ctx,
		`SELECT user_id FROM workspace.audit_events WHERE action = $1`, action).Scan(&stored); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if stored != "device:kiosk-7" {
		t.Fatalf("actor was not stored verbatim: %q", stored)
	}
}

// Recording must never be able to fail the act it is recording. With no pool at
// all — which is every test server and the window before UseDatabase is called
// — Record still writes its log line and returns.
func TestRecordWithoutDatabaseIsHarmless(t *testing.T) {
	audit.UseDatabase(nil)
	audit.Record(context.Background(), "", "someone", "test.no_database", "nothing", nil)
}

// An act that belongs to no organisation is still written down.
//
// The regression: two call sites passed the word "unknown" as the tenant, the
// column is uuid, and the insert was refused every time — so from 2026-08-08 no
// failed sign-in and no consent-before-the-account-is-known reached the audit
// table on any deployment. Nothing surfaced it: audit.Record logs the event
// before it stores it, so the line an operator would grep for was there, and
// the only trace of the loss was a WARN beside it.
//
// Asserted through Record rather than through the SQL, because the SQL was
// always right: persist has cast through NULLIF($1, ”) since it was written.
// What was wrong was the value handed to it, which is the shape of bug that
// lives at the call site and is invisible one layer down.
func TestAnActWithNoOrganisationIsStillRecorded(t *testing.T) {
	pool := openPool(t)
	audit.UseDatabase(pool)
	t.Cleanup(func() { audit.UseDatabase(nil) })
	ctx := context.Background()

	action := "test.no_tenant." + uuid.NewString()[:12]
	audit.Record(ctx, audit.NoTenant, audit.Anonymous, action, "user",
		map[string]any{"email": "nobody@example.mn"})
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM workspace.audit_events WHERE action = $1`, action)
	})

	var rows int
	var userID *string
	var tenantID *string
	if err := pool.QueryRow(ctx,
		`SELECT count(*), max(user_id), max(tenant_id::text)
		   FROM workspace.audit_events WHERE action = $1`, action).
		Scan(&rows, &userID, &tenantID); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("an act with no organisation left %d audit row(s), want one", rows)
	}
	if tenantID != nil {
		t.Errorf("the row names organisation %q; it should name none", *tenantID)
	}
	// And the person is still named as far as they are known, which is the
	// asymmetry the two constants exist to hold: user_id is text and stores the
	// word, tenant_id is a uuid and cannot.
	if userID == nil || *userID != audit.Anonymous {
		t.Errorf("the row's user is %v, want %q", userID, audit.Anonymous)
	}
}
