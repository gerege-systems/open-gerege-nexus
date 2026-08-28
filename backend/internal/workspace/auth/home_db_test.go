/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Nobody is turned away at the door.
 *
 * Before migration 00085 a person who belonged to no organisation could not
 * sign in at all: FirstTenantFor found no membership and returned
 * ErrNoOrganisation, and the session was never made. The schema had said
 * otherwise for months — registry.users is global and ge_id is unique across
 * the deployment — so this was the one place where the code refused what the
 * data model allowed.
 *
 * 00085 answered it by making every such person a workspace of their own, and
 * that is the answer this platform keeps. It was taken away for a day on the
 * strength of a measurement — a million people is 3.9 GB of rows in the
 * customer table and its children — and put back because a cost is not a
 * verdict: every platform of this shape gives each person a space, so that
 * "personal" and "paid team" differ by a plan rather than by a migration.
 *
 * What the detour left behind is in the tests below: 00092 stopped seeding an
 * organisation's three roles into a space with one member, and 00093 keyed a
 * person's own rows on the person rather than on the space, so they follow
 * their owner into a company and out again.
 */

package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/memo"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedPerson is an account and nothing else: no organisation, no membership.
func seedPerson(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	var userID string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO registry.users (email, password_hash, name) VALUES ($1,'x',$2) RETURNING id::text`,
		"hometest-"+suffix+"@example.mn", "Иргэн "+suffix).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id=$1`, userID)
	})
	return userID
}

func handlersFor(pool *pgxpool.Pool) *auth.Handlers {
	return auth.New(auth.Deps{DB: pool})
}

// The whole point: a person in no organisation still gets a workspace.
func TestSomebodyInNoOrganisationSignsIntoTheirOwn(t *testing.T) {
	pool := openPool(t)
	h := handlersFor(pool)
	ctx := context.Background()
	userID := seedPerson(t, pool)

	opened, err := h.FirstTenantFor(ctx, userID)
	if err != nil {
		t.Fatalf("a person in no organisation was refused a workspace to sign in to: %v", err)
	}
	if opened == "" {
		t.Fatal("FirstTenantFor returned no workspace and no error")
	}

	var kind, owner string
	if err := pool.QueryRow(ctx,
		`SELECT kind, COALESCE(owner_user_id::text,'') FROM registry.tenants WHERE id = $1::uuid`,
		opened).Scan(&kind, &owner); err != nil {
		t.Fatalf("the sign-in opened a workspace that is not there: %v", err)
	}
	if kind != "personal" || owner != userID {
		t.Errorf("the sign-in opened a %q workspace owned by %q", kind, owner)
	}
}

// Signing in twice is one workspace, not two.
//
// The check that matters is the second call taking the first call's row rather
// than making its own: the partial unique index is what enforces it, and a test
// that only called once would pass with no index at all.
func TestASecondSignInFindsTheSameWorkspace(t *testing.T) {
	pool := openPool(t)
	h := handlersFor(pool)
	ctx := context.Background()
	userID := seedPerson(t, pool)

	first, err := h.FirstTenantFor(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.FirstTenantFor(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("two sign-ins opened two workspaces: %s and %s", first, second)
	}

	var workspaces, memberships int
	if err := pool.QueryRow(ctx,
		`SELECT (SELECT count(*) FROM registry.tenants WHERE owner_user_id = $1::uuid),
		        (SELECT count(*) FROM workspace.memberships WHERE user_id = $1::uuid)`,
		userID).Scan(&workspaces, &memberships); err != nil {
		t.Fatal(err)
	}
	if workspaces != 1 || memberships != 1 {
		t.Errorf("signing in twice left %d workspace(s) and %d membership(s); want one of each",
			workspaces, memberships)
	}
}

// One role, not three.
//
// What 00092 left behind. An organisation gets admin, manager and user so an
// administrator can hand different levels to different staff; a space with one
// member who owns it has nobody to hand anything to. The one that survives is
// the one assign_default_membership_role() looks for, so the owner is no worse
// off than an employee on their first day.
func TestAPersonalWorkspaceIsSeededWithOneRole(t *testing.T) {
	pool := openPool(t)
	h := handlersFor(pool)
	ctx := context.Background()
	userID := seedPerson(t, pool)

	opened, err := h.FirstTenantFor(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}

	var roles, granted int
	if err := pool.QueryRow(ctx,
		`SELECT (SELECT count(*) FROM workspace.roles WHERE tenant_id = $1::uuid),
		        (SELECT count(*) FROM workspace.memberships m
		           JOIN workspace.membership_roles mr ON mr.membership_id = m.id
		           JOIN workspace.role_permissions rp ON rp.role_id = mr.role_id
		          WHERE m.tenant_id = $1::uuid AND m.user_id = $2::uuid)`,
		opened, userID).Scan(&roles, &granted); err != nil {
		t.Fatal(err)
	}
	if roles != 1 {
		t.Errorf("a personal workspace was seeded with %d roles, want one", roles)
	}
	if granted == 0 {
		t.Error("the owner holds no permissions in their own workspace")
	}
}

// Somebody who works somewhere opens there, and still has their own space.
//
// This has been four things and the history is in FirstTenantFor. Briefly: the
// organisation, then the personal space always, then the organisation with
// nowhere as the fallback, and now the organisation with a space of their own
// as the fallback — which is where every comparable platform ends up.
func TestAMemberOfAnOrganisationOpensInIt(t *testing.T) {
	pool := openPool(t)
	h := handlersFor(pool)
	ctx := context.Background()
	userID := seedPerson(t, pool)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	var orgID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.tenants (slug, name) VALUES ($1,$1) RETURNING id::text`,
		"work-"+suffix).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id=$1`, orgID) })
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1::uuid,$2::uuid)`,
		orgID, userID); err != nil {
		t.Fatal(err)
	}

	opened, err := h.FirstTenantFor(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if opened != orgID {
		t.Errorf("an employee opened in %q, want their organisation %q", opened, orgID)
	}
}

// An organisation may not claim an owner, and a personal space may not go
// without one.
//
// The constraint is what makes the idea coherent: kind and owner_user_id agree
// or the row means nothing. A row with kind='organisation' and an owner would
// be filtered off the console's list by nothing and found by a person's lookup
// by nothing — visible in neither place, the worst of the two ways to be wrong.
func TestKindAndOwnerMustAgree(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	userID := seedPerson(t, pool)
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]

	_, err := pool.Exec(ctx,
		`INSERT INTO registry.tenants (slug, name, kind) VALUES ($1,$1,'personal')`, "orphan-"+suffix)
	if err == nil {
		t.Error("a personal workspace was made with no owner")
	} else if !strings.Contains(err.Error(), "tenants_home_has_an_owner") {
		t.Errorf("refused for an unexpected reason: %v", err)
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO registry.tenants (slug, name, owner_user_id) VALUES ($1,$1,$2::uuid)`,
		"owned-org-"+suffix, userID)
	if err == nil {
		t.Error("an organisation was given an owner")
	} else if !strings.Contains(err.Error(), "tenants_home_has_an_owner") {
		t.Errorf("refused for an unexpected reason: %v", err)
	}
}

// Signing in with a password, belonging to no organisation.
//
// The regression this exists for: HandleLogin looked the account up with an
// inner join onto memberships, so somebody with none matched no row and was
// told "invalid email or password". It was found on production, by signing in
// as a citizen for the first time.
//
// Asserted through the handler rather than through FirstTenantFor, because
// FirstTenantFor was right the whole time. What was wrong was that nothing
// called it here — which is exactly the shape of bug a test one layer down
// cannot see.
func TestSomebodyInNoOrganisationCanSignInWithAPassword(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	email := "passwordonly-" + suffix + "@example.mn"

	hash, err := auth.HashPassword("Password123!")
	if err != nil {
		t.Fatal(err)
	}
	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name) VALUES ($1,$2,$3) RETURNING id::text`,
		email, hash, "Иргэн "+suffix).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id=$1`, userID) })

	body := `{"email":"` + email + `","password":"Password123!"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	request.Header.Set("Origin", "https://nexus.invalid")
	recorder := httptest.NewRecorder()
	auth.New(auth.Deps{
		DB:        pool,
		Sessions:  auth.NewSessionStore(pool, time.Hour),
		Suspended: memo.New[bool](auth.SuspendedTTL),
	}).HandleLogin(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("a citizen with no organisation was refused a password sign-in: %d %s",
			recorder.Code, recorder.Body.String())
	}

	var answer struct {
		User struct {
			TenantID string `json:"tenant_id"`
			IsAdmin  bool   `json:"is_admin"`
		} `json:"user"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	var kind string
	if err := pool.QueryRow(ctx,
		`SELECT kind FROM registry.tenants WHERE id = $1::uuid`, answer.User.TenantID).Scan(&kind); err != nil {
		t.Fatalf("the sign-in opened a workspace that is not there: %v", err)
	}
	if kind != "personal" {
		t.Errorf("the sign-in opened a %q workspace, want the person's own", kind)
	}
	// Nobody administers a workspace with one person in it, and saying
	// otherwise would draw an administrator's rail in it.
	if answer.User.IsAdmin {
		t.Error("a citizen is an administrator of their own workspace")
	}
}
