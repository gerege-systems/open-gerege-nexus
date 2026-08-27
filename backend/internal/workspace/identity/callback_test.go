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
	if got, err := validEIDCallback("https://nexus.gerege.mn/auth/eid/callback"); err != nil || got == "" {
		t.Fatalf("expected callback to be accepted: %q, %v", got, err)
	}
	for _, raw := range []string{"http://nexus.gerege.mn/auth/eid/callback", "https://evil.example/auth/eid/callback", "https://nexus.gerege.mn/login"} {
		if _, err := validEIDCallback(raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}
