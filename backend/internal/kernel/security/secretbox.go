/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

// Sealing a credential so that a database backup is not a list of other
// people's passwords.
//
// This was internal/workspace/integration's, where the first credentials a
// deployment held were a connector's OAuth tokens. The platform plane now holds
// some too — the keys an operator sets from the console — and the two planes do
// not import each other, so the cipher moved here rather than being written a
// second time. A deployment with two ways to encrypt a secret is a deployment
// where half of them are encrypted differently and nothing says which half.
//
// The variable keeps its old name. Renaming it would be a change every existing
// deployment had to make on the day they upgraded, in exchange for a tidier
// word, and the key it names is the same key.

// EncryptionKeyEnv is where the key comes from.
const EncryptionKeyEnv = "INTEGRATION_ENCRYPTION_KEY"

// ErrNoEncryptionKey is a deployment being asked to store a credential without
// having configured a key to seal it with.
//
// It fails the write rather than storing the credential in the clear. A token
// sitting in plaintext in a database backup is worse than a save that did not
// happen, and the second failure is the one an operator can see and fix.
var ErrNoEncryptionKey = errors.New(
	EncryptionKeyEnv + " is not set, so credentials cannot be stored safely")

var (
	keyOnce sync.Once
	keyVal  []byte
	keyErr  error
)

// encryptionKey resolves the 32-byte AES key once per process.
//
// The variable is accepted as base64, hex, or raw text. Raw text is hashed to
// the required length rather than rejected, because an operator who sets a
// passphrase should get a working deployment, not a startup failure they have
// to decode an error message to understand — and hashing is what they meant.
func encryptionKey() ([]byte, error) {
	keyOnce.Do(func() {
		raw := strings.TrimSpace(os.Getenv(EncryptionKeyEnv))
		if raw == "" {
			keyErr = ErrNoEncryptionKey
			return
		}
		if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil && len(decoded) == 32 {
			keyVal = decoded
			return
		}
		if decoded, err := hex.DecodeString(raw); err == nil && len(decoded) == 32 {
			keyVal = decoded
			return
		}
		sum := sha256.Sum256([]byte(raw))
		keyVal = sum[:]
	})
	return keyVal, keyErr
}

// EncryptionConfigured reports whether credentials can be stored. Screens ask
// so they can say why saving is unavailable instead of failing at the moment
// somebody presses save.
func EncryptionConfigured() bool {
	_, err := encryptionKey()
	return err == nil
}

// Seal encrypts plaintext with AES-256-GCM. The nonce is prepended to the
// ciphertext, which is how Open finds it again.
func Seal(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, nil
	}
	key, err := encryptionKey()
	if err != nil {
		return nil, err
	}
	gcm, err := mode(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("security: nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Open reverses Seal. A ciphertext that does not authenticate is an error, not
// an empty credential: silently returning "" would turn a rotated or corrupted
// key into unsigned webhooks rather than a visible failure.
func Open(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}
	key, err := encryptionKey()
	if err != nil {
		return nil, err
	}
	gcm, err := mode(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("security: the stored credential is truncated")
	}
	nonce, body := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("security: the stored credential could not be decrypted "+
			"(has %s changed?): %w", EncryptionKeyEnv, err)
	}
	return plaintext, nil
}

func mode(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("security: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("security: gcm: %w", err)
	}
	return gcm, nil
}

// ResetEncryptionKeyForTest clears the memoised key.
//
// Exported, and only ever called from a test. The key resolves once per
// process, which is right in production and wrong in a test that changes the
// environment between cases — and the test that needs it is in another package
// (internal/workspace/integration has had one since the cipher lived there), which
// is what an unexported helper in a _test file cannot serve.
func ResetEncryptionKeyForTest() {
	keyOnce = sync.Once{}
	keyVal = nil
	keyErr = nil
}

// Sealer presents the three functions above as nexus.SecretSealer.
//
// The methods are on a value rather than the package because a capability is
// asked for by type; there is no state, and two of these are the same one.
// server.go publishes it while it boots, which is what lets a module in another
// repository store an OAuth token without carrying a cipher of its own.
type Sealer struct{}

func (Sealer) Seal(plaintext []byte) ([]byte, error)  { return Seal(plaintext) }
func (Sealer) Open(ciphertext []byte) ([]byte, error) { return Open(ciphertext) }
func (Sealer) Configured() bool                       { return EncryptionConfigured() }
