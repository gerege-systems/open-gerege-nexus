/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Package emailverify proves that somebody controls an email address, for every
 * app module in the binary, through the hosted verification service.
 */

package emailverify

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/async"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/mailrail"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Why this is platform furniture rather than an app module, and why it sends
// nothing itself.
//
// Contacts wants an address proven before it trusts it, Documents wants it
// before a signing link leaves for an outsider, and Gov Services wants it
// before it answers a citizen at one. That is one capability, so it lives in
// the platform and every module reaches it the same way.
//
// The mail itself is somebody else's problem on purpose. Delivering mail that
// arrives is not a matter of holding an SMTP password: it is SPF, DKIM, DMARC,
// reverse DNS and a sending reputation, maintained continuously. enigma.mn runs
// that, so this platform holds no mailbox credential, composes no message and
// owns no sender address. It asks for a link to be sent and finds out when it
// was followed.
//
// What stays here is what only this platform can know: which module asked, for
// whom, why, and whether the person came back.

const (
	// DefaultProviderURL is the hosted service. Overridable so a deployment can
	// point at its own instance.
	DefaultProviderURL = "https://enigma.mn/api/verify"

	// AdminURL is where the keys are administered — deliberately not here.
	// A key belongs to the sending service, not to this database.
	AdminURL = "https://admin.enigma.mn"

	// LinkTTL mirrors the hosted service's own 24-hour expiry. It is not
	// authoritative — the service decides — but a local row has to stop being
	// reported as outstanding at some point, and this is when.
	LinkTTL = 24 * time.Hour

	// TenantHourlyLimit caps what one tenant can ask for in an hour.
	//
	// The whole platform now sends under one key, so a loop in one module
	// spends an allowance every other module shares. This is the local guard in
	// front of that: it is not the provider's limit, it is ours.
	TenantHourlyLimit = 500

	// ResendInterval is the pause enforced per recipient. The provider allows
	// five sends an hour to one address; asking again a second later is a
	// mail-bombing tool either way, so the pause is here rather than only
	// there — a 429 we can avoid provoking is one we do not have to explain.
	ResendInterval = 60 * time.Second

	// Retention is how long the record of a verification is kept. It is an
	// audit trail of who was asked to prove what, not a mailing list.
	Retention = 90 * 24 * time.Hour

	// upstreamTimeout bounds the call to the provider. It is a request a person
	// is waiting on, and a hung socket must not become a hung page.
	upstreamTimeout = 10 * time.Second
)

// The shape of a request and its answer is in internal/kernel/mail: the
// console asks for the same thing from the other plane, and one struct
// described twice is two structs the first time somebody edits one.
type (
	Status       = mailrail.Status
	Verification = mailrail.Verification
	Request      = mailrail.Request
)

const (
	StatusPending  = mailrail.StatusPending
	StatusVerified = mailrail.StatusVerified
	StatusExpired  = mailrail.StatusExpired
)

// PublicOrigin is where a recipient's browser comes back to.
func PublicOrigin() string { return mailrail.PublicOrigin() }

var (
	// ErrNotConfigured means no key was supplied, so nothing can be sent. It is
	// an operator's problem, not the caller's, and is reported as such.
	ErrNotConfigured = errors.New("EMAIL_VERIFY_API_KEY is not set, so verification mail cannot be requested")

	// ErrOriginNotHTTPS is the other way this is misconfigured: the provider
	// refuses a plain-HTTP return address, so a deployment whose PUBLIC_ORIGIN
	// is HTTP cannot use the service at all. Saying so beats a bare 400 from
	// somebody else's API.
	ErrOriginNotHTTPS = errors.New("the verification service accepts only an HTTPS return address, so PUBLIC_ORIGIN must be HTTPS")

	// ErrUnauthorizedKey is the provider refusing our key: wrong, revoked, or
	// disabled at admin.enigma.mn. Nothing a caller did.
	ErrUnauthorizedKey = errors.New("the verification service rejected this platform's API key")

	// ErrUpstream is a failure at the provider — it could not send, or it could
	// not be reached. Retryable, unlike everything else here.
	ErrUpstream = errors.New("the verification service could not send the message")

	// ErrLinkSpent covers a return that was already honoured, has expired, or
	// never existed. One answer for all three, so somebody holding a stale link
	// learns only that it is no longer good.
	ErrLinkSpent = errors.New("this verification link is no longer valid")
)

// InvalidError is a caller mistake worth quoting back: a malformed address, a
// destination that is not allowed. It maps to 400.
type InvalidError struct{ msg string }

func (e *InvalidError) Error() string { return e.msg }

func invalid(format string, args ...any) error {
	return &InvalidError{msg: fmt.Sprintf(format, args...)}
}

// RateLimitedError carries how long the caller should wait, so the handler can
// answer 429 with a Retry-After somebody can actually obey.
type RateLimitedError struct {
	RetryAfter time.Duration
	msg        string
}

func (e *RateLimitedError) Error() string { return e.msg }

// Stats is the Overview screen's header.
type Stats struct {
	Total       int     `json:"total"`
	Verified    int     `json:"verified"`
	Pending     int     `json:"pending"`
	Expired     int     `json:"expired"`
	Last24h     int     `json:"last_24h"`
	VerifiedPct float64 `json:"verified_pct"`
}

// Overview is what one screen needs in one request.
type Overview struct {
	Stats  Stats          `json:"stats"`
	Recent []Verification `json:"recent"`

	// Configured is whether a key is present at all. The key itself is never
	// echoed — not a prefix of it, not a length: nothing on this screen needs
	// it, and a browser is not where it belongs.
	Configured bool `json:"configured"`
	// Reachable is the provider's own health check, and Health is what it said
	// when it was not. An administrator seeing no verifications should be able
	// to tell "nobody asked" from "the service is down".
	Reachable bool   `json:"reachable"`
	Health    string `json:"health,omitempty"`

	ProviderURL string `json:"provider_url"`
	AdminURL    string `json:"admin_url"`
	// ReturnURL is what the provider sends people back to. Useful to an
	// administrator diagnosing a deployment whose PUBLIC_ORIGIN is wrong.
	ReturnURL string `json:"return_url"`
}

// Service is the whole capability: a client of the hosted service plus this
// platform's own record of what it asked for.
type Service struct {
	store *store
	http  *http.Client
}

// NewService builds the service over a database pool.
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		store: &store{db: db},
		http:  &http.Client{Timeout: upstreamTimeout},
	}
}

// ProviderURL is the hosted service's base address.
func ProviderURL() string {
	if custom := strings.TrimRight(strings.TrimSpace(os.Getenv("EMAIL_VERIFY_BASE_URL")), "/"); custom != "" {
		return custom
	}
	return DefaultProviderURL
}

// apiKey is the platform's credential with the provider. It is read at call
// time rather than captured at construction so that rotating it is a restart of
// nothing, and it is never returned to any caller.
func apiKey() string { return strings.TrimSpace(os.Getenv("EMAIL_VERIFY_API_KEY")) }

// Configured reports whether this deployment can ask for mail at all.
// Configured reports whether a key was supplied. The answer is the mail
// rail's configuration rather than this package's, so it is read from there.
func Configured() bool { return mailrail.Configured() }

// ReturnURL is where the provider sends people once they have confirmed.
func ReturnURL() string { return mailrail.PublicOrigin() + "/api/v1/verify/landed" }

// Send asks the provider for a link and records what was asked.
//
// This is the entry point for app modules: a module takes *Service in its
// constructor, the way gov_services takes the integration manager, and calls
// this with its own app id as the source.
//
//	v, err := m.emailVerify.Send(ctx, tenantID, emailverify.Request{
//	    Email:   contact.Email,
//	    Source:  m.ID(),
//	    Purpose: "contact_invite",
//	})
func (s *Service) Send(ctx context.Context, tenantID string, req Request) (*Verification, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, invalid("a tenant is required")
	}
	if !Configured() {
		return nil, ErrNotConfigured
	}
	// The provider refuses a plain-HTTP return address, and the failure would
	// otherwise arrive as a 400 about a URL the caller never wrote.
	if !strings.HasPrefix(PublicOrigin(), "https://") && config.IsProduction() {
		return nil, ErrOriginNotHTTPS
	}

	address, err := NormalizeEmail(req.Email)
	if err != nil {
		return nil, err
	}
	destination, err := ValidateRedirect(req.RedirectURL)
	if err != nil {
		return nil, err
	}
	if err := s.checkQuota(ctx, tenantID, address); err != nil {
		return nil, err
	}

	ref, err := randomRef()
	if err != nil {
		return nil, err
	}

	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "platform"
	}

	// The row is written before the provider is called, because the return
	// address has to name it. A request the provider refuses is withdrawn
	// below rather than left behind as a verification nobody was asked for.
	verification, err := s.store.insertVerification(ctx, newVerification{
		TenantID:    tenantID,
		Source:      truncate(source, 128),
		Purpose:     truncate(strings.TrimSpace(req.Purpose), 64),
		Email:       address,
		RefHash:     hashSecret(ref),
		RedirectURL: destination,
		RequestedIP: truncate(req.ClientIP, 64),
		ExpiresAt:   time.Now().Add(LinkTTL),
	})
	if err != nil {
		return nil, err
	}

	expiresAt, err := s.requestSend(ctx, address, ReturnURL()+"?ref="+url.QueryEscape(ref))
	if err != nil {
		if delErr := s.store.deleteVerification(ctx, verification.ID); delErr != nil {
			slog.Error("emailverify: could not withdraw a request the provider refused",
				"id", verification.ID, "error", delErr)
		}
		return nil, err
	}
	if !expiresAt.IsZero() {
		// The provider owns the deadline; ours was a placeholder.
		if updated, updErr := s.store.setExpiry(ctx, verification.ID, expiresAt); updErr == nil {
			verification = updated
		} else {
			slog.Warn("emailverify: could not record the provider's expiry",
				"id", verification.ID, "error", updErr)
		}
	}
	return verification, nil
}

// sendResponse is the provider's answer. Only expires_at is of any use to us —
// ok is implied by the status code, and the address is the one we sent.
type sendResponse struct {
	OK        bool   `json:"ok"`
	Email     string `json:"email"`
	ExpiresAt string `json:"expires_at"`
	Error     string `json:"error"`
}

// requestSend performs the call and turns the provider's status codes into this
// package's errors. The mapping is the contract: 400 and 401 are final, 429 and
// 5xx are worth retrying, and anything unrecognised is treated as upstream
// rather than as success.
func (s *Service) requestSend(ctx context.Context, address, returnURL string) (time.Time, error) {
	return telemetry.ObserveExternalValue(ctx, telemetry.SystemEmailVerify, "send",
		func(ctx context.Context) (time.Time, error) { return s.doRequestSend(ctx, address, returnURL) })
}

func (s *Service) doRequestSend(ctx context.Context, address, returnURL string) (time.Time, error) {
	payload, err := json.Marshal(map[string]string{
		"email":        address,
		"redirect_url": returnURL,
	})
	if err != nil {
		return time.Time{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		ProviderURL()+"/send", bytes.NewReader(payload))
	if err != nil {
		return time.Time{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey())
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := s.http.Do(httpReq)
	if err != nil {
		slog.Error("emailverify: the verification service could not be reached", "error", err)
		return time.Time{}, ErrUpstream
	}
	defer func() { _ = res.Body.Close() }()

	// Bounded: the body is a small JSON object, and an unbounded read here
	// would make somebody else's server able to exhaust this one's memory.
	body, _ := io.ReadAll(io.LimitReader(res.Body, 8<<10))
	var answer sendResponse
	_ = json.Unmarshal(body, &answer)

	switch res.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		if answer.ExpiresAt == "" {
			return time.Time{}, nil
		}
		expiresAt, parseErr := time.Parse(time.RFC3339, answer.ExpiresAt)
		if parseErr != nil {
			// Not worth failing a send that happened; the local placeholder
			// deadline stands.
			slog.Warn("emailverify: the service returned an expiry we could not read",
				"expires_at", answer.ExpiresAt)
			return time.Time{}, nil
		}
		return expiresAt, nil

	case http.StatusBadRequest:
		// The caller's input, in the provider's words where it gave any.
		switch answer.Error {
		case "invalid_email":
			return time.Time{}, invalid("the email address is not valid")
		case "redirect_url_must_be_https":
			// The return address is ours, not the caller's, so this is a
			// deployment fault dressed as a caller fault.
			return time.Time{}, ErrOriginNotHTTPS
		default:
			return time.Time{}, invalid("the verification service refused the request: %s", firstLine(answer.Error, body))
		}

	case http.StatusUnauthorized, http.StatusForbidden:
		return time.Time{}, ErrUnauthorizedKey

	case http.StatusTooManyRequests:
		return time.Time{}, &RateLimitedError{
			RetryAfter: retryAfterHeader(res, time.Hour),
			msg:        "the verification service is rate limiting this address",
		}

	default:
		slog.Error("emailverify: the verification service failed",
			"status", res.StatusCode, "body", firstLine(answer.Error, body))
		return time.Time{}, ErrUpstream
	}
}

// Confirm honours a return from the provider, exactly once.
//
// The reference is single-use *here* even though the token was single-use
// there: the return address travels in the mail and then through a browser's
// history, so replaying it must not be able to re-assert a verification. The
// claim is one conditional UPDATE, so two clicks arriving together cannot both
// win.
//
// What this proves is bounded, and worth stating plainly: the provider only
// redirects after it has honoured its own token, so a return carrying a live
// reference means the address was confirmed. It is not a server-to-server
// signal — there is no webhook yet — so it is trusted exactly as far as the
// single-use reference allows and no further.
func (s *Service) Confirm(ctx context.Context, ref string) (*Verification, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, ErrLinkSpent
	}
	return s.store.claimVerification(ctx, hashSecret(ref))
}

// Health asks the provider whether it is up. Unauthenticated by its own design.
func (s *Service) Health(ctx context.Context) error {
	return telemetry.ObserveExternal(ctx, telemetry.SystemEmailVerify, "health",
		func(ctx context.Context) error { return s.health(ctx) })
}

func (s *Service) health(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ProviderURL()+"/health", nil)
	if err != nil {
		return err
	}
	res, err := s.http.Do(httpReq)
	if err != nil {
		return ErrUpstream
	}
	defer func() { _ = res.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4<<10))
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: health check answered %d", ErrUpstream, res.StatusCode)
	}
	return nil
}

// checkQuota is the local guard in front of a shared key.
func (s *Service) checkQuota(ctx context.Context, tenantID, address string) error {
	since := time.Now().Add(-time.Hour)

	used, oldest, err := s.store.countTenantSends(ctx, tenantID, since)
	if err != nil {
		return err
	}
	if used >= TenantHourlyLimit {
		return &RateLimitedError{
			RetryAfter: retryAfter(oldest),
			msg:        fmt.Sprintf("this tenant may request %d verifications per hour", TenantHourlyLimit),
		}
	}

	last, err := s.store.lastSendTo(ctx, tenantID, address)
	if err != nil {
		return err
	}
	if last != nil {
		if wait := ResendInterval - time.Since(*last); wait > 0 {
			// The timestamp came from the database's clock and the comparison
			// is against this process's. A database a second ahead would
			// otherwise produce a wait longer than the interval itself.
			if wait > ResendInterval {
				wait = ResendInterval
			}
			return &RateLimitedError{
				RetryAfter: wait,
				msg:        "a verification was just sent to this address",
			}
		}
	}
	return nil
}

// Overview is the settings screen in one request.
func (s *Service) Overview(ctx context.Context, tenantID string, limit int) (*Overview, error) {
	if limit <= 0 || limit > 200 {
		limit = 25
	}
	stats, err := s.store.stats(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	recent, err := s.store.recent(ctx, tenantID, limit)
	if err != nil {
		return nil, err
	}
	if stats.Total > 0 {
		stats.VerifiedPct = float64(stats.Verified) / float64(stats.Total) * 100
	}

	overview := &Overview{
		Stats:       *stats,
		Recent:      recent,
		Configured:  Configured(),
		ProviderURL: ProviderURL(),
		AdminURL:    AdminURL,
		ReturnURL:   ReturnURL(),
	}
	// The health check is a network call on a screen load, so it is bounded
	// tightly: a slow provider should make this screen say "unreachable", not
	// make it hang.
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.Health(healthCtx); err != nil {
		overview.Health = err.Error()
	} else {
		overview.Reachable = true
	}
	return overview, nil
}

// StartHousekeeping ages out links nobody followed and drops history past the
// retention window. Both run until ctx is cancelled at shutdown.
//
// Without a webhook, a link that was confirmed but whose return never reached
// us is indistinguishable from one nobody opened. Both end up EXPIRED here,
// which is the honest reading: this platform did not see it happen.
func (s *Service) StartHousekeeping(ctx context.Context) {
	async.Go("email-verification-housekeeping", func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			s.sweep(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
}

func (s *Service) sweep(ctx context.Context) {
	sweepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if expired, err := s.store.expirePending(sweepCtx); err != nil {
		slog.Warn("emailverify: could not expire stale verifications", "error", err)
	} else if expired > 0 {
		slog.Info("emailverify: expired verification links", "count", expired)
	}
	if purged, err := s.store.purgeOlderThan(sweepCtx, time.Now().Add(-Retention)); err != nil {
		slog.Warn("emailverify: could not purge verification history", "error", err)
	} else if purged > 0 {
		slog.Info("emailverify: purged verification history", "count", purged)
	}
}

// NormalizeEmail accepts one plain address and returns it lowercased.
//
// A display name ("Ops <ops@example.com>") is refused rather than unwrapped.
// The address is handed to a sending service as a recipient, and accepting
// anything with structure in it is how a second recipient gets appended.
func NormalizeEmail(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", invalid("an email address is required")
	}
	if len(trimmed) > 320 {
		return "", invalid("the email address is too long")
	}
	if strings.ContainsAny(trimmed, "\r\n<>,;\"") {
		return "", invalid("the email address is not a plain address")
	}
	address, err := mail.ParseAddress(trimmed)
	if err != nil || address.Name != "" || address.Address != trimmed {
		return "", invalid("the email address is not valid")
	}
	if strings.Count(address.Address, "@") != 1 {
		return "", invalid("the email address is not valid")
	}
	return strings.ToLower(address.Address), nil
}

// ValidateRedirect decides where a person may be sent after they come back.
//
// The provider returns them to this platform; this platform then forwards them
// wherever the calling module asked. That second hop is a redirect issued from
// our own domain, so an unchecked destination makes Gerege Nexus the open
// redirector a phishing link wants to borrow — HTTPS only (HTTP is tolerated
// for localhost outside production, where a developer has no certificate), and
// no embedded credentials.
func ValidateRedirect(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		// No destination is a legitimate choice: this platform answers the
		// click itself and the caller reads the outcome from its own records.
		return "", nil
	}
	if len(trimmed) > 2048 {
		return "", invalid("the redirect URL is too long")
	}
	if strings.ContainsAny(trimmed, "\r\n") {
		return "", invalid("the redirect URL is not valid")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return "", invalid("the redirect URL must be absolute")
	}
	if parsed.User != nil {
		return "", invalid("the redirect URL must not carry credentials")
	}
	host := strings.ToLower(parsed.Hostname())
	localhost := host == "localhost" || host == "127.0.0.1" || host == "::1"
	switch {
	case parsed.Scheme == "https":
	case parsed.Scheme == "http" && localhost && !config.IsProduction():
	default:
		return "", invalid("the redirect URL must use HTTPS (HTTP is allowed only for localhost in development)")
	}

	// HTTPS was never the part that mattered. The link goes out in a mail, over
	// this platform's name, and /verify/landed then forwards the person
	// wherever it says — so an unchecked destination makes nexus.gerege.mn the
	// redirector a phishing link wants to borrow, and the recipient sees a
	// government hostname in the mail they were told to trust.
	//
	// Any signed-in member of any tenant could choose that destination. The
	// allowlist moves the choice to whoever operates the deployment.
	if !config.HostAllowed(host, config.RedirectHosts(redirectHostsEnv)) {
		return "", invalid(
			"the redirect URL must point at this platform or a host named in %s (%s does not qualify)",
			redirectHostsEnv, host)
	}
	return trimmed, nil
}

// redirectHostsEnv names the extra hostnames a verification link may return to.
// PUBLIC_ORIGIN is always allowed without being listed.
const redirectHostsEnv = "EMAIL_VERIFY_REDIRECT_HOSTS"

// retryAfter turns the oldest request inside the window into how long until it
// leaves the window and frees an allowance.
func retryAfter(oldest *time.Time) time.Duration {
	if oldest == nil {
		return time.Minute
	}
	wait := time.Until(oldest.Add(time.Hour))
	if wait < time.Minute {
		return time.Minute
	}
	// The timestamp is the database's, the comparison is this process's; a skew
	// between them must not turn into a wait longer than the window.
	if wait > time.Hour {
		return time.Hour
	}
	return wait
}

// retryAfterHeader reads the provider's own Retry-After when it sent one, so a
// caller is told the real wait rather than our guess at it.
func retryAfterHeader(res *http.Response, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(res.Header.Get("Retry-After"))
	if raw == "" {
		return fallback
	}
	if seconds, err := time.ParseDuration(raw + "s"); err == nil && seconds > 0 {
		return seconds
	}
	if when, err := http.ParseTime(raw); err == nil {
		if wait := time.Until(when); wait > 0 {
			return wait
		}
	}
	return fallback
}

// firstLine keeps a provider's message out of a browser when it is not a short,
// recognisable code — a stack trace or an HTML error page is for the log.
func firstLine(code string, body []byte) string {
	if code != "" {
		return code
	}
	text := strings.TrimSpace(string(body))
	if index := strings.IndexAny(text, "\r\n"); index >= 0 {
		text = text[:index]
	}
	return truncate(text, 200)
}

// randomRef mints the single-use reference carried in the return address.
func randomRef() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("emailverify: could not read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// hashSecret is what the database stores for the reference. SHA-256 rather than
// bcrypt on purpose: it is a full-entropy random string, not a password, so
// there is nothing for a work factor to defend against.
func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	// Back up to the start of the rune the cut landed inside; Purpose is prose
	// a caller writes, and Cyrillic cut mid-rune is stored as mojibake.
	for max > 0 && !utf8.RuneStart(value[max]) {
		max--
	}
	return value[:max]
}
