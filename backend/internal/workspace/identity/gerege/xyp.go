/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Package gerege provides State Data Exchange Service (xyp.gerege.mn) integration
 * for citizen civil registration and company legal entity verification.
 */

package gerege

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
)

type CitizenInfo struct {
	RegNumber      string `json:"reg_number"`
	CivilID        string `json:"civil_id"`
	LastName       string `json:"last_name"`
	FirstName      string `json:"first_name"`
	Gender         string `json:"gender"`
	Address        string `json:"address"`
	PassportStatus string `json:"passport_status"`
	Verified       bool   `json:"verified"`
}

type CompanyInfo struct {
	CompanyReg   string `json:"company_reg"`
	Name         string `json:"name"`
	Executive    string `json:"executive"`
	Address      string `json:"address"`
	VatPayer     bool   `json:"vat_payer"`
	Status       string `json:"status"`
	FoundingDate string `json:"founding_date"`
}

type GeregeService struct {
	endpoint     string
	mockMode     bool
	clientID     string
	clientSecret string
	http         *http.Client
}

func NewGeregeService() *GeregeService {
	// The gateway serves its lookups at /v1/..., so the endpoint is an origin
	// rather than a path prefix. The previous default said /api/v1, which no
	// deployment can have depended on: there was no live implementation behind
	// it to reach an address with.
	endpoint := strings.TrimRight(os.Getenv("XYP_ENDPOINT"), "/")
	if endpoint == "" {
		endpoint = "https://xyp.gerege.mn"
	}

	return &GeregeService{
		endpoint:     endpoint,
		mockMode:     config.MockEnabled("XYP_MOCK_MODE"),
		clientID:     os.Getenv("XYP_CLIENT_ID"),
		clientSecret: os.Getenv("XYP_CLIENT_SECRET"),
		// A timeout rather than none: a registry that stops answering must not
		// hold a request handler open until the client gives up first.
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

// lookup posts one query and decodes the reply into out.
//
// One function for both registries because the two endpoints differ only in
// their path and the shape they answer with — the credential, the framing and
// every failure are the same, and writing them twice is how the two drift.
func (s *GeregeService) lookup(ctx context.Context, path string, request, out any) error {
	if s.clientID == "" || s.clientSecret == "" {
		return fmt.Errorf("XYP live service at %s needs XYP_CLIENT_ID and XYP_CLIENT_SECRET", s.endpoint)
	}

	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.SetBasicAuth(s.clientID, s.clientSecret)

	response, err := s.http.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("XYP %s: %w", path, err)
	}
	// The repo's idiom for a close nobody can act on: an ignored error stated
	// rather than an unchecked one, which is what errcheck is for.
	defer func() { _ = response.Body.Close() }()

	// The status is named in the error because the three that happen mean
	// different things to whoever has to fix it: 401 is the credential, 403 is
	// a scope the client was not issued, 429 is the rate limit.
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("XYP %s: the registry answered %d", path, response.StatusCode)
	}
	if err := json.NewDecoder(response.Body).Decode(out); err != nil {
		return fmt.Errorf("XYP %s: decode the reply: %w", path, err)
	}
	return nil
}

// GetCitizenInfo queries citizen data from ХУР (xyp.gerege.mn)
//
// Timed as an external call even while the live implementation is missing: what
// the histogram then reports is that every citizen lookup on this deployment
// fails immediately, which is the truth an operator needs and would otherwise
// have to read from a handler's error rate.
func (s *GeregeService) GetCitizenInfo(ctx context.Context, regNumber string) (*CitizenInfo, error) {
	return telemetry.ObserveExternalValue(ctx, telemetry.SystemXYP, "citizen_query",
		func(ctx context.Context) (*CitizenInfo, error) { return s.getCitizenInfo(ctx, regNumber) })
}

func (s *GeregeService) getCitizenInfo(ctx context.Context, regNumber string) (*CitizenInfo, error) {
	if regNumber == "" {
		return nil, errors.New("empty registration number")
	}

	cleanReg := strings.ToUpper(strings.TrimSpace(regNumber))
	if len(cleanReg) < 8 {
		return nil, errors.New("invalid registration number format: must be at least 8 characters")
	}

	if s.mockMode {
		return &CitizenInfo{
			RegNumber:      cleanReg,
			CivilID:        "CID-" + cleanReg,
			LastName:       "Бат",
			FirstName:      "Болд",
			Gender:         "Male",
			Address:        "Улаанбаатар хот, Сүхбаатар дүүрэг, 1-р хороо",
			PassportStatus: "ACTIVE",
			Verified:       true,
		}, nil
	}

	var reply struct {
		Found   bool `json:"found"`
		Citizen struct {
			RegNo       string `json:"reg_no"`
			CivilID     string `json:"civil_id"`
			LastName    string `json:"last_name"`
			FirstName   string `json:"first_name"`
			Gender      string `json:"gender"`
			PassportNum string `json:"passport_num"`
			Address     string `json:"passport_address"`
		} `json:"citizen"`
	}
	if err := s.lookup(ctx, "/v1/citizen/lookup", map[string]string{"reg_no": cleanReg}, &reply); err != nil {
		return nil, err
	}
	if !reply.Found {
		return nil, fmt.Errorf("no citizen is registered under %s", cleanReg)
	}

	// Verified reports that the registry answered for this person, which is the
	// only thing this call can establish. It is not a claim about the passport.
	return &CitizenInfo{
		RegNumber:      reply.Citizen.RegNo,
		CivilID:        reply.Citizen.CivilID,
		LastName:       reply.Citizen.LastName,
		FirstName:      reply.Citizen.FirstName,
		Gender:         reply.Citizen.Gender,
		Address:        reply.Citizen.Address,
		PassportStatus: reply.Citizen.PassportNum,
		Verified:       true,
	}, nil
}

// GetCompanyInfo queries legal entity data from ХУР (xyp.gerege.mn)
func (s *GeregeService) GetCompanyInfo(ctx context.Context, companyReg string) (*CompanyInfo, error) {
	return telemetry.ObserveExternalValue(ctx, telemetry.SystemXYP, "company_query",
		func(ctx context.Context) (*CompanyInfo, error) { return s.getCompanyInfo(ctx, companyReg) })
}

func (s *GeregeService) getCompanyInfo(ctx context.Context, companyReg string) (*CompanyInfo, error) {
	if companyReg == "" {
		return nil, errors.New("empty company registration number")
	}

	cleanReg := strings.TrimSpace(companyReg)

	if s.mockMode {
		return &CompanyInfo{
			CompanyReg:   cleanReg,
			Name:         "Гэрэгэ Системс ХХК",
			Executive:    "Ц.Эрдэнэбат",
			Address:      "Улаанбаатар хот, Хан-Уул дүүрэг, Гэрэгэ тауэр 5 давхар",
			VatPayer:     true,
			Status:       "ACTIVE",
			FoundingDate: "2018-05-15",
		}, nil
	}

	var reply struct {
		Found        bool `json:"found"`
		Organization struct {
			RegNo   string `json:"reg_no"`
			Name    string `json:"name"`
			Type    string `json:"type"`
			CEO     string `json:"ceo"`
			Address string `json:"address"`
		} `json:"organization"`
	}
	if err := s.lookup(ctx, "/v1/org/lookup", map[string]string{"reg_no": cleanReg}, &reply); err != nil {
		return nil, err
	}
	if !reply.Found {
		return nil, fmt.Errorf("no legal entity is registered under %s", cleanReg)
	}

	// VatPayer, Status and FoundingDate stay at their zero values: the gateway
	// does not carry them, and a false that reads as "not a VAT payer" would be
	// worse than an absent one. Status holds the entity's legal form, which is
	// the nearest thing the registry does answer with.
	return &CompanyInfo{
		CompanyReg: reply.Organization.RegNo,
		Name:       reply.Organization.Name,
		Executive:  reply.Organization.CEO,
		Address:    reply.Organization.Address,
		Status:     reply.Organization.Type,
	}, nil
}

// Mode reports how this deployment reaches ХУР, for the screen that has to say
// so out loud.
//
// It is a string rather than a bool because the three states are genuinely
// different: "live" means a credential is held and a real registry answers,
// "mock" means the fixtures answer and nothing about the reply is authoritative,
// and "unconfigured" means every lookup will fail. A screen that showed only
// "connected" would report the middle one as the first.
func (s *GeregeService) Mode() string {
	switch {
	case s.mockMode:
		return "mock"
	case s.clientID != "" && s.clientSecret != "":
		return "live"
	default:
		return "unconfigured"
	}
}

// Endpoint is the address lookups are sent to.
func (s *GeregeService) Endpoint() string { return s.endpoint }
