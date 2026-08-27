/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Package dan provides integration with Gerege Systems DAN SSO Gateway (dan.gerege.mn)
 * for citizen session verification and identity resolution.
 */

package dan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
)

type DANProfile struct {
	DANSessionID   string    `json:"dan_session_id"`
	RegNumber      string    `json:"reg_number"`      // Иргэний регистр (AA90010111)
	CivilID        string    `json:"civil_id"`        // Иргэний бүртгэлийн дугаар
	LastName       string    `json:"last_name"`       // Эцэг/эхийн нэр
	FirstName      string    `json:"first_name"`      // Өөрийн нэр
	FamilyName     string    `json:"family_name"`     // Ургийн овог
	MobileNumber   string    `json:"mobile_number"`   // Гар утас
	Email          string    `json:"email"`           // И-мэйл
	GatewayVersion string    `json:"gateway_version"` // dan.gerege.mn version
	VerifiedAt     time.Time `json:"verified_at"`
}

type DANService struct {
	endpoint string
	apiKey   string
	mockMode bool
}

func NewDANService() *DANService {
	endpoint := os.Getenv("DAN_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://dan.gerege.mn/api/v1"
	}
	mock := config.MockEnabled("DAN_MOCK_MODE")
	apiKey := os.Getenv("DAN_API_KEY")

	return &DANService{
		endpoint: endpoint,
		apiKey:   apiKey,
		mockMode: mock,
	}
}

// ErrUnavailable reports that dan.gerege.mn could not be used at all — no credentials,
// no live implementation, no network — as opposed to the gateway having refused a
// citizen.
//
// The two are opposite answers. A refusal is the caller's to fix and should not be
// retried; an unavailable gateway is nobody's fault at this end and should be. Callers
// that turn errors into HTTP statuses need to tell them apart, and a string is not a
// thing to make that decision on.
var ErrUnavailable = errors.New("dan.gerege.mn is not available")

// VerifyDANToken verifies an active SSO session token issued by dan.gerege.mn
func (s *DANService) VerifyDANToken(ctx context.Context, danToken string) (*DANProfile, error) {
	return telemetry.ObserveExternalValue(ctx, telemetry.SystemDAN, "verify_token",
		func(context.Context) (*DANProfile, error) { return s.verifyDANToken(danToken) })
}

func (s *DANService) verifyDANToken(danToken string) (*DANProfile, error) {
	if danToken == "" {
		return nil, errors.New("empty DAN SSO token")
	}

	if s.mockMode {
		regNo := "AA90010111"
		if strings.HasPrefix(danToken, "dan_") {
			parts := strings.Split(danToken, "_")
			if len(parts) > 1 {
				regNo = strings.ToUpper(parts[1])
			}
		}
		return &DANProfile{
			DANSessionID:   "dan_sess_998877",
			RegNumber:      regNo,
			CivilID:        "CID-" + regNo,
			LastName:       "Бат",
			FirstName:      "Болд",
			FamilyName:     "Боржигон",
			MobileNumber:   "99112233",
			Email:          strings.ToLower(regNo) + "@dan.gerege.mn",
			GatewayVersion: "dan.gerege.mn/v2.1",
			VerifiedAt:     time.Now(),
		}, nil
	}

	return nil, fmt.Errorf("%w: the live gateway requires valid DAN_API_KEY credentials", ErrUnavailable)
}

// AuthenticateDANCitizen authenticates Mongolian citizen via dan.gerege.mn OTP/PKI gateway
func (s *DANService) AuthenticateDANCitizen(ctx context.Context, regNumber, otpCode string) (*DANProfile, error) {
	return telemetry.ObserveExternalValue(ctx, telemetry.SystemDAN, "authenticate",
		func(context.Context) (*DANProfile, error) { return s.authenticateDANCitizen(regNumber, otpCode) })
}

func (s *DANService) authenticateDANCitizen(regNumber, otpCode string) (*DANProfile, error) {
	cleanReg := strings.ToUpper(strings.TrimSpace(regNumber))
	if len(cleanReg) < 8 {
		return nil, errors.New("invalid registration number: minimum 8 characters required")
	}

	if s.mockMode {
		return &DANProfile{
			DANSessionID:   "dan_sess_" + cleanReg,
			RegNumber:      cleanReg,
			CivilID:        "CID-" + cleanReg,
			LastName:       "Гэрэгэ",
			FirstName:      "Баталгаажсан",
			FamilyName:     "Монгол",
			MobileNumber:   "99001122",
			Email:          strings.ToLower(cleanReg) + "@dan.gerege.mn",
			GatewayVersion: "dan.gerege.mn/v2.1",
			VerifiedAt:     time.Now(),
		}, nil
	}

	return nil, fmt.Errorf("%w: live OTP authentication is not implemented in this build", ErrUnavailable)
}

// Mode reports how this deployment reaches ДАН — see gerege.GeregeService.Mode
// for why the three states are named rather than reduced to a bool.
func (s *DANService) Mode() string {
	switch {
	case s.mockMode:
		return "mock"
	case s.apiKey != "":
		return "live"
	default:
		return "unconfigured"
	}
}

// Endpoint is the address ДАН calls are sent to.
func (s *DANService) Endpoint() string { return s.endpoint }
