/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// Hashing a password, for whoever is doing the hashing.
//
// Both planes do. A person signing into an organisation and an operator signing
// into the console are different accounts in different tables under different
// database roles, and the one thing they are not allowed to differ on is how a
// password is stored — two answers to that question is one deployment where
// half the passwords are weaker and nothing says which half.
//
// So it is here rather than in either plane: the cost is a single decision, and
// changing it changes it for everybody.
//
// New passwords are argon2id. It is the algorithm a password hash should be
// today for the reason bcrypt is not: bcrypt's work factor buys processor time
// and nothing else, and the machines that guess passwords are graphics cards
// with thousands of cores and very little memory each. argon2id makes each
// guess cost memory as well, which is the part that does not parallelise
// cheaply. It is also what OWASP names first, with the parameters below.
//
// Hashes already written stay readable. bcrypt verification is kept for exactly
// that reason and for no other: a deployment with existing accounts must not
// have to reset every password to take this change, and NeedsRehash lets a
// sign-in replace the old hash with the new one for the person who just proved
// they know the password. Nothing writes bcrypt any more.
const (
	// The OWASP baseline for argon2id: 19 MiB, two passes, one lane.
	//
	// Memory is the parameter that matters and 19 MiB is deliberately modest —
	// this platform runs on one small server, sign-in is rate limited to five
	// attempts a minute per address, and a figure that makes a login cost more
	// than the request it precedes is a figure somebody turns off. Raising it
	// later needs no migration: NeedsRehash rewrites each hash at the next
	// sign-in.
	argonMemory  = 19 * 1024
	argonTime    = 2
	argonKeyLen  = 32
	argonSaltLen = 16

	// The range a stored key's length may be in. HashPassword writes
	// argonKeyLen; these bound what CheckPasswordHash will derive against a
	// value that came out of a column.
	argonMinKeyLen = 16
	argonMaxKeyLen = 64
)

// argonLanes is the parallelism parameter, kept at one lane unless the machine
// has cores to spare. It is read into the hash string either way, so a hash
// written on one machine verifies on another.
func argonLanes() uint8 {
	if runtime.NumCPU() >= 4 {
		return 2
	}
	return 1
}

// HashPIN hashes a numeric PIN, at a cost the PIN's own strength justifies.
//
// A four-digit PIN has ten thousand values. No key derivation makes that space
// large: an attacker holding the table walks it in minutes whatever the memory
// parameter, so the protection has to be — and now is — the attempt counter on
// the till (workspace.devices.staff_pin_failures, migration 00105).
//
// What the parameter does decide is what a *correct* PIN costs us. The till
// sends the digits alone, with no name attached, so the platform tries them
// against every active credential in the organisation: an organisation with
// fifty staff pays fifty derivations for one tap on a screen. At the password
// parameters that is a second and a half of one core, and the shop assistant
// waits for it.
//
// So: one pass over 8 MiB — an eighth of the password cost, still far beyond a
// plain hash, and the number that keeps a fifty-person till answering in a
// quarter of a second.
func HashPIN(pin string) (string, error) {
	return hashWith([]byte(pin), pinMemory, pinTime)
}

const (
	pinMemory = 8 * 1024
	pinTime   = 1
)

// HashPassword returns an argon2id hash in the PHC string format:
//
//	$argon2id$v=19$m=19456,t=2,p=1$<salt>$<key>
//
// Ninety-odd characters, which registry.users.password_hash (255) holds with
// room to spare.
func HashPassword(password string) (string, error) {
	return hashWith([]byte(password), argonMemory, argonTime)
}

// hashWith is the one place a hash is written. The parameters travel inside the
// string, so CheckPasswordHash verifies both kinds without being told which it
// is holding — and a cost raised later still verifies what was written before.
func hashWith(secret []byte, memory, time uint32) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read a salt: %w", err)
	}
	lanes := argonLanes()
	key := argon2.IDKey(secret, salt, time, memory, lanes, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memory, time, lanes,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// CheckPasswordHash reports whether the password is the one behind the hash,
// in whichever format the hash was written.
//
// A hash it cannot parse is a failed comparison rather than an error: the one
// caller is a sign-in, the answer it can act on is yes or no, and a stored
// value nobody wrote — an empty column, a truncated row — must not become a
// way in.
func CheckPasswordHash(password, hash string) bool {
	if strings.HasPrefix(hash, "$argon2id$") {
		return checkArgon2id(password, hash)
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// NeedsRehash reports whether a hash that has just verified should be replaced.
//
// True for every bcrypt hash, and for an argon2id hash written with weaker
// parameters than this build uses. The caller holds the plaintext exactly once
// — in the request that just proved it — so this is the only moment the
// upgrade is free.
func NeedsRehash(hash string) bool {
	if !strings.HasPrefix(hash, "$argon2id$") {
		return true
	}
	memory, time, _, err := argonParams(hash)
	if err != nil {
		return true
	}
	// PIN-ийн параметр (HashPIN) нь зориудаар доогуур бөгөөд түүнийг
	// нууц үгийн хэмжүүрээр хэмжвэл тап бүрт дахин бичих гэж оролдоно.
	if memory == pinMemory && time == pinTime {
		return false
	}
	return memory < argonMemory || time < argonTime
}

func checkArgon2id(password, hash string) bool {
	memory, timeCost, lanes, err := argonParams(hash)
	if err != nil {
		return false
	}
	fields := strings.Split(hash, "$")
	salt, err := base64.RawStdEncoding.DecodeString(fields[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(fields[5])
	if err != nil {
		return false
	}
	// The stored key's length decides how much key to derive, and it is read
	// from a column — so it is bounded here rather than trusted. Sixteen bytes
	// is below anything this package has ever written; sixty-four is above it,
	// and a value outside that range is a row nobody made.
	if len(want) < argonMinKeyLen || len(want) > argonMaxKeyLen {
		return false
	}
	// #nosec G115 -- the length is bounded to [16, 64] three lines above, which
	// is the check the rule is asking for; it cannot see it.
	got := argon2.IDKey([]byte(password), salt, timeCost, memory, lanes, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// argonParams reads m, t and p back out of a PHC string.
func argonParams(hash string) (memory, timeCost uint32, lanes uint8, err error) {
	// $ argon2id $ v=19 $ m=,t=,p= $ salt $ key — six fields, the first empty.
	fields := strings.Split(hash, "$")
	if len(fields) != 6 || fields[1] != "argon2id" {
		return 0, 0, 0, errors.New("not an argon2id hash")
	}
	var version int
	if _, err := fmt.Sscanf(fields[2], "v=%d", &version); err != nil || version != argon2.Version {
		return 0, 0, 0, errors.New("unsupported argon2 version")
	}
	var parallelism uint8
	if _, err := fmt.Sscanf(fields[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &parallelism); err != nil {
		return 0, 0, 0, errors.New("unreadable argon2 parameters")
	}
	if memory == 0 || timeCost == 0 || parallelism == 0 {
		return 0, 0, 0, errors.New("argon2 parameters out of range")
	}
	return memory, timeCost, parallelism, nil
}
