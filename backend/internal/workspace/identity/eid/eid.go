/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Package eid provides integration with National Digital Identity (eidmongolia.mn & developer.gerege.mn)
 * supporting PKI, Mobile OTP, Bank SSO, and Biometric authentication.
 */

package eid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/credentials"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/geregecore"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	coreeid "github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/eidrp"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
)

// PollWindow is how long the relying party holds a session poll open before it
// answers RUNNING. It is the longest a /auth/eid/poll request legitimately
// takes, so the API's write deadline has to outlast it — see cmd/api. When it
// did not, the connection was closed with no response written and the citizen
// got nginx's 502 page on every check that was not answered immediately.
const PollWindow = 25 * time.Second

// AuthMethod represents official E-ID Mongolia authentication channels
type AuthMethod string

const (
	AuthMethodPKISignature AuthMethod = "PKI_DIGITAL_SIGNATURE" // Тоон гарын үсэг
	AuthMethodMobileOTP    AuthMethod = "MOBILE_OTP"            // Нэг удаагийн нууц код
	AuthMethodBankSSO      AuthMethod = "BANK_SSO"              // Банкны системээр
	AuthMethodBiometric    AuthMethod = "BIOMETRIC_FACE"        // Царай танилт
)

// EIDIdentity matches official DAN / E-ID Mongolia user profile schema
type EIDIdentity struct {
	CivilID    string `json:"civil_id"`    // Иргэний бүртгэлийн дугаар
	RegNumber  string `json:"reg_number"`  // Регистрийн дугаар (e.g. AA90010111)
	FirstName  string `json:"first_name"`  // Өөрийн нэр
	LastName   string `json:"last_name"`   // Эцэг/Эхийн нэр
	FamilyName string `json:"family_name"` // Ургийн овог
	// The Latin transliterations, as the passport spells them. eID began
	// returning these with the person block; there is no Latin clan name,
	// because the certificate does not carry one.
	FirstNameEn string `json:"first_name_en,omitempty"`
	LastNameEn  string `json:"last_name_en,omitempty"`
	// BirthDate is a date and nothing more: YYYY-MM-DD. Neither this nor
	// Gender is in the certificate, which is why the person block carries them.
	BirthDate       string     `json:"birth_date,omitempty"`
	Gender          string     `json:"gender"`          // Хүйс: M | F | X
	Email           string     `json:"email"`           // И-мэйл хаяг
	Phone           string     `json:"phone"`           // Утасны дугаар
	AuthMethod      AuthMethod `json:"auth_method"`     // Танилт хийсэн арга
	SignatureHash   string     `json:"signature_hash"`  // Тоон гарын үсгийн хеш
	VerifiedStatus  bool       `json:"verified_status"` // Төрийн сангийн баталгаажуулалт
	AuthenticatedAt time.Time  `json:"authenticated_at"`
	// CertificateSerial and CertificateIssuer identify the certificate the citizen
	// approved with. eID returns them only on a completed session, and they are
	// the durable reference an e-signature record should keep: a session id says
	// an approval happened, a certificate says whose key gave it.
	CertificateSerial string `json:"certificate_serial,omitempty"`
	CertificateIssuer string `json:"certificate_issuer,omitempty"`
	// GeID is the citizen's number in the Gerege register (core.gerege.mn's
	// `users.id`), which eID returns as `person.geID` on a completed session.
	//
	// It is what lets this platform line a person up with the rest of the
	// ecosystem — wallet, POS, kiosk — by something stable rather than by a
	// Cyrillic registration number. eID's own documentation is explicit that an
	// RP must not require it: a citizen whose record predates the Core backfill
	// has none, and the field is then absent rather than zero.
	GeID int64 `json:"ge_id,omitempty"`
}

type Provider interface {
	GetAuthorizeURL(redirectURI, state string) string
	ExchangeCode(ctx context.Context, code, redirectURI string) (*EIDIdentity, error)
	AuthenticateWithMethod(ctx context.Context, regNumber, otpCode string, method AuthMethod) (*EIDIdentity, error)
}

type EIDService struct {
	clientID     string
	clientSecret string
	authorizeURL string
	tokenURL     string
	userInfoURL  string
	mockMode     bool
	httpClient   *http.Client
	rpClient     coreeid.Client
	mockMu       sync.Mutex
	mockSessions map[string]mockSession
}

type StartResult struct {
	SessionID        string `json:"session_id"`
	DeviceLinkURL    string `json:"device_link_url,omitempty"`
	VerificationCode string `json:"verification_code"`
	ExpiresAt        string `json:"expires_at"`
}

type PollResult struct {
	State    string       `json:"state"`
	Identity *EIDIdentity `json:"identity,omitempty"`
}

type mockSession struct {
	created  time.Time
	identity EIDIdentity
}

func NewEIDService() *EIDService {
	mock := config.MockEnabled("EID_MOCK_MODE")
	clientID := os.Getenv("EID_CLIENT_ID")
	if clientID == "" {
		// Legacy compatibility name, kept deliberately through the Gerege Nexus
		// rebrand: this is the client ID registered with the identity provider,
		// not a display string. Renaming it here would not rename it there, and
		// the mismatch would fail every sign-in. Change it only alongside a new
		// registration, via EID_CLIENT_ID.
		clientID = "gerege-open-erp-client"
	}
	clientSecret := os.Getenv("EID_CLIENT_SECRET")

	authURL := os.Getenv("EID_AUTHORIZE_URL")
	if authURL == "" {
		authURL = "https://sso.gov.mn/oauth2/authorize"
	}
	tokenURL := os.Getenv("EID_TOKEN_URL")
	if tokenURL == "" {
		// #nosec G101 -- this is the gateway's public OAuth2 token endpoint,
		// flagged because the variable is named for the endpoint it calls. The
		// credentials that go to it are EID_RP_UUID and EID_RP_SECRET.
		tokenURL = "https://sso.gov.mn/oauth2/token"
	}
	userURL := os.Getenv("EID_USERINFO_URL")
	if userURL == "" {
		userURL = "https://sso.gov.mn/oauth2/userinfo"
	}

	return &EIDService{
		clientID:     clientID,
		clientSecret: clientSecret,
		authorizeURL: authURL,
		tokenURL:     tokenURL,
		userInfoURL:  userURL,
		mockMode:     mock,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
		rpClient: coreeid.NewClient(
			os.Getenv("EID_BASE_URL"), os.Getenv("EID_RP_UUID"),
			valueOr(os.Getenv("EID_RP_NAME"), "Gerege Nexus"), os.Getenv("EID_RP_SECRET"),
			valueOr(os.Getenv("EID_CERT_LEVEL"), "ADVANCED"),
		),
		mockSessions: make(map[string]mockSession),
	}
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// StartDeviceLink starts the same QR/App2App contract used by Gerege Platform.
func (s *EIDService) StartDeviceLink(ctx context.Context, callbackURL string) (*StartResult, error) {
	return telemetry.ObserveExternalValue(ctx, telemetry.SystemEID, "start_device_link",
		func(ctx context.Context) (*StartResult, error) { return s.startDeviceLink(ctx, callbackURL) })
}

func (s *EIDService) startDeviceLink(ctx context.Context, callbackURL string) (*StartResult, error) {
	if s.mockMode {
		return s.startMock("", true), nil
	}
	started, err := s.rpClient.QRInitiate(ctx, valueOr(os.Getenv("EID_DISPLAY_TEXT"), "Gerege Nexus-д нэвтрэх"), callbackURL, "")
	if err != nil {
		return nil, err
	}
	return normalizeStart(started), nil
}

// StartByNationalID pushes an approval request to the citizen's eID Mongolia app.
func (s *EIDService) StartByNationalID(ctx context.Context, nationalID, callbackURL string) (*StartResult, error) {
	return telemetry.ObserveExternalValue(ctx, telemetry.SystemEID, "start_by_national_id",
		func(ctx context.Context) (*StartResult, error) {
			return s.startByNationalID(ctx, nationalID, callbackURL)
		})
}

func (s *EIDService) startByNationalID(ctx context.Context, nationalID, callbackURL string) (*StartResult, error) {
	nationalID = strings.ToUpper(strings.TrimSpace(nationalID))
	if len(nationalID) < 8 {
		return nil, errors.New("invalid registration number")
	}
	if s.mockMode {
		return s.startMock(nationalID, false), nil
	}
	started, err := s.rpClient.Initiate(ctx, nationalID, valueOr(os.Getenv("EID_DISPLAY_TEXT"), "Gerege Nexus-д нэвтрэх"), callbackURL)
	if err != nil {
		return nil, err
	}
	return normalizeStart(started), nil
}

// StartSignature pushes an approval request that names what is being signed, so
// the citizen reads the document on their own device rather than a generic
// sign-in prompt.
//
// eID has no separate document-signing endpoint: the approval a citizen gives
// with their own credentials *is* the signature, and the display text is the only
// thing that tells them what they are approving. Everything after this — session
// id, verification code, polling — is the sign-in flow's, which is why callers
// finish with Poll.
func (s *EIDService) StartSignature(ctx context.Context, nationalID, displayText, callbackURL string) (*StartResult, error) {
	return telemetry.ObserveExternalValue(ctx, telemetry.SystemEID, "start_signature",
		func(ctx context.Context) (*StartResult, error) {
			return s.startSignature(ctx, nationalID, displayText, callbackURL)
		})
}

func (s *EIDService) startSignature(ctx context.Context, nationalID, displayText, callbackURL string) (*StartResult, error) {
	nationalID = strings.ToUpper(strings.TrimSpace(nationalID))
	if len(nationalID) < 8 {
		return nil, errors.New("invalid registration number")
	}
	displayText = strings.TrimSpace(displayText)
	if displayText == "" {
		return nil, errors.New("display text is required: the citizen has to see what they are approving")
	}
	if s.mockMode {
		return s.startMock(nationalID, false), nil
	}
	started, err := s.rpClient.Initiate(ctx, nationalID, displayText, callbackURL)
	if err != nil {
		return nil, err
	}
	return normalizeStart(started), nil
}

// normalizeStart passes the relying party's own deadline through, and passes
// nothing through when it gives none.
//
// It used to invent "two minutes from now" in that case. The relying party
// keeps a push session alive far longer — measured at over nine minutes and
// still RUNNING — so the invented figure was not a deadline, it was a cutoff:
// a citizen who took longer than two minutes to unlock a phone, find the
// notification and enter a PIN had the browser abandon a session eID was
// still waiting on. The relying party's own EXPIRED state is what ends a wait.
func normalizeStart(started *coreeid.StartResult) *StartResult {
	return &StartResult{SessionID: started.SessionID, DeviceLinkURL: started.DeviceLinkURL, VerificationCode: started.VerificationCode, ExpiresAt: started.ExpiresAt}
}

func (s *EIDService) startMock(nationalID string, deviceLink bool) *StartResult {
	if nationalID == "" {
		nationalID = "AA90010111"
	}
	sessionID := fmt.Sprintf("mock-%d", time.Now().UnixNano())
	s.mockMu.Lock()
	s.mockSessions[sessionID] = mockSession{created: time.Now(), identity: EIDIdentity{CivilID: "CID-" + nationalID, RegNumber: nationalID, FirstName: "Баталгаажсан", LastName: "Иргэн", Email: strings.ToLower(nationalID) + "@eidmongolia.mn", AuthMethod: AuthMethodPKISignature, VerifiedStatus: true, AuthenticatedAt: time.Now()}}
	s.mockMu.Unlock()
	link := ""
	if deviceLink {
		link = sessionID
	}
	return &StartResult{SessionID: sessionID, DeviceLinkURL: link, VerificationCode: "2026", ExpiresAt: time.Now().Add(2 * time.Minute).Format(time.RFC3339)}
}

// Poll long-polls the authoritative RP session and returns a normalized state.
//
// Its latency is mostly the citizen reaching for their phone rather than eID
// being slow, which is why the external histogram's buckets run out to two
// minutes: a p99 of ninety seconds on operation="poll" is a normal sign-in, not
// an incident.
func (s *EIDService) Poll(ctx context.Context, sessionID string) (*PollResult, error) {
	result, err := telemetry.ObserveExternalValue(ctx, telemetry.SystemEID, "poll",
		func(ctx context.Context) (*PollResult, error) { return s.poll(ctx, sessionID) })
	// All three poll routes answer a failure with the same fixed sentence, and
	// nothing wrote down why. What the log held was `status:502` and a request
	// id — eID's own reason for refusing the check was thrown away at the point
	// it was known, which left the 502s in production undiagnosable.
	if err != nil {
		slog.Warn("eID session poll failed", "error", err)
	}
	return result, err
}

func (s *EIDService) poll(ctx context.Context, sessionID string) (*PollResult, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, errors.New("session_id is required")
	}
	if s.mockMode {
		s.mockMu.Lock()
		session, ok := s.mockSessions[sessionID]
		s.mockMu.Unlock()
		if !ok {
			return &PollResult{State: coreeid.StateExpired}, nil
		}
		if time.Since(session.created) < 1500*time.Millisecond {
			return &PollResult{State: coreeid.StateRunning}, nil
		}
		identity := session.identity
		return &PollResult{State: coreeid.StateComplete, Identity: &identity}, nil
	}
	session, err := s.rpClient.Session(ctx, sessionID, int(PollWindow/time.Millisecond))
	if err != nil {
		return nil, err
	}
	result := &PollResult{State: session.State}
	if session.State == coreeid.StateComplete && session.Identity != nil {
		id := session.Identity
		result.Identity = &EIDIdentity{
			CivilID: id.CivilID, RegNumber: id.NationalID,
			// Three names, three things: FirstName is the citizen's own name,
			// LastName the patronymic, FamilyName the clan name. eID says which
			// is which now, so this stops guessing.
			FirstName: id.FirstName, LastName: id.LastName, FamilyName: id.FamilyName,
			FirstNameEn: id.FirstNameEn, LastNameEn: id.LastNameEn,
			BirthDate: id.BirthDate, Gender: id.Gender, GeID: id.GeID,
			AuthMethod: AuthMethodPKISignature, VerifiedStatus: true, AuthenticatedAt: time.Now(),
		}
		resolveGeID(ctx, result.Identity)
		// The certificate is optional — a login does not stop when it cannot be
		// parsed — but when it is there it is what an e-signature record anchors on.
		if cert := id.Certificate; cert != nil {
			result.Identity.CertificateSerial = cert.Serial
			result.Identity.CertificateIssuer = cert.Issuer
		}
	}
	return result, nil
}

// GetAuthorizeURL constructs official OAuth2 authorization link for eidmongolia.mn / sso.gov.mn
func (s *EIDService) GetAuthorizeURL(redirectURI, state string) string {
	v := url.Values{}
	v.Set("response_type", "code")
	v.Set("client_id", s.clientID)
	v.Set("redirect_uri", redirectURI)
	v.Set("scope", "openid profile regnum civil_id phone email")
	v.Set("state", state)
	return fmt.Sprintf("%s?%s", s.authorizeURL, v.Encode())
}

// ExchangeCode exchanges OAuth2 authorization code for E-ID Identity profile
func (s *EIDService) ExchangeCode(ctx context.Context, code, redirectURI string) (*EIDIdentity, error) {
	return telemetry.ObserveExternalValue(ctx, telemetry.SystemEID, "exchange_code",
		func(ctx context.Context) (*EIDIdentity, error) { return s.exchangeCode(ctx, code, redirectURI) })
}

func (s *EIDService) exchangeCode(ctx context.Context, code, redirectURI string) (*EIDIdentity, error) {
	if code == "" {
		return nil, errors.New("empty authorization code")
	}

	if s.mockMode {
		return &EIDIdentity{
			CivilID:         "CID-99887766",
			RegNumber:       "AA90010111",
			FirstName:       "Болд",
			LastName:        "Бат",
			FamilyName:      "Боржигон",
			Gender:          "MALE",
			Email:           "bat.bold@eidmongolia.mn",
			Phone:           "99112233",
			AuthMethod:      AuthMethodPKISignature,
			SignatureHash:   "sha256_mock_pki_signature_eidmongolia",
			VerifiedStatus:  true,
			AuthenticatedAt: time.Now(),
		}, nil
	}

	// Live OAuth2 Token Request to SSO / E-ID Mongolia
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", s.clientID)
	data.Set("client_secret", s.clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", s.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("E-ID Mongolia token exchange failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("E-ID token error (%d): %s", resp.StatusCode, string(body))
	}

	var tokenRes struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenRes); err != nil {
		return nil, err
	}

	return s.fetchUserInfo(ctx, tokenRes.AccessToken)
}

func (s *EIDService) AuthenticateWithMethod(ctx context.Context, regNumber, otpCode string, method AuthMethod) (*EIDIdentity, error) {
	if len(regNumber) < 8 {
		return nil, errors.New("invalid registration number: minimum 8 characters required")
	}

	if method == "" {
		method = AuthMethodMobileOTP
	}

	if s.mockMode {
		return &EIDIdentity{
			CivilID:         "CID-" + regNumber,
			RegNumber:       strings.ToUpper(regNumber),
			FirstName:       "Баталгаажсан",
			LastName:        "Иргэн",
			FamilyName:      "Монгол",
			Gender:          "MALE",
			Email:           strings.ToLower(regNumber) + "@eidmongolia.mn",
			Phone:           "99001122",
			AuthMethod:      method,
			SignatureHash:   "pki_signed_token_" + regNumber,
			VerifiedStatus:  true,
			AuthenticatedAt: time.Now(),
		}, nil
	}

	return nil, fmt.Errorf("E-ID Mongolia OTP verification requires production SSO credentials")
}

func (s *EIDService) fetchUserInfo(ctx context.Context, accessToken string) (*EIDIdentity, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.userInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var raw struct {
		Sub       string `json:"sub"`
		RegNum    string `json:"regnum"`
		CivilID   string `json:"civil_id"`
		GivenName string `json:"given_name"`
		Family    string `json:"family_name"`
		Email     string `json:"email"`
		Phone     string `json:"phone_number"`
		// Spelled as eID spells it. Absent for a citizen the Core register has
		// not been matched to, which is an ordinary answer and not an error.
		GeID int64 `json:"geID"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	return &EIDIdentity{
		CivilID:         raw.CivilID,
		RegNumber:       raw.RegNum,
		FirstName:       raw.GivenName,
		LastName:        raw.Family,
		Email:           raw.Email,
		Phone:           raw.Phone,
		GeID:            raw.GeID,
		AuthMethod:      AuthMethodPKISignature,
		VerifiedStatus:  true,
		AuthenticatedAt: time.Now(),
	}, nil
}

// Mode reports how this deployment reaches eID Mongolia — see
// gerege.GeregeService.Mode for why the three states are named rather than
// reduced to a bool.
func (s *EIDService) Mode() string {
	switch {
	case s.mockMode:
		return "mock"
	case s.clientSecret != "":
		return "live"
	default:
		return "unconfigured"
	}
}

// Endpoint is where a sign-in is started.
func (s *EIDService) Endpoint() string { return s.authorizeURL }

// resolveGeID fills in the citizen's Gerege number when the session did not
// carry it.
//
// eID returns `person.geID` on a completed session and internal/kernel/eidrp
// reads it, so this normally does nothing. It is what answers for the two cases
// that remain: an eID that has not been upgraded to send the field, and a
// citizen the Gerege register has not been matched to. In both, the number is
// fetched from the register it comes from: geID *is*
// core.gerege.mn's `users.id`, by the eID platform's own definition, and a
// lookup by registration number returns the same value eID would have handed
// over.
//
// Best effort on purpose. A deployment with no Core token, an unreachable
// register, or a citizen the register does not know all end the same way — no
// geID — and none of them is a reason to fail a sign-in that eID has already
// approved.
func resolveGeID(ctx context.Context, identity *EIDIdentity) {
	if identity == nil || identity.GeID != 0 {
		return
	}
	regNumber := strings.TrimSpace(identity.RegNumber)
	if regNumber == "" {
		return
	}
	client := geregecore.New(os.Getenv("GEREGE_CORE_URL"),
		func() string { return credentials.Get(credentials.CoreAPIToken) })
	if !client.Configured() {
		return
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	person, err := client.FindPerson(lookupCtx, regNumber, "")
	if err != nil {
		if !errors.Is(err, geregecore.ErrNotFound) {
			slog.Warn("could not resolve the citizen's Gerege number", "error", err)
		}
		return
	}
	identity.GeID = person.ID
}
