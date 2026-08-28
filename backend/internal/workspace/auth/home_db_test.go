/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Nobody is turned away at the door, and nobody is given a room they never
 * asked for.
 *
 * Before migration 00085 a person who belonged to no organisation could not
 * sign in at all: FirstTenantFor found no membership and returned
 * ErrNoOrganisation, and the session was never made. The schema had said
 * otherwise for months — registry.users is global and ge_id is unique across
 * the deployment — so this was the one place where the code refused what the
 * data model allowed.
 *
 * 00085 answered it by making every such person a workspace of their own. That
 * worked, and it put a row in registry.tenants for every human being who ever
 * authenticated — on the table that is the customer list and the parent of
 * thirty-nine others. Measured at a million people it was 3.9 GB, most of it
 * access-control rows for workspaces with one member who owned them.
 *
 * 00094 answered it properly: a session may carry no workspace. These tests are
 * what say the door is still open and that nothing is built behind it.
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

// The whole point: a person in no organisation signs in, and into nothing.
//
// An empty workspace is the answer and not a failure, which is why this asserts
// the absence of an error as loudly as the absence of an id. The two used to be
// the same thing — no membership meant ErrNoOrganisation meant no session — and
// separating them is what let a citizen through the door.
func TestSomebodyInNoOrganisationSignsInWithNoWorkspace(t *testing.T) {
	pool := openPool(t)
	h := handlersFor(pool)
	userID := seedPerson(t, pool)

	opened, err := h.FirstTenantFor(context.Background(), userID)
	if err != nil {
		t.Fatalf("a person in no organisation was refused a workspace to sign in to: %v", err)
	}
	if opened != "" {
		t.Errorf("the sign-in opened workspace %q; somebody who belongs to no "+
			"organisation should open with none", opened)
	}
}

// And nothing is built for them.
//
// The assertion 00094 exists for. It is written against registry.tenants rather
// than against the return value above because the cost this removes was never
// visible in the return value: 00085 made a workspace, a profile, three roles
// and seventeen role_permissions per person, and every one of those was correct
// by its own lights.
func TestSigningInMakesNoWorkspaceForACitizen(t *testing.T) {
	pool := openPool(t)
	h := handlersFor(pool)
	ctx := context.Background()
	userID := seedPerson(t, pool)

	for range 2 {
		if _, err := h.FirstTenantFor(ctx, userID); err != nil {
			t.Fatal(err)
		}
	}

	var workspaces, memberships int
	if err := pool.QueryRow(ctx,
		`SELECT (SELECT count(*) FROM registry.tenants WHERE owner_user_id = $1::uuid),
		        (SELECT count(*) FROM workspace.memberships WHERE user_id = $1::uuid)`,
		userID).Scan(&workspaces, &memberships); err != nil {
		t.Fatal(err)
	}
	if workspaces != 0 || memberships != 0 {
		t.Errorf("signing in twice left %d workspace(s) and %d membership(s); want none of either",
			workspaces, memberships)
	}
}

// Somebody who works somewhere opens there.
//
// This has been three things in three days and the history is in FirstTenantFor.
// Briefly: opening in the organisation was right, then wrong because people
// with no organisation had nowhere to go, and is right again now that having
// nowhere to go is a state the platform can hold.
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

// A person may still own a workspace, and the schema still says what one is.
//
// 00094 stopped *making* personal workspaces; it did not remove the idea. The
// columns stay because the pattern every comparable platform settles on is to
// have one — Vercel's hobby team, GitHub's personal account — created when
// somebody starts using the product rather than when they prove who they are.
// There is no trigger for that in this codebase yet, and inventing one would be
// speculation; leaving the schema able to express it costs two columns.
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
	if answer.User.TenantID != "" {
		t.Errorf("the sign-in opened workspace %q; a citizen opens with none", answer.User.TenantID)
	}
	// Nobody administers nothing, and saying otherwise would draw an
	// administrator's rail for somebody with no organisation to administer.
	if answer.User.IsAdmin {
		t.Error("a citizen with no organisation is an administrator")
	}

	// And the session it made is real: resolvable, and carrying no workspace.
	// A session row is where the change actually lands — sessions.tenant_id
	// was NOT NULL until 00094 — so a sign-in that returned the right JSON and
	// wrote a row nothing could read afterwards would pass every line above.
	var sessions int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM workspace.sessions WHERE user_id = $1::uuid AND tenant_id IS NULL`,
		userID).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 1 {
		t.Errorf("the sign-in left %d workspace-less session(s), want exactly one", sessions)
	}
}
