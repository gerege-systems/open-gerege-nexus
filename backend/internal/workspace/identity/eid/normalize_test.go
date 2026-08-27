package eid

import (
	"testing"
	"time"

	coreeid "github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/eidrp"
)

// The relying party reports no deadline for push sessions and keeps them alive
// far longer than any figure worth inventing — one was observed still RUNNING
// after nine minutes. Inventing "two minutes from now" and sending it as fact
// made the browser abandon sessions eID was still waiting on, mid-approval.
func TestNormalizeStartDoesNotInventADeadline(t *testing.T) {
	got := normalizeStart(&coreeid.StartResult{SessionID: "s-1", VerificationCode: "4722"})
	if got.ExpiresAt != "" {
		t.Errorf("ExpiresAt = %q, want empty: the relying party stated none", got.ExpiresAt)
	}
	if got.SessionID != "s-1" || got.VerificationCode != "4722" {
		t.Errorf("normalizeStart mangled the session: %+v", got)
	}
}

// When the relying party does state a deadline it is authoritative, so it
// passes through untouched.
func TestNormalizeStartKeepsTheRelyingPartysDeadline(t *testing.T) {
	const stated = "2026-08-06T17:32:42+08:00"
	got := normalizeStart(&coreeid.StartResult{SessionID: "s-2", VerificationCode: "1414", ExpiresAt: stated})
	if got.ExpiresAt != stated {
		t.Errorf("ExpiresAt = %q, want %q", got.ExpiresAt, stated)
	}
}

// cmd/api derives its write deadline from PollWindow, and the relying party's
// own HTTP client gives up at 30s. Growing this past either brings back the
// unwritten responses that nginx served as 502.
func TestPollWindowFitsTheSurroundingDeadlines(t *testing.T) {
	if PollWindow <= 0 {
		t.Fatal("PollWindow must be positive")
	}
	if PollWindow > 30*time.Second {
		t.Errorf("PollWindow %s exceeds the relying party HTTP client timeout", PollWindow)
	}
}
