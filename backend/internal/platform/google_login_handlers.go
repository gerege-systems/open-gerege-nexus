/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Signing in with Google.
 *
 * This is not the federation next door in sso_client_handlers.go, and the
 * difference is the whole design. Federation replaces this deployment's idea of
 * who people are: name a provider and the local sign-in paths close. Google is
 * an *additional* button on this platform's own screen, sitting beside eID and
 * the administrator's password, and it closes nothing.
 *
 * The protocol underneath is identical — Google is an ordinary OpenID Connect
 * provider — so both paths run the same discovery, the same PKCE, the same
 * id_token verification, and land on the same account resolution. What is
 * written twice is only what differs: which cookie the flow parks in, and who
 * is allowed through.
 */

package platform

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/observability"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/ssoclient"
)

// googleLoginEnabled reports whether this deployment offers the Google button.
func (s *Server) googleLoginEnabled() bool {
	return s.googleLogin != nil && s.googleLogin.Config().Enabled()
}

// googleStartURL is where the sign-in screen sends somebody who presses it.
func (s *Server) googleStartURL() string {
	return config.SelfOrigin() + "/api/v1/auth/google/start"
}

// handleGoogleStart begins a sign-in at Google.
func (s *Server) handleGoogleStart(w http.ResponseWriter, r *http.Request) {
	if !s.googleLoginEnabled() {
		httpx.Error(w, http.StatusNotFound, "Google sign-in is not configured on this deployment")
		return
	}
	// Federation closing the local front door closes this too. Google here is
	// one of *this* platform's ways of establishing who somebody is, and a
	// deployment that has handed that question to a provider must not keep a
	// second answer to it — see requireLocalLogin.
	if !s.localLoginAllowed() {
		httpx.JSON(w, http.StatusForbidden, map[string]any{
			"error":     "this deployment signs in through its SSO provider",
			"code":      "sso_required",
			"start_url": s.ssoStartURL(),
		})
		return
	}

	request, err := s.googleLogin.BeginAuthorization(r.Context())
	if err != nil {
		slog.Error("could not start a sign-in at Google", "error", err)
		s.failGoogle(w, r, "provider_unreachable")
		return
	}

	ssoclient.SetFlowCookie(w, ssoclient.GoogleFlow, ssoclient.Flow{
		State:        request.State,
		Nonce:        request.Nonce,
		CodeVerifier: request.CodeVerifier,
		Next:         ssoclient.SafeNext(r.URL.Query().Get("next"), defaultLandingPath),
	})
	http.Redirect(w, r, request.URL, http.StatusFound)
}

// handleGoogleLinkStart adds Google to the account somebody is already in.
//
// The same authorisation request as signing in, with one difference in what
// the answer means: the person's identity is not in question here, so there is
// nothing for eID to establish. They proved who they are when they signed in;
// this only records that a particular Google account is also theirs.
//
// It is a navigation rather than a fetch because it ends at Google. The
// session cookie rides along on a same-origin top-level GET, which is what
// makes the callback able to tell whose account to attach the result to.
func (s *Server) handleGoogleLinkStart(w http.ResponseWriter, r *http.Request) {
	// Every refusal here is a redirect rather than a status code. This handler
	// is reached by pressing a button, not by a fetch — the browser navigates
	// to it — so a JSON body would replace the person's screen with a line of
	// machine-readable text and no way back.
	if !s.googleLoginEnabled() {
		s.failGoogleLink(w, r, "google_not_configured")
		return
	}
	// A linked Google account is a way in, not merely a label on a profile —
	// the sign-in path will find it tomorrow. So a deployment that has handed
	// the question of who somebody is to its provider must not let people cut
	// themselves a second door from inside, which is the same reason
	// handleGoogleStart refuses.
	if !s.localLoginAllowed() {
		s.failGoogleLink(w, r, "sso_required")
		return
	}
	// Checked before the trip to Google rather than only on the way back. The
	// callback has to verify it again — a session can end mid-flow — but
	// sending somebody through a consent screen that cannot possibly succeed
	// is a worse way to say "you are not signed in".
	if _, err := s.sessions.Resolve(r.Context(), auth.TokenFromRequest(r)); err != nil {
		slog.Info("a Google link was started without a live session", "error", err)
		s.failGoogleLink(w, r, "session_expired")
		return
	}

	request, err := s.googleLogin.BeginAuthorization(r.Context())
	if err != nil {
		slog.Error("could not start a Google link", "error", err)
		s.failGoogleLink(w, r, "provider_unreachable")
		return
	}

	ssoclient.SetFlowCookie(w, ssoclient.GoogleFlow, ssoclient.Flow{
		State:        request.State,
		Nonce:        request.Nonce,
		CodeVerifier: request.CodeVerifier,
		Next:         "/profile",
		Link:         true,
	})
	http.Redirect(w, r, request.URL, http.StatusFound)
}

// linkGoogleToCurrentAccount finishes a flow that began on the profile screen.
//
// Split out of the callback because it answers a different question. Signing
// in asks "whose account is this?", and the answer may be nobody's — which is
// what sends a first-time arrival to eID. Linking already knows whose account
// it is and only asks whether this Google account is free to attach.
func (s *Server) linkGoogleToCurrentAccount(w http.ResponseWriter, r *http.Request, identity *ssoclient.Identity, next string) {
	// Resolved here rather than read from the context: the callback is a public
	// route, because Google has to be able to reach it, so nothing upstream has
	// established who is asking. The session cookie arrives on this navigation
	// like any other same-origin GET, and it is the only thing that decides
	// whose account this attaches to.
	claims, err := s.sessions.Resolve(r.Context(), auth.TokenFromRequest(r))
	if err != nil {
		// The session ended somewhere between the profile screen and Google's
		// answer. Nothing is linked, and saying so is better than silently
		// turning this into a sign-in for whoever holds the browser.
		slog.Info("a Google link came back without a live session", "error", err)
		s.failGoogleLink(w, r, "session_expired")
		return
	}

	issuer := s.googleLogin.Config().Issuer

	// Refuse rather than move. The insert this would otherwise reach reassigns
	// user_id on conflict, which is right when somebody signs in and wrong
	// here: it would take a Google account off whoever it belongs to, without
	// telling them, at the request of somebody else who happens to be able to
	// authenticate at Google as them.
	var owner string
	err = s.db.QueryRow(r.Context(),
		`SELECT user_id::text FROM user_sso_identities WHERE issuer = $1 AND subject = $2`,
		issuer, identity.Subject).Scan(&owner)
	switch {
	case err == nil && owner != claims.UserID:
		slog.Info("refused to move a Google identity between accounts",
			"issuer", issuer, "requested_by", claims.UserID)
		s.failGoogleLink(w, r, "already_linked_elsewhere")
		return
	case err == nil:
		// Already theirs. Re-running the flow refreshes what Google says about
		// them, which is a reasonable thing to want and not an error.
	}

	s.linkSSOIdentity(r.Context(), claims.UserID, issuer, identity)
	slog.Info("a person linked Google to their account", "user_id", claims.UserID)
	http.Redirect(w, r, config.WebOrigin()+next, http.StatusFound)
}

// handleGoogleCallback is where Google returns the browser.
func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if !s.googleLoginEnabled() || !s.localLoginAllowed() {
		httpx.Error(w, http.StatusNotFound, "Google sign-in is not configured on this deployment")
		return
	}
	query := r.URL.Query()

	flow, err := ssoclient.ReadFlow(w, r, ssoclient.GoogleFlow, query.Get("state"))
	if err != nil {
		slog.Info("refused a Google callback", "error", err)
		s.failGoogle(w, r, "stale_request")
		return
	}
	if providerError := query.Get("error"); providerError != "" {
		slog.Info("Google refused a sign-in", "error", providerError)
		s.failGoogle(w, r, providerError)
		return
	}
	code := query.Get("code")
	if code == "" {
		s.failGoogle(w, r, "no_code")
		return
	}

	identity, err := s.googleLogin.Exchange(r.Context(), code, flow.CodeVerifier, flow.Nonce)
	if err != nil {
		slog.Error("could not redeem a Google authorization code", "error", err)
		s.failGoogle(w, r, "exchange_failed")
		return
	}

	// Google is asked for the email scope and always returns the claim, so an
	// unverified address here is a real answer rather than a missing one — and
	// it is the answer that matters most, because the address is what an
	// existing local account is matched on. An unverified one would let anybody
	// who can type an address into a Google profile claim somebody else's
	// account here.
	if identity.Email == "" || !identity.EmailVerified {
		slog.Info("refused a Google sign-in with no verified address", "subject", identity.Subject)
		s.failGoogle(w, r, "email_unverified")
		return
	}
	if !ssoclient.EmailInDomains(identity.Email, ssoclient.GoogleAllowedDomains()) {
		slog.Info("refused a Google sign-in from a domain that is not allowed here",
			"domain", domainOf(identity.Email))
		s.failGoogle(w, r, "domain_not_allowed")
		return
	}

	// Adding a provider to an account rather than using one to reach it. The
	// verification checks above still applied: an address Google has not
	// verified is no more usable as a label on somebody's profile than it is
	// as a way in.
	if flow.Link {
		s.linkGoogleToCurrentAccount(w, r, identity, ssoclient.SafeNext(flow.Next, "/profile"))
		return
	}

	userID, tenantID, err := s.resolveOrProvisionSSOUser(r.Context(), s.googleLogin.Config(), identity)
	if err != nil {
		var refusal signInError
		if errors.As(err, &refusal) {
			// Not a refusal any more. Google has said which Google account this
			// is; it cannot say who the person is, and that is what this
			// platform's accounts are held by. So the identity is parked and
			// they are asked to prove themselves once with eID — see
			// identity_binding.go.
			token, bindErr := s.startIdentityBinding(r.Context(), s.googleLogin.Config().Issuer, identity)
			if bindErr != nil {
				slog.Error("could not start an identity binding", "error", bindErr)
				s.failGoogle(w, r, "no_account")
				return
			}
			slog.Info("a first Google sign-in is waiting on eID", "email", identity.Email)
			http.Redirect(w, r, config.WebOrigin()+"/login/bind?b="+url.QueryEscape(token), http.StatusFound)
			return
		}
		slog.Error("could not link a verified Google identity to an account", "error", err)
		s.failGoogle(w, r, "provisioning_failed")
		return
	}

	token, expiresAt, err := s.issueSession(r, userID, tenantID, "google")
	if err != nil {
		slog.Error("could not establish a session for a Google sign-in", "error", err, "user_id", userID)
		s.failGoogle(w, r, "session_failed")
		return
	}
	auth.SetSessionCookie(w, token, expiresAt)
	// Deliberately no id_token cookie. That one exists so signing out here can
	// end the session at the provider this deployment federates to; ending
	// somebody's Google session because they signed out of this platform is not
	// something this platform has any business doing.

	observability.RecordLogin(observability.LoginGoogle, true)
	audit.Record(r.Context(), tenantID, userID, "auth.login_success", "user", map[string]any{
		"method": "google",
		"email":  identity.Email,
	})
	http.Redirect(w, r, config.WebOrigin()+flow.Next, http.StatusFound)
}

// failGoogle returns somebody to the sign-in screen with a reason it can render.
//
// Every refusal on this rail comes through here, which is what makes it the one
// place the failure counter belongs.
func (s *Server) failGoogle(w http.ResponseWriter, r *http.Request, reason string) {
	observability.RecordLogin(observability.LoginGoogle, false)
	http.Redirect(w, r, config.WebOrigin()+"/login?sso_error="+reason, http.StatusFound)
}

// failGoogleLink reports a failure to somebody who is already signed in.
//
// Separate from failGoogle because the sign-in screen is the wrong place to
// send them: they have a session, so /login bounces them straight back, and
// the round trip looks from the outside like the button did nothing at all.
// Sent back to the screen they pressed it on, carrying the reason.
func (s *Server) failGoogleLink(w http.ResponseWriter, r *http.Request, reason string) {
	slog.Info("a Google link attempt failed", "reason", reason)
	http.Redirect(w, r, config.WebOrigin()+"/profile?link_error="+reason, http.StatusFound)
}

// domainOf is for the log line only: the address itself is somebody's, and the
// domain is the part that answers "should this deployment have allowed it".
func domainOf(email string) string {
	if at := strings.LastIndex(email, "@"); at >= 0 {
		return email[at+1:]
	}
	return ""
}
