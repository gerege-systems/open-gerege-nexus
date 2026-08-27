/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package migrations_test

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	tenantRole    = "gerege_nexus_tenant"
	oldTenantRole = "gerege_nexus_app"
)

func schemaPool(t *testing.T) *pgxpool.Pool {
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

	var migrated bool
	if err := pool.QueryRow(context.Background(), `
		SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'workspace')
		   AND EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'registry')
		   AND EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'operator')`).Scan(&migrated); err != nil {
		t.Fatal(err)
	}
	if !migrated {
		t.Skip("the database is not migrated through 00084_workspace_schema")
	}
	return pool
}

// The database result is checked against the same ownership decision that
// generated the migration. Counting 32, 20 and 7 is useful in a review, but the
// names are the invariant: swapping one table each way would preserve every
// count and still put both in the wrong schema.
func TestEveryPlatformMigrationTableLandsOnItsDeclaredSchema(t *testing.T) {
	pool := schemaPool(t)
	rows, err := pool.Query(context.Background(), `
		SELECT schemaname, tablename
		  FROM pg_tables
		 WHERE schemaname IN ('workspace', 'registry', 'operator')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	found := make(map[string]string, len(platformTables))
	for rows.Next() {
		var schema, name string
		if err := rows.Scan(&schema, &name); err != nil {
			t.Fatal(err)
		}
		if _, ours := platformTables[name]; ours {
			found[name] = schema
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	counts := map[string]int{}
	for name := range platformTables {
		actual, exists := found[name]
		if !exists {
			t.Errorf("%s is in none of the workspace, registry and operator schemas", name)
			continue
		}
		counts[actual]++
		if want := schemaOf(name); actual != want {
			t.Errorf("%s is in %s, ownership_test.go declares %s", name, actual, want)
		}
	}
	// The counts are written down so that a table appearing in the wrong schema
	// cannot pass by moving another one, and so that the shape of the split
	// stays legible: twenty of the operator plane's tables are ones a tenant may
	// also read and seven are not. A number edited alongside a migration is a
	// number somebody looked at.
	if counts["workspace"] != 33 || counts["registry"] != 21 || counts["operator"] != 7 {
		t.Errorf("schema counts: workspace=%d registry=%d operator=%d; want 33, 21 and 7",
			counts["workspace"], counts["registry"], counts["operator"])
	}
}

// PostgreSQL stores policy roles by OID. Renaming the role should therefore
// change the displayed name without rebuilding forty policies; this test is
// the proof that lets the migration leave their USING/WITH CHECK expressions
// untouched.
func TestTenantRoleRenameReachedEveryIsolationPolicy(t *testing.T) {
	pool := schemaPool(t)
	var oldExists bool
	if err := pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, oldTenantRole).Scan(&oldExists); err != nil {
		t.Fatal(err)
	}
	if oldExists {
		t.Errorf("the old database role %s still exists", oldTenantRole)
	}

	rows, err := pool.Query(context.Background(), `
		SELECT schemaname, tablename, roles
		  FROM pg_policies
		 WHERE policyname = 'tenant_isolation'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	seen := 0
	for rows.Next() {
		var schema, table string
		var roles []string
		if err := rows.Scan(&schema, &table, &roles); err != nil {
			t.Fatal(err)
		}
		if _, ours := platformTables[table]; !ours {
			continue
		}
		seen++
		if !slices.Contains(roles, tenantRole) || slices.Contains(roles, oldTenantRole) {
			t.Errorf("%s.%s policy roles = %v, want only the renamed tenant role", schema, table, roles)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if seen == 0 {
		t.Fatal("no core-owned tenant_isolation policies were found")
	}
}

// The boundary is now two locks rather than one.
//
// Before migration 00083 it was one: the tenant role held USAGE on `platform`
// because five of the tables in it are the boundary the planes meet on, so
// operator_audit was shut by its own table grant and by nothing else. This
// test said so — "platform USAGE cannot be revoked from the tenant role".
//
// The split moved the seven tables a tenant may never reach into `operator`
// and left the boundary five in `registry`, so the sentence is no longer true
// and the schema itself is the outer lock. Both are checked: the table grant
// must still be absent, because a schema that is later granted by accident
// must not open anything on its own.
func TestTenantRoleReadsTheBoundaryButNotTheOperatorSchema(t *testing.T) {
	pool := schemaPool(t)
	ctx := context.Background()

	for _, table := range []string{
		"announcements", "feature_flag_overrides", "operator_impersonations",
		"tenant_quotas", "usage_events",
	} {
		var allowed bool
		if err := pool.QueryRow(ctx, `SELECT has_table_privilege($1, $2, 'SELECT')`,
			tenantRole, "registry."+table).Scan(&allowed); err != nil {
			t.Fatal(err)
		}
		if !allowed {
			t.Errorf("%s cannot SELECT registry.%s", tenantRole, table)
		}
	}

	var schemaUsage bool
	if err := pool.QueryRow(ctx, `SELECT has_schema_privilege($1, 'operator', 'USAGE')`,
		tenantRole).Scan(&schemaUsage); err != nil {
		t.Fatal(err)
	}
	if schemaUsage {
		t.Error("the tenant role has USAGE on the operator schema; the outer lock is open")
	}

	var auditAllowed bool
	if err := pool.QueryRow(ctx, `SELECT has_table_privilege($1, 'operator.operator_audit', 'SELECT')`,
		tenantRole).Scan(&auditAllowed); err != nil {
		t.Fatal(err)
	}
	if auditAllowed {
		t.Fatal("the tenant role has SELECT on operator.operator_audit")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_tenant`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM operator.operator_audit LIMIT 1`); err == nil {
		t.Fatal("a query running as the tenant role read operator.operator_audit")
	} else if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("operator_audit was refused for an unexpected reason: %v", err)
	}
}

// USAGE lets the tenant role resolve the twenty registry tables; it must not
// turn into an inheritance rule for the rest of the schema. In particular, a
// later migration that creates a registry table without thinking about this
// boundary must leave it closed by default — registry is the open side of the
// 00083 split, which is exactly why a new table landing there must still
// arrive shut.
func TestNewRegistryTableIsClosedToTenantRole(t *testing.T) {
	pool := schemaPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const table = "registry.tenant_role_default_privilege_probe"
	if _, err := tx.Exec(ctx, `CREATE TABLE `+table+` (id integer)`); err != nil {
		t.Fatal(err)
	}

	for _, privilege := range []string{
		"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER",
	} {
		var allowed bool
		if err := tx.QueryRow(ctx, `SELECT has_table_privilege($1, $2, $3)`,
			tenantRole, table, privilege).Scan(&allowed); err != nil {
			t.Fatal(err)
		}
		if allowed {
			t.Errorf("a newly created registry table grants %s to %s", privilege, tenantRole)
		}
	}

	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_tenant`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM `+table); err == nil {
		t.Fatal("the tenant role read a newly created registry table")
	} else if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("the new registry table was refused for an unexpected reason: %v", err)
	}
}

func TestLoginPathSearchesBothPlanes(t *testing.T) {
	pool := schemaPool(t)
	var path string
	if err := pool.QueryRow(context.Background(), `SHOW search_path`).Scan(&path); err != nil {
		t.Fatal(err)
	}
	if path != "workspace, registry, operator" {
		t.Errorf("login role search_path = %q, want workspace, registry, operator", path)
	}
}
