/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package tenants

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The first administrator is chosen from the people this platform has watched
// prove who they are, so the list is exactly those people: an account with no
// eID identity is not on it, however ordinary it looks.
func TestOnlyVerifiedPeopleCanBeChosen(t *testing.T) {
	pool := optest.Pool(t)
	service := New(operator.New(pool), Deps{DB: pool})
	tenantID, _ := optest.Tenant(t, pool)
	verified := verifiedUser(t, pool, tenantID)
	plain, _ := optest.Person(t, pool, tenantID)
	ctx := context.Background()

	people, err := service.VerifiedPeople(ctx, "")
	if err != nil {
		t.Fatalf("list the verified people: %v", err)
	}
	listed := map[string]VerifiedPerson{}
	for _, person := range people {
		listed[person.UserID] = person
	}
	if _, ok := listed[verified]; !ok {
		t.Fatal("somebody who signed in with eID is not on the list")
	}
	if _, ok := listed[plain]; ok {
		t.Fatal("somebody who has never used eID is on the list")
	}
	if listed[verified].Organisations < 1 {
		t.Error("the list does not say how many organisations the person is already in")
	}
}

// Creating an organisation with a chosen administrator makes them its
// administrator without inviting them: they already have an account and a way
// back into it.
func TestAChosenAdministratorIsNotInvited(t *testing.T) {
	pool := optest.Pool(t)
	op := operator.New(pool)
	service := New(op, Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	home, _ := optest.Tenant(t, pool)
	chosen := verifiedUser(t, pool, home)
	ctx := context.Background()

	slug := fmt.Sprintf("chosen-%d", time.Now().UnixNano())
	created, err := service.CreateTenant(ctx, optest.Session(account), NewTenant{
		Name: "Chosen Admin Test", Slug: slug, AdminUserID: chosen,
		Reason: "prove the chosen administrator",
	})
	if err != nil {
		t.Fatalf("create the organisation: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1::uuid`, created.ID)
	})

	if !created.AdminExisted || created.Invited {
		t.Fatalf("the answer reads %+v; a chosen administrator is not invited", created)
	}

	var isAdmin bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM workspace.memberships m
		    JOIN workspace.membership_roles mr ON mr.membership_id = m.id
		    JOIN workspace.roles r ON r.id = mr.role_id
		   WHERE m.tenant_id = $1::uuid AND m.user_id = $2::uuid AND r.code = 'admin')`,
		created.ID, chosen).Scan(&isAdmin); err != nil {
		t.Fatalf("read the membership: %v", err)
	}
	if !isAdmin {
		t.Error("the chosen person did not become the organisation's administrator")
	}
}

// An id for somebody who never signed in with eID is refused rather than
// quietly treated as an ordinary account: choosing from the verified list is
// the whole point of choosing.
func TestAnUnverifiedAdministratorIsRefused(t *testing.T) {
	pool := optest.Pool(t)
	op := operator.New(pool)
	service := New(op, Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	home, _ := optest.Tenant(t, pool)
	plain, _ := optest.Person(t, pool, home)

	_, err := service.CreateTenant(context.Background(), optest.Session(account), NewTenant{
		Name: "Unverified", Slug: fmt.Sprintf("unverified-%d", time.Now().UnixNano()),
		AdminUserID: plain, Reason: "prove the guard",
	})
	if err == nil {
		t.Fatal("an account with no eID identity was accepted as the first administrator")
	}
}

// verifiedUser makes a person who has signed in with eID.
func verifiedUser(t *testing.T, pool *pgxpool.Pool, tenantID string) string {
	t.Helper()
	userID, _ := optest.Person(t, pool, tenantID)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO registry.user_eid_identities (user_id, civil_id, reg_number, person_etsi, given_name, surname)
		VALUES ($1::uuid, $2, $2, $2, 'Test', 'Person')`,
		userID, fmt.Sprintf("EID%d", time.Now().UnixNano())); err != nil {
		t.Fatalf("link an eID identity: %v", err)
	}
	return userID
}

// Somebody added to an organisation gets the least the platform offers, and
// gets it from the schema rather than from this screen: 00008's trigger grants
// `user` to every new membership.
func TestAnAddedPersonGetsTheSmallestRole(t *testing.T) {
	pool := optest.Pool(t)
	op := operator.New(pool)
	service := New(op, Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	sess := optest.Session(account)
	home, _ := optest.Tenant(t, pool)
	target, _ := optest.Tenant(t, pool)
	person := verifiedUser(t, pool, home)
	ctx := context.Background()

	if err := service.AddMember(ctx, sess, target, person, "prove the smallest role"); err != nil {
		t.Fatalf("add the person: %v", err)
	}

	var roles []string
	rows, err := pool.Query(ctx, `
		SELECT r.code FROM workspace.memberships m
		  JOIN workspace.membership_roles mr ON mr.membership_id = m.id
		  JOIN workspace.roles r ON r.id = mr.role_id
		 WHERE m.tenant_id = $1::uuid AND m.user_id = $2::uuid`, target, person)
	if err != nil {
		t.Fatalf("read the roles: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			t.Fatalf("read a role: %v", err)
		}
		roles = append(roles, code)
	}
	if len(roles) != 1 || roles[0] != "user" {
		t.Fatalf("the person was given %v, want exactly the user role", roles)
	}

	// Twice is refused rather than silently doing nothing: an operator who
	// clicks again is asking a question, and "already there" is the answer.
	if err := service.AddMember(ctx, sess, target, person, "again"); err == nil {
		t.Error("the same person was added twice")
	}
}

// The console shows a limit on one screen and must not be the way past it on
// another — when the limit is one the platform actually enforces.
func TestAHardLimitRefusesAnotherPerson(t *testing.T) {
	pool := optest.Pool(t)
	op := operator.New(pool)
	service := New(op, Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	sess := optest.Session(account)
	home, _ := optest.Tenant(t, pool)
	target, _ := optest.Tenant(t, pool)
	ctx := context.Background()

	none := 0
	if err := service.SetQuota(ctx, sess, target, Quota{MaxUsers: &none, Enforcement: EnforcementHard},
		"prove the limit"); err != nil {
		t.Fatalf("set the limit: %v", err)
	}
	if err := service.AddMember(ctx, sess, target, verifiedUser(t, pool, home), "past the limit"); err == nil {
		t.Fatal("a hard limit of zero let somebody in")
	}

	// Soft enforcement warns; it does not refuse.
	if err := service.SetQuota(ctx, sess, target, Quota{MaxUsers: &none, Enforcement: EnforcementSoft},
		"soften it"); err != nil {
		t.Fatalf("soften the limit: %v", err)
	}
	if err := service.AddMember(ctx, sess, target, verifiedUser(t, pool, home), "soft limit"); err != nil {
		t.Fatalf("a soft limit refused: %v", err)
	}
}
