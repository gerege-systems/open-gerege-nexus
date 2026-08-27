package eid_test

import (
	"context"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/eid"
)

func TestEIDServiceMockOAuth2Exchange(t *testing.T) {
	svc := eid.NewEIDService()

	identity, err := svc.ExchangeCode(context.Background(), "mock_oauth_code_123", "http://localhost:3000/callback")
	if err != nil {
		t.Fatalf("unexpected error during mock E-ID exchange: %v", err)
	}

	if identity.RegNumber != "AA90010111" {
		t.Errorf("expected RegNumber AA90010111, got %s", identity.RegNumber)
	}
	if identity.AuthMethod != eid.AuthMethodPKISignature {
		t.Errorf("expected AuthMethod PKI_DIGITAL_SIGNATURE")
	}
}

func TestEIDAppSessionFlow(t *testing.T) {
	t.Setenv("EID_MOCK_MODE", "true")
	service := eid.NewEIDService()
	started, err := service.StartByNationalID(context.Background(), "AA90010111", "")
	if err != nil || started.SessionID == "" || started.VerificationCode == "" {
		t.Fatalf("invalid start result: %#v, %v", started, err)
	}
	first, err := service.Poll(context.Background(), started.SessionID)
	if err != nil || first.State != "RUNNING" {
		t.Fatalf("expected RUNNING, got %#v, %v", first, err)
	}
	time.Sleep(1600 * time.Millisecond)
	complete, err := service.Poll(context.Background(), started.SessionID)
	if err != nil || complete.State != "COMPLETE" || complete.Identity == nil || !complete.Identity.VerifiedStatus {
		t.Fatalf("expected verified COMPLETE, got %#v, %v", complete, err)
	}
}

func TestEIDServiceAuthenticateWithMethod(t *testing.T) {
	svc := eid.NewEIDService()

	identity, err := svc.AuthenticateWithMethod(context.Background(), "AA90010111", "123456", eid.AuthMethodMobileOTP)
	if err != nil {
		t.Fatalf("unexpected error during OTP authentication: %v", err)
	}

	if identity.RegNumber != "AA90010111" {
		t.Errorf("expected RegNumber AA90010111, got %s", identity.RegNumber)
	}
	if identity.AuthMethod != eid.AuthMethodMobileOTP {
		t.Errorf("expected AuthMethod MOBILE_OTP")
	}
}
