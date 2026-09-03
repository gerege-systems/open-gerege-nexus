/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package identity

import "testing"

// Where the national identity provider is allowed to send somebody back to.
//
// It was beside the permission tests because both were in one package. What it
// guards is this one: a callback the caller supplies, on a rail that hands the
// address to somebody else's service.
func TestValidEIDCallback(t *testing.T) {
	t.Setenv("PUBLIC_ORIGIN", "https://nexus.gerege.mn")
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("EID_APP_CALLBACKS", "")
	if got, err := validEIDCallback("https://nexus.gerege.mn/auth/eid/callback"); err != nil || got == "" {
		t.Fatalf("expected callback to be accepted: %q, %v", got, err)
	}
	for _, raw := range []string{"http://nexus.gerege.mn/auth/eid/callback", "https://evil.example/auth/eid/callback", "https://nexus.gerege.mn/login"} {
		if _, err := validEIDCallback(raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}

	// A native client returns over its own scheme, which no origin check can
	// ever pass. Exact match against EID_APP_CALLBACKS, or nothing: a scheme
	// that merely looks similar, or the right scheme with a different path, is
	// a different address and must not be handed to the identity provider.
	if _, err := validEIDCallback("gerege-nexus://auth"); err == nil {
		t.Fatal("expected an unlisted app callback to be rejected")
	}
	t.Setenv("EID_APP_CALLBACKS", "gerege-nexus://auth")
	if got, err := validEIDCallback("gerege-nexus://auth"); err != nil || got != "gerege-nexus://auth" {
		t.Fatalf("expected the listed app callback to be accepted: %q, %v", got, err)
	}
	for _, raw := range []string{"gerege-nexus://auth/steal", "gerege-nexus://evil", "gerege-nexus-evil://auth", "GEREGE-NEXUS://auth"} {
		if _, err := validEIDCallback(raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}
