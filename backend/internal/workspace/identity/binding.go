/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Binding a first sign-in from an external provider to a national identity.
 *
 * Google can say which Google account somebody holds. It cannot say who they
 * are, and this platform's accounts are held by people who have proved that
 * with eID. So a Google identity nobody here recognises is not refused any
 * more, and it is not admitted either: it is parked, the person is shown what
 * each side hands over, and they are asked to prove themselves once with eID.
 * From then on Google alone is enough, because the link exists.
 *
 * The order matters. Consent is taken before eID is touched — a push
 * notification at somebody's phone is not the moment to explain what is about
 * to be shared — and the account is created only when both halves are done, so
 * an abandoned binding leaves nothing behind but a row that expires.
 */

package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/ssoclient"
	"github.com/jackc/pgx/v5"
)

// bindingTTL is how long a parked identity stays redeemable. It has to cover
// reading a consent screen and reaching for a phone, and no longer: the row
// holds verified claims about somebody, and one that outlives the attempt is
// just a copy of their identity sitting in a table.
const bindingTTL = 20 * time.Minute

// startIdentityBinding parks a verified provider identity and returns the
// token that names it. The token is shown once, in the redirect; only its
// digest is stored, so the row cannot be replayed out of a database copy.
func (h *Handlers) startIdentityBinding(ctx context.Context, issuer string, identity *ssoclient.Identity) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	sum := sha256.Sum256([]byte(token))

	if _, err := h.db.Exec(ctx,
		`INSERT INTO registry.identity_binding_sessions
		     (token_hash, issuer, subject, email, name, claims, expires_at)
		 VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,$7)`,
		hex.EncodeToString(sum[:]), issuer, identity.Subject,
		identity.Email, identity.Name, claimsJSON(identity.Claims),
		time.Now().Add(bindingTTL)); err != nil {
		return "", err
	}
	return token, nil
}

// pendingBinding is one parked identity, as the server holds it.
type pendingBinding struct {
	TokenHash string
	Issuer    string
	Subject   string
	Email     string
	Name      string
	Claims    []byte
	Consented bool
}

// ErrNoBinding covers every reason a token does not name a live binding —
// unknown, spent, or expired. They are one answer on purpose: which of the
// three it is tells a caller something they have no use for.
var ErrNoBinding = errors.New("no sign-in is waiting to be completed")

func (h *Handlers) loadBinding(ctx context.Context, token string) (*pendingBinding, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrNoBinding
	}
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])

	var b pendingBinding
	var email, name *string
	var consentedAt *time.Time
	err := h.db.QueryRow(ctx,
		`SELECT token_hash, issuer, subject, email, name, claims, consented_at
		   FROM registry.identity_binding_sessions
		  WHERE token_hash = $1 AND expires_at > NOW()`, hash).
		Scan(&b.TokenHash, &b.Issuer, &b.Subject, &email, &name, &b.Claims, &consentedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoBinding
	}
	if err != nil {
		return nil, err
	}
	if email != nil {
		b.Email = *email
	}
	if name != nil {
		b.Name = *name
	}
	b.Consented = consentedAt != nil
	return &b, nil
}

// HandleBindingSession describes the pending binding to the screen that has to
// render it.
//
// Unauthenticated by definition — nobody is signed in yet — and the token in
// the query is the whole authority. What it answers with is what the provider
// said about the person holding that token, which is what they are about to be
// asked to consent to sharing; it discloses nothing they did not just supply.
func (h *Handlers) HandleBindingSession(w http.ResponseWriter, r *http.Request) {
	binding, err := h.loadBinding(r.Context(), r.URL.Query().Get("b"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "no sign-in is waiting to be completed")
		return
	}

	var claims map[string]any
	_ = json.Unmarshal(binding.Claims, &claims)

	httpx.JSON(w, http.StatusOK, map[string]any{
		"provider":  h.BindingProviderName(binding.Issuer),
		"email":     binding.Email,
		"name":      binding.Name,
		"consented": binding.Consented,
		// What the provider actually said, so the consent screen can show it
		// rather than describe it. A person agreeing to share something is
		// entitled to see the thing.
		"claims": claims,
		// What eID will be asked for, named here so the same screen can state
		// both halves before either happens.
		"eid_claims": []string{
			"Регистрийн дугаар", "Овог, нэр", "Иргэний бүртгэлийн дугаар",
		},
	})
}

func (h *Handlers) BindingProviderName(issuer string) string {
	if h.GoogleLoginEnabled() && issuer == h.googleLogin.Config().Issuer {
		return h.googleLogin.Config().DisplayName()
	}
	if h.SsoClientEnabled() && issuer == h.ssoClient.Config().Issuer {
		return h.ssoClient.Config().DisplayName()
	}
	return issuer
}

// HandleBindingConsent records that the person agreed to what the screen
// showed them.
//
// Server-side, and required before eID can be started. A consent that lived in
// the browser would be a claim by the caller that they had consented, which is
// not the same thing and not worth recording.
func (h *Handlers) HandleBindingConsent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Binding string `json:"binding"`
	}
	if httpx.DecodeLimited(r, &req, 8<<10) != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	binding, err := h.loadBinding(r.Context(), req.Binding)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "no sign-in is waiting to be completed")
		return
	}
	if _, err := h.db.Exec(r.Context(),
		`UPDATE registry.identity_binding_sessions SET consented_at = NOW()
		  WHERE token_hash = $1 AND consented_at IS NULL`, binding.TokenHash); err != nil {
		slog.Error("could not record a binding consent", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "could not record the consent")
		return
	}
	audit.Record(r.Context(), "unknown", "anonymous", "auth.binding_consented", "identity",
		map[string]any{"issuer": binding.Issuer, "email": binding.Email})
	httpx.JSON(w, http.StatusOK, map[string]any{"consented": true})
}

// HandleBindingEIDStart begins the eID half, once consent is recorded.
func (h *Handlers) HandleBindingEIDStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Binding    string `json:"binding"`
		NationalID string `json:"national_id"`
	}
	if httpx.DecodeLimited(r, &req, 8<<10) != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	binding, err := h.loadBinding(r.Context(), req.Binding)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "no sign-in is waiting to be completed")
		return
	}
	if !binding.Consented {
		httpx.Error(w, http.StatusForbidden, "the sharing has not been agreed to yet")
		return
	}

	if id := strings.TrimSpace(req.NationalID); id != "" {
		started, err := h.eidSvc.StartByNationalID(r.Context(), id, "")
		if err != nil {
			httpx.Error(w, http.StatusBadRequest,
				"Регистрийн дугаар олдсонгүй эсвэл eID апп-д бүртгэлгүй байна")
			return
		}
		httpx.JSON(w, http.StatusOK, started)
		return
	}
	started, err := h.eidSvc.StartDeviceLink(r.Context(), "")
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "eID Mongolia session could not be started")
		return
	}
	httpx.JSON(w, http.StatusOK, started)
}

// HandleBindingEIDPoll finishes the binding when eID answers.
//
// This is the only place an account is created for a first Google sign-in, and
// it happens after both halves are done: the person agreed to the sharing, and
// eID says they are who the registration number belongs to.
func (h *Handlers) HandleBindingEIDPoll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Binding   string `json:"binding"`
		SessionID string `json:"session_id"`
	}
	if httpx.DecodeLimited(r, &req, 8<<10) != nil || strings.TrimSpace(req.SessionID) == "" {
		httpx.Error(w, http.StatusBadRequest, "session_id is required")
		return
	}
	binding, err := h.loadBinding(r.Context(), req.Binding)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "no sign-in is waiting to be completed")
		return
	}
	if !binding.Consented {
		httpx.Error(w, http.StatusForbidden, "the sharing has not been agreed to yet")
		return
	}

	result, err := h.eidSvc.Poll(r.Context(), req.SessionID)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "eID Mongolia session check failed")
		return
	}
	if result.State != "COMPLETE" {
		httpx.JSON(w, http.StatusOK, result)
		return
	}
	if result.Identity == nil || !result.Identity.VerifiedStatus {
		httpx.Error(w, http.StatusUnauthorized, "eID identity verification failed")
		return
	}

	// eID decides which account this is. A citizen who already has one here is
	// linked to it rather than given a second — which is the case that makes
	// this flow worth having: somebody who signed in with eID last month and
	// with Google today is one person, and now the platform knows it.
	userID, tenantID, err := h.authn.ResolveOrProvisionEIDUser(r.Context(), result.Identity)
	if err != nil {
		auth.ReportSignInFailure(w, err)
		return
	}
	h.authn.LinkEIDIdentity(r.Context(), userID, result.Identity)

	// And the provider identity that started all this, with everything it said.
	var claims map[string]any
	_ = json.Unmarshal(binding.Claims, &claims)
	h.linkSSOIdentity(r.Context(), userID, binding.Issuer, &ssoclient.Identity{
		Subject: binding.Subject,
		Email:   binding.Email,
		Name:    binding.Name,
		Claims:  claims,
	})

	// Spent. The row held verified claims about somebody and has no further
	// use; leaving it would be leaving a copy of their identity behind.
	if _, err := h.db.Exec(r.Context(),
		`DELETE FROM registry.identity_binding_sessions WHERE token_hash = $1`, binding.TokenHash); err != nil {
		slog.Warn("could not clear a spent binding", "error", err)
	}

	token, expiresAt, err := h.authn.IssueSession(r, userID, tenantID, "bind-eid")
	if err != nil {
		auth.ReportSessionFailure(w, err)
		return
	}
	auth.SetSessionCookie(w, token, expiresAt)
	audit.Record(r.Context(), tenantID, userID, "auth.identity_bound", "identity", map[string]any{
		"issuer": binding.Issuer, "email": binding.Email, "verified_by": "eid",
	})
	httpx.JSON(w, http.StatusOK, map[string]any{
		"state": result.State, "expires_at": expiresAt, "bound": true,
	})
}

// SweepExpiredBindings clears abandoned attempts. Nothing depends on a dead
// row being gone promptly — it cannot be redeemed once it has expired — but it
// carries verified claims about a person, so it does not get to sit there.
func (h *Handlers) SweepExpiredBindings(ctx context.Context) {
	if _, err := h.db.Exec(ctx,
		`DELETE FROM registry.identity_binding_sessions WHERE expires_at < NOW()`); err != nil {
		slog.Warn("could not sweep expired identity bindings", "error", err)
	}
}
