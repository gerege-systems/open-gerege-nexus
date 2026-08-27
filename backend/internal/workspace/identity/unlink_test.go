/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The rule that keeps somebody from locking themselves out.
 *
 * Unlinking is a single click on a screen a person reaches from the header, and
 * the click before the last one is indistinguishable from any other. So the
 * refusal has to come from the server, and it has to be counted from the
 * database rather than from the list the browser happens to be holding.
 *
 *	AUTH_TEST_DATABASE_URL=postgres://... go test ./internal/operator/...
 */

package identity

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func unlinkPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to a migrated test database to run the unlink tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatalf("ping: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// unlinkUser inserts a throwaway account. The identity rows cascade with it, so
// the cleanup is one delete.
func unlinkUser(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	var id string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users(email, password_hash, name) VALUES($1, 'x', 'unlink probe') RETURNING id::text`,
		"unlink+"+uuid.NewString()+"@identity.invalid").Scan(&id); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id=$1`, id)
	})
	return id
}

func addSSOIdentity(t *testing.T, pool *pgxpool.Pool, userID, issuer, subject string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO registry.user_sso_identities(user_id, issuer, subject, email, name, claims)
		 VALUES($1,$2,$3,$4,'probe','{}'::jsonb)`,
		userID, issuer, subject, subject+"@identity.invalid"); err != nil {
		t.Fatalf("insert sso identity: %v", err)
	}
}

func addEIDIdentity(t *testing.T, pool *pgxpool.Pool, userID, etsi string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO registry.user_eid_identities(user_id, person_etsi, given_name, surname, claims)
		 VALUES($1,$2,'probe','probe','{}'::jsonb)`, userID, etsi); err != nil {
		t.Fatalf("insert eid identity: %v", err)
	}
}

// One identity is not expendable, however the screen asks.
func TestALoneIdentityIsNotRemovable(t *testing.T) {
	pool := unlinkPool(t)
	s := New(Deps{DB: pool})
	userID := unlinkUser(t, pool)
	addSSOIdentity(t, pool, userID, "https://accounts.google.com", "sole-"+uuid.NewString())

	identities := s.LinkedIdentities(context.Background(), userID)
	if len(identities) != 1 {
		t.Fatalf("expected exactly one identity, got %d", len(identities))
	}
	if identities[0].Removable {
		t.Error("the only way of signing in was offered as removable")
	}
}

// Two, and either may go — but only until one is left.
func TestTheSecondIdentityMakesBothRemovable(t *testing.T) {
	pool := unlinkPool(t)
	s := New(Deps{DB: pool})
	userID := unlinkUser(t, pool)
	addSSOIdentity(t, pool, userID, "https://accounts.google.com", "pair-"+uuid.NewString())
	addEIDIdentity(t, pool, userID, "ETSI-"+uuid.NewString())

	identities := s.LinkedIdentities(context.Background(), userID)
	if len(identities) != 2 {
		t.Fatalf("expected two identities, got %d", len(identities))
	}
	for _, identity := range identities {
		if !identity.Removable {
			t.Errorf("%s was not removable despite a second identity existing", identity.Kind)
		}
	}
}

// The guard reads the database rather than the request, so removing one and
// then asking about the other has to flip the answer without anybody saying so.
func TestRemovingOneOfTwoLeavesTheOtherPinned(t *testing.T) {
	pool := unlinkPool(t)
	s := New(Deps{DB: pool})
	ctx := context.Background()
	userID := unlinkUser(t, pool)
	subject := "drop-" + uuid.NewString()
	addSSOIdentity(t, pool, userID, "https://accounts.google.com", subject)
	addEIDIdentity(t, pool, userID, "ETSI-"+uuid.NewString())

	if _, err := pool.Exec(ctx,
		`DELETE FROM registry.user_sso_identities WHERE user_id=$1 AND subject=$2`, userID, subject); err != nil {
		t.Fatalf("delete sso identity: %v", err)
	}

	identities := s.LinkedIdentities(ctx, userID)
	if len(identities) != 1 {
		t.Fatalf("expected one identity after the removal, got %d", len(identities))
	}
	if identities[0].Removable {
		t.Error("the survivor stayed removable, so the account could be stranded")
	}
}
