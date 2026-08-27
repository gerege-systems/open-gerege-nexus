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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
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

// A first eID sign-in opens an account with what eID handed over.
//
// Before this it opened one under a synthesised address and kept the citizen's
// own email and telephone number only inside the claims blob, where no screen
// and no query reaches them: somebody signing in for the first time landed on a
// profile that knew their name and nothing else they had just been asked to
// share.
func TestAFirstEIDSignInKeepsWhatEIDGave(t *testing.T) {
	pool := openPool(t)
	h := handlersFor(pool)
	ctx := context.Background()
	t.Setenv("EID_RP_SECRET", "test-linking-key")
	openPlatform(t, pool)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	identity := &eid.EIDIdentity{
		CivilID:   "ШИН" + suffix,
		FirstName: "Сарантуяа",
		LastName:  "Ганбат",
		Email:     "saran-" + suffix + "@example.mn",
		Phone:     "99112233",
	}
	userID, tenantID, err := h.ResolveOrProvisionEIDUser(ctx, identity)
	if err != nil {
		t.Fatalf("a first eID sign-in did not open an account: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id=$1`, userID) })

	var email, phone, name, kind string
	if err := pool.QueryRow(ctx,
		`SELECT u.email, u.phone, u.name, t.kind
		   FROM registry.users u, registry.tenants t
		  WHERE u.id = $1::uuid AND t.id = $2::uuid`, userID, tenantID).Scan(&email, &phone, &name, &kind); err != nil {
		t.Fatal(err)
	}
	if email != identity.Email {
		t.Errorf("the account was opened under %q, not the address eID gave", email)
	}
	if phone != identity.Phone {
		t.Errorf("the telephone number eID gave was dropped: %q", phone)
	}
	if name != "Ганбат Сарантуяа" {
		t.Errorf("the name is %q", name)
	}
	// And it lands somewhere: a first-time citizen belongs to no organisation,
	// so the workspace opened for them is their own.
	if kind != "personal" {
		t.Errorf("a first eID sign-in opened a %q workspace", kind)
	}
}

// eID's email opens an account; it never claims one.
//
// The address eID hands over is what the citizen told the civil registry, not
// proof they control that mailbox today. Matching on it would let anybody
// holding an eID walk into an account opened by whoever once used the same
// address — so a taken address is left with its owner and the new account gets
// the synthesised one instead.
func TestEIDsEmailNeverClaimsSomebodyElsesAccount(t *testing.T) {
	pool := openPool(t)
	h := handlersFor(pool)
	ctx := context.Background()
	t.Setenv("EID_RP_SECRET", "test-linking-key")
	openPlatform(t, pool)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	shared := "shared-" + suffix + "@example.mn"
	var incumbent string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name) VALUES ($1,'x','Эзэн') RETURNING id::text`,
		shared).Scan(&incumbent); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id=$1`, incumbent) })

	identity := &eid.EIDIdentity{
		CivilID: "ДАВ" + suffix, FirstName: "Дорж", LastName: "Бат", Email: shared,
	}
	userID, _, err := h.ResolveOrProvisionEIDUser(ctx, identity)
	if err != nil {
		t.Fatalf("the sign-in failed instead of falling back: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id=$1`, userID) })

	if userID == incumbent {
		t.Fatal("an eID sign-in took over an account that merely shared an address")
	}
	var email string
	if err := pool.QueryRow(ctx, `SELECT email FROM registry.users WHERE id=$1::uuid`, userID).Scan(&email); err != nil {
		t.Fatal(err)
	}
	if email == shared {
		t.Errorf("two accounts hold %q", shared)
	}
}

// openPlatform lets strangers in for the length of one test.
//
// Provisioning through eID is gated by the access mode, and the shared test
// database is private — correctly, since that is the safe default. The
// settings store is a package-level global, so this is set and put back
// rather than left; tests in this package run one at a time, which is what
// makes that safe.
func openPlatform(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	store := settings.NewStore(pool)
	settings.UseStore(store)
	t.Cleanup(func() { settings.UseStore(nil) })

	ctx := context.Background()
	if _, err := pool.Exec(ctx,
		`INSERT INTO registry.platform_settings (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		settings.AccessMode, settings.AccessPublic); err != nil {
		t.Fatalf("open the platform: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM registry.platform_settings WHERE key = $1`, settings.AccessMode)
	})
	if err := store.Load(ctx); err != nil {
		t.Fatalf("load the settings: %v", err)
	}
}
