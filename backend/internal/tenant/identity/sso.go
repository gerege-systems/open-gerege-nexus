/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Signing in through another OpenID Connect provider — the deployment-level
 * counterpart to the eID and DAN sign-in paths next door.
 *
 * The shape is the same as those: a protocol package (ssoclient) proves who
 * somebody is, and these handlers decide what that means here — which account
 * it is, which organisation it belongs to, and what session it gets. What is
 * different is that this one is not an option on the sign-in screen. When
 * SSO_CLIENT_ISSUER names a provider, it *is* the sign-in screen: the local
 * paths stop answering, because a deployment that federates its identity and
 * also keeps its own password login has not federated anything — it has two
 * front doors and one of them is unmanaged.
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

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/ssoclient"
	"github.com/jackc/pgx/v5"
)

// defaultLandingPath is where somebody who signed in without asking for
// anywhere in particular ends up.
//
// Their own record rather than the app store. Somebody who just proved who
// they are has a question the app store cannot answer — which organisation am
// I in, and did that sign-in land where I expected — and somebody arriving by
// a route they did not choose (a first Google sign-in, a fresh eID binding)
// most needs to see what was linked to them. A person heading for a particular
// screen still gets it: `next` is honoured, and this is only the fallback.
const defaultLandingPath = "/profile"

// SsoClientEnabled reports whether this deployment signs people in elsewhere.
func (h *Handlers) SsoClientEnabled() bool {
	return h.ssoClient != nil && h.ssoClient.Config().Enabled()
}

// LocalLoginAllowed reports whether this deployment's own sign-in paths answer.
//
// They always do unless a provider has been named, and even then an operator
// can keep them with SSO_CLIENT_LOCAL_LOGIN — the way back in when the provider
// is the thing that is broken.
func (h *Handlers) LocalLoginAllowed() bool {
	return !h.SsoClientEnabled() || h.ssoClient.Config().LocalLogin
}

// RequireLocalLogin wraps the sign-in handlers this deployment may have given
// away. It answers rather than 404s: a native client or a stale browser tab
// posting a password to a federated deployment needs to be told where sign-in
// actually happens, not that the endpoint has vanished.
func (h *Handlers) RequireLocalLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.LocalLoginAllowed() {
			next(w, r)
			return
		}
		httpx.JSON(w, http.StatusForbidden, map[string]any{
			"error":     "this deployment signs in through its SSO provider",
			"code":      "sso_required",
			"start_url": h.SsoStartURL(),
		})
	}
}

// SsoStartURL is the address a client sends somebody to in order to sign in.
func (h *Handlers) SsoStartURL() string {
	return config.SelfOrigin() + "/api/v1/auth/sso/start"
}

// HandleSSOConfig tells a browser how this deployment signs people in.
//
// It is unauthenticated and says nothing secret: whether sign-in is federated
// is visible from the redirect anyway, and the login screen has to know before
// it renders which of two completely different things it is.
func (h *Handlers) HandleSSOConfig(w http.ResponseWriter, r *http.Request) {
	answer := map[string]any{"enabled": false, "local_login": true}
	if h.SsoClientEnabled() {
		cfg := h.ssoClient.Config()
		answer["enabled"] = true
		answer["provider_name"] = cfg.DisplayName()
		answer["start_url"] = h.SsoStartURL()
		// Whether the screen should still offer the password and eID forms
		// underneath the provider's button.
		answer["local_login"] = cfg.LocalLogin
	}

	// Google is reported only when it can actually be used. A deployment that
	// federates has closed its local sign-in paths, and Google is one of them,
	// so the button would be an offer this server would refuse.
	// Whether this platform admits strangers. The sign-in screen reads it to
	// decide whether to offer a way in at all — a "sign up" affordance on a
	// private deployment is an invitation to a refusal.
	answer["access_mode"] = auth.AccessMode()

	answer["google"] = map[string]any{"enabled": false}
	if h.GoogleLoginEnabled() && h.LocalLoginAllowed() {
		answer["google"] = map[string]any{"enabled": true, "start_url": h.GoogleStartURL()}
	}
	httpx.JSON(w, http.StatusOK, answer)
}

// HandleSSOStart begins a sign-in at the provider.
//
// It is a browser endpoint: it answers with a redirect, and the destination is
// built from the provider's own discovery document rather than from anything in
// the request.
func (h *Handlers) HandleSSOStart(w http.ResponseWriter, r *http.Request) {
	if !h.SsoClientEnabled() {
		httpx.Error(w, http.StatusNotFound, "this deployment is not a client of an SSO provider")
		return
	}

	request, err := h.ssoClient.BeginAuthorization(r.Context())
	if err != nil {
		slog.Error("could not start a sign-in at the SSO provider", "error", err)
		h.failSSO(w, r, "provider_unreachable")
		return
	}

	ssoclient.SetFlowCookie(w, ssoclient.FederationFlow, ssoclient.Flow{
		State:        request.State,
		Nonce:        request.Nonce,
		CodeVerifier: request.CodeVerifier,
		Next:         ssoclient.SafeNext(r.URL.Query().Get("next"), defaultLandingPath),
	})
	http.Redirect(w, r, request.URL, http.StatusFound)
}

// HandleSSOCallback is where the provider returns the browser.
//
// Unauthenticated by definition — the person is signing in — and the authority
// here is the pair of the state cookie this deployment set and the code the
// provider issued against it. Neither alone is enough: the cookie without a
// matching state is a stale attempt, and a code without the cookie is somebody
// replaying a callback URL.
func (h *Handlers) HandleSSOCallback(w http.ResponseWriter, r *http.Request) {
	if !h.SsoClientEnabled() {
		httpx.Error(w, http.StatusNotFound, "this deployment is not a client of an SSO provider")
		return
	}
	query := r.URL.Query()

	flow, err := ssoclient.ReadFlow(w, r, ssoclient.FederationFlow, query.Get("state"))
	if err != nil {
		slog.Info("refused an SSO callback", "error", err)
		h.failSSO(w, r, "stale_request")
		return
	}

	// The provider refusing is not our failure to report as one. It has already
	// told the person why on its own screen; what is left is to put them back
	// somewhere they can act.
	if providerError := query.Get("error"); providerError != "" {
		slog.Info("the SSO provider refused a sign-in",
			"error", providerError, "description", query.Get("error_description"))
		h.failSSO(w, r, providerError)
		return
	}

	code := query.Get("code")
	if code == "" {
		h.failSSO(w, r, "no_code")
		return
	}

	identity, err := h.ssoClient.Exchange(r.Context(), code, flow.CodeVerifier, flow.Nonce)
	if err != nil {
		slog.Error("could not redeem an SSO authorization code", "error", err)
		h.failSSO(w, r, "exchange_failed")
		return
	}

	userID, tenantID, err := h.resolveOrProvisionSSOUser(r.Context(), h.ssoClient.Config(), identity)
	if err != nil {
		var refusal auth.SignInError
		if errors.As(err, &refusal) {
			slog.Info("refused a verified SSO identity", "reason", refusal.Error(), "subject", identity.Subject)
			h.failSSO(w, r, "no_account")
			return
		}
		slog.Error("could not link a verified SSO identity to an account", "error", err)
		h.failSSO(w, r, "provisioning_failed")
		return
	}

	token, expiresAt, err := h.authn.IssueSession(r, userID, tenantID, "sso")
	if err != nil {
		slog.Error("could not establish a session for an SSO sign-in", "error", err, "user_id", userID)
		h.failSSO(w, r, "session_failed")
		return
	}
	auth.SetSessionCookie(w, token, expiresAt)
	// Kept so that signing out here can sign the person out there. See
	// handleLogout: this is what becomes the id_token_hint.
	ssoclient.SetIDTokenCookie(w, identity.IDToken, expiresAt)

	telemetry.RecordLogin(telemetry.LoginSSO, true)
	audit.Record(r.Context(), tenantID, userID, "auth.login_success", "user", map[string]any{
		"method":   "sso",
		"issuer":   h.ssoClient.Config().Issuer,
		"provider": h.ssoClient.Config().DisplayName(),
	})
	http.Redirect(w, r, config.WebOrigin()+flow.Next, http.StatusFound)
}

// failSSO puts somebody back on the sign-in screen with a reason it can render.
// The reason is a short code rather than a message: it crosses an origin as a
// query parameter, and the screen it lands on is the one that knows the
// person's language.
func (h *Handlers) failSSO(w http.ResponseWriter, r *http.Request, reason string) {
	telemetry.RecordLogin(telemetry.LoginSSO, false)
	http.Redirect(w, r, config.WebOrigin()+"/login?sso_error="+reason, http.StatusFound)
}

// resolveOrProvisionSSOUser maps a verified provider identity onto a local
// account, creating one if the deployment is configured to.
//
// The subject is what the account is keyed on, never the email address. An
// email is a label the provider may change, and treating one as an identity
// means that whoever is given a departed colleague's address inherits their
// account. The subject is the one claim a provider guarantees to be stable and
// never to reassign.
func (h *Handlers) resolveOrProvisionSSOUser(ctx context.Context, cfg ssoclient.Config, identity *ssoclient.Identity) (userID, tenantID string, err error) {
	issuer := cfg.Issuer

	// Two questions, asked separately. Whether this provider account is known
	// here is one; which organisation the person it belongs to works in is
	// another, and answering them in a single join makes the second silently
	// decide the first.
	//
	// It joined memberships once, so somebody with a linked identity and no
	// membership read as somebody with no linked identity — and was sent
	// through the first-time flow again, and again, while their profile went on
	// listing the provider as connected. Being in no organisation is a real
	// state: an administrator removes the last one, or an account is created
	// ahead of the membership.
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

	// Nobody has signed in as this subject here before. An account with the
	// same address may still exist — somebody who used to sign in locally, or
	// who was created by an administrator ahead of time — and adopting it is
	// what makes federating an existing deployment possible at all. The address
	// has to be one the provider says it verified: a provider that lets people
	// set an unverified email would otherwise let anyone claim any account by
	// typing its address into their profile.
	email := strings.ToLower(strings.TrimSpace(identity.Email))
	if email != "" && identity.EmailVerified {
		if err = h.db.QueryRow(ctx,
			`SELECT u.id::text, m.tenant_id::text
			   FROM registry.users u JOIN tenant.memberships m ON m.user_id = u.id
			  WHERE lower(u.email) = $1
			  ORDER BY m.created_at, m.tenant_id LIMIT 1`, email).Scan(&userID, &tenantID); err == nil {
			h.linkSSOIdentity(ctx, userID, issuer, identity)
			return userID, tenantID, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return "", "", err
		}
	}

	return h.provisionSSOUser(ctx, cfg, identity)
}

// provisionSSOUser creates the local account for a provider identity nothing
// here has seen before.
//
// It only happens when SSO_CLIENT_TENANT names an organisation to put them in.
// Without one, an identity the provider vouches for is refused: for most
// deployments membership is a deliberate act, and a federation that silently
// admits everyone the provider knows is a wider door than the operator asked
// for.
func (h *Handlers) provisionSSOUser(ctx context.Context, cfg ssoclient.Config, identity *ssoclient.Identity) (userID, tenantID string, err error) {
	issuer, slug := cfg.Issuer, cfg.TenantSlug
	if slug == "" {
		return "", "", auth.NewSignInError("your identity is verified but this deployment has no account for you")
	}
	if err = h.db.QueryRow(ctx, `SELECT id::text FROM registry.tenants WHERE slug = $1`, slug).Scan(&tenantID); err != nil {
		return "", "", err
	}

	// A synthetic, non-routable address when the provider sent none. The column
	// is unique and NOT NULL, and a shared placeholder would collide on the
	// second person; keying it on the subject makes it as unique as the subject
	// is. It is deliberately at .invalid (RFC 2606) so nothing ever tries to
	// deliver to it.
	email := strings.ToLower(strings.TrimSpace(identity.Email))
	if email == "" {
		email = "sso+" + ssoSubjectHandle(issuer, identity.Subject) + "@identity.invalid"
	}
	name := identity.Name
	if name == "" {
		name = email
	}

	// This account has no password login path, and the column is NOT NULL, so
	// what fills it is the hash of a value nobody — including this process a
	// moment later — knows. Deriving it from the subject instead would have
	// been stable and reproducible, which is the problem: the synthetic address
	// above is derived from the same two public strings, so anybody who could
	// recompute the password would hold a working local credential for a
	// federated account.
	passwordHash, err := auth.HashPassword(unguessableSecret())
	if err != nil {
		return "", "", err
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err = tx.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name, is_admin) VALUES ($1,$2,$3,FALSE)
		 ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id::text`, email, passwordHash, name).Scan(&userID); err != nil {
		return "", "", err
	}
	// The platform's access mode. A federated provider vouching for somebody
	// is not the same as this platform having decided to admit them, and in
	// private mode it is the second half that is missing.
	if err = auth.MayProvisionAccount("sso"); err != nil {
		return "", "", err
	}
	// The organisation's limit, checked before it grows. A provider that
	// authenticates the whole of a company would otherwise walk an
	// organisation past whatever the console set for it, one sign-in at a
	// time, with nobody deciding to.
	if err = h.authn.CheckUserQuota(ctx, tenantID); err != nil {
		return "", "", auth.NewSignInError("this organisation has reached the number of people it may have")
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO tenant.memberships (tenant_id, user_id) VALUES ($1,$2)
		 ON CONFLICT (tenant_id, user_id) DO NOTHING`, tenantID, userID); err != nil {
		return "", "", err
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO registry.user_sso_identities (user_id, issuer, subject, email, name, claims, last_seen_at)
		 VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,NOW())
		 ON CONFLICT (issuer, subject) DO UPDATE SET
		     email = EXCLUDED.email, name = EXCLUDED.name,
		     claims = EXCLUDED.claims, last_seen_at = NOW()`,
		userID, issuer, identity.Subject, identity.Email, identity.Name,
		claimsJSON(identity.Claims)); err != nil {
		return "", "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", err
	}

	slog.Info("provisioned an account for a federated identity",
		"issuer", issuer, "tenant_slug", slug, "user_id", userID)
	return userID, tenantID, nil
}

// claimsJSON renders a provider's claims for the jsonb column.
//
// An unmarshallable payload becomes an empty object rather than failing the
// sign-in: the claims are a record to read later, and losing them is not worth
// refusing somebody who has just proved who they are.
func claimsJSON(claims map[string]any) []byte {
	if len(claims) == 0 {
		return []byte("{}")
	}
	encoded, err := json.Marshal(claims)
	if err != nil {
		slog.Warn("could not record the claims a provider returned", "error", err)
		return []byte("{}")
	}
	return encoded
}

// linkSSOIdentity records that a local account is this provider subject.
//
// Best effort, like the eID linking next door: the sign-in has already
// succeeded, and failing it because a bookkeeping row could not be written
// would trade a working session for none. The cost of a missed write is that
// the next sign-in matches on the address again.
func (h *Handlers) linkSSOIdentity(ctx context.Context, userID, issuer string, identity *ssoclient.Identity) {
	if _, err := h.db.Exec(ctx,
		`INSERT INTO registry.user_sso_identities (user_id, issuer, subject, email, name, claims, last_seen_at)
		 VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,NOW())
		 ON CONFLICT (issuer, subject) DO UPDATE SET
		     user_id = EXCLUDED.user_id, email = EXCLUDED.email,
		     name = EXCLUDED.name, claims = EXCLUDED.claims, last_seen_at = NOW()`,
		userID, issuer, identity.Subject, identity.Email, identity.Name,
		claimsJSON(identity.Claims)); err != nil {
		slog.Warn("could not link a federated identity to the platform account",
			"user_id", userID, "issuer", issuer, "error", err)
	}
}

// touchSSOIdentity keeps the recorded profile and the last-seen stamp current
// for somebody who already has a link.
func (h *Handlers) touchSSOIdentity(ctx context.Context, issuer string, identity *ssoclient.Identity) {
	if _, err := h.db.Exec(ctx,
		`UPDATE registry.user_sso_identities
		    SET email = COALESCE(NULLIF($3,''), email),
		        name = COALESCE(NULLIF($4,''), name),
		        claims = $5,
		        last_seen_at = NOW()
		  WHERE issuer = $1 AND subject = $2`,
		issuer, identity.Subject, identity.Email, identity.Name,
		claimsJSON(identity.Claims)); err != nil {
		slog.Warn("could not record a federated sign-in", "issuer", issuer, "error", err)
	}
}

// ssoSubjectHandle is a stable, unique, non-reversible handle for a provider
// subject, for the local part of a synthetic address.
//
// Hashed rather than used directly because a subject can be a national
// identifier at some providers, and an address is the one column of a user row
// that gets read aloud, pasted into tickets and shown in lists.
func ssoSubjectHandle(issuer, subject string) string {
	sum := sha256.Sum256([]byte("sso-identity:" + issuer + "\x00" + subject))
	return hex.EncodeToString(sum[:])[:32]
}

// unguessableSecret returns 256 bits of crypto/rand as hex — a value used once,
// to fill a password column that will never be checked against anything.
func unguessableSecret() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand does not fail on a supported platform, and a predictable
		// fallback would turn "no password login path" into a guessable one.
		panic("platform: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(buf)
}
