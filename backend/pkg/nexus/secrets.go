/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import "errors"

// SecretSealer is this deployment's one cipher for credentials at rest.
//
// A module that stores somebody else's token — an OAuth refresh token, a
// webhook signing secret, an API key for a system outside this platform — has
// to encrypt it, and must not decide how. The key belongs to the deployment
// (INTEGRATION_ENCRYPTION_KEY, named that since before any of this was an app
// and kept because renaming it would be a change every operator had to make in
// exchange for a tidier word), and a second implementation would mean half a
// deployment's credentials encrypted differently with nothing saying which
// half.
//
// This is what "can two of these exist in one installation" asks about and
// answers no: two ciphers are two answers to "can this deployment still read
// what it stored last year". So the implementation stays in the platform and
// this is how a module reaches it.
//
// It is published rather than assumed: a deployment with no key provides a
// sealer that answers Configured() false and refuses to Seal, which is the
// state a module has to handle anyway — see ErrNoSecretKey.
type SecretSealer interface {
	// Seal encrypts. It refuses rather than storing plaintext when the
	// deployment has no key: a token sitting in the clear in a database backup
	// is worse than a save that did not happen, and the failed save is the one
	// an operator can see and fix.
	Seal(plaintext []byte) ([]byte, error)

	// Open reverses Seal. A ciphertext that does not authenticate is an error
	// rather than an empty credential — returning "" would turn a rotated key
	// into unsigned webhooks instead of a visible failure.
	Open(ciphertext []byte) ([]byte, error)

	// Configured reports whether this deployment can store a credential at
	// all. Screens ask so they can say why saving is unavailable instead of
	// failing at the moment somebody presses save.
	Configured() bool
}

// ErrNoSecretKey is what Seal answers on a deployment that has configured no
// key. Compared with errors.Is by callers that want to say so in their own
// words.
var ErrNoSecretKey = errors.New("nexus: this deployment has no credential encryption key")

// Secrets returns the deployment's cipher, or an error naming its absence.
func Secrets() (SecretSealer, error) { return Capability[SecretSealer]() }
