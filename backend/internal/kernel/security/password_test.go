/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package security

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestAPasswordVerifiesAgainstItsOwnHash(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("a new hash is not argon2id: %q", hash)
	}
	if !CheckPasswordHash("correct horse battery staple", hash) {
		t.Error("the password did not verify against its own hash")
	}
	if CheckPasswordHash("correct horse battery stapl", hash) {
		t.Error("a wrong password verified")
	}
}

// The salt is what keeps two people who chose the same password from having
// the same row.
func TestTwoHashesOfOnePasswordDiffer(t *testing.T) {
	first, err := HashPassword("the same password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword("the same password")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("two hashes of one password are identical: the salt is not random")
	}
}

// The whole point of keeping bcrypt: a deployment with existing accounts takes
// this change without resetting a single password.
func TestABcryptHashStillVerifies(t *testing.T) {
	legacy, err := bcrypt.GenerateFromPassword([]byte("an old password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPasswordHash("an old password", string(legacy)) {
		t.Error("an account hashed before this change can no longer sign in")
	}
	if !NeedsRehash(string(legacy)) {
		t.Error("a bcrypt hash was not reported as needing a rehash")
	}
}

func TestAnArgon2HashAtTheCurrentParametersIsLeftAlone(t *testing.T) {
	hash, err := HashPassword("a current password")
	if err != nil {
		t.Fatal(err)
	}
	if NeedsRehash(hash) {
		t.Error("a hash this build just wrote was reported as stale")
	}
}

// Raising the parameters must not need a migration: the next sign-in rewrites
// the hash, and this is the check that decides it.
func TestAWeakerArgon2HashIsRewritten(t *testing.T) {
	weak := "$argon2id$v=19$m=1024,t=1,p=1$c2FsdHNhbHRzYWx0c2E$" +
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if !NeedsRehash(weak) {
		t.Error("a hash below the current parameters was not reported as stale")
	}
}

// A stored value nobody wrote must not become a way in.
func TestAnUnreadableHashRefusesEveryPassword(t *testing.T) {
	for _, hash := range []string{
		"",
		"not a hash at all",
		"$argon2id$",
		"$argon2id$v=19$m=19456,t=2,p=1$$",
		"$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$",
		"$argon2id$v=99$m=19456,t=2,p=1$c2FsdA$c2VjcmV0",
		"$argon2id$v=19$m=0,t=0,p=0$c2FsdA$c2VjcmV0",
		"$argon2i$v=19$m=19456,t=2,p=1$c2FsdA$c2VjcmV0",
	} {
		if CheckPasswordHash("", hash) || CheckPasswordHash("anything", hash) {
			t.Errorf("%q verified a password", hash)
		}
	}
}

// bcrypt refused anything over 72 bytes outright, and a citizen once saw that
// error in the eID card. argon2id has no such limit.
func TestALongPasswordIsAccepted(t *testing.T) {
	long := strings.Repeat("ө", 100) // 200 bytes of UTF-8
	hash, err := HashPassword(long)
	if err != nil {
		t.Fatalf("a long password was refused: %v", err)
	}
	if !CheckPasswordHash(long, hash) {
		t.Error("a long password did not verify")
	}
	if CheckPasswordHash(long[:len(long)-2], hash) {
		t.Error("a truncated password verified: the tail is not being read")
	}
}
