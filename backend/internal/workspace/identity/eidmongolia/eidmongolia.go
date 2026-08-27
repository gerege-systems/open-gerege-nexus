/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Package eidmongolia is the single surface for everything eID Mongolia:
 * authentication, the citizen's PKI dashboard, organisation representation and
 * qualified PDF signing.
 *
 * All of it is delegated to internal/kernel/eidrp and internal/kernel/eidsign
 * rather than reimplemented here. Those two carry the parts that are easy to
 * get subtly wrong and expensive to discover:
 *
 *   - PAdES-T output — an RFC 3161 timestamp plus an eidmongolia.mn/verify
 *     page — via eID's stamp endpoint, with a server Document-Signer as the
 *     fallback when that endpoint is unavailable.
 *   - Session ownership checks, so one citizen cannot poll or download
 *     another's ceremony.
 *   - An SSRF-hardened fetcher for the signature and stamp images, whose URLs
 *     come from the user and would otherwise reach loopback and private ranges.
 *
 * This package's own job is small and deliberately so: adapt the library to
 * this deployment (durable state in Postgres instead of Redis, configuration
 * from the environment) and present one type the app modules can hold.
 */
package eidmongolia

import (
	"context"
	"os"
	"strings"
	"time"

	coreeid "github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/eidrp"
	"github.com/jackc/pgx/v5/pgxpool"

	signuc "github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/eidsign"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
)

// Session states, as the shared usecase reports them. The browser polls for
// exactly these strings.
const (
	StateRunning   = "running"
	StateCompleted = "completed"
	StateFailed    = "failed"
	StateExpired   = "expired"
	StateRejected  = "rejected"
)

// Service is the whole eID surface.
type Service struct {
	db   *pgxpool.Pool
	rp   coreeid.Client
	sign signuc.Usecase
	mock bool

	displayText string
}

// Representation is an organisation a citizen may act for.
//
// Deliberately not an alias for the library's type: that one carries no JSON
// tags, so serialising it would put Go field names on the wire and the signing
// view reads snake_case. Mapping here also keeps a library field rename from
// silently changing this deployment's API.
type Representation struct {
	OrgEtsi     string     `json:"org_etsi"`
	OrgRegister string     `json:"org_register"`
	OrgName     string     `json:"org_name"`
	OrgNameEn   string     `json:"org_name_en,omitempty"`
	Role        string     `json:"role,omitempty"`
	RightType   string     `json:"right_type,omitempty"`
	ValidFrom   *time.Time `json:"valid_from,omitempty"`
	ValidTo     *time.Time `json:"valid_to,omitempty"`
}

// InitResult is a started signing ceremony.
type InitResult = signuc.InitResult

// DownloadResult is a finished, signed document.
type DownloadResult = signuc.DownloadResult

// New builds the service from the environment.
//
// The relying-party identity is shared with authentication — one registration
// carries both the authentication and the SIGNATURE permission — so signing
// adds only its own overrides:
//
//	EID_SIGN_MOCK_MODE       serve canned ceremonies instead of calling eID
//	EID_SIGN_CERT_LEVEL      minimum certificate level, defaults to QUALIFIED
//	EID_SIGN_DISPLAY_TEXT    prompt shown above the PIN2 entry
//	EID_SIGN_SIGNER_CERT_PEM server Document-Signer certificate (PEM)
//	EID_SIGN_SIGNER_KEY_PEM  its ECDSA private key (PEM)
//
// The Document-Signer is only reached when eID's own stamp endpoint fails, so
// a deployment without one still signs — it just loses the fallback. The
// library reports that as an error at download time rather than at boot, which
// is the right trade: a missing fallback should not stop a citizen signing.
func New(db *pgxpool.Pool) (*Service, error) {
	certLevel := firstNonEmpty(os.Getenv("EID_SIGN_CERT_LEVEL"), "QUALIFIED")

	// Certificate level differs from authentication's on purpose. Sign-in
	// accepts ADVANCED because that is what most citizens hold and requiring
	// more would lock them out; a signature accepting ADVANCED would silently
	// produce something that is not a qualified electronic signature.
	rp := coreeid.NewClient(
		os.Getenv("EID_BASE_URL"),
		os.Getenv("EID_RP_UUID"),
		firstNonEmpty(os.Getenv("EID_RP_NAME"), config.BrandName()),
		os.Getenv("EID_RP_SECRET"),
		certLevel,
	)

	signer, err := signuc.NewUsecase(&stateStore{db: db}, signuc.Config{
		V3BaseURL:     strings.TrimSuffix(firstNonEmpty(os.Getenv("EID_SIGN_BASE_URL"), os.Getenv("EID_BASE_URL"), "https://eidmongolia.mn/v3"), "/v3"),
		RPUUID:        os.Getenv("EID_RP_UUID"),
		RPName:        firstNonEmpty(os.Getenv("EID_RP_NAME"), config.BrandName()),
		APISecret:     os.Getenv("EID_RP_SECRET"),
		SignerCertPEM: pemFromEnv("EID_SIGN_SIGNER_CERT_PEM", "EID_SIGN_SIGNER_CERT_FILE"),
		SignerKeyPEM:  pemFromEnv("EID_SIGN_SIGNER_KEY_PEM", "EID_SIGN_SIGNER_KEY_FILE"),
		IsProduction:  config.IsProduction(),
	})
	if err != nil {
		return nil, err
	}

	return &Service{
		db:          db,
		rp:          rp,
		sign:        signer,
		mock:        config.MockEnabled("EID_SIGN_MOCK_MODE"),
		displayText: strings.TrimSpace(os.Getenv("EID_SIGN_DISPLAY_TEXT")),
	}, nil
}

// Enabled reports whether signing can reach a live relying party.
func (s *Service) Enabled() bool {
	return s.mock || (os.Getenv("EID_RP_UUID") != "" && os.Getenv("EID_RP_SECRET") != "")
}

// Mock reports whether canned ceremonies are being served.
func (s *Service) Mock() bool { return s.mock }

// ─── Signing ─────────────────────────────────────────────────────────────────

// SignPDF starts a ceremony over a PDF and pushes the PIN2 prompt to the
// citizen's phone.
//
// onBehalfOfOrg is an organisation ETSI id (NTRMN-<register>) when the citizen
// signs in a company's name; eID checks the representation right against the
// registry when the ceremony starts. The signature is still made with the
// citizen's personal certificate — this is a delegation record, not a seal.
//
// signatureURL and stampURL overlay the citizen's saved signature image and
// the organisation's stamp onto the last page. They are user-supplied URLs and
// the library fetches them through an SSRF-hardened client.
func (s *Service) SignPDF(ctx context.Context, in SignRequest) (InitResult, error) {
	if s.mock {
		return s.mockStart(ctx, in)
	}
	return s.sign.Init(ctx, in.RegNo, in.FullName, in.FileName, in.PDF,
		in.OnBehalfOfOrg, in.SignatureURL, in.StampURL)
}

// SignRequest is one PDF signing ceremony.
type SignRequest struct {
	// RegNo owns the ceremony. Poll and Download are checked against it, so a
	// citizen cannot reach another's session by guessing an id.
	RegNo         string
	FullName      string
	FileName      string
	PDF           []byte
	OnBehalfOfOrg string
	SignatureURL  string
	StampURL      string
}

// SignDigest starts a ceremony over an arbitrary SHA-256 digest rather than a
// PDF — for approving a transaction, where the citizen confirms a display text
// and the signature covers a canonical hash of the payload.
func (s *Service) SignDigest(ctx context.Context, regNo, fullName, digestHex, displayText, docName string) (InitResult, error) {
	if displayText == "" {
		displayText = s.displayText
	}
	return s.sign.InitDigest(ctx, regNo, fullName, digestHex, displayText, docName)
}

// PollSign returns the ceremony's state: running, completed, failed, expired
// or rejected.
func (s *Service) PollSign(ctx context.Context, ownerRegNo, sessionID string) (string, error) {
	if s.mock {
		return s.mockPoll(ctx, sessionID)
	}
	return s.sign.Poll(ctx, ownerRegNo, sessionID)
}

// DownloadSigned returns the PAdES-signed document.
func (s *Service) DownloadSigned(ctx context.Context, ownerRegNo, sessionID string) (DownloadResult, error) {
	if s.mock {
		return s.mockDownload(ctx, sessionID)
	}
	return s.sign.Download(ctx, ownerRegNo, sessionID)
}

// VerifiedDigest returns the digest a completed ceremony actually signed,
// base64. It is how a caller confirms that what was approved is what it sent.
func (s *Service) VerifiedDigest(ctx context.Context, ownerRegNo, sessionID string) (string, error) {
	return s.sign.VerifiedDigest(ctx, ownerRegNo, sessionID)
}

// ─── Representation and PKI ──────────────────────────────────────────────────

// Representations lists the organisations a citizen may currently sign for.
// Rights are read live from the registry rather than from a certificate,
// because a director who resigned yesterday still holds yesterday's
// certificate.
func (s *Service) Representations(ctx context.Context, personEtsi string) ([]Representation, error) {
	if s.mock {
		return []Representation{{
			OrgEtsi: "NTRMN-1234567", OrgRegister: "1234567",
			OrgName: "Demo Corporation", RightType: "ADMIN",
		}}, nil
	}
	found, err := s.rp.Representations(ctx, personEtsi)
	if err != nil {
		return nil, err
	}
	list := make([]Representation, 0, len(found))
	for _, rep := range found {
		list = append(list, Representation{
			OrgEtsi: rep.OrgEtsi, OrgRegister: rep.OrgRegister,
			OrgName: rep.OrgName, OrgNameEn: rep.OrgNameEn,
			Role: rep.Role, RightType: rep.RightType,
			ValidFrom: rep.ValidFrom, ValidTo: rep.ValidTo,
		})
	}
	return list, nil
}

// ─── Identifiers ─────────────────────────────────────────────────────────────

// PersonEtsi builds the ETSI EN 319 412-1 identifier eID keys citizens by.
func PersonEtsi(id string) string {
	s := strings.TrimSpace(id)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToUpper(s), "PNO") {
		return strings.ToUpper(s)
	}
	return "PNOMN-" + s
}

// OrgEtsi builds the organisation identifier, NTRMN-<registrationNumber>.
func OrgEtsi(register string) string {
	s := strings.TrimSpace(register)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToUpper(s), "NTR") {
		return strings.ToUpper(s)
	}
	return "NTRMN-" + s
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// pemFromEnv reads a PEM blob from either an inline variable or a file path.
// A container orchestrator usually mounts a key as a file, while a plain
// docker-compose deployment passes it inline; supporting both avoids forcing
// one shape of secret management.
func pemFromEnv(inlineVar, fileVar string) []byte {
	if inline := strings.TrimSpace(os.Getenv(inlineVar)); inline != "" {
		// Escaped newlines survive a round trip through most secret stores far
		// better than real ones do.
		return []byte(strings.ReplaceAll(inline, `\n`, "\n"))
	}
	if path := strings.TrimSpace(os.Getenv(fileVar)); path != "" {
		// #nosec G304 -- the path is an environment variable set by whoever
		// operates the deployment, pointing at their own signing certificate.
		// A caller cannot reach it: nothing in a request contributes to it.
		if raw, err := os.ReadFile(path); err == nil {
			return raw
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// PollWindow is how long a status request may block waiting on eID. It sits
// under the API's write deadline: a request that outlasts it is closed with no
// response written, and the citizen gets the proxy's error page instead.
const PollWindow = 20 * time.Second
