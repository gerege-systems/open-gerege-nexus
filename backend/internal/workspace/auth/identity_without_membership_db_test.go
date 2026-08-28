/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Who somebody is, asked without asking where they work.
 *
 * This repository has made the same mistake four times. A sign-in path looks an
 * account up with `JOIN workspace.memberships`, so an account belonging to no
 * organisation matches no row — and is then treated not as a person with no
 * organisation but as a person who does not exist. What happens next depends on
 * the path: HandleLogin said "invalid email or password", the eID paths fell
 * through to provisioning and were refused on a private deployment, the SSO
 * path the same. Every one of them looked like a different bug.
 *
 * It is one bug, and it is a category error. Identity is answered by identity —
 * an address, a Gerege number, a provider subject. Where somebody stands is a
 * second question with exactly one answer, FirstTenantFor, and joining the two
 * makes the first depend on the second.
 *
 * It is worth saying that this is not about whether a person gets a workspace of
 * their own. They do. The bug bites while they have none *yet* — the moment
 * between an administrator removing their last membership and their next
 * sign-in — and in that moment the lookup could not see them at all.
 *
 * The first three were found on production, twice by a person unable to sign
 * in. This is the test that was missing: it asserts the property directly, on
 * an account deliberately left with no membership at all.
 */

package auth_test

import (
	"context"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/eid"
	"github.com/google/uuid"
)

// An account with no membership is still found by its Gerege number.
//
// Asserted through ResolveOrProvisionEIDUser rather than through the query,
// because the query was never wrong on its own terms: it found accounts that
// had a membership, which is what it asked for. What was wrong was asking.
func TestAnEIDSignInFindsAnAccountThatBelongsNowhere(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	h := auth.New(auth.Deps{DB: pool})
	// Checked before the lookups even though they do not use it, so without it
	// the branches under test are unreachable.
	t.Setenv("EID_RP_SECRET", "test-linking-key")

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	// A Gerege number nothing else in the test database carries.
	geID := int64(9_000_000_000) + int64(uuid.New().ID()%1_000_000)

	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name, ge_id)
		 VALUES ($1,'x',$2,$3) RETURNING id::text`,
		"nomember-"+suffix+"@example.mn", "Иргэн "+suffix, geID).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id=$1`, userID) })

	// Said out loud: the whole test rests on this account belonging nowhere.
	var memberships int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM workspace.memberships WHERE user_id = $1::uuid`, userID).
		Scan(&memberships); err != nil {
		t.Fatal(err)
	}
	if memberships != 0 {
		t.Fatalf("the account under test has %d membership(s); the test proves nothing", memberships)
	}

	found, tenantID, err := h.ResolveOrProvisionEIDUser(ctx, &eid.EIDIdentity{
		CivilID:        "ИД" + suffix,
		RegNumber:      "ИД" + suffix,
		GeID:           geID,
		FirstName:      "Сарантуяа",
		LastName:       "Ганбат",
		VerifiedStatus: true,
	})
	if err != nil {
		// This is the production failure, in the form it arrived in: the
		// account exists, the sign-in cannot see it, and the error is about
		// opening a new one.
		t.Fatalf("an eID sign-in could not find an account that belongs to no organisation: %v", err)
	}
	if found != userID {
		t.Errorf("the sign-in resolved to %s, want the existing account %s — a second copy of "+
			"the same citizen was opened instead of finding them", found, userID)
	}
	// And they are put somewhere, rather than left standing in nothing.
	if tenantID == "" {
		t.Error("the sign-in opened no workspace at all")
	}
}

// And the same account, found by the address it was opened under.
//
// The second of the two copies inside ResolveOrProvisionEIDUser. They were
// separate statements a few lines apart and had to be found separately, which
// is why this asserts both rather than trusting that fixing one fixed the pair.
func TestAnEIDSignInFindsAnAccountByAddressWithNoMembership(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	h := auth.New(auth.Deps{DB: pool})
	// Checked before the lookups even though they do not use it, so without it
	// the branches under test are unreachable.
	t.Setenv("EID_RP_SECRET", "test-linking-key")

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	geID := int64(9_500_000_000) + int64(uuid.New().ID()%1_000_000)
	// Opened under the address the platform derives from the Gerege number,
	// which is the form the lookup searches for, and carrying no ge_id — so
	// the branch above cannot be the one that answers.
	email := auth.GeIDEmail(geID)

	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name) VALUES ($1,'x',$2) RETURNING id::text`,
		email, "Иргэн "+suffix).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id=$1`, userID) })

	found, tenantID, err := h.ResolveOrProvisionEIDUser(ctx, &eid.EIDIdentity{
		CivilID:        "АД" + suffix,
		RegNumber:      "АД" + suffix,
		GeID:           geID,
		FirstName:      "Батаа",
		LastName:       "Дорж",
		VerifiedStatus: true,
	})
	if err != nil {
		t.Fatalf("an eID sign-in could not find an account by its address: %v", err)
	}
	if found != userID {
		t.Errorf("the sign-in resolved to %s, want the existing account %s", found, userID)
	}
	if tenantID == "" {
		t.Error("the sign-in opened no workspace at all")
	}
}
