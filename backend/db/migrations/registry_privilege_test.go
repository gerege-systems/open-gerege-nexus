/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package migrations_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// What 00106 did — `organisations_are_read_across`,
// `an_organisation_is_changed_by_its_own` and
// `the_console_sees_every_organisation` — asserted from the role the
// application actually uses.
//
// Every check below runs after SET LOCAL ROLE gerege_nexus_tenant, because a
// test that stays the login role proves nothing about a policy: the login role
// is outside them all, so a revoked grant and a granted one look identical from
// it.

const deniedCode = "42501"

func registryPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// mayNot reports whether the tenant role has had a privilege taken away.
//
// A REVOKE is asserted here rather than by running the statement, because a
// statement can be refused for a reason that is not the grant: an INSERT into
// registry.tenants fires seed_tenant_access_roles, whose own INSERT is
// rejected by the roles policy with the same SQLSTATE. That refusal proves
// nothing about this migration, and it looked exactly like proof.
func mayNot(t *testing.T, tx pgx.Tx, table, privilege string) bool {
	t.Helper()
	var held bool
	if err := tx.QueryRow(context.Background(),
		`SELECT has_table_privilege('gerege_nexus_tenant', $1, $2)`,
		"registry."+table, privilege).Scan(&held); err != nil {
		t.Fatalf("read the privilege: %v", err)
	}
	return !held
}

// denied runs one statement inside a savepoint and reports whether PostgreSQL
// refused it for want of a privilege. Used only where nothing else in the
// statement can raise the same code.
//
// The savepoint is not tidiness: a permission error aborts the transaction, and
// the checks after it would then fail for the wrong reason.
func denied(t *testing.T, tx pgx.Tx, sql string, args ...any) bool {
	t.Helper()
	ctx := context.Background()
	nested, err := tx.Begin(ctx)
	if err != nil {
		t.Fatalf("savepoint: %v", err)
	}
	defer func() { _ = nested.Rollback(ctx) }()

	_, err = nested.Exec(ctx, sql, args...)
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == deniedCode {
		return true
	}
	t.Fatalf("%s\nrefused for the wrong reason: %v", sql, err)
	return false
}

// twoOrganisations makes A and B inside a transaction the caller rolls back.
func twoOrganisations(t *testing.T, tx pgx.Tx) (string, string) {
	t.Helper()
	ctx := context.Background()
	stamp := time.Now().UnixNano()
	var a, b string
	for _, made := range []struct {
		slug string
		into *string
	}{
		{fmt.Sprintf("privilege-a-%d", stamp), &a},
		{fmt.Sprintf("privilege-b-%d", stamp), &b},
	} {
		if err := tx.QueryRow(ctx,
			`INSERT INTO registry.tenants (slug, name) VALUES ($1, $1) RETURNING id::text`,
			made.slug).Scan(made.into); err != nil {
			t.Fatalf("set up: %v", err)
		}
	}
	return a, b
}

func asTenant(t *testing.T, tx pgx.Tx, tenantID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_tenant`); err != nil {
		t.Fatalf("become the tenant role: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`SELECT set_config('app.current_tenant', $1, true), set_config('app.allowed_tenants', $2, true)`,
		tenantID, "{"+tenantID+"}"); err != nil {
		t.Fatalf("bind: %v", err)
	}
}

func TestAnOrganisationIsChangedOnlyByItsOwn(t *testing.T) {
	pool := registryPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	a, b := twoOrganisations(t, tx)
	asTenant(t, tx, a)

	// Its own name, which is what profile/organisation.go writes.
	tag, err := tx.Exec(ctx, `UPDATE registry.tenants SET name = 'renamed' WHERE id = $1::uuid`, a)
	if err != nil {
		t.Fatalf("an organisation could not rename itself: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("an organisation could not rename itself: %d rows", tag.RowsAffected())
	}

	// The neighbour's. The policy makes this a no-op rather than an error,
	// which is the shape every other tenant policy in this database has.
	tag, err = tx.Exec(ctx, `UPDATE registry.tenants SET name = 'seized' WHERE id = $1::uuid`, b)
	if err != nil {
		t.Fatalf("updating a neighbour: %v", err)
	}
	if tag.RowsAffected() != 0 {
		t.Errorf("one organisation renamed another: %d rows changed", tag.RowsAffected())
	}

	if !mayNot(t, tx, "tenants", "DELETE") {
		t.Error("the tenant role can delete an organisation; deletion belongs to the console and the sweep")
	}
	if !mayNot(t, tx, "tenants", "INSERT") {
		t.Error("the tenant role can open an organisation; that belongs to the console and the wizard")
	}
}

// The read that is open on purpose.
//
// Narrowing it would not fail anywhere: the person plane's provider directory,
// the workspace switcher, a report grant's two names and a child's parent name
// are all JOINs, so what they would return is an empty screen. 00106's comment
// says that in prose; this says it in a way that breaks.
func TestOneOrganisationStillReadsAnother(t *testing.T) {
	pool := registryPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	a, b := twoOrganisations(t, tx)
	asTenant(t, tx, a)

	var seen int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM registry.tenants WHERE id = $1::uuid`, b).Scan(&seen); err != nil {
		t.Fatalf("count: %v", err)
	}
	if seen != 1 {
		t.Error(`an organisation can no longer read another's row.

Four callers cross organisations on purpose — internal/person, the workspace
switcher, reporting grants, and a child reading its parent — and each one is a
JOIN, so this does not fail for them: it empties them. If the read was narrowed
deliberately, those four need their own answer first.`)
	}
}

func TestTheTenantRoleCannotWriteThePlatformsSettings(t *testing.T) {
	pool := registryPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	a, _ := twoOrganisations(t, tx)
	asTenant(t, tx, a)

	// Reading stays: the values are not secret, and the cache reads them on
	// the platform path anyway.
	var stored int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM registry.platform_settings`).Scan(&stored); err != nil {
		t.Fatalf("the tenant role can no longer read the settings: %v", err)
	}

	for _, privilege := range []string{"INSERT", "UPDATE", "DELETE"} {
		if !mayNot(t, tx, "platform_settings", privilege) {
			t.Errorf("the tenant role holds %s on the platform settings; only the console's transaction writes them",
				privilege)
		}
	}
}

// Invite tokens and parked identities are reached by people with no session.
// A bound connection has no business in either table, so the grant is gone
// rather than the rows being filtered.
func TestTheTenantRoleCannotReachTheSessionlessTables(t *testing.T) {
	pool := registryPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	a, _ := twoOrganisations(t, tx)
	asTenant(t, tx, a)

	for _, table := range []string{"credential_grants", "identity_binding_sessions"} {
		if !denied(t, tx, `SELECT count(*) FROM registry.`+table) {
			t.Errorf("the tenant role reads registry.%s; it is reached only without a session", table)
		}
		for _, privilege := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
			if !mayNot(t, tx, table, privilege) {
				t.Errorf("the tenant role holds %s on registry.%s", privilege, table)
			}
		}
	}
}

// The console keeps what 00049 gave it. Suspending, restoring, scheduling a
// deletion and the maintenance switch are all UPDATEs on this table, and RLS
// closes a table to a role with no policy however many grants it holds.
func TestTheConsoleStillActsOnEveryOrganisation(t *testing.T) {
	pool := registryPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	a, b := twoOrganisations(t, tx)
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_operator`); err != nil {
		t.Fatalf("become the operator role: %v", err)
	}
	for _, id := range []string{a, b} {
		tag, err := tx.Exec(ctx,
			`UPDATE registry.tenants SET suspended_at = NOW(), suspension_reason = 'test' WHERE id = $1::uuid`, id)
		if err != nil {
			t.Fatalf("the console could not suspend an organisation: %v", err)
		}
		if tag.RowsAffected() != 1 {
			t.Errorf("the console could not suspend an organisation: %d rows", tag.RowsAffected())
		}
	}
}
