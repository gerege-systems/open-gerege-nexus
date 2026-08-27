package integration

import (
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
)

// The connector credentials this package stores are sealed by the platform's
// one cipher, which lives in internal/kernel/security. It was here first; it
// moved when the console started holding credentials too, because the two
// planes cannot import each other and a second implementation would have meant
// a deployment encrypting half its secrets one way and half another.

// ErrNoEncryptionKey is a deployment with no key configured.
var ErrNoEncryptionKey = security.ErrNoEncryptionKey

const encryptionKeyEnv = security.EncryptionKeyEnv

// EncryptionConfigured reports whether credentials can be stored. The
// integrations screen asks so it can say why a provider is unavailable instead
// of failing at the moment of saving.
func EncryptionConfigured() bool { return security.EncryptionConfigured() }

func seal(plaintext []byte) ([]byte, error) { return security.Seal(plaintext) }

func open(ciphertext []byte) ([]byte, error) { return security.Open(ciphertext) }
