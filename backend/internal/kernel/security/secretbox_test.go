/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package security_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
)

// These two came from internal/workspace/integration on 2026-08-27, when the
// connectors became an app in another repository and this cipher was the one
// thing that could not go with them: a deployment has exactly one key, and a
// module reaches it through nexus.SecretSealer. The tests followed the code
// rather than the product — what they assert is true of every credential this
// platform stores, not of connectors.

func TestSealRoundTrip(t *testing.T) {
	t.Setenv(security.EncryptionKeyEnv, "a-passphrase-an-operator-would-actually-type")
	security.ResetEncryptionKeyForTest()

	plaintext := []byte("ya29.a0-refresh-token")
	sealed, err := security.Seal(plaintext)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	// The point of sealing: the secret must not be findable in what is stored.
	if strings.Contains(string(sealed), string(plaintext)) {
		t.Fatal("the plaintext survives in the ciphertext")
	}

	opened, err := security.Open(sealed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if string(opened) != string(plaintext) {
		t.Fatalf("round trip gave %q, want %q", opened, plaintext)
	}
}

// A rotated or mistyped key must surface, not decode to an empty credential —
// that would turn a configuration error into silently unsigned webhooks.
func TestOpenUnderTheWrongKeyIsAnError(t *testing.T) {
	t.Setenv(security.EncryptionKeyEnv, "the-original-key")
	security.ResetEncryptionKeyForTest()
	sealed, err := security.Seal([]byte("secret"))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	t.Setenv(security.EncryptionKeyEnv, "a-different-key-entirely")
	security.ResetEncryptionKeyForTest()
	if _, err := security.Open(sealed); err == nil {
		t.Fatal("open under the wrong key succeeded")
	}
}

// With no key at all the write fails rather than storing the credential in the
// clear. The failed save is the one an operator can see and fix.
func TestSealWithoutAKeyRefuses(t *testing.T) {
	t.Setenv(security.EncryptionKeyEnv, "")
	security.ResetEncryptionKeyForTest()

	if _, err := security.Seal([]byte("secret")); !errors.Is(err, security.ErrNoEncryptionKey) {
		t.Fatalf("seal without a key returned %v, want ErrNoEncryptionKey", err)
	}
	if security.EncryptionConfigured() {
		t.Error("EncryptionConfigured is true with no key set")
	}
	// And the same through the capability the modules use, which is the shape
	// a distribution sees.
	if _, err := (security.Sealer{}).Seal([]byte("secret")); err == nil {
		t.Error("the published sealer sealed something without a key")
	}
}
