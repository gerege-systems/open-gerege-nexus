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
 * The workaround was EID_JIT_TENANT_SLUG, which answered the question by making
 * the citizen a *member* of whichever organisation an environment variable
 * named: counted against its quota, listed in its directory, given its default
 * role. These tests are what say that path is gone and what replaced it.
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
		// The home goes with the person: owner_user_id is ON DELETE CASCADE, so
		// deleting the account is enough and a test that forgot the workspace
		// would still leave nothing behind.
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id=$1`, userID)
	})
	return userID
}

func handlersFor(pool *pgxpool.Pool) *auth.Handlers {
	return auth.New(auth.Deps{DB: pool})
}

// The whole point: a person in no organisation still gets a workspace.
func TestSomebodyInNoOrganisationSignsIntoTheirOwnHome(t *testing.T) {
	pool := openPool(t)
	h := handlersFor(pool)
	userID := seedPerson(t, pool)
	ctx := context.Background()

	tenantID, err := h.FirstTenantFor(ctx, userID)
	if err != nil {
		t.Fatalf("a person with no organisation could not sign in: %v", err)
	}
	if tenantID == "" {
		t.Fatal("FirstTenantFor returned no workspace and no error")
	}

	var kind, owner string
	if err := pool.QueryRow(ctx,
		`SELECT kind, owner_user_id::text FROM registry.tenants WHERE id = $1::uuid`,
		tenantID).Scan(&kind, &owner); err != nil {
		t.Fatal(err)
	}
	if kind != "personal" {
		t.Errorf("the workspace made for a person is %q, not personal", kind)
	}
	if owner != userID {
		t.Errorf("the home is owned by %s, not by %s", owner, userID)
	}

	// Owning it is not enough. The rest of the platform reads memberships, so a
	// home without one is a workspace whose owner every screen refuses.
	var member bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM workspace.memberships WHERE tenant_id=$1::uuid AND user_id=$2::uuid)`,
		tenantID, userID).Scan(&member); err != nil {
		t.Fatal(err)
	}
	if !member {
		t.Error("the person owns their home but is not a member of it")
	}
}

// Signing in twice is one home, not two.
//
// The check that matters is the second call taking the first call's row rather
// than making its own: the index is what enforces it, and a test that only
// called once would pass with no index at all.
func TestASecondSignInFindsTheSameHome(t *testing.T) {
	pool := openPool(t)
	h := handlersFor(pool)
	userID := seedPerson(t, pool)
	ctx := context.Background()

	first, err := h.FirstTenantFor(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.FirstTenantFor(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("two sign-ins opened two homes: %s and %s", first, second)
	}

	var homes int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM registry.tenants WHERE owner_user_id = $1::uuid`, userID).Scan(&homes); err != nil {
		t.Fatal(err)
	}
	if homes != 1 {
		t.Errorf("the person has %d homes", homes)
	}
}

// Somebody who works somewhere opens at work.
//
// A home is a place to stand for people who have none, not a lobby everybody
// walks through. Getting this backwards would move every existing user's
// landing screen on the day 00085 was deployed.
func TestAnOrganisationComesBeforeTheHome(t *testing.T) {
	pool := openPool(t)
	h := handlersFor(pool)
	ctx := context.Background()
	userID, orgID := seedMember(t, pool)

	// The home exists first, and is still not the answer.
	home, err := h.HomeFor(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := h.FirstTenantFor(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if opened == home {
		t.Error("a member of an organisation was signed into their home instead")
	}
	if opened != orgID {
		t.Errorf("signed into %s, want the organisation %s", opened, orgID)
	}
}

// The database refuses a second home even if the code stops asking.
//
// Two tabs signing in at once both see no home and both insert; the partial
// unique index is what makes the second one lose. Without it the loser would
// win too, and the person would have two homes with a membership in each.
func TestTheDatabaseRefusesASecondHome(t *testing.T) {
	pool := openPool(t)
	h := handlersFor(pool)
	userID := seedPerson(t, pool)
	ctx := context.Background()

	if _, err := h.FirstTenantFor(ctx, userID); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx,
		`INSERT INTO registry.tenants (slug, name, kind, owner_user_id)
		 VALUES ($1, 'second', 'personal', $2::uuid)`,
		"home-second-"+strings.ReplaceAll(userID, "-", ""), userID)
	if err == nil {
		t.Fatal("a second home was created for one person")
	}
	if !strings.Contains(err.Error(), "tenants_one_home_per_person") {
		t.Fatalf("the second home was refused for an unexpected reason: %v", err)
	}
}

// An organisation may not claim an owner, and a home may not go without one.
//
// Written as one test because it is one decision: kind and owner_user_id agree
// or the row is meaningless. A row with kind='organisation' and an owner would
// be filtered off the console's list by nothing and would appear in a person's
// home lookup by nothing — visible in neither place, which is the worst of the
// two ways to get this wrong.
func TestKindAndOwnerMustAgree(t *testing.T) {
	pool := openPool(t)
	userID := seedPerson(t, pool)
	ctx := context.Background()

	for _, bad := range []struct {
		name, kind string
		owner      any
	}{
		{"an organisation with an owner", "organisation", userID},
		{"a home with nobody", "personal", nil},
	} {
		_, err := pool.Exec(ctx,
			`INSERT INTO registry.tenants (slug, name, kind, owner_user_id)
			 VALUES ($1, $1, $2, $3::uuid)`,
			"agree-"+strings.ReplaceAll(uuid.NewString(), "-", "")[:12], bad.kind, bad.owner)
		if err == nil {
			t.Errorf("%s was accepted", bad.name)
			continue
		}
		if !strings.Contains(err.Error(), "tenants_home_has_an_owner") {
			t.Errorf("%s was refused for an unexpected reason: %v", bad.name, err)
		}
	}
}

// The switcher can tell the two kinds apart, and the home sorts last.
//
// The list is the only place a person sees their workspaces side by side, and
// after migration 00085 two of them can carry the same name — a citizen who
// also works somewhere is "Бат Дорж" in one row and their employer in the
// other. The slug does not help: a home's is derived from a user id. So the
// kind travels with the row and the shell draws from it.
//
// Last rather than first because a home is where somebody ends up when they
// have nowhere else, not the place they reach for. The same order the sign-in
// path uses.
func TestTheSwitcherSeesTheKindAndSortsTheHomeLast(t *testing.T) {
	pool := openPool(t)
	h := handlersFor(pool)
	ctx := context.Background()
	userID, orgID := seedMember(t, pool)

	home, err := h.HomeFor(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}

	options, err := auth.NewSessionStore(pool, time.Hour).TenantsForUser(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	var order []string
	for _, option := range options {
		kinds[option.ID] = option.Kind
		order = append(order, option.ID)
	}
	if kinds[orgID] != "organisation" {
		t.Errorf("the organisation came back as %q", kinds[orgID])
	}
	if kinds[home] != "personal" {
		t.Errorf("the home came back as %q", kinds[home])
	}
	if len(order) < 2 || order[len(order)-1] != home {
		t.Errorf("the home is not last in %v", order)
	}
}

// Signing in with a password, belonging to no organisation.
//
// The regression this exists for: HandleLogin looked the account up with an
// inner join onto memberships, so somebody with none matched no row and was
// told "invalid email or password". Everything else about 00085 worked — the
// eID and federated paths ask FirstTenantFor — and this one path carried its
// own copy of the question, so it kept the old answer. It was found on
// production, by signing in as a citizen for the first time.
//
// Asserted through the handler rather than through FirstTenantFor, because
// FirstTenantFor was right the whole time. What was wrong was that nothing
// called it here.
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
		t.Errorf("the sign-in opened a %q workspace, want the person's own home", kind)
	}
	// Nobody administers a workspace with one person in it, and saying
	// otherwise would draw an administrator's rail in a home.
	if answer.User.IsAdmin {
		t.Error("a citizen is an administrator of their own home")
	}
}
