/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package identity

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/dan"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/eid"
)

func (h *Handlers) HandleEIDStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CallbackURL string `json:"callbackUrl"`
	}
	if r.Body != nil {
		// An absent body is supported by QR clients; a supplied body must be valid.
		if err := httpx.DecodeLimited(r, &req, 8<<10); err != nil && !errors.Is(err, io.EOF) {
			httpx.Error(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	callback, err := validEIDCallback(req.CallbackURL)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid eID callback URL")
		return
	}
	started, err := h.eidSvc.StartDeviceLink(r.Context(), callback)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "eID Mongolia session could not be started")
		return
	}
	httpx.JSON(w, http.StatusOK, started)
}

func (h *Handlers) HandleEIDStartByNationalID(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NationalID  string `json:"national_id"`
		CallbackURL string `json:"callbackUrl"`
	}
	if httpx.DecodeLimited(r, &req, 8<<10) != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	callback, err := validEIDCallback(req.CallbackURL)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid eID callback URL")
		return
	}
	started, err := h.eidSvc.StartByNationalID(r.Context(), req.NationalID, callback)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "Регистрийн дугаар олдсонгүй эсвэл eID апп-д бүртгэлгүй байна")
		return
	}
	httpx.JSON(w, http.StatusOK, started)
}

func validEIDCallback(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	// A native client comes back through its own URL scheme, not over https, so
	// it can never satisfy the origin check below. Those are allowed by exact
	// match from EID_APP_CALLBACKS (comma-separated) and nothing else: no
	// parsing, no normalisation, no prefix rules — the whole string or nothing.
	//
	// Listing one here is half the job. Its scheme (for this callback,
	// `gerege-nexus://`) has to be registered on the eID Mongolia side against
	// this deployment's RP callback_hosts, or eID drops it silently and the
	// citizen simply stays in that app.
	for _, allowed := range strings.Split(os.Getenv("EID_APP_CALLBACKS"), ",") {
		if allowed = strings.TrimSpace(allowed); allowed != "" && allowed == raw {
			return raw, nil
		}
	}
	callback, err := url.Parse(raw)
	if err != nil || callback.User != nil || (callback.Scheme != "https" && (config.IsProduction() || callback.Scheme != "http")) {
		return "", errors.New("invalid callback")
	}
	publicOrigin := strings.TrimSpace(os.Getenv("PUBLIC_ORIGIN"))
	if publicOrigin == "" {
		publicOrigin = "http://localhost:3000"
	}
	origin, err := url.Parse(publicOrigin)
	if err != nil || !strings.EqualFold(callback.Host, origin.Host) || callback.Path != "/auth/eid/callback" {
		return "", errors.New("callback not allowed")
	}
	return callback.String(), nil
}

func (h *Handlers) HandleEIDPoll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
	}
	if httpx.DecodeLimited(r, &req, 8<<10) != nil || strings.TrimSpace(req.SessionID) == "" {
		httpx.Error(w, http.StatusBadRequest, "session_id is required")
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
	userID, tenantID, err := h.authn.ResolveOrProvisionEIDUser(r.Context(), result.Identity)
	if err != nil {
		auth.ReportSignInFailure(w, err)
		return
	}
	h.authn.LinkEIDIdentity(r.Context(), userID, result.Identity)
	token, expiresAt, err := h.authn.IssueSession(r, userID, tenantID, "eid-app")
	if err != nil {
		auth.ReportSessionFailure(w, err)
		return
	}
	auth.SetSessionCookie(w, token, expiresAt)
	audit.Record(r.Context(), tenantID, userID, "auth.eid_app_login_success", "eid", map[string]any{"verified": true, "method": "eid-app"})
	httpx.JSON(w, http.StatusOK, map[string]any{"state": result.State, "expires_at": expiresAt, "identity": result.Identity})
}

func (h *Handlers) HandleEIDLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code        string         `json:"code"`
		RedirectURI string         `json:"redirect_uri"`
		RegNumber   string         `json:"reg_number"`
		OTPCode     string         `json:"otp_code"`
		AuthMethod  eid.AuthMethod `json:"auth_method"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}

	var identity *eid.EIDIdentity
	var err error
	if req.Code != "" {
		identity, err = h.eidSvc.ExchangeCode(r.Context(), req.Code, req.RedirectURI)
	} else if req.RegNumber != "" {
		identity, err = h.eidSvc.AuthenticateWithMethod(r.Context(), req.RegNumber, req.OTPCode, req.AuthMethod)
	} else {
		httpx.Error(w, http.StatusBadRequest, "missing authorization code or registration number")
		return
	}

	// err may be nil while identity is nil — calling err.Error() unguarded
	// panicked the request goroutine.
	//
	// The reason is logged, not rendered. eID's own failures arrive with the
	// provider's words in them — `E-ID token error (400): {…}` carries the
	// upstream response body verbatim (identity/eid/eid.go) — and this is the
	// path a citizen stands in front of. It is the same mistake that once put
	// "bcrypt: password length exceeds 72 bytes" on somebody's card screen,
	// which is why auth.ReportSignInFailure exists at all.
	if err != nil || identity == nil {
		if err != nil {
			slog.Warn("eID verification failed", "error", err)
		}
		telemetry.RecordLogin(telemetry.LoginEID, false)
		httpx.Error(w, http.StatusUnauthorized, "E-ID verification failed")
		return
	}

	userID, tenantID, err := h.authn.ResolveOrProvisionEIDUser(r.Context(), identity)
	if err != nil {
		auth.ReportSignInFailure(w, err)
		return
	}
	h.authn.LinkEIDIdentity(r.Context(), userID, identity)

	token, expiresAt, err := h.authn.IssueSession(r, userID, tenantID, "eid")
	if err != nil {
		auth.ReportSessionFailure(w, err)
		return
	}
	auth.SetSessionCookie(w, token, expiresAt)

	telemetry.RecordLogin(telemetry.LoginEID, true)
	audit.Record(r.Context(), tenantID, userID, "auth.eid_login_success", "eid", map[string]any{
		"reg_number": identity.RegNumber,
		"civil_id":   identity.CivilID,
	})

	claims, _ := h.sessions.Resolve(r.Context(), token)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"expires_at": expiresAt,
		"identity":   identity,
		"user": map[string]any{
			"id":        userID,
			"tenant_id": tenantID,
			"name":      identity.FirstName + " " + identity.LastName,
			"email":     claims.Email,
			"is_admin":  claims.IsAdmin,
		},
	})
}

func (h *Handlers) HandleDANLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DANToken  string `json:"dan_token"`
		RegNumber string `json:"reg_number"`
		OTPCode   string `json:"otp_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}

	var profile *dan.DANProfile
	var err error
	if req.DANToken != "" {
		profile, err = h.danSvc.VerifyDANToken(r.Context(), req.DANToken)
	} else if req.RegNumber != "" {
		profile, err = h.danSvc.AuthenticateDANCitizen(r.Context(), req.RegNumber, req.OTPCode)
	} else {
		httpx.Error(w, http.StatusBadRequest, "missing dan_token or registration number")
		return
	}

	if err != nil || profile == nil {
		msg := "dan.gerege.mn verification failed"
		if err != nil {
			msg = "dan.gerege.mn verification failed: " + err.Error()
		}
		telemetry.RecordLogin(telemetry.LoginDAN, false)
		httpx.Error(w, http.StatusUnauthorized, msg)
		return
	}

	identity := &eid.EIDIdentity{
		CivilID: profile.CivilID, RegNumber: profile.RegNumber, FirstName: profile.FirstName, LastName: profile.LastName,
		VerifiedStatus: true,
	}
	userID, tenantID, err := h.authn.ResolveOrProvisionEIDUser(r.Context(), identity)
	if err != nil {
		auth.ReportSignInFailure(w, err)
		return
	}
	h.authn.LinkEIDIdentity(r.Context(), userID, identity)

	token, expiresAt, err := h.authn.IssueSession(r, userID, tenantID, "dan")
	if err != nil {
		auth.ReportSessionFailure(w, err)
		return
	}
	auth.SetSessionCookie(w, token, expiresAt)

	telemetry.RecordLogin(telemetry.LoginDAN, true)
	audit.Record(r.Context(), tenantID, userID, "auth.dan_gerege_login_success", "dan", map[string]any{
		"reg_number":  profile.RegNumber,
		"dan_session": profile.DANSessionID,
	})

	claims, _ := h.sessions.Resolve(r.Context(), token)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"expires_at":  expiresAt,
		"dan_profile": profile,
		"user": map[string]any{
			"id":        userID,
			"tenant_id": tenantID,
			"name":      profile.FirstName + " " + profile.LastName,
			"email":     claims.Email,
			"is_admin":  claims.IsAdmin,
		},
	})
}
