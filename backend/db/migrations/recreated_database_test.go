/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package migrations_test

import (
	"context"
	"testing"
)

// The privileges have to be on the role the platform actually connects as.
//
// This is one assertion about the end state and it stands for a whole class of
// failure: a database migrated in a cluster that had already run these
// migrations used to get every pre-00079 grant on gerege_nexus_app while the
// platform SET ROLEs to gerege_nexus_tenant. Signing in worked and every screen
// after it answered 500 with `permission denied for table users` — the exact
// query below.
func TestTheTenantRoleCanReadWhatTheShellAsksFor(t *testing.T) {
	pool := schemaPool(t)
	ctx := context.Background()

	// A transaction, so the role change cannot leak into another test through
	// a pooled connection.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_tenant`); err != nil {
		t.Fatalf("become the tenant role: %v", err)
	}
	for _, table := range []string{"registry.users", "registry.tenants", "workspace.roles"} {
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&count); err != nil {
			t.Errorf("the tenant role cannot read %s: %v", table, err)
		}
	}
}

// The database's own search_path is what makes an unqualified name resolve.
//
// A restore from `pg_dump` of a single database carries every table and none of
// this, and the platform's own "give this person a home" path names no schema:
// without it that path fails with `relation "roles" does not exist` at the
// moment somebody first signs in with eID.
func TestTheDatabaseCarriesItsSearchPath(t *testing.T) {
	pool := schemaPool(t)
	var configured []string
	if err := pool.QueryRow(context.Background(), `
		SELECT s.setconfig FROM pg_db_role_setting s
		  JOIN pg_database d ON d.oid = s.setdatabase
		 WHERE d.datname = current_database() AND s.setrole = 0`).Scan(&configured); err != nil {
		t.Fatalf("read the database settings: %v", err)
	}
	var found bool
	for _, setting := range configured {
		if setting == "search_path=workspace, registry, operator" {
			found = true
		}
	}
	if !found {
		t.Errorf("the database's search_path is %v, not the one the platform's unqualified queries need", configured)
	}
}

// The console's front page shows which migration this database has seen.
func TestTheConsoleCanReadTheMigrationVersion(t *testing.T) {
	pool := schemaPool(t)
	var allowed bool
	if err := pool.QueryRow(context.Background(),
		`SELECT has_table_privilege('gerege_nexus_operator', 'public.goose_db_version', 'SELECT')`).
		Scan(&allowed); err != nil {
		t.Fatalf("read the privilege: %v", err)
	}
	if !allowed {
		t.Error("the console's role cannot read goose_db_version, so its version panel is blank")
	}
}
