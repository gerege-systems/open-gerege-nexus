package dan_test

import (
	"context"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/dan"
)

func TestDANServiceMockTokenVerification(t *testing.T) {
	svc := dan.NewDANService()

	profile, err := svc.VerifyDANToken(context.Background(), "dan_AA90010111")
	if err != nil {
		t.Fatalf("unexpected error during mock DAN verification: %v", err)
	}

	if profile.RegNumber != "AA90010111" {
		t.Errorf("expected RegNumber AA90010111, got %s", profile.RegNumber)
	}
	if profile.GatewayVersion != "dan.gerege.mn/v2.1" {
		t.Errorf("expected gateway version dan.gerege.mn/v2.1, got %s", profile.GatewayVersion)
	}
}

func TestDANServiceAuthenticateCitizen(t *testing.T) {
	svc := dan.NewDANService()

	profile, err := svc.AuthenticateDANCitizen(context.Background(), "AA90010111", "123456")
	if err != nil {
		t.Fatalf("unexpected error during citizen authentication: %v", err)
	}

	if profile.RegNumber != "AA90010111" {
		t.Errorf("expected RegNumber AA90010111, got %s", profile.RegNumber)
	}
}
