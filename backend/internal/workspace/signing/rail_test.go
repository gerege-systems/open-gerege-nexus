package signing

import (
	"context"
	"errors"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// An installation with no eID registration is most of them, and what it must
// not do is answer as though a ceremony had started.
//
// A module that skipped Enabled() and got an empty session back would write a
// signature row for a citizen nobody asked. So every call refuses, with an
// error that is deliberately not the citizen's refusal: an operator can
// configure the rail and the caller may try again.
func TestAnInstallationWithNoRailRefusesRatherThanAnsweringEmpty(t *testing.T) {
	rail := Rail(nil)

	if rail.Enabled() {
		t.Fatal("a deployment with no eID service cannot sign")
	}

	session, err := rail.SignDigest(context.Background(), nexus.SignatureRequest{
		RegNumber: "УБ99010111", DigestHex: "abcd", DisplayText: "Гэрээ",
	})
	if !errors.Is(err, nexus.ErrSigningUnavailable) {
		t.Fatalf("starting a ceremony: %v", err)
	}
	if session.SessionID != "" {
		t.Fatalf("a refused ceremony must name no session, got %q", session.SessionID)
	}

	if _, err := rail.PollSignature(context.Background(), "УБ99010111", "s-1"); !errors.Is(err, nexus.ErrSigningUnavailable) {
		t.Fatalf("polling: %v", err)
	}
	if _, err := rail.VerifiedDigest(context.Background(), "УБ99010111", "s-1"); !errors.Is(err, nexus.ErrSigningUnavailable) {
		t.Fatalf("confirming what was signed: %v", err)
	}
}

// The states a poller sees are the rail's own strings, and only one of them
// means "keep asking".
func TestOnlyARunningCeremonyIsWorthPollingAgain(t *testing.T) {
	if nexus.SignatureRunning.Settled() {
		t.Fatal("a running ceremony has not settled, and the poller must keep asking")
	}
	for _, state := range []nexus.SignatureState{
		nexus.SignatureCompleted, nexus.SignatureFailed,
		nexus.SignatureExpired, nexus.SignatureRejected,
	} {
		if !state.Settled() {
			t.Fatalf("%s is an answer and the poller should stop", state)
		}
	}
}

// The compile-time half: the wrapper is the capability, checked here rather
// than at the first module that asks for one.
var _ nexus.Signer = signingRail{}
