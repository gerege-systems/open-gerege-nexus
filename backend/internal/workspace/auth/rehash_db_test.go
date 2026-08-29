/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package auth

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// An account hashed before argon2id signs in with the password it always had,
// and leaves with the hash this build writes.
//
// The upgrade cannot be asserted anywhere but against a database: what makes it
// safe is that the UPDATE names the hash it replaces, so two sign-ins racing
// each other write one row between them.
func TestSigningInUpgradesABcryptHash(t *testing.T) {
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("TEST_DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("neither AUTH_TEST_DATABASE_URL nor TEST_DATABASE_URL is set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	// Registered before the row below, so it runs after it: cleanups are
	// last-in-first-out, and a `defer pool.Close()` here would shut the pool
	// before the account it made could be removed — which is how a test that
	// looks tidy leaves a row in every database it is ever run against.
	t.Cleanup(pool.Close)
	ctx := context.Background()

	const password = "an old password nobody has changed"
	legacy, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	email := fmt.Sprintf("rehash-%d@auth.test", time.Now().UnixNano())
	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name) VALUES ($1, $2, 'Rehash') RETURNING id::text`,
		email, string(legacy)).Scan(&userID); err != nil {
		t.Fatalf("create the account: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id = $1::uuid`, userID)
	})

	handlers := &Handlers{db: pool}
	handlers.rehashIfStale(ctx, userID, string(legacy), password)

	var stored string
	if err := pool.QueryRow(ctx,
		`SELECT password_hash FROM registry.users WHERE id = $1::uuid`, userID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "$argon2id$") {
		t.Fatalf("the stored hash was not upgraded: %q", stored)
	}
	if !security.CheckPasswordHash(password, stored) {
		t.Error("the upgraded hash does not verify the password it was made from")
	}

	// A second sign-in writes nothing: the hash is already current.
	handlers.rehashIfStale(ctx, userID, stored, password)
	var after string
	if err := pool.QueryRow(ctx,
		`SELECT password_hash FROM registry.users WHERE id = $1::uuid`, userID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != stored {
		t.Error("a current hash was rewritten anyway")
	}
}
