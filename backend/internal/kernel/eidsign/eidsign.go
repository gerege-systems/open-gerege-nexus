/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package eidsign signs a PDF, or a bare digest, with a citizen's eID.
//
// This is the third and last piece to come home from open-gerege-core, after
// the eID relying-party client and the Gemini client. The reason is the same
// and applies most strongly here: a signature is a legal act, and the code that
// performs one should be in the repository of the team answerable for it rather
// than in a library released elsewhere on somebody else's schedule.
//
// The ceremony, unchanged:
//
//  1. the citizen hands over a PDF; the server overlays their signature image
//     and their organisation's stamp — before hashing, so both are part of what
//     gets signed — and takes the SHA-256;
//  2. the digest goes to eID's /v3 signature endpoint, which pushes a PIN2
//     prompt to the citizen's phone;
//  3. the citizen approves on the phone. That approval is the legal consent;
//  4. the server polls the session until it is terminal;
//  5. on download, eID's own stamp endpoint produces a PAdES-T signature with a
//     timestamp and a public verification page. If that fails, this server
//     embeds a PAdES signature with its own Document-Signer certificate.
//
// Signing in an organisation's name (onBehalfOf, NTRMN-<register>) does not
// change whose key signs: it is still the citizen's PIN2 certificate. What it
// adds is eID checking the representation right when the session opens, and
// the confirmed organisation name being written into the signature's reason.
// It is a record of delegation, not a company seal.
package eidsign

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/digitorus/pdf"
	"github.com/digitorus/pdfsign/sign"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcpumodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	pdfcputypes "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/eidrp"
)

// What can go wrong, as errors a caller can match on.
//
// The library this replaced returned typed domain errors and the HTTP layer
// above recovered the meaning by searching the message text for "represent" and
// "not found". These sentinels are the same meanings said once: errors.Is is
// exact, and the wording is then free to change without changing a status code.
var (
	// ErrNotRepresentative is the citizen not being registered as able to act
	// for the organisation they asked to sign for — eID decides this, not us.
	ErrNotRepresentative = errors.New("eidsign: not authorised to represent this organisation")
	// ErrSessionNotFound is an unknown session, and also somebody else's: the
	// two are answered identically so that an id cannot be probed for existence.
	ErrSessionNotFound = errors.New("eidsign: signature session not found")
	// ErrNotCompleted is asking for the result of a ceremony still running.
	ErrNotCompleted = errors.New("eidsign: the signature is not finished")
	// ErrRefused is the citizen declining on their phone.
	ErrRefused = errors.New("eidsign: the citizen refused the signature")
	// ErrFailed is a ceremony that ended in neither approval nor refusal.
	ErrFailed = errors.New("eidsign: the signature failed")
	// ErrBadDocument is a PDF that is empty or larger than the limit.
	ErrBadDocument = errors.New("eidsign: the document must be between 1 byte and 25 MB")
	// ErrBadDigest is a document hash that is not 64 hex characters of SHA-256.
	ErrBadDigest = errors.New("eidsign: the document hash must be a 64-character SHA-256 hex string")
	// ErrNoRegNumber is a ceremony with nobody to sign it.
	ErrNoRegNumber = errors.New("eidsign: the signer's registration number is missing")
)

// cache is the session store: a string round trip, holding the document and its
// state between the three calls of a ceremony. Narrow on purpose — the platform
// passes its own implementation.
type cache interface {
	Set(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string) (string, error)
}

// Config is the relying party as eID knows it, plus this server's own signing
// certificate.
type Config struct {
	V3BaseURL string // https://eidmongolia.mn — the /v3 is added per path
	RPUUID    string
	RPName    string
	APISecret string // sent as Bearer; empty means the registry is off and none is needed

	// SignerCertPEM and SignerKeyPEM are this server's *permanent*
	// Document-Signer: the certificate and ECDSA key it embeds a PAdES
	// signature with when eID's own stamp is unavailable. Loaded from outside
	// so that every replica and every restart signs as the same entity.
	//
	// Empty is refused in production and falls back to a development
	// self-signed key elsewhere: an ephemeral key is not reproducible, not
	// verifiable by anybody else, and not revocable.
	SignerCertPEM []byte
	SignerKeyPEM  []byte
	IsProduction  bool
}

// Usecase is the ceremony.
type Usecase interface {
	// Init signs a PDF. An empty onBehalfOfOrg is a personal signature; an
	// NTRMN-<register> is one made in that organisation's name.
	Init(ctx context.Context, regNo, fullName, filename string, pdf []byte, onBehalfOfOrg, signatureURL, stampURL string) (InitResult, error)
	// InitDigest signs a bare SHA-256 digest, with no document at all.
	//
	// A payment app uses this: it hashes the canonical content of a transfer
	// and the citizen sees only displayText on their phone. The caller is
	// therefore responsible for displayText actually describing what is being
	// approved — it is the only thing the citizen reads before entering PIN2.
	InitDigest(ctx context.Context, regNo, fullName, digestHex, displayText, docName string) (InitResult, error)
	// VerifiedDigest returns the signed digest once the ceremony has completed.
	VerifiedDigest(ctx context.Context, ownerRegNo, sessionID string) (string, error)
	// Poll advances the session and reports its state.
	Poll(ctx context.Context, ownerRegNo, sessionID string) (string, error)
	// Download returns the signed PDF.
	Download(ctx context.Context, ownerRegNo, sessionID string) (DownloadResult, error)
}

type InitResult struct {
	SessionID        string `json:"session_id"`
	DocumentHash     string `json:"document_hash"`
	VerificationCode string `json:"verification_code"`
	Filename         string `json:"filename"`
}

type DownloadResult struct {
	PDF      []byte
	Filename string
}

type usecase struct {
	// signerErr is why the Document-Signer could not be loaded. Held rather
	// than returned at construction: the citizen's own eID signature needs no
	// server certificate, so a deployment without one still signs — it loses
	// only the fallback, and finds out at download time.
	signerErr error
	cache     cache
	cfg       Config
	// client reaches eID, which is configured and trusted.
	client *http.Client
	// assetClient reaches URLs a *user* supplied for their signature and stamp
	// images. Separate, and hardened against SSRF, because that is the
	// difference between the two.
	assetClient *http.Client
	signer      signerIdentity
}

// signState is what is kept between the calls of one ceremony.
type signState struct {
	RegNo        string `json:"reg_no"`
	FullName     string `json:"full_name"`
	Filename     string `json:"filename"`
	PDFBase64    string `json:"pdf_b64"`
	DocHashB64   string `json:"doc_hash_b64"`
	V3SessionID  string `json:"v3_session_id"`
	State        string `json:"state"` // running | completed | failed | expired | rejected
	SignerName   string `json:"signer_name"`
	SignerSerial string `json:"signer_serial"`
	CompletedAt  string `json:"completed_at"`
	// OnBehalfOfOrg is the organisation's ETSI id; empty for a personal signature.
	OnBehalfOfOrg string `json:"on_behalf_of_org"`
	// OnBehalfOfOrgName is the name eID *confirmed* when polling — not the one
	// the client sent, which is not evidence of anything.
	OnBehalfOfOrgName string `json:"on_behalf_of_org_name"`
}

const (
	statePrefix = "pdfsign:"
	maxPDFBytes = 25 << 20
)

// NewUsecase loads the Document-Signer and builds the ceremony.
func NewUsecase(store cache, cfg Config) (Usecase, error) {
	// The relying-party API, with the client certificate when the registration
	// requires mTLS. Unlike a missing Document-Signer below, a certificate that
	// was configured and cannot be loaded IS a reason to refuse to start: every
	// ceremony would fail at eID with a 401 that names nothing about files.
	client, err := eidrp.HTTPClient(15 * time.Second)
	if err != nil {
		return nil, err
	}
	signer, err := resolveSigner(cfg)
	// A missing Document-Signer is not a reason to refuse to start: it is
	// needed only to embed a PAdES signature on download, and the citizen's own
	// eID signature does not involve it. The error is kept and reported there.
	return &usecase{
		cache:       store,
		cfg:         cfg,
		client:      client,
		assetClient: newAssetFetchClient(15 * time.Second),
		signer:      signer,
		signerErr:   err,
	}, nil
}

func (u *usecase) saveState(ctx context.Context, id string, st signState) error {
	encoded, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("eidsign: encode the session: %w", err)
	}
	return u.cache.Set(ctx, statePrefix+id, string(encoded))
}

func (u *usecase) loadState(ctx context.Context, id string) (signState, error) {
	var st signState
	raw, err := u.cache.Get(ctx, statePrefix+id)
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return st, err
	}
	return st, nil
}

// newAssetFetchClient fetches a URL the user typed, without letting it reach
// this deployment's own network.
//
// Two guards, and the second is the one people forget: the address is checked
// after it is resolved, at dial time, which closes DNS rebinding; and redirects
// are not followed, because a redirect is how an allowed host sends a fetch to
// a forbidden one.
func newAssetFetchClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	safeDial := func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if ip := net.ParseIP(host); ip != nil && isDisallowedFetchIP(ip) {
			return nil, fmt.Errorf("eidsign: refusing to fetch from %s", address)
		}
		conn, err := dialer.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		// The address actually connected to, which is the one that matters.
		if tcp, ok := conn.RemoteAddr().(*net.TCPAddr); ok && isDisallowedFetchIP(tcp.IP) {
			_ = conn.Close()
			return nil, fmt.Errorf("eidsign: refusing to fetch from %s", tcp.IP)
		}
		return conn, nil
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{DialContext: safeDial},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func isDisallowedFetchIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified()
}

// resolveSigner loads the permanent Document-Signer, or refuses in production.
func resolveSigner(cfg Config) (signerIdentity, error) {
	if len(cfg.SignerCertPEM) > 0 && len(cfg.SignerKeyPEM) > 0 {
		return loadSignerPEM(cfg.SignerCertPEM, cfg.SignerKeyPEM)
	}
	if cfg.IsProduction {
		return signerIdentity{}, errors.New(
			"eidsign: production needs a permanent Document-Signer; " +
				"set EID_SIGN_SIGNER_CERT_PEM and EID_SIGN_SIGNER_KEY_PEM")
	}
	return newSelfSignedSigner()
}

// loadSignerPEM reads the certificate and its ECDSA key, in either SEC1 or
// PKCS#8 form.
func loadSignerPEM(certPEM, keyPEM []byte) (signerIdentity, error) {
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return signerIdentity{}, errors.New("eidsign: the signer certificate is not a PEM certificate")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return signerIdentity{}, fmt.Errorf("eidsign: parse the signer certificate: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return signerIdentity{}, errors.New("eidsign: the signer key is not PEM")
	}
	var key *ecdsa.PrivateKey
	switch keyBlock.Type {
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(keyBlock.Bytes)
	case "PRIVATE KEY":
		var parsed any
		if parsed, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes); err == nil {
			ecKey, ok := parsed.(*ecdsa.PrivateKey)
			if !ok {
				return signerIdentity{}, errors.New("eidsign: the signer key is not ECDSA")
			}
			key = ecKey
		}
	default:
		return signerIdentity{}, fmt.Errorf("eidsign: unsupported signer key type %q", keyBlock.Type)
	}
	if err != nil {
		return signerIdentity{}, fmt.Errorf("eidsign: parse the signer key: %w", err)
	}
	return signerIdentity{key: key, cert: cert}, nil
}

// toEtsi turns a registration number into an ETSI semantic identifier.
func toEtsi(id string) string {
	value := strings.ToUpper(strings.TrimSpace(id))
	if strings.HasPrefix(value, "PNOMN-") || strings.HasPrefix(value, "NTRMN-") {
		return value
	}
	return "PNOMN-" + value
}

// regNoMatches compares only the digits of the certificate's serial number with
// the session owner's registration number.
//
// Digits only, and true when the certificate carries none: some eID certificates
// put no registration number in the serial at all, and a strict comparison
// turned those into a refusal of a signature the citizen had already approved.
// The session's own binding to the citizen is what authorises this; the check
// below only notices a disagreement worth logging.
func regNoMatches(certSerial, regNo string) bool {
	digits := func(s string) string {
		var out strings.Builder
		for _, r := range s {
			if r >= '0' && r <= '9' {
				out.WriteRune(r)
			}
		}
		return out.String()
	}
	fromCert := digits(certSerial)
	if fromCert == "" {
		return true
	}
	return fromCert == digits(regNo)
}

// Init hashes the document and opens the ceremony.
func (u *usecase) Init(ctx context.Context, regNo, fullName, filename string, pdfBytes []byte, onBehalfOfOrg, signatureURL, stampURL string) (InitResult, error) {
	if len(pdfBytes) == 0 || len(pdfBytes) > maxPDFBytes {
		return InitResult{}, ErrBadDocument
	}
	if strings.TrimSpace(regNo) == "" {
		return InitResult{}, ErrNoRegNumber
	}
	onBehalfOfOrg = strings.ToUpper(strings.TrimSpace(onBehalfOfOrg))

	// The images go on before the hash, so that what the citizen approves
	// includes what they will see. Best effort: a failed overlay leaves the
	// document as it was rather than stopping the signature.
	pdfBytes = u.applyVisualAssets(ctx, pdfBytes, signatureURL, stampURL)
	sum := sha256.Sum256(pdfBytes)
	digestB64 := base64.StdEncoding.EncodeToString(sum[:])

	v3SessionID, code, err := u.startV3Sign(ctx, toEtsi(regNo), digestB64, fullName,
		onBehalfOfOrg, "Gerege — баримтад гарын үсэг", filename)
	if err != nil {
		return InitResult{}, err
	}

	sessionID := randID()
	if err := u.saveState(ctx, sessionID, signState{
		RegNo: regNo, FullName: fullName, Filename: filename,
		PDFBase64:  base64.StdEncoding.EncodeToString(pdfBytes),
		DocHashB64: digestB64, V3SessionID: v3SessionID, State: "running",
		OnBehalfOfOrg: onBehalfOfOrg,
	}); err != nil {
		return InitResult{}, fmt.Errorf("eidsign: store the session: %w", err)
	}
	return InitResult{
		SessionID: sessionID, DocumentHash: digestB64,
		VerificationCode: code, Filename: filename,
	}, nil
}

// applyVisualAssets overlays the organisation's stamp and the citizen's
// signature on the last page, bottom right, stamp to the left of the signature.
func (u *usecase) applyVisualAssets(ctx context.Context, pdfBytes []byte, signatureURL, stampURL string) []byte {
	out := pdfBytes
	if image := u.fetchAssetImage(ctx, stampURL); image != nil {
		if stamped, err := overlayImageLastPage(out, image, "scale:0.20, pos:br, off:-170 30, rot:0"); err == nil {
			out = stamped
		} else {
			slog.WarnContext(ctx, "eidsign: could not overlay the stamp; continuing without it", "error", err)
		}
	}
	if image := u.fetchAssetImage(ctx, signatureURL); image != nil {
		if signed, err := overlayImageLastPage(out, image, "scale:0.15, pos:br, off:-30 30, rot:0"); err == nil {
			out = signed
		} else {
			slog.WarnContext(ctx, "eidsign: could not overlay the signature; continuing without it", "error", err)
		}
	}
	return out
}

// fetchAssetImage downloads one user-supplied image. Anything that goes wrong
// answers nil: this is decoration on a document that is about to be signed, and
// none of it is worth failing the signature over.
func (u *usecase) fetchAssetImage(ctx context.Context, imageURL string) []byte {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return nil
	}
	// HTTPS only: file://, http:// and the rest are refused before the client
	// with its SSRF guard is even asked.
	if parsed, err := url.Parse(imageURL); err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		slog.WarnContext(ctx, "eidsign: refusing an image URL that is not https")
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, http.NoBody)
	if err != nil {
		return nil
	}
	resp, err := u.assetClient.Do(req)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return nil
	}
	image, err := io.ReadAll(io.LimitReader(resp.Body, 6<<20))
	if err != nil || len(image) == 0 {
		return nil
	}
	return image
}

// overlayImageLastPage stamps an image onto the last page only.
func overlayImageLastPage(pdfBytes, imageBytes []byte, description string) ([]byte, error) {
	conf := pdfcpumodel.NewDefaultConfiguration()
	pages, err := api.PageCount(bytes.NewReader(pdfBytes), conf)
	if err != nil || pages < 1 {
		return nil, fmt.Errorf("eidsign: count the pages: %w", err)
	}
	watermark, err := api.ImageWatermarkForReader(bytes.NewReader(imageBytes), description, true, false, pdfcputypes.POINTS)
	if err != nil {
		return nil, fmt.Errorf("eidsign: build the overlay: %w", err)
	}
	var out bytes.Buffer
	if err := api.AddWatermarks(bytes.NewReader(pdfBytes), &out, []string{strconv.Itoa(pages)}, watermark, conf); err != nil {
		return nil, fmt.Errorf("eidsign: apply the overlay: %w", err)
	}
	return out.Bytes(), nil
}

// Poll asks eID how the ceremony is going and records what it says.
func (u *usecase) Poll(ctx context.Context, ownerRegNo, sessionID string) (string, error) {
	st, err := u.loadState(ctx, sessionID)
	if err != nil {
		return "", ErrSessionNotFound
	}
	// Whose ceremony this is. An unknown session and somebody else's answer the
	// same way, so an id cannot be probed.
	if st.RegNo != ownerRegNo {
		return "", ErrSessionNotFound
	}
	if st.State != "running" {
		return st.State, nil
	}

	res, err := u.pollV3(ctx, st.V3SessionID)
	if err != nil {
		// A failed poll is not a failed ceremony: eID is slow or a connection
		// dropped, and the citizen may be holding their phone right now. The
		// session stays running and the caller asks again.
		slog.WarnContext(ctx, "eidsign: could not reach eID to poll the ceremony", "error", err)
		return "running", nil //nolint:nilerr // a transient failure leaves the ceremony running
	}
	switch {
	case res.State == "COMPLETE" && res.EndResult == "OK":
		if !regNoMatches(res.SubjectSerial, st.RegNo) {
			slog.WarnContext(ctx, "eidsign: the certificate serial does not match the signer's registration number",
				"cert_serial", res.SubjectSerial, "has_reg_number", st.RegNo != "")
		}
		st.State = "completed"
		st.SignerName = res.SubjectName
		st.SignerSerial = res.SubjectSerial
		st.CompletedAt = time.Now().UTC().Format(time.RFC3339)
		// The name eID confirmed, for the signature's reason. Not the client's.
		if res.OrgName != "" {
			st.OnBehalfOfOrgName = res.OrgName
		}
	case res.State == "COMPLETE" && res.EndResult == "USER_REFUSED":
		st.State = "rejected"
	case res.State == "COMPLETE":
		slog.WarnContext(ctx, "eidsign: the ceremony ended with neither approval nor refusal",
			"end_result", res.EndResult)
		st.State = "failed"
	default:
		return "running", nil
	}
	if err := u.saveState(ctx, sessionID, st); err != nil {
		slog.WarnContext(ctx, "eidsign: could not record the ceremony's outcome", "error", err)
	}
	return st.State, nil
}

// Download returns the signed document.
func (u *usecase) Download(ctx context.Context, ownerRegNo, sessionID string) (DownloadResult, error) {
	st, err := u.loadState(ctx, sessionID)
	if err != nil {
		return DownloadResult{}, ErrSessionNotFound
	}
	if st.RegNo != ownerRegNo {
		return DownloadResult{}, ErrSessionNotFound
	}
	if st.State != "completed" {
		return DownloadResult{}, ErrNotCompleted
	}
	pdfBytes, err := base64.StdEncoding.DecodeString(st.PDFBase64)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("eidsign: decode the stored document: %w", err)
	}

	// eID's own stamp is the one to want: PAdES-T, a timestamp, and a public
	// page anybody can verify against. This server's own signature is the
	// fallback for when that endpoint is unavailable.
	signed, err := u.stampV3(ctx, st.V3SessionID, st.Filename, pdfBytes)
	if err != nil {
		slog.WarnContext(ctx, "eidsign: eID could not stamp the document; embedding our own signature",
			"error", err)
		signed, err = u.embedPAdES(pdfBytes, st)
		if err != nil {
			return DownloadResult{}, fmt.Errorf("eidsign: embed the signature: %w", err)
		}
	}
	return DownloadResult{
		PDF:      signed,
		Filename: strings.TrimSuffix(st.Filename, ".pdf") + "-signed.pdf",
	}, nil
}

// stampV3 asks eID to produce the official PAdES-T signature.
func (u *usecase) stampV3(ctx context.Context, v3SessionID, filename string, pdfBytes []byte) ([]byte, error) {
	if v3SessionID == "" {
		return nil, errors.New("eidsign: the eID session id is empty")
	}
	query := url.Values{}
	query.Set("fileName", filename)
	endpoint := strings.TrimRight(u.cfg.V3BaseURL, "/") + "/v3/signature/stamp/" +
		url.PathEscape(v3SessionID) + "?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(pdfBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/pdf")
	u.setRPAuth(req)

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("eidsign: stamp answered %d: %s", resp.StatusCode, string(body))
	}
	signed, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(signed) == 0 {
		return nil, errors.New("eidsign: the stamp endpoint returned an empty document")
	}
	return signed, nil
}

// embedPAdES signs the document with this server's Document-Signer.
func (u *usecase) embedPAdES(pdfBytes []byte, st signState) ([]byte, error) {
	if u.signerErr != nil {
		return nil, fmt.Errorf("eidsign: no Document-Signer is configured: %w", u.signerErr)
	}
	reader, err := pdf.NewReader(bytes.NewReader(pdfBytes), int64(len(pdfBytes)))
	if err != nil {
		return nil, fmt.Errorf("eidsign: read the document: %w", err)
	}

	name := st.SignerName
	if name == "" {
		name = st.FullName
	}
	// The signature is the citizen's. When it was made in an organisation's
	// name, the reason says so — the equivalent of eID's own "ON BEHALF OF".
	reason := "eID PIN2 гарын үсэг — РД " + st.RegNo
	switch {
	case st.OnBehalfOfOrgName != "":
		reason += " · " + st.OnBehalfOfOrgName + "-ийн нэрийн өмнөөс"
	case st.OnBehalfOfOrg != "":
		reason += " · " + st.OnBehalfOfOrg + "-ийн нэрийн өмнөөс"
	}

	var out bytes.Buffer
	err = sign.Sign(bytes.NewReader(pdfBytes), &out, reader, int64(len(pdfBytes)), sign.SignData{
		Signature: sign.SignDataSignature{
			Info: sign.SignDataSignatureInfo{
				Name:   name,
				Reason: reason,
				Date:   time.Now().Local(),
			},
			CertType:   sign.CertificationSignature,
			DocMDPPerm: sign.AllowFillingExistingFormFieldsAndSignaturesPerms,
		},
		Signer:          u.signer.key,
		Certificate:     u.signer.cert,
		DigestAlgorithm: crypto.SHA256,
	})
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// signerIdentity is this server's Document-Signer.
type signerIdentity struct {
	key  *ecdsa.PrivateKey
	cert *x509.Certificate
}

// newSelfSignedSigner is the development fallback, and only that: the key lives
// as long as the process, so nothing it signs can be verified afterwards.
func newSelfSignedSigner() (signerIdentity, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return signerIdentity{}, err
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "Gerege Document Signer", Country: []string{"MN"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(5, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return signerIdentity{}, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return signerIdentity{}, err
	}
	return signerIdentity{key: key, cert: cert}, nil
}

func randID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%x", buf)
}

// setRPAuth presents the relying party's secret, when there is one.
func (u *usecase) setRPAuth(req *http.Request) {
	if u.cfg.APISecret != "" {
		req.Header.Set("Authorization", "Bearer "+u.cfg.APISecret)
	}
}

// startV3Sign opens the signature session and pushes PIN2 to the citizen.
func (u *usecase) startV3Sign(ctx context.Context, etsi, digestB64, displayName, onBehalfOfOrg, displayText, fileName string) (sessionID, code string, err error) {
	_ = displayName // eID takes the name from the certificate, not from us
	body := map[string]any{
		"relyingPartyUUID": u.cfg.RPUUID,
		"relyingPartyName": u.cfg.RPName,
		// QUALIFIED, unlike sign-in's ADVANCED. Accepting ADVANCED here would
		// quietly produce something that is not a qualified signature.
		"certificateLevel":  "QUALIFIED",
		"signatureProtocol": "ACSP_V2",
		"digest":            digestB64,
		"hashType":          "SHA256",
		"interactions": []map[string]string{
			{"type": "displayTextAndPIN", "displayText60": clampDisplayText(displayText)},
		},
	}
	if onBehalfOfOrg != "" {
		body["onBehalfOf"] = onBehalfOfOrg
	}
	// The document's name, for the public verification page. Omitted when
	// empty, which leaves eID to guess it from the interaction text.
	if name := clampFileName(fileName); name != "" {
		body["fileName"] = name
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return "", "", fmt.Errorf("eidsign: encode the request: %w", err)
	}
	endpoint := strings.TrimRight(u.cfg.V3BaseURL, "/") + "/v3/signature/notification/etsi/" + url.PathEscape(etsi)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	u.setRPAuth(req)

	resp, err := u.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()

	// 403 is the citizen not being a representative of that organisation — or
	// this deployment not holding the right to sign at all. Either way it is a
	// refusal with a reason, and hiding it behind a 500 helps nobody.
	if resp.StatusCode == http.StatusForbidden {
		return "", "", ErrNotRepresentative
	}
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", "", fmt.Errorf("eidsign: eID answered %d: %s", resp.StatusCode, string(raw))
	}

	var answer struct {
		SessionID string `json:"sessionID"`
		VC        struct {
			Value string `json:"value"`
		} `json:"vc"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		return "", "", err
	}
	return answer.SessionID, answer.VC.Value, nil
}

// v3PollResult is one look at an eID signature session.
type v3PollResult struct {
	State         string
	EndResult     string
	SubjectName   string
	SubjectSerial string
	// OrgName is the organisation eID *confirmed* the citizen signed for.
	OrgName string
}

func (u *usecase) pollV3(ctx context.Context, v3SessionID string) (v3PollResult, error) {
	endpoint := strings.TrimRight(u.cfg.V3BaseURL, "/") + "/v3/session/" +
		url.PathEscape(v3SessionID) + "?timeoutMs=1000"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return v3PollResult{}, err
	}
	u.setRPAuth(req)

	resp, err := u.client.Do(req)
	if err != nil {
		return v3PollResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return v3PollResult{}, fmt.Errorf("eidsign: poll answered %d", resp.StatusCode)
	}

	var answer struct {
		State  string `json:"state"`
		Result struct {
			EndResult string `json:"endResult"`
		} `json:"result"`
		Cert struct {
			Value string `json:"value"`
		} `json:"cert"`
		OnBehalfOf struct {
			OrgEtsi string `json:"orgEtsi"`
			OrgName string `json:"orgName"`
		} `json:"onBehalfOf"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		return v3PollResult{}, err
	}

	out := v3PollResult{
		State:     answer.State,
		EndResult: answer.Result.EndResult,
		OrgName:   answer.OnBehalfOf.OrgName,
	}
	if answer.Cert.Value != "" {
		if der, err := base64.StdEncoding.DecodeString(answer.Cert.Value); err == nil {
			if cert, err := x509.ParseCertificate(der); err == nil {
				out.SubjectName = strings.TrimSpace(cert.Subject.CommonName)
				out.SubjectSerial = cert.Subject.SerialNumber
			}
		}
	}
	return out, nil
}
