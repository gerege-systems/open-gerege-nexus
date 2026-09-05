/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package eidrp is this platform's relying-party client for eID Mongolia.
//
// It replaces open-gerege-core/pkg/eid, which this repository used until the
// person block changed. Two reasons, and the second is the one that matters:
//
//   - The library is released from another repository on its own schedule. When
//     eID added `geID` to the person block, the number was dropped a layer
//     below this platform and there was nothing to do here but wait. A sign-in
//     that this platform depends on should not be a version bump away.
//   - Only four calls of that library's surface were ever used — two initiates,
//     the poll, and the organisations a citizen represents. The rest (signers,
//     the PKI panel, representation writes) is somebody else's product, carried
//     along and compiled in.
//
// The wire protocol is eID Mongolia v3 (Smart-ID compatible, ACSP_V2) and is
// unchanged. What is here is the same protocol, read into this platform's own
// types, in a file the team that depends on it can edit:
//
//	POST {base}/authentication/device-link/anonymous            QR sign-in
//	POST {base}/authentication/notification/etsi/PNOMN-{civil}  push sign-in
//	GET  {base}/session/{id}?timeoutMs=25000                    long-poll
//	GET  {base}/organization/representations/etsi/{personEtsi}  the citizen's organisations
//
// Authorization is `Bearer <rp secret>` and the relying party names itself in
// the body. A COMPLETE response says only that the session ended; the terminal
// result is in `result.endResult`, which this maps onto the four states below.
package eidrp

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrInitiateRejected is eID answering an initiate with a 4xx — an unknown
// registration number, a malformed one, or a relying party without the right.
// Separated from a 5xx so a caller can report the citizen's mistake as theirs
// rather than as this platform's.
var ErrInitiateRejected = errors.New("eid: initiate rejected")

// The session states this platform works in. eID reports RUNNING or COMPLETE
// and puts the real outcome in endResult; the terminal failures are mapped onto
// EXPIRED and REFUSED here so that everything above this package sees four
// states rather than a state and a result.
const (
	StateComplete = "COMPLETE"
	StateExpired  = "EXPIRED"
	StateRefused  = "REFUSED"
	StateRunning  = "RUNNING"
)

const (
	defaultBase   = "https://eidmongolia.mn/v3"
	defaultRPName = "gerege-nexus"
	// defaultCertLevel is the *lowest* certificate this platform will accept.
	// Asking for QUALIFIED turns away citizens whose sign-in certificate is
	// ADVANCED — which is most of them — and the failure they see is "could not
	// reach the server" rather than anything about certificates.
	defaultCertLevel = "ADVANCED"
	maxRespBytes     = 256 << 10
	// httpTimeout outlasts the longest poll eID holds open (25s).
	httpTimeout = 30 * time.Second
)

// Identity is the citizen as eID vouches for them.
//
// The three names are three different things and eID is deliberate about
// saying so, because Mongolian usage and the certificate disagree:
//
//	FirstName  — нэр,         the person's own name
//	LastName   — эцгийн нэр,   the patronymic, and the only one in the certificate
//	FamilyName — ургийн овог,  the clan name, which the certificate does not carry
//
// A caller that treats LastName as a family name is wrong in a way that reads
// correctly in English and is visibly wrong to anybody Mongolian.
type Identity struct {
	CivilID    string
	NationalID string
	FirstName  string
	LastName   string
	FamilyName string
	// The Latin transliterations, in the ICAO upper-case form the passport
	// uses. There is no Latin family name: it is not in the certificate.
	FirstNameEn string
	LastNameEn  string
	// BirthDate is a date and nothing more: `YYYY-MM-DD`, no clock and no zone.
	// The certificate carries neither this nor Gender, which is why the person
	// block exists at all.
	BirthDate string
	// Gender is "M", "F" or "X".
	Gender string
	// GeID is the citizen's number in the Gerege register (core.gerege.mn's
	// `users.id`). Absent for a citizen the register has not been matched to,
	// which is ordinary: a relying party must not require it.
	GeID           int64
	KYCLevel       string
	DocumentNumber string
	// Certificate is the citizen's certificate as parsed from the session, when
	// it was there and could be read. A signature record anchors on it: a
	// session id says an approval happened, a certificate says whose key gave it.
	Certificate *Certificate
}

// Certificate is the public half of the citizen's eID certificate.
type Certificate struct {
	Serial    string
	NotBefore time.Time
	NotAfter  time.Time
	Issuer    string
	KeyType   string
}

// StartResult is what an initiate returns.
type StartResult struct {
	SessionID        string
	VerificationCode string
	ExpiresAt        string
	DeviceLinkURL    string
}

// SessionResult is one poll. Identity is filled only on COMPLETE.
type SessionResult struct {
	State    string
	Identity *Identity
}

// Representation is one organisation a citizen may act for.
type Representation struct {
	OrgEtsi     string
	OrgRegister string
	OrgName     string
	OrgNameEn   string
	Role        string
	RightType   string // ADMIN | MANAGER
	ValidFrom   *time.Time
	ValidTo     *time.Time // nil = open-ended
}

// Client is what this platform asks of eID. Four calls, which is what it uses:
// an interface rather than a struct so a test can answer them without a server.
type Client interface {
	// QRInitiate starts a device-link sign-in. An empty callbackURL means
	// cross-device (a desktop showing a QR code); a callback means same-device,
	// where the eID app returns the browser afterwards.
	QRInitiate(ctx context.Context, displayText, callbackURL, nonce string) (*StartResult, error)
	// Initiate starts a sign-in by registration number: eID pushes the request
	// to the device that number is enrolled on.
	Initiate(ctx context.Context, nationalID, displayText, callbackURL string) (*StartResult, error)
	// Session long-polls one session for up to timeoutMs.
	Session(ctx context.Context, sessionID string, timeoutMs int) (*SessionResult, error)
	// Representations lists the organisations this citizen may act for.
	Representations(ctx context.Context, personEtsi string) ([]Representation, error)
}

type client struct {
	base      string
	rpUUID    string
	rpName    string
	secret    string
	certLevel string
	http      *http.Client
}

// NewClient builds the client. Empty base, name and certificate level take the
// defaults above; the UUID and secret are the relying-party credentials eID
// issued, and the secret travels in the Authorization header and never in a log.
func NewClient(base, rpUUID, rpName, secret, certLevel string) Client {
	if base = strings.TrimRight(strings.TrimSpace(base), "/"); base == "" {
		base = defaultBase
	}
	if rpName = strings.TrimSpace(rpName); rpName == "" {
		rpName = defaultRPName
	}
	if certLevel = strings.TrimSpace(certLevel); certLevel == "" {
		certLevel = defaultCertLevel
	}
	return &client{
		base: base, rpUUID: rpUUID, rpName: rpName, secret: secret, certLevel: certLevel,
		http: &http.Client{Timeout: httpTimeout},
	}
}

// interaction is what the eID app shows the citizen while it asks them to
// approve. displayText60 is capped at sixty characters by the protocol.
type interaction struct {
	Type          string `json:"type"`
	DisplayText60 string `json:"displayText60,omitempty"`
}

// authInitiateBody is the ACSP_V2 initiate request.
//
// The challenge field is `rpChallenge` — authentication's, not signing's
// `digest`/`hashType`. Sending the wrong one leaves the server with an empty
// challenge, and the citizen's approval then fails inside the app with a
// message about processing rather than about anything they did.
type authInitiateBody struct {
	RelyingPartyUUID  string        `json:"relyingPartyUUID"`
	RelyingPartyName  string        `json:"relyingPartyName"`
	CertificateLevel  string        `json:"certificateLevel"`
	SignatureProtocol string        `json:"signatureProtocol"`
	RPChallenge       string        `json:"rpChallenge"`
	Interactions      []interaction `json:"interactions"`
	// RPApp is the name the eID app shows on its approval screen.
	RPApp    string `json:"rp_app,omitempty"`
	RPAppURL string `json:"rp_app_url,omitempty"`
	// InitialCallbackURL is the same-device return address. Empty means
	// cross-device: the browser polls and eID sends the phone nowhere.
	InitialCallbackURL string `json:"initialCallbackUrl,omitempty"`
}

func (c *client) newAuthBody(displayText, callbackURL string) (authInitiateBody, error) {
	challenge, err := randomChallenge()
	if err != nil {
		return authInitiateBody{}, err
	}
	text := strings.TrimSpace(displayText)
	if text == "" {
		text = c.rpName
	}
	// Sixty characters, counted in runes: the text is Mongolian, and cutting a
	// UTF-8 string by bytes ends it in half a letter.
	if runes := []rune(text); len(runes) > 60 {
		text = string(runes[:60])
	}
	return authInitiateBody{
		RelyingPartyUUID:   c.rpUUID,
		RelyingPartyName:   c.rpName,
		CertificateLevel:   c.certLevel,
		SignatureProtocol:  "ACSP_V2",
		RPChallenge:        challenge,
		Interactions:       []interaction{{Type: "displayTextAndPIN", DisplayText60: text}},
		RPApp:              c.rpName,
		InitialCallbackURL: callbackURL,
	}, nil
}

func (c *client) QRInitiate(ctx context.Context, displayText, callbackURL, _ string) (*StartResult, error) {
	body, err := c.newAuthBody(displayText, callbackURL)
	if err != nil {
		return nil, fmt.Errorf("eid: build the challenge: %w", err)
	}
	raw, status, err := c.post(ctx, "/authentication/device-link/anonymous", body)
	if err != nil {
		return nil, err
	}
	if err := checkInitiateStatus(raw, status); err != nil {
		return nil, err
	}
	var out struct {
		SessionID    string          `json:"sessionID"`
		SessionToken string          `json:"sessionToken"`
		VC           json.RawMessage `json:"vc"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.SessionID == "" {
		return nil, fmt.Errorf("eid initiate: no session id in the answer: %s", snippet(raw))
	}
	// What the QR encodes is the bare session id, not a device-link URL. The
	// eID app's scanner reads a UUID and resolves it against its own server; a
	// `https://…/dl?deviceLinkType=…` URL is something it cannot parse.
	return &StartResult{
		SessionID:        out.SessionID,
		VerificationCode: parseVerificationCode(out.VC),
		DeviceLinkURL:    out.SessionID,
	}, nil
}

func (c *client) Initiate(ctx context.Context, nationalID, displayText, callbackURL string) (*StartResult, error) {
	body, err := c.newAuthBody(displayText, callbackURL)
	if err != nil {
		return nil, fmt.Errorf("eid: build the challenge: %w", err)
	}
	// The semantics identifier is ETSI EN 319 412-1's for a natural person:
	// PNOMN-<civil id>.
	path := "/authentication/notification/etsi/PNOMN-" + url.PathEscape(strings.TrimSpace(nationalID))
	raw, status, err := c.post(ctx, path, body)
	if err != nil {
		return nil, err
	}
	if err := checkInitiateStatus(raw, status); err != nil {
		return nil, err
	}
	var out struct {
		SessionID string          `json:"sessionID"`
		VC        json.RawMessage `json:"vc"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.SessionID == "" {
		return nil, fmt.Errorf("eid initiate: no session id in the answer: %s", snippet(raw))
	}
	return &StartResult{SessionID: out.SessionID, VerificationCode: parseVerificationCode(out.VC)}, nil
}

// sessionResponse is eID's session answer.
//
// The person block is the reason this package exists in this repository. Its
// field names are eID's own and line up with core.gerege.mn's register, which
// is what keeps the three names from being confused: firstName is the name,
// lastName is the patronymic, familyName is the clan name.
type sessionResponse struct {
	State  string `json:"state"`
	Result *struct {
		EndResult      string `json:"endResult"`
		DocumentNumber string `json:"documentNumber"`
	} `json:"result"`
	Cert *struct {
		Value            string `json:"value"` // base64 DER
		CertificateLevel string `json:"certificateLevel"`
	} `json:"cert"`
	Person *struct {
		FirstName   string `json:"firstName"`
		LastName    string `json:"lastName"`
		FamilyName  string `json:"familyName"`
		FirstNameEn string `json:"firstNameEn"`
		LastNameEn  string `json:"lastNameEn"`
		CivilID     string `json:"civilId"`
		RegNo       string `json:"regNo"`
		BirthDate   string `json:"birthDate"`
		Gender      string `json:"gender"`
		GeID        int64  `json:"geID"`
		// The names eID used before it aligned them with the register. Read as
		// a fallback so that a deployment pointed at an older eID keeps working
		// through the change rather than losing every citizen's name on the day
		// one side is upgraded and the other is not.
		GivenName string `json:"givenName"`
		Surname   string `json:"surname"`
	} `json:"person"`
}

func (c *client) Session(ctx context.Context, sessionID string, timeoutMs int) (*SessionResult, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, errors.New("eid: empty session id")
	}
	path := fmt.Sprintf("/session/%s?timeoutMs=%d", url.PathEscape(sessionID), timeoutMs)
	raw, status, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	// eID keeps a session only while it lives: once it has ended and been read,
	// or its deadline has passed, the id is gone and every further poll is a
	// 404. That is the session's answer, not a failure of this platform — as a
	// failure it reached the citizen as a 502 on the sign-in card, one per
	// check, until the screen gave up on a session that was merely over.
	if status == http.StatusNotFound {
		return &SessionResult{State: StateExpired}, nil
	}
	if status >= 300 {
		return nil, fmt.Errorf("eid session: status %d: %s", status, snippet(raw))
	}

	var out sessionResponse
	if err := json.Unmarshal(raw, &out); err != nil || out.State == "" {
		return nil, fmt.Errorf("eid session: invalid answer: %s", snippet(raw))
	}
	if out.State != "COMPLETE" {
		return &SessionResult{State: StateRunning}, nil
	}

	// COMPLETE says the session ended, not that it succeeded.
	endResult := ""
	if out.Result != nil {
		endResult = out.Result.EndResult
	}
	if endResult != "OK" {
		if endResult == "TIMEOUT" {
			return &SessionResult{State: StateExpired}, nil
		}
		// USER_REFUSED*, WRONG_VC, DOCUMENT_UNUSABLE and the rest: the citizen
		// did not approve this.
		return &SessionResult{State: StateRefused}, nil
	}
	if out.Person == nil {
		return nil, fmt.Errorf("eid session: approved with no person block: %s", snippet(raw))
	}

	id := &Identity{
		CivilID:     out.Person.CivilID,
		NationalID:  out.Person.RegNo,
		FirstName:   firstNonEmpty(out.Person.FirstName, out.Person.GivenName),
		LastName:    firstNonEmpty(out.Person.LastName, out.Person.Surname),
		FamilyName:  out.Person.FamilyName,
		FirstNameEn: out.Person.FirstNameEn,
		LastNameEn:  out.Person.LastNameEn,
		BirthDate:   out.Person.BirthDate,
		Gender:      out.Person.Gender,
		GeID:        out.Person.GeID,
	}
	if out.Cert != nil {
		id.KYCLevel = out.Cert.CertificateLevel
		// A certificate that cannot be parsed is skipped rather than fatal: it
		// is additional information, and the citizen has already approved.
		id.Certificate = parseCertificate(out.Cert.Value)
	}
	if out.Result != nil {
		id.DocumentNumber = out.Result.DocumentNumber
	}
	return &SessionResult{State: StateComplete, Identity: id}, nil
}

func (c *client) Representations(ctx context.Context, personEtsi string) ([]Representation, error) {
	personEtsi = strings.TrimSpace(personEtsi)
	if personEtsi == "" {
		return nil, errors.New("eid: empty person identifier")
	}
	raw, status, err := c.get(ctx, "/organization/representations/etsi/"+url.PathEscape(personEtsi))
	if err != nil {
		return nil, err
	}
	// Not found is an answer, not a failure: this citizen represents nobody.
	if status == http.StatusNotFound {
		return []Representation{}, nil
	}
	if status >= 300 {
		return nil, fmt.Errorf("eid representations: status %d: %s", status, snippet(raw))
	}
	var out struct {
		Representations []struct {
			OrgEtsi     string     `json:"orgEtsi"`
			OrgRegister string     `json:"orgRegister"`
			OrgName     string     `json:"orgName"`
			OrgNameEn   string     `json:"orgNameEn"`
			Role        string     `json:"role"`
			RightType   string     `json:"rightType"`
			ValidFrom   *time.Time `json:"validFrom"`
			ValidTo     *time.Time `json:"validTo"`
		} `json:"representations"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("eid representations: invalid answer: %s", snippet(raw))
	}
	reps := make([]Representation, 0, len(out.Representations))
	for _, r := range out.Representations {
		reps = append(reps, Representation(r))
	}
	return reps, nil
}

func (c *client) post(ctx context.Context, path string, body any) ([]byte, int, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("eid: encode the request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(buf))
	if err != nil {
		return nil, 0, fmt.Errorf("eid: build the request: %w", err)
	}
	return c.do(req)
}

func (c *client) get(ctx context.Context, path string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, http.NoBody)
	if err != nil {
		return nil, 0, fmt.Errorf("eid: build the request: %w", err)
	}
	return c.do(req)
}

func (c *client) do(req *http.Request) ([]byte, int, error) {
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("eid: http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	return raw, resp.StatusCode, nil
}

// checkInitiateStatus separates the citizen's mistake from this platform's.
func checkInitiateStatus(raw []byte, status int) error {
	if status >= 400 && status < 500 {
		return fmt.Errorf("%w: status %d: %s", ErrInitiateRejected, status, snippet(raw))
	}
	if status >= 300 {
		return fmt.Errorf("eid initiate: status %d: %s", status, snippet(raw))
	}
	return nil
}

// randomChallenge is the 32-byte ACSP_V2 challenge, base64 standard encoded.
func randomChallenge() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

// parseVerificationCode reads the code the citizen compares on their phone. The
// anonymous flow returns a bare string ("7270"), the notification flow an
// object; both arrive here.
func parseVerificationCode(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var object struct {
		Value string `json:"value"`
	}
	if json.Unmarshal(raw, &object) == nil {
		return object.Value
	}
	return ""
}

func parseCertificate(b64 string) *Certificate {
	der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil || len(der) == 0 {
		return nil
	}
	crt, err := x509.ParseCertificate(der)
	if err != nil {
		return nil
	}
	return &Certificate{
		Serial:    crt.SerialNumber.Text(16),
		NotBefore: crt.NotBefore,
		NotAfter:  crt.NotAfter,
		Issuer:    crt.Issuer.CommonName,
		KeyType:   keyTypeOf(crt),
	}
}

func keyTypeOf(crt *x509.Certificate) string {
	switch pub := crt.PublicKey.(type) {
	case *ecdsa.PublicKey:
		return "ECDSA " + pub.Curve.Params().Name
	case *rsa.PublicKey:
		return fmt.Sprintf("RSA %d", pub.N.BitLen())
	default:
		return crt.PublicKeyAlgorithm.String()
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if runes := []rune(s); len(runes) > 200 {
		return string(runes[:200])
	}
	return s
}
