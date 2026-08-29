/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
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

package identity

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/ssoclient"
	"github.com/jackc/pgx/v5"
)

// GoogleLoginEnabled reports whether this deployment offers the Google button.
func (h *Handlers) GoogleLoginEnabled() bool {
	return h.googleLogin != nil && h.googleLogin.Config().Enabled()
}

// GoogleStartURL is where the sign-in screen sends somebody who presses it.
func (h *Handlers) GoogleStartURL() string {
	return config.SelfOrigin() + "/api/v1/auth/google/start"
}

// HandleGoogleStart begins a sign-in at Google.
func (h *Handlers) HandleGoogleStart(w http.ResponseWriter, r *http.Request) {
	if !h.GoogleLoginEnabled() {
		httpx.Error(w, http.StatusNotFound, "Google sign-in is not configured on this deployment")
		return
	}
	// Federation closing the local front door closes this too. Google here is
	// one of *this* platform's ways of establishing who somebody is, and a
	// deployment that has handed that question to a provider must not keep a
	// second answer to it — see RequireLocalLogin.
	if !h.LocalLoginAllowed() {
		httpx.JSON(w, http.StatusForbidden, map[string]any{
			"error":     "this deployment signs in through its SSO provider",
			"code":      "sso_required",
			"start_url": h.SsoStartURL(),
		})
		return
	}

	request, err := h.googleLogin.BeginAuthorization(r.Context())
	if err != nil {
		slog.Error("could not start a sign-in at Google", "error", err)
		h.failGoogle(w, r, "provider_unreachable")
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

// HandleGoogleLinkStart adds Google to the account somebody is already in.
//
// The same authorisation request as signing in, with one difference in what
// the answer means: the person's identity is not in question here, so there is
// nothing for eID to establish. They proved who they are when they signed in;
// this only records that a particular Google account is also theirs.
//
// It is a navigation rather than a fetch because it ends at Google. The
// session cookie rides along on a same-origin top-level GET, which is what
// makes the callback able to tell whose account to attach the result to.
func (h *Handlers) HandleGoogleLinkStart(w http.ResponseWriter, r *http.Request) {
	// Every refusal here is a redirect rather than a status code. This handler
	// is reached by pressing a button, not by a fetch — the browser navigates
	// to it — so a JSON body would replace the person's screen with a line of
	// machine-readable text and no way back.
	if !h.GoogleLoginEnabled() {
		h.failGoogleLink(w, r, "google_not_configured")
		return
	}
	// A linked Google account is a way in, not merely a label on a profile —
	// the sign-in path will find it tomorrow. So a deployment that has handed
	// the question of who somebody is to its provider must not let people cut
	// themselves a second door from inside, which is the same reason
	// HandleGoogleStart refuses.
	if !h.LocalLoginAllowed() {
		h.failGoogleLink(w, r, "sso_required")
		return
	}
	// Checked before the trip to Google rather than only on the way back. The
	// callback has to verify it again — a session can end mid-flow — but
	// sending somebody through a consent screen that cannot possibly succeed
	// is a worse way to say "you are not signed in".
	if _, err := h.sessions.Resolve(r.Context(), auth.TokenFromRequest(r)); err != nil {
		slog.Info("a Google link was started without a live session", "error", err)
		h.failGoogleLink(w, r, "session_expired")
		return
	}

	request, err := h.googleLogin.BeginAuthorization(r.Context())
	if err != nil {
		slog.Error("could not start a Google link", "error", err)
		h.failGoogleLink(w, r, "provider_unreachable")
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
func (h *Handlers) linkGoogleToCurrentAccount(w http.ResponseWriter, r *http.Request, identity *ssoclient.Identity, next string) {
	// Resolved here rather than read from the context: the callback is a public
	// route, because Google has to be able to reach it, so nothing upstream has
	// established who is asking. The session cookie arrives on this navigation
	// like any other same-origin GET, and it is the only thing that decides
	// whose account this attaches to.
	claims, err := h.sessions.Resolve(r.Context(), auth.TokenFromRequest(r))
	if err != nil {
		// The session ended somewhere between the profile screen and Google's
		// answer. Nothing is linked, and saying so is better than silently
		// turning this into a sign-in for whoever holds the browser.
		slog.Info("a Google link came back without a live session", "error", err)
		h.failGoogleLink(w, r, "session_expired")
		return
	}

	issuer := h.googleLogin.Config().Issuer

	// Refuse rather than move. The insert this would otherwise reach reassigns
	// user_id on conflict, which is right when somebody signs in and wrong
	// here: it would take a Google account off whoever it belongs to, without
	// telling them, at the request of somebody else who happens to be able to
	// authenticate at Google as them.
	var owner string
	err = h.db.QueryRow(r.Context(),
		`SELECT user_id::text FROM registry.user_sso_identities WHERE issuer = $1 AND subject = $2`,
		issuer, identity.Subject).Scan(&owner)
	switch {
	case err == nil && owner != claims.UserID:
		slog.Info("refused to move a Google identity between accounts",
			"issuer", issuer, "requested_by", claims.UserID)
		h.failGoogleLink(w, r, "already_linked_elsewhere")
		return
	case err == nil:
		// Already theirs. Re-running the flow refreshes what Google says about
		// them, which is a reasonable thing to want and not an error.
	}

	h.linkSSOIdentity(r.Context(), claims.UserID, issuer, identity)
	slog.Info("a person linked Google to their account", "user_id", claims.UserID)
	http.Redirect(w, r, config.WebOrigin()+next, http.StatusFound)
}

// HandleGoogleCallback is where Google returns the browser.
func (h *Handlers) HandleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if !h.GoogleLoginEnabled() || !h.LocalLoginAllowed() {
		httpx.Error(w, http.StatusNotFound, "Google sign-in is not configured on this deployment")
		return
	}
	query := r.URL.Query()

	flow, err := ssoclient.ReadFlow(w, r, ssoclient.GoogleFlow, query.Get("state"))
	if err != nil {
		slog.Info("refused a Google callback", "error", err)
		h.failGoogle(w, r, "stale_request")
		return
	}
	if providerError := query.Get("error"); providerError != "" {
		slog.Info("Google refused a sign-in", "error", providerError)
		h.failGoogle(w, r, providerError)
		return
	}
	code := query.Get("code")
	if code == "" {
		h.failGoogle(w, r, "no_code")
		return
	}

	identity, err := h.googleLogin.Exchange(r.Context(), code, flow.CodeVerifier, flow.Nonce)
	if err != nil {
		slog.Error("could not redeem a Google authorization code", "error", err)
		h.failGoogle(w, r, "exchange_failed")
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
		h.failGoogle(w, r, "email_unverified")
		return
	}
	if !ssoclient.EmailInDomains(identity.Email, ssoclient.GoogleAllowedDomains()) {
		slog.Info("refused a Google sign-in from a domain that is not allowed here",
			"domain", domainOf(identity.Email))
		h.failGoogle(w, r, "domain_not_allowed")
		return
	}

	// Adding a provider to an account rather than using one to reach it. The
	// verification checks above still applied: an address Google has not
	// verified is no more usable as a label on somebody's profile than it is
	// as a way in.
	if flow.Link {
		h.linkGoogleToCurrentAccount(w, r, identity, ssoclient.SafeNext(flow.Next, "/profile"))
		return
	}

	userID, tenantID, err := h.resolveGoogleUser(r.Context(), h.googleLogin.Config(), identity)
	if err != nil {
		var refusal auth.SignInError
		if errors.As(err, &refusal) {
			// Not a refusal any more. Google has said which Google account this
			// is; it cannot say who the person is, and that is what this
			// platform's accounts are held by. So the identity is parked and
			// they are asked to prove themselves once with eID — see
			// identity_binding.go.
			token, bindErr := h.startIdentityBinding(r.Context(), h.googleLogin.Config().Issuer, identity)
			if bindErr != nil {
				slog.Error("could not start an identity binding", "error", bindErr)
				// Not "no account": there is no account *yet*, and the screen
				// that would have made one could not be reached. Saying
				// no_account here sent somebody to a message telling them to
				// ask their administrator for an account they were three
				// clicks away from opening themselves.
				h.failGoogle(w, r, "binding_failed")
				return
			}
			slog.Info("a first Google sign-in is waiting on eID", "email", identity.Email)
			targetURL := config.WebOrigin() + "/login/bind?b=" + url.QueryEscape(token)
			if flow.Next != "" {
				targetURL += "&next=" + url.QueryEscape(flow.Next)
			}
			http.Redirect(w, r, targetURL, http.StatusFound)
			return
		}
		slog.Error("could not link a verified Google identity to an account", "error", err)
		h.failGoogle(w, r, "provisioning_failed")
		return
	}

	token, expiresAt, err := h.authn.IssueSession(r, userID, tenantID, "google")
	if err != nil {
		slog.Error("could not establish a session for a Google sign-in", "error", err, "user_id", userID)
		h.failGoogle(w, r, "session_failed")
		return
	}
	auth.SetSessionCookie(w, token, expiresAt)
	// Deliberately no id_token cookie. That one exists so signing out here can
	// end the session at the provider this deployment federates to; ending
	// somebody's Google session because they signed out of this platform is not
	// something this platform has any business doing.

	telemetry.RecordLogin(telemetry.LoginGoogle, true)
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
func (h *Handlers) failGoogle(w http.ResponseWriter, r *http.Request, reason string) {
	telemetry.RecordLogin(telemetry.LoginGoogle, false)
	http.Redirect(w, r, config.WebOrigin()+"/login?sso_error="+reason, http.StatusFound)
}

// failGoogleLink reports a failure to somebody who is already signed in.
//
// Separate from failGoogle because the sign-in screen is the wrong place to
// send them: they have a session, so /login bounces them straight back, and
// the round trip looks from the outside like the button did nothing at all.
// Sent back to the screen they pressed it on, carrying the reason.
func (h *Handlers) failGoogleLink(w http.ResponseWriter, r *http.Request, reason string) {
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

// resolveGoogleUser maps a verified Google identity onto a local account.
//
// Unlike federated SSO, Google sign-in does not auto-link by email or
// auto-provision users without eID. Accounts on this platform are held by
// national identity verified via eID.
//
// If the Google identity (issuer, subject) is already bound to a user account,
// it logs them in directly. Otherwise, it returns a auth.SignInError so the caller
// can park the identity and require a one-time eID binding.
func (h *Handlers) resolveGoogleUser(ctx context.Context, cfg ssoclient.Config, identity *ssoclient.Identity) (userID, tenantID string, err error) {
	issuer := cfg.Issuer

	err = h.db.QueryRow(ctx,
		`SELECT user_id::text FROM registry.user_sso_identities WHERE issuer = $1 AND subject = $2`,
		issuer, identity.Subject).Scan(&userID)
	if err == nil {
		h.touchSSOIdentity(ctx, issuer, identity)
		tenantID, err = h.authn.FirstTenantFor(ctx, userID)
		if err != nil {
			return "", "", err
		}
		return userID, tenantID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", err
	}

	return "", "", auth.NewSignInError("Google account is not bound to any user")
}
