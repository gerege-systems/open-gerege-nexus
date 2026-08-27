/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package migrations_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// What a brand-new organisation's manager and user can do on the day it is made.
//
// seed_tenant_access_roles() is a trigger, so this is not a migration's history
// — it runs every time a tenant is created, and until 00074 it handed out four
// gov.* permissions to managers and one to users by name. Those codes left with
// gerege-gov and matched nothing, which is the only reason the list was
// harmless.
//
// This is the compatibility assertion for removing them: the set a fresh tenant
// gets must be exactly what it was, minus five codes that were never there.
func TestANewTenantsDefaultGrantsAreUnchanged(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a migrated test database to run the seeded-role tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	// Three probe permissions: one of each shape the grammar rule covers, and
	// one of the departed codes. Named so they cannot collide with a real one.
	probes := map[string]string{
		"zz_probe.read":   "read",
		"zz_probe.manage": "manage",
		"gov.process":     "departed",
	}
	for code := range probes {
		if _, err := pool.Exec(ctx,
			`INSERT INTO registry.permissions (id, code, name, description) VALUES ($1,$2,$3,$4)
			 ON CONFLICT (code) DO NOTHING`, uuid.New().String(), code, code, code); err != nil {
			t.Fatalf("insert the probe permission %s: %v", code, err)
		}
	}
	t.Cleanup(func() {
		for code := range probes {
			_, _ = pool.Exec(ctx, `DELETE FROM registry.permissions WHERE code = $1`, code)
		}
	})

	tenantID := uuid.New().String()
	if _, err := pool.Exec(ctx,
		`INSERT INTO registry.tenants (id, slug, name) VALUES ($1, $2, $2)`,
		tenantID, "zz-probe-"+tenantID[:8]); err != nil {
		t.Fatalf("create the tenant: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM registry.tenants WHERE id = $1`, tenantID) })

	granted := func(role string) map[string]bool {
		rows, err := pool.Query(ctx, `
			SELECT p.code FROM tenant.role_permissions rp
			  JOIN tenant.roles r ON r.id = rp.role_id
			  JOIN registry.permissions p ON p.id = rp.permission_id
			 WHERE r.tenant_id = $1 AND r.code = $2 AND p.code = ANY($3)`,
			tenantID, role, []string{"zz_probe.read", "zz_probe.manage", "gov.process"})
		if err != nil {
			t.Fatalf("read the %s grants: %v", role, err)
		}
		defer rows.Close()
		out := map[string]bool{}
		for rows.Next() {
			var code string
			if err := rows.Scan(&code); err != nil {
				t.Fatal(err)
			}
			out[code] = true
		}
		return out
	}

	manager := granted("manager")
	if !manager["zz_probe.read"] || !manager["zz_probe.manage"] {
		t.Errorf("a new tenant's manager did not get the read and manage permissions: %v", manager)
	}
	user := granted("user")
	if !user["zz_probe.read"] {
		t.Errorf("a new tenant's user did not get the read permission: %v", user)
	}
	if user["zz_probe.manage"] {
		t.Errorf("a new tenant's user was given a manage permission: %v", user)
	}

	for _, role := range []string{"manager", "user"} {
		if granted(role)[gov] {
			t.Errorf("a new tenant's %s was granted %s, which belongs to an app in another repository", role, gov)
		}
	}
}

const gov = "gov.process"
