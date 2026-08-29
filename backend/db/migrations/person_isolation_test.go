/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package migrations_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// A person is isolated by being that person.
//
// registry.users and the two identity tables carry no tenant_id, so 00029 never
// reached them: until 00100 the tenant role could read every account on the
// deployment, every e-mail address, and every eID or SSO link. The application
// filter was the only thing between a forgotten WHERE and that list.
//
// The rule is "me, or somebody I work with", and the second half is inherited
// rather than restated: the EXISTS reads workspace.memberships, which is itself
// under row-level security, so it can only see the memberships of the
// organisations the caller is acting in.

func personPool(t *testing.T) *pgxpool.Pool {
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

func TestOneOrganisationsPeopleAreInvisibleToAnother(t *testing.T) {
	pool := personPool(t)
	ctx := context.Background()

	// One transaction, rolled back: this runs against the database every other
	// package's tests share.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	stamp := time.Now().UnixNano()
	var tenantA, tenantB, personA, personB string
	for _, step := range []struct {
		sql  string
		args []any
		into *string
	}{
		{`INSERT INTO registry.tenants (slug, name) VALUES ($1, $1) RETURNING id::text`,
			[]any{fmt.Sprintf("person-a-%d", stamp)}, &tenantA},
		{`INSERT INTO registry.tenants (slug, name) VALUES ($1, $1) RETURNING id::text`,
			[]any{fmt.Sprintf("person-b-%d", stamp)}, &tenantB},
		{`INSERT INTO registry.users (email, password_hash, name) VALUES ($1, 'x', 'A') RETURNING id::text`,
			[]any{fmt.Sprintf("person-a-%d@isolation.test", stamp)}, &personA},
		{`INSERT INTO registry.users (email, password_hash, name) VALUES ($1, 'x', 'B') RETURNING id::text`,
			[]any{fmt.Sprintf("person-b-%d@isolation.test", stamp)}, &personB},
	} {
		if err := tx.QueryRow(ctx, step.sql, step.args...).Scan(step.into); err != nil {
			t.Fatalf("set up: %v", err)
		}
	}
	for _, member := range []struct{ tenant, person string }{{tenantA, personA}, {tenantB, personB}} {
		if _, err := tx.Exec(ctx,
			`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1::uuid, $2::uuid)`,
			member.tenant, member.person); err != nil {
			t.Fatalf("make a membership: %v", err)
		}
	}
	// A second person in A, so "somebody I work with" is a real answer rather
	// than a synonym for "me".
	var colleague string
	if err := tx.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name) VALUES ($1, 'x', 'Colleague') RETURNING id::text`,
		fmt.Sprintf("colleague-%d@isolation.test", stamp)).Scan(&colleague); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1::uuid, $2::uuid)`,
		tenantA, colleague); err != nil {
		t.Fatalf("make a membership: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO registry.user_sso_identities (user_id, issuer, subject, email)
		 VALUES ($1::uuid, 'https://issuer.test', $2, $3)`,
		personB, fmt.Sprintf("subject-%d", stamp), fmt.Sprintf("person-b-%d@isolation.test", stamp)); err != nil {
		t.Fatalf("link an identity: %v", err)
	}

	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_tenant`); err != nil {
		t.Fatalf("become the tenant role: %v", err)
	}
	bind := func(t *testing.T, tenant, person string) {
		t.Helper()
		if _, err := tx.Exec(ctx,
			`SELECT set_config('app.current_tenant', $1, true), set_config('app.allowed_tenants', $2, true),
			        set_config('app.current_user', $3, true)`,
			tenant, "{"+tenant+"}", person); err != nil {
			t.Fatalf("bind: %v", err)
		}
	}
	visible := func(t *testing.T, id string) bool {
		t.Helper()
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM registry.users WHERE id = $1::uuid`, id).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n == 1
	}

	bind(t, tenantA, personA)
	if !visible(t, personA) {
		t.Error("a person cannot see their own account")
	}
	if !visible(t, colleague) {
		t.Error("a person cannot see somebody they work with")
	}
	if visible(t, personB) {
		t.Error("a person can see an account in another organisation")
	}

	var identities int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM registry.user_sso_identities WHERE user_id = $1::uuid`, personB).Scan(&identities); err != nil {
		t.Fatalf("count identities: %v", err)
	}
	if identities != 0 {
		t.Error("another organisation's sign-in identity is readable")
	}

	// And the other way round, so the rule is the caller's own binding rather
	// than something about the rows.
	bind(t, tenantB, personB)
	if !visible(t, personB) {
		t.Error("a person cannot see their own account from their own organisation")
	}
	if visible(t, personA) || visible(t, colleague) {
		t.Error("the other organisation's people are readable")
	}

	// Writing across the boundary is refused by the same expression.
	if _, err := tx.Exec(ctx,
		`UPDATE registry.users SET name = 'renamed by a stranger' WHERE id = $1::uuid`, personA); err == nil {
		var renamed int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM registry.users WHERE name = 'renamed by a stranger'`).Scan(&renamed); err == nil && renamed > 0 {
			t.Error("a stranger renamed somebody in another organisation")
		}
	}
}

// A person standing in no organisation — dbguard's person path — sees
// themselves and nobody else. The binding is the strictest one the guard makes,
// and this is the table where "nobody else" has to include every account on the
// deployment.
func TestSomebodyInNoOrganisationSeesOnlyThemselves(t *testing.T) {
	pool := personPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	stamp := time.Now().UnixNano()
	var alone, other string
	if err := tx.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name) VALUES ($1, 'x', 'Alone') RETURNING id::text`,
		fmt.Sprintf("alone-%d@isolation.test", stamp)).Scan(&alone); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name) VALUES ($1, 'x', 'Other') RETURNING id::text`,
		fmt.Sprintf("other-%d@isolation.test", stamp)).Scan(&other); err != nil {
		t.Fatalf("set up: %v", err)
	}

	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_tenant`); err != nil {
		t.Fatalf("become the tenant role: %v", err)
	}
	// The person path: a user, no tenant, no allowed list.
	if _, err := tx.Exec(ctx,
		`SELECT set_config('app.current_tenant', '', true), set_config('app.allowed_tenants', '', true),
		        set_config('app.current_user', $1, true)`, alone); err != nil {
		t.Fatalf("bind: %v", err)
	}

	var mine, theirs int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM registry.users WHERE id = $1::uuid`, alone).Scan(&mine); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM registry.users WHERE id = $1::uuid`, other).Scan(&theirs); err != nil {
		t.Fatal(err)
	}
	if mine != 1 {
		t.Error("somebody in no organisation cannot see their own account")
	}
	if theirs != 0 {
		t.Error("somebody in no organisation can see another account")
	}
}

// The deployment's signing key is not the tenant role's to read or to rotate.
// Every id_token this platform issues is trusted because of that key.
func TestTheTenantRoleCannotReachTheSigningKey(t *testing.T) {
	pool := personPool(t)
	ctx := context.Background()

	for _, privilege := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
		var allowed bool
		if err := pool.QueryRow(ctx,
			`SELECT has_table_privilege('gerege_nexus_tenant', 'registry.oauth2_signing_keys', $1)`,
			privilege).Scan(&allowed); err != nil {
			t.Fatal(err)
		}
		if allowed {
			t.Errorf("the tenant role may %s the deployment's signing key", privilege)
		}
	}
}
