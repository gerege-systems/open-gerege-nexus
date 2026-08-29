/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package migrations_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The five tables that hold one organisation's rows without carrying a
// tenant_id, and the parent each one's isolation comes from.
//
// ownership_test.go names six such tables; report_grants is the sixth and is
// not here because it carries two tenant columns of its own and has had a
// policy since 00071. These five had none at all until 00095.
var childTables = map[string]string{
	"esign_batch_items":    "esign_batches",
	"installation_events":  "app_installations",
	"membership_roles":     "memberships",
	"role_permissions":     "roles",
	"oauth2_access_tokens": "oauth2_clients",
}

func migrationsPool(t *testing.T) *pgxpool.Pool {
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

// A child table with no policy is a table the second layer does not cover.
func TestChildTablesInheritTheirParentsIsolation(t *testing.T) {
	pool := migrationsPool(t)
	for child, parent := range childTables {
		var enabled, forced bool
		var using string
		err := pool.QueryRow(context.Background(), `
			SELECT cls.relrowsecurity, cls.relforcerowsecurity,
			       COALESCE((SELECT qual FROM pg_policies p
			                  WHERE p.schemaname = 'workspace' AND p.tablename = $1
			                    AND p.policyname = 'parent_isolation'), '')
			  FROM pg_class cls
			  JOIN pg_namespace n ON n.oid = cls.relnamespace AND n.nspname = 'workspace'
			 WHERE cls.relname = $1`, child).Scan(&enabled, &forced, &using)
		if err != nil {
			t.Fatalf("%s: %v", child, err)
		}
		if !enabled || !forced {
			t.Errorf("%s: RLS enabled=%v forced=%v", child, enabled, forced)
		}
		// The policy must ask about the parent and nothing else. A policy that
		// reads app.current_tenant here would be the parent's rule copied, and
		// a copied rule is one that stops matching the day the original moves —
		// which is what 00037 left behind on a quarter of the tables.
		if !strings.Contains(using, parent) {
			t.Errorf("%s: parent_isolation does not read %s: %q", child, parent, using)
		}
		if strings.Contains(using, "app.current_tenant") {
			t.Errorf("%s: parent_isolation restates the tenant rule instead of inheriting it: %q", child, using)
		}
	}
}

// What the policies are for, asked of the database rather than of the shape of
// the SQL: one organisation must not read or write another's child rows.
//
// role_permissions carries the proof for all five. It is the one whose parent
// (roles) a test can create without an installed module, a signed document or
// an OAuth client, and every policy here is the same expression.
func TestASiblingCannotReachAnotherOrganisationsChildRows(t *testing.T) {
	pool := migrationsPool(t)
	ctx := context.Background()

	// One transaction, rolled back: this runs against the same database as
	// every other package's tests, and an organisation left behind would be
	// somebody else's failure tomorrow.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const (
		tenantA = "aaaaaaaa-0000-4000-8000-00000000000a"
		tenantB = "bbbbbbbb-0000-4000-8000-00000000000b"
		roleA   = "aaaaaaaa-0000-4000-8000-0000000000a1"
		roleB   = "bbbbbbbb-0000-4000-8000-0000000000b1"
	)
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO registry.tenants (id, slug, name) VALUES ($1::uuid, 'parent-isolation-a', 'A'), ($2::uuid, 'parent-isolation-b', 'B')`, []any{tenantA, tenantB}},
		{`INSERT INTO workspace.roles (id, tenant_id, code, name) VALUES ($1::uuid, $2::uuid, 'parent-isolation-a', 'A'), ($3::uuid, $4::uuid, 'parent-isolation-b', 'B')`, []any{roleA, tenantA, roleB, tenantB}},
		{`INSERT INTO workspace.role_permissions (role_id, permission_id) SELECT $1::uuid, id FROM registry.permissions LIMIT 1`, []any{roleA}},
		{`INSERT INTO workspace.role_permissions (role_id, permission_id) SELECT $1::uuid, id FROM registry.permissions LIMIT 1`, []any{roleB}},
	} {
		if _, err := tx.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("set up: %v", err)
		}
	}

	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_tenant`); err != nil {
		t.Fatalf("become the tenant role: %v", err)
	}

	count := func(t *testing.T, actingAs, roleID string) int {
		t.Helper()
		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true)`, actingAs); err != nil {
			t.Fatalf("bind the tenant: %v", err)
		}
		var n int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM workspace.role_permissions WHERE role_id = $1::uuid`, roleID).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}

	if n := count(t, tenantA, roleA); n != 1 {
		t.Errorf("A sees %d of its own rows, want 1", n)
	}
	if n := count(t, tenantA, roleB); n != 0 {
		t.Errorf("A sees %d of B's rows, want 0", n)
	}
	if n := count(t, tenantB, roleB); n != 1 {
		t.Errorf("B sees %d of its own rows, want 1", n)
	}
	if n := count(t, tenantB, roleA); n != 0 {
		t.Errorf("B sees %d of A's rows, want 0", n)
	}

	// Writing into a sibling's role is refused by the same policy, so a
	// handler that lost its WHERE clause cannot grant a permission inside
	// another organisation either.
	if _, err := tx.Exec(ctx,
		`INSERT INTO workspace.role_permissions (role_id, permission_id)
		 SELECT $1::uuid, id FROM registry.permissions OFFSET 1 LIMIT 1`, roleA); err == nil {
		t.Error("B wrote a row into A's role")
	}
}

// The audit trail is not something the role that writes it may rewrite.
func TestAuditEventsCannotBeRewritten(t *testing.T) {
	pool := migrationsPool(t)
	ctx := context.Background()

	var canUpdate, canDelete bool
	if err := pool.QueryRow(ctx, `
		SELECT has_table_privilege('gerege_nexus_tenant', 'workspace.audit_events', 'UPDATE'),
		       has_table_privilege('gerege_nexus_tenant', 'workspace.audit_events', 'DELETE')`).
		Scan(&canUpdate, &canDelete); err != nil {
		t.Fatal(err)
	}
	if canUpdate || canDelete {
		t.Errorf("the tenant role may still edit the audit trail: update=%v delete=%v", canUpdate, canDelete)
	}

	// The trigger is the half that also binds the owner, and it is why an
	// UPDATE is refused rather than merely unprivileged. DELETE is left alone
	// on purpose: audit_events cascades from tenants, and an organisation
	// whose grace period ends must be able to take its rows with it.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const tenant = "cccccccc-0000-4000-8000-00000000000c"
	if _, err := tx.Exec(ctx,
		`INSERT INTO registry.tenants (id, slug, name) VALUES ($1::uuid, 'append-only-check', 'C')`, tenant); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO workspace.audit_events (tenant_id, action, resource, details)
		 VALUES ($1::uuid, 'append-only.check', 'test', '{}'::jsonb)`, tenant); err != nil {
		t.Fatalf("write an audit row: %v", err)
	}
	// The refusal aborts the transaction, so it is taken inside a savepoint —
	// the cascade below has to run on a live one.
	attempt, err := tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = attempt.Exec(ctx,
		`UPDATE workspace.audit_events SET action = 'rewritten' WHERE tenant_id = $1::uuid`, tenant)
	if err == nil {
		t.Fatal("an audit row was rewritten")
	}
	if !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("the refusal does not say why: %v", err)
	}
	if err := attempt.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	// And the row still leaves with the organisation it belongs to: audit_events
	// cascades from tenants, so blocking DELETE outright would have made an
	// organisation whose grace period ended impossible to remove.
	if _, err := tx.Exec(ctx, `DELETE FROM registry.tenants WHERE id = $1::uuid`, tenant); err != nil {
		t.Fatalf("an organisation could not be deleted with its audit rows: %v", err)
	}
}
