package auth

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

// The JIT preimage was once the digest plus a ":eid-only" suffix — 73 bytes
// against bcrypt's hard 72 — so every first-time eID sign-in died at account
// creation and the citizen read the library's complaint in the eID card.
func TestEIDLinkingDigestHashesUnderBcrypt(t *testing.T) {
	subjects := []string{"AA90010111", "CID-99887766", strings.Repeat("Ө", 64)}
	for _, subject := range subjects {
		digest := eidLinkingDigest("linking-key", subject)
		if len(digest) != 64 {
			t.Fatalf("digest for %q is %d bytes, want 64", subject, len(digest))
		}
		if len(digest) > 72 {
			t.Fatalf("digest for %q exceeds bcrypt's 72-byte input limit", subject)
		}
		if _, err := HashPassword(digest); err != nil {
			t.Fatalf("bcrypt rejected the digest for %q: %v", subject, err)
		}
	}
}

// The digest is the account's identity across sign-ins, so it has to be stable
// for one subject and distinct between two.
func TestEIDLinkingDigestIsStableAndSubjectBound(t *testing.T) {
	if a, b := eidLinkingDigest("k", "AA90010111"), eidLinkingDigest("k", "AA90010111"); a != b {
		t.Errorf("same key and subject produced different digests: %s vs %s", a, b)
	}
	if a, b := eidLinkingDigest("k", "AA90010111"), eidLinkingDigest("k", "AA90010112"); a == b {
		t.Error("two subjects share a digest, so they would share an account")
	}
	if a, b := eidLinkingDigest("k1", "AA90010111"), eidLinkingDigest("k2", "AA90010111"); a == b {
		t.Error("the linking key does not affect the digest")
	}
}

func TestReportSignInFailureHidesInternalReasons(t *testing.T) {
	rec := httptest.NewRecorder()
	ReportSignInFailure(rec, errors.New("bcrypt: password length exceeds 72 bytes"))
	if rec.Code != 500 {
		t.Errorf("internal failure answered %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "bcrypt") {
		t.Errorf("internal reason reached the caller: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	ReportSignInFailure(rec, NewSignInError("eID identity is verified but account provisioning is disabled"))
	if rec.Code != 403 {
		t.Errorf("caller-facing failure answered %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "account provisioning is disabled") {
		t.Errorf("caller-facing reason was suppressed: %s", rec.Body.String())
	}
}
