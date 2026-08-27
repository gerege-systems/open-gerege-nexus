/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * One eID is one person.
 */

package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/eid"
)

// Linking writes the identity and the Gerege number, and the number is the
// point: pkg/nexus.PersonFeed names a person by it, so an account without one
// cannot be told anything by a supplier's module.
func TestLinkingAnEIDRecordsTheGeregeNumber(t *testing.T) {
	pool := openPool(t)
	h := handlersFor(pool)
	ctx := context.Background()
	userID := seedPerson(t, pool)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	identity := &eid.EIDIdentity{
		CivilID:   "ЦТ" + suffix,
		FirstName: "Бат",
		LastName:  "Дорж",
		GeID:      770000000000 + int64(len(suffix)),
	}
	if err := h.LinkEIDIdentityStrict(ctx, userID, identity); err != nil {
		t.Fatalf("link: %v", err)
	}

	var geID *int64
	if err := pool.QueryRow(ctx, `SELECT ge_id FROM registry.users WHERE id=$1::uuid`, userID).Scan(&geID); err != nil {
		t.Fatal(err)
	}
	if geID == nil || *geID != identity.GeID {
		t.Errorf("the account's Gerege number is %v, want %d", geID, identity.GeID)
	}
	var linked bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM registry.user_eid_identities WHERE user_id=$1::uuid)`,
		userID).Scan(&linked); err != nil {
		t.Fatal(err)
	}
	if !linked {
		t.Error("the eID identity was not recorded")
	}
}

// A second account cannot claim the same citizen.
//
// The refusal is the unique index on person_etsi, and this asserts it arrives
// as something a screen can say rather than as a driver error. Splitting one
// person's signing history across two accounts is the failure it prevents —
// and it is the failure a profile screen invites, because that is where
// somebody would press the button twice from two accounts they hold.
func TestASecondAccountCannotClaimTheSameEID(t *testing.T) {
	pool := openPool(t)
	h := handlersFor(pool)
	ctx := context.Background()
	first, second := seedPerson(t, pool), seedPerson(t, pool)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	identity := &eid.EIDIdentity{CivilID: "ЦТ" + suffix, FirstName: "Бат", LastName: "Дорж"}

	if err := h.LinkEIDIdentityStrict(ctx, first, identity); err != nil {
		t.Fatalf("the first link failed: %v", err)
	}
	err := h.LinkEIDIdentityStrict(ctx, second, identity)
	if err == nil {
		t.Fatal("two accounts hold the same eID identity")
	}
	if !errors.Is(err, auth.ErrEIDBelongsToSomebodyElse) {
		t.Errorf("refused for an unexpected reason: %v", err)
	}
}

// Linking twice from the same account is not a conflict — it is a refresh.
//
// Somebody who links, changes their name at the registry and links again
// should end up with what eID says today, not an error.
func TestRelinkingTheSameAccountRefreshesIt(t *testing.T) {
	pool := openPool(t)
	h := handlersFor(pool)
	ctx := context.Background()
	userID := seedPerson(t, pool)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	identity := &eid.EIDIdentity{CivilID: "ЦТ" + suffix, FirstName: "Бат", LastName: "Дорж"}
	if err := h.LinkEIDIdentityStrict(ctx, userID, identity); err != nil {
		t.Fatal(err)
	}
	identity.FirstName = "Батбаяр"
	if err := h.LinkEIDIdentityStrict(ctx, userID, identity); err != nil {
		t.Fatalf("relinking the same account failed: %v", err)
	}

	var given string
	if err := pool.QueryRow(ctx,
		`SELECT given_name FROM registry.user_eid_identities WHERE user_id=$1::uuid`, userID).Scan(&given); err != nil {
		t.Fatal(err)
	}
	if given != "Батбаяр" {
		t.Errorf("the linked identity still says %q", given)
	}
}
