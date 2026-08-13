/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 */

package ssoclient

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

// CookieSpec names a cookie and the path it is confined to.
//
// A deployment can have more than one sign-in in flight at once — it may
// federate to a provider *and* offer Google — and two flows sharing one cookie
// would mean starting the second silently discarded the first. One spec per
// flow keeps them independent, and keeps each confined to the callback that
// needs it.
type CookieSpec struct{ Name, Path string }

// The flows this package serves, and the cookie each parks its state in.
//
// Neither is sent with an ordinary API call: each is scoped to the one endpoint
// that reads it. FederationFlow reaches only the federated callback,
// GoogleFlow only Google's, and IDTokenCookie only the logout endpoint — an
// id_token riding along on every /auth/me would be a kilobyte of nothing on
// the platform's most frequent request.
var (
	FederationFlow = CookieSpec{Name: "sso_flow", Path: "/api/v1/auth/sso"}
	GoogleFlow     = CookieSpec{Name: "google_flow", Path: "/api/v1/auth/google"}
)

const (
	// #nosec G101 -- a cookie name, not a credential. gosec matches the
	// identifier rather than the value, and every honest name for "the cookie
	// the id_token is kept in" has "token" in it.
	IDTokenCookieName = "sso_id_token"
	// #nosec G101 -- and this one is a URL path, for the same reason.
	IDTokenCookiePath = "/api/v1/auth/logout"
)

// flowTTL bounds how long a started sign-in may take to come back. It is the
// time between clicking "sign in" and finishing at the provider — long enough
// to read a consent screen and approve a push on a phone, short enough that an
// abandoned attempt does not stay redeemable.
const flowTTL = 15 * time.Minute

// Flow is the state of one sign-in in progress, parked in a cookie between the
// redirect out and the callback back.
//
// A cookie rather than a database row deliberately: the row would be written on
// every click of a sign-in button, including every crawler and every abandoned
// attempt, and would need its own sweep. What the row would buy — that the
// state cannot be tampered with — a cookie also buys, because it is HttpOnly
// and set by this server: a page on another site can neither read it nor
// replace it, and a caller who edits their own copy only breaks their own
// sign-in.
type Flow struct {
	State        string `json:"state"`
	Nonce        string `json:"nonce"`
	CodeVerifier string `json:"code_verifier"`
	// Next is where in the app the person was heading. Always a path on this
	// deployment; see SafeNext.
	Next string `json:"next"`
	// Link marks a flow started by somebody who is already signed in and is
	// adding a provider to the account they are in, rather than using one to
	// get in. The cookie is not signed, so this is a statement of intent and
	// not an authorisation: the callback still requires a live session and
	// attaches the result to *that* session's account. Somebody who forges it
	// can only connect their own provider account to their own record, which
	// is the feature.
	Link bool `json:"link,omitempty"`
}

// ErrNoFlow means the callback arrived without the cookie that started it —
// usually a bookmarked callback URL, an expired attempt, or a browser that
// dropped the cookie between the two requests.
var ErrNoFlow = errors.New("no sign-in is in progress in this browser")

// ErrStateMismatch means the callback's state is not the one this browser
// started with. That is the check that makes the callback answerable only to a
// sign-in this deployment began, and the reason a link to the callback URL
// cannot be used to sign somebody in.
var ErrStateMismatch = errors.New("the sign-in answer does not match the request that started it")

// SetFlowCookie parks a pending sign-in in the browser.
func SetFlowCookie(w http.ResponseWriter, spec CookieSpec, flow Flow) {
	encoded, err := json.Marshal(flow)
	if err != nil {
		// Four strings; encoding them cannot fail.
		panic("ssoclient: encode flow: " + err.Error())
	}
	http.SetCookie(w, &http.Cookie{
		Name:     spec.Name,
		Value:    base64.RawURLEncoding.EncodeToString(encoded),
		Path:     spec.Path,
		MaxAge:   int(flowTTL.Seconds()),
		HttpOnly: true,
		Secure:   isProduction(),
		// Lax, not Strict. The callback is a top-level navigation arriving from
		// the provider's origin, which is by definition cross-site, and a
		// Strict cookie is not sent on one — the flow would lose its own state
		// on the single request that needs it. Lax is what "sent on a top-level
		// navigation, and nothing else" means, which is exactly this case.
		SameSite: http.SameSiteLaxMode,
	})
}

// ReadFlow takes the pending sign-in out of the browser and checks it answers
// the state that has come back. The cookie is cleared whatever the outcome: a
// flow is single-use, and one left behind after a failure is one that can be
// retried by somebody who kept the code.
func ReadFlow(w http.ResponseWriter, r *http.Request, spec CookieSpec, state string) (Flow, error) {
	ClearFlowCookie(w, spec)

	cookie, err := r.Cookie(spec.Name)
	if err != nil || cookie.Value == "" {
		return Flow{}, ErrNoFlow
	}
	raw, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return Flow{}, ErrNoFlow
	}
	var flow Flow
	if err := json.Unmarshal(raw, &flow); err != nil {
		return Flow{}, ErrNoFlow
	}
	if flow.State == "" || subtle.ConstantTimeCompare([]byte(flow.State), []byte(state)) != 1 {
		return Flow{}, ErrStateMismatch
	}
	return flow, nil
}

// ClearFlowCookie removes a pending sign-in.
func ClearFlowCookie(w http.ResponseWriter, spec CookieSpec) {
	http.SetCookie(w, &http.Cookie{
		Name: spec.Name, Value: "", Path: spec.Path,
		MaxAge: -1, HttpOnly: true, Secure: isProduction(), SameSite: http.SameSiteLaxMode,
	})
}

// SetIDTokenCookie keeps the provider's id_token for the length of the session,
// so that signing out can present it as an id_token_hint. Providers differ on
// whether they need one; the ones that do will not end a session without it.
func SetIDTokenCookie(w http.ResponseWriter, idToken string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     IDTokenCookieName,
		Value:    idToken,
		Path:     IDTokenCookiePath,
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   isProduction(),
		SameSite: http.SameSiteLaxMode,
	})
}

// IDTokenFromRequest reads the stored hint, if this session has one.
func IDTokenFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(IDTokenCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// ClearIDTokenCookie drops the stored hint. Called on the way out, once it has
// been spent on the logout redirect.
func ClearIDTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: IDTokenCookieName, Value: "", Path: IDTokenCookiePath,
		MaxAge: -1, HttpOnly: true, Secure: isProduction(), SameSite: http.SameSiteLaxMode,
	})
}

// SafeNext narrows a requested destination to a path on this deployment.
//
// The value comes off a query string, so it is whatever anybody put there. A
// bare path is the only shape that cannot leave: "//evil.example" is a
// protocol-relative URL a browser will happily follow off-site, and a
// backslash is treated as a slash by enough browsers to be worth refusing too.
func SafeNext(raw, fallback string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "/") ||
		strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "/\\") {
		return fallback
	}
	return raw
}

func isProduction() bool { return os.Getenv("ENVIRONMENT") == "production" }
