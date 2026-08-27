package dbguard_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/dbguard"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A guard that has not been probed must not touch the connection at all. A
// deployment whose migrations have not reached 00079 keeps working exactly as
// it did, on the application filter alone.
func TestDormantGuardLeavesConnectionsAlone(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://u:p@127.0.0.1:1/none")
	if err != nil {
		t.Fatal(err)
	}
	guard := &dbguard.Guard{}
	guard.Install(cfg)
	if guard.Enabled() {
		t.Fatal("a freshly built guard reports itself enabled")
	}
	if cfg.PrepareConn == nil {
		t.Fatal("Install did not attach a PrepareConn hook")
	}
	// nil connection would panic if the hook tried to talk to it.
	ok, err := cfg.PrepareConn(context.Background(), nil)
	if err != nil {
		t.Fatalf("a dormant guard returned an error: %v", err)
	}
	if !ok {
		t.Fatal("a dormant guard refused a connection")
	}
}

// openGuardedPool builds the pool the API builds, against a migrated database.
func openGuardedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	guard := &dbguard.Guard{}
	guard.Install(cfg)
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := guard.Probe(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	if !guard.Enabled() {
		t.Skip("the database is not carrying the tenant policies; run migrations to 00079")
	}
	return pool
}

// probeTable is a tenant-scoped table this test owns.
//
// These tests used to borrow `contacts`, which was a commerce table and left
// with commerce: migration 00075 dropped it and five tests failed at once. What
// they need is not that table, or any app's — it is *a* table with a tenant_id
// and the platform's policy on it, which is a thing a test can make for itself.
// Borrowing one made a test of the platform's tenant isolation depend on which
// apps happened to be installed.
func probeTable(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	name := "dbguard_probe"

	// The login role owns it, and the policy names the tenant role the guard
	// switches to — the same arrangement every platform table has.
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS ` + name + ` (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id uuid REFERENCES tenants(id) ON DELETE CASCADE,
			name text NOT NULL)`,
		`ALTER TABLE ` + name + ` ENABLE ROW LEVEL SECURITY`,
		`ALTER TABLE ` + name + ` FORCE ROW LEVEL SECURITY`,
		`DROP POLICY IF EXISTS tenant_isolation ON ` + name,
		`CREATE POLICY tenant_isolation ON ` + name + ` TO ` + dbguard.TenantRole + `
			USING (tenant_id IS NULL OR tenant_id = ANY (COALESCE(
				NULLIF(current_setting('app.allowed_tenants', true), '')::uuid[],
				ARRAY[NULLIF(current_setting('app.current_tenant', true), '')::uuid])))
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)`,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON ` + name + ` TO ` + dbguard.TenantRole,
	} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("prepare the probe table: %v", err)
		}
	}
	return name
}

// seedTwoTenants leaves one probe row in each of two tenants and returns their ids.
func seedTwoTenants(t *testing.T, pool *pgxpool.Pool) (string, string) {
	t.Helper()
	ctx := context.Background()
	probeTable(t, pool)
	var first, second string
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	for i, target := range []*string{&first, &second} {
		slug := "guardtest-" + suffix + "-" + string(rune('a'+i))
		if err := pool.QueryRow(ctx,
			`INSERT INTO registry.tenants (name, slug) VALUES ($1,$1) RETURNING id::text`, slug).Scan(target); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO dbguard_probe (id, tenant_id, name)
			 VALUES ($1,$2,$3)`,
			uuid.NewString(), *target, "contact-"+slug); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		// Deleting through the platform path: the sweep is not acting for a tenant.
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = ANY($1)`,
			[]string{first, second})
	})
	return first, second
}

// The whole point, stated as a test: a query with no tenant clause at all —
// the mistake this layer exists to survive — returns only the caller's rows.
func TestQueryWithoutATenantClauseSeesOnlyItsOwnTenant(t *testing.T) {
	pool := openGuardedPool(t)
	first, second := seedTwoTenants(t, pool)

	const noFilter = `SELECT count(*) FROM dbguard_probe WHERE name LIKE 'contact-guardtest-%'`

	var visible int
	ctx := nexus.WithTenantID(context.Background(), first)
	if err := pool.QueryRow(ctx, noFilter).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 1 {
		t.Errorf("acting for the first tenant, an unfiltered query saw %d rows, want 1", visible)
	}

	ctx = nexus.WithTenantID(context.Background(), second)
	if err := pool.QueryRow(ctx, noFilter).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 1 {
		t.Errorf("acting for the second tenant, an unfiltered query saw %d rows, want 1", visible)
	}

	// No tenant in context is the platform path — signing in, sweeping, syncing
	// the catalogue. It must still see everything, or half the platform breaks.
	if err := pool.QueryRow(context.Background(), noFilter).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 2 {
		t.Errorf("the platform path saw %d rows, want 2", visible)
	}
}

// Reading another tenant's rows is one half; writing into them is the other.
func TestWritingIntoAnotherTenantIsRefused(t *testing.T) {
	pool := openGuardedPool(t)
	first, second := seedTwoTenants(t, pool)

	ctx := nexus.WithTenantID(context.Background(), first)
	_, err := pool.Exec(ctx,
		`INSERT INTO dbguard_probe (id, tenant_id, name)
		 VALUES ($1,$2,'planted')`, uuid.NewString(), second)
	if err == nil {
		t.Fatal("a tenant inserted a row belonging to another tenant")
	}
	if !strings.Contains(err.Error(), "row-level security") {
		t.Fatalf("refused, but not by the policy: %v", err)
	}

	tag, err := pool.Exec(ctx,
		`UPDATE dbguard_probe SET name='rewritten' WHERE tenant_id=$1`, second)
	if err != nil {
		t.Fatal(err)
	}
	if tag.RowsAffected() != 0 {
		t.Errorf("updated %d of another tenant's rows, want 0", tag.RowsAffected())
	}
}

// ai_prompts and ai_knowledge carry a NULL tenant_id for the rows the platform
// publishes to everybody, and the copilot reads them with
// `tenant_id IS NULL OR tenant_id = $1`. A policy of bare equality would have
// hidden them — the shared prompt would have stopped being applied, quietly,
// with the copilot still answering.
func TestPlatformWideRowsStayReadableAndCannotBeForged(t *testing.T) {
	pool := openGuardedPool(t)
	first, _ := seedTwoTenants(t, pool)
	key := "guardtest-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]

	if _, err := pool.Exec(context.Background(),
		`INSERT INTO workspace.ai_prompts (tenant_id, prompt_key, content, active) VALUES (NULL,$1,'shared',true)`,
		key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM workspace.ai_prompts WHERE prompt_key=$1`, key)
	})

	ctx := nexus.WithTenantID(context.Background(), first)
	var content string
	if err := pool.QueryRow(ctx,
		`SELECT content FROM workspace.ai_prompts WHERE prompt_key=$1 AND tenant_id IS NULL`, key).Scan(&content); err != nil {
		t.Fatalf("a tenant could not read the shared prompt: %v", err)
	}

	// Readable, but not writable: a tenant that could create a NULL-tenant row
	// would be publishing a system prompt to every other tenant.
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace.ai_prompts (tenant_id, prompt_key, content, active) VALUES (NULL,$1,'planted',true)`,
		key+"-forged"); err == nil {
		t.Error("a tenant created a platform-wide prompt")
	}
}

// Connections are shared, so the binding has to be re-established on every
// acquisition rather than once per connection. Interleaving two tenants over a
// pool small enough to force reuse is what catches a binding that leaks.
func TestConnectionReuseDoesNotLeakTheBinding(t *testing.T) {
	pool := openGuardedPool(t)
	first, second := seedTwoTenants(t, pool)

	for round := range 12 {
		want := first
		if round%2 == 1 {
			want = second
		}
		ctx := nexus.WithTenantID(context.Background(), want)
		var seen string
		if err := pool.QueryRow(ctx,
			`SELECT tenant_id::text FROM dbguard_probe WHERE name LIKE 'contact-guardtest-%'`).Scan(&seen); err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		if seen != want {
			t.Fatalf("round %d: bound to %s but read a row of %s", round, want, seen)
		}
	}
}

// Reading across organisations is the point of the active set; writing across
// them is the thing that must stay impossible.
//
// The policies are what draw that line — USING spans the set, WITH CHECK does
// not — so this is the test that would fail if either half were widened by
// accident.
func TestASessionReadsAcrossItsOrganisationsButWritesIntoOne(t *testing.T) {
	pool := openGuardedPool(t)
	ctx := context.Background()

	here, there := seedTwoTenants(t, pool)
	// The probe table, like every other test in this file. This one used to
	// borrow `departments`, which was the organisation app's and left with it
	// on 2026-08-23 — the same mistake `contacts` taught, made again against a
	// different app's table. See probeTable.
	seed := func(tenantID, code string) {
		t.Helper()
		if _, err := pool.Exec(nexus.WithoutTenant(ctx),
			`INSERT INTO dbguard_probe (tenant_id, name) VALUES ($1, $2)`, tenantID, code); err != nil {
			t.Fatal(err)
		}
	}
	seed(here, "here-unit")
	seed(there, "there-unit")

	// Acting in one organisation and reading across both.
	both := nexus.WithAllowedTenants(nexus.WithTenantID(ctx, here), []string{here, there})

	var seen int
	if err := pool.QueryRow(both,
		`SELECT count(*) FROM dbguard_probe WHERE name IN ('here-unit', 'there-unit')`).Scan(&seen); err != nil {
		t.Fatal(err)
	}
	if seen != 2 {
		t.Fatalf("expected both organisations' units to be readable, saw %d", seen)
	}

	// The same connection, the same set: a row may still only be created in the
	// organisation being acted in.
	_, err := pool.Exec(both,
		`INSERT INTO dbguard_probe (tenant_id, name) VALUES ($1, 'sneaked')`, there)
	if err == nil {
		t.Fatal("a row was written into an organisation the session is not acting in")
	}
	if !strings.Contains(err.Error(), "row-level security") {
		t.Fatalf("expected the policy to refuse the write, got %v", err)
	}

	// And into the acting one it works, which is what makes the refusal above
	// about the organisation rather than about writing at all.
	if _, err := pool.Exec(both,
		`INSERT INTO dbguard_probe (tenant_id, name) VALUES ($1, 'written')`, here); err != nil {
		t.Fatalf("writing into the acting organisation was refused: %v", err)
	}

	// A session that asked for nothing is unchanged: one organisation, exactly
	// as before this existed.
	var alone int
	if err := pool.QueryRow(nexus.WithTenantID(ctx, here),
		`SELECT count(*) FROM dbguard_probe WHERE name IN ('here-unit', 'there-unit')`).Scan(&alone); err != nil {
		t.Fatal(err)
	}
	if alone != 1 {
		t.Fatalf("a session with no active set saw %d organisations' rows", alone)
	}
}

// Every connection this pool hands out reads a calendar on the platform's
// clock.
//
// This is what makes `created_at::date`, `CURRENT_DATE` and `date_trunc` agree
// with the Go side across the whole codebase, and it is asserted here because
// Install is the one place every pool in this repository is configured — the
// production one and every database-backed test's. A pool built without it
// would answer with the server's default, which is UTC, and every daily figure
// would belong to a day that ends at eight in the morning in Ulaanbaatar.
func TestConnectionsReadTheCalendarOnThePlatformsClock(t *testing.T) {
	pool := openGuardedPool(t)

	var zone string
	if err := pool.QueryRow(context.Background(), `SELECT current_setting('timezone')`).Scan(&zone); err != nil {
		t.Fatalf("read the session timezone: %v", err)
	}
	if zone != nexus.TimezoneName() {
		t.Errorf("connections are in %q, want the platform's %q", zone, nexus.TimezoneName())
	}

	// And the reduction that matters actually follows it: the same instant, in
	// two zones twenty-six hours apart, is one date to the database.
	instant := time.Date(2026, 8, 16, 3, 0, 0, 0, time.UTC)
	var east, west string
	if err := pool.QueryRow(context.Background(),
		`SELECT ($1::timestamptz)::date::text, ($2::timestamptz)::date::text`,
		instant.In(time.FixedZone("east", 14*3600)),
		instant.In(time.FixedZone("west", -12*3600))).Scan(&east, &west); err != nil {
		t.Fatalf("reduce an instant to a date: %v", err)
	}
	if east != west {
		t.Errorf("one instant became two dates (%s and %s); the reduction is following the caller's zone", east, west)
	}
}
