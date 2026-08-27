package auth

import (
	"context"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/identity/eid"

	"github.com/google/uuid"
)

// What the Gerege number does to an account that already exists, asked of a
// real schema: the unique index, the address rewrite and the "leave their own
// address alone" rule are all decided by the statement rather than by Go.

func TestAnInventedAddressIsUpgradedToTheGeregeNumber(t *testing.T) {
	pool := lockoutPool(t)
	server := &Handlers{db: pool}
	ctx := context.Background()

	// An account as it was opened before eID returned the number: a synthetic
	// address nobody can read and no Gerege number.
	old := "eid+" + uuid.NewString()[:32] + "@identity.invalid"
	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users(email, password_hash, name) VALUES($1, 'x', 'Иргэн') RETURNING id::text`,
		old).Scan(&userID); err != nil {
		t.Fatalf("seed the account: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id=$1::uuid`, userID) })

	// A number no other row in the shared test database can be holding.
	geID := int64(900000000) + int64(uuid.New().ID()%1000000)
	server.rememberGeID(ctx, userID, &eid.EIDIdentity{RegNumber: "AA00112233", GeID: geID})

	var email string
	var stored *int64
	if err := pool.QueryRow(ctx,
		`SELECT email, ge_id FROM registry.users WHERE id=$1::uuid`, userID).Scan(&email, &stored); err != nil {
		t.Fatalf("read it back: %v", err)
	}
	if stored == nil || *stored != geID {
		t.Fatalf("ge_id is %v, not the number eID returned", stored)
	}
	if email != GeIDEmail(geID) {
		t.Fatalf("the address is %q, not the readable one", email)
	}
}

func TestAPersonsOwnAddressSurvivesTheirEID(t *testing.T) {
	pool := lockoutPool(t)
	server := &Handlers{db: pool}
	ctx := context.Background()

	theirs := "person-" + uuid.NewString()[:8] + "@example.mn"
	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users(email, password_hash, name) VALUES($1, 'x', 'Иргэн') RETURNING id::text`,
		theirs).Scan(&userID); err != nil {
		t.Fatalf("seed the account: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id=$1::uuid`, userID) })

	geID := int64(900000000) + int64(uuid.New().ID()%1000000)
	server.rememberGeID(ctx, userID, &eid.EIDIdentity{RegNumber: "AA00112233", GeID: geID})

	var email string
	var stored *int64
	if err := pool.QueryRow(ctx,
		`SELECT email, ge_id FROM registry.users WHERE id=$1::uuid`, userID).Scan(&email, &stored); err != nil {
		t.Fatalf("read it back: %v", err)
	}
	// The number is recorded — that is what it is for — and the address they
	// chose is not touched. Linking a sign-in method is not a rename.
	if stored == nil || *stored != geID {
		t.Fatalf("ge_id is %v, not the number eID returned", stored)
	}
	if email != theirs {
		t.Fatalf("the person's own address became %q", email)
	}
}
