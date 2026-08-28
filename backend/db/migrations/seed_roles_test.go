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
			SELECT p.code FROM workspace.role_permissions rp
			  JOIN workspace.roles r ON r.id = rp.role_id
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

// A home is seeded with one role, and it is the one its owner actually gets.
//
// seed_tenant_access_roles() gives an organisation three roles so an
// administrator can hand different levels to different staff. A home has no
// staff: nobody but the owner will ever be a member, so two of the three are
// rows nobody is ever assigned. On a deployment with a million homes those two
// were most of the database — role_permissions reached 17 million rows and 2 GB
// of a 3.9 GB total, for workspaces with one member who owns them.
//
// Which one survives is not a preference. assign_default_membership_role()
// looks for the code 'user' and nothing else, so seeding 'admin' instead would
// leave the owner with no permissions at all in their own home. The two
// triggers are joined by that one string, and this test is what holds the joint
// together: it asserts the home owner's permissions are exactly what an
// organisation's ordinary member gets, which is what they were before.
func TestAHomeIsSeededWithTheRoleItsOwnerReceives(t *testing.T) {
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

	suffix := uuid.NewString()[:12]
	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name) VALUES ($1,'x',$2) RETURNING id::text`,
		"seedhome-"+suffix+"@example.mn", "Иргэн "+suffix).Scan(&userID); err != nil {
		t.Fatalf("make a citizen: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id = $1::uuid`, userID)
	})

	var homeID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.tenants (slug, name, kind, owner_user_id)
		 VALUES ($1,$2,'personal',$3::uuid) RETURNING id::text`,
		"seedhome-"+suffix, "Иргэн "+suffix, userID).Scan(&homeID); err != nil {
		t.Fatalf("make a home: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1::uuid`, homeID)
	})

	// An organisation alongside it, so "the home got one role" cannot pass
	// because the trigger stopped seeding anything at all.
	var orgID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.tenants (slug, name) VALUES ($1,$2) RETURNING id::text`,
		"seedorg-"+suffix, "Байгууллага "+suffix).Scan(&orgID); err != nil {
		t.Fatalf("make an organisation: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1::uuid`, orgID)
	})

	codes := func(tenantID string) []string {
		rows, err := pool.Query(ctx,
			`SELECT code FROM workspace.roles WHERE tenant_id = $1::uuid ORDER BY code`, tenantID)
		if err != nil {
			t.Fatalf("read the roles: %v", err)
		}
		defer rows.Close()
		var found []string
		for rows.Next() {
			var code string
			if err := rows.Scan(&code); err != nil {
				t.Fatalf("scan a role: %v", err)
			}
			found = append(found, code)
		}
		return found
	}

	if got := codes(homeID); len(got) != 1 || got[0] != "user" {
		t.Errorf("a home was seeded with %v, want exactly [user]", got)
	}
	if got := codes(orgID); len(got) != 3 {
		t.Errorf("an organisation was seeded with %v, want three roles", got)
	}

	// The joint between the two triggers: the owner's membership has to find
	// the role that was seeded, or they hold nothing in their own home.
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1::uuid,$2::uuid)`,
		homeID, userID); err != nil {
		t.Fatalf("make the owner a member: %v", err)
	}

	granted := func(tenantID, memberID string) int {
		var count int
		if err := pool.QueryRow(ctx,
			`SELECT count(*)
			   FROM workspace.memberships m
			   JOIN workspace.membership_roles mr ON mr.membership_id = m.id
			   JOIN workspace.role_permissions rp ON rp.role_id = mr.role_id
			  WHERE m.tenant_id = $1::uuid AND m.user_id = $2::uuid`,
			tenantID, memberID).Scan(&count); err != nil {
			t.Fatalf("count the permissions: %v", err)
		}
		return count
	}

	// The same person added to an ordinary organisation gets the same default
	// role, so the two counts are the compatibility assertion: a home owner is
	// no worse off than an employee on their first day.
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1::uuid,$2::uuid)`,
		orgID, userID); err != nil {
		t.Fatalf("add the person to the organisation: %v", err)
	}

	atHome, atWork := granted(homeID, userID), granted(orgID, userID)
	if atHome == 0 {
		t.Error("the home's owner holds no permissions at all in their own home")
	}
	if atHome != atWork {
		t.Errorf("the home owner holds %d permission(s), an ordinary member holds %d — "+
			"the two triggers no longer agree on a role code", atHome, atWork)
	}
}
