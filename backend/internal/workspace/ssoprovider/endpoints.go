/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package ssoprovider

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// SessionResolver turns a platform session token into the signed-in user. The
// provider takes it as an interface so it depends on the session store's
// behaviour rather than on the Server that owns it.
type SessionResolver interface {
	Resolve(ctx context.Context, token string) (auth.UserClaims, error)
}

// AttachSessions wires the end-user session store into the provider. Without it
// the authorization endpoint cannot tell who is signing in, and says so rather
// than guessing.
func (s *SSOProvider) AttachSessions(sessions SessionResolver) { s.sessions = sessions }

// InstallGate answers whether a tenant may sign in to a client.
//
// It exists for external apps: a third-party platform installed from the store
// gets an OAuth2 client here, and the app gate that keeps an uninstalled
// module's routes unreachable has to keep reaching once the app in question is
// somebody else's service. Without this, any user of any tenant could sign in
// to any registered external app the moment it was published, whether or not
// their organisation had installed it — the installation would decide what
// appears in the menu and nothing else.
//
// Clients that belong to no external app — the developer portal's own, every
// first-party integration — are not the gate's business and it says so by
// answering true.
type InstallGate interface {
	AllowClient(ctx context.Context, tenantID, clientID string) (bool, error)
}

// AttachInstallGate wires the store's view of installations into the
// authorization endpoint. Without one, nothing is gated — which is the state
// this provider was in before external apps existed.
func (s *SSOProvider) AttachInstallGate(gate InstallGate) { s.installs = gate }

// allowedForTenant reports whether this tenant may use this client.
//
// A gate that fails is a refusal, not a bypass. The question it answers is
// whether an organisation has installed a third-party app, and answering "yes"
// because the database was unreachable would hand somebody's HR system a user
// their employer never onboarded.
func (s *SSOProvider) allowedForTenant(ctx context.Context, tenantID, clientID string) bool {
	if s.installs == nil {
		return true
	}
	allowed, err := s.installs.AllowClient(ctx, tenantID, clientID)
	if err != nil {
		slog.Error("could not check whether the tenant installed this app",
			"error", err, "client_id", clientID, "tenant_id", tenantID)
		return false
	}
	return allowed
}

// authRequest is a validated /oauth2/auth query.
type authRequest struct {
	Client              *Client
	RedirectURI         string
	Scopes              []string
	State               string
	Nonce               string
	CodeChallenge       string
	CodeChallengeMethod string
	Prompt              string
}

// HandleAuthorize is the OAuth2 authorization endpoint (RFC 6749 §3.1).
//
// It is a browser endpoint, not an API: it answers with redirects. Errors that
// happen before the redirect_uri is known are shown to the user, because
// bouncing them to an unverified URI is how open redirectors are built.
func (s *SSOProvider) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	clientID := q.Get("client_id")
	if clientID == "" {
		s.authFailure(w, r, "invalid_request", "client_id is required")
		return
	}

	client, err := s.store.GetClient(ctx, clientID)
	if err != nil || client.Disabled {
		s.authFailure(w, r, "unauthorized_client", "unknown or disabled client")
		return
	}

	// The redirect URI must match a registered one exactly. Prefix or wildcard
	// matching is how an attacker turns a legitimate client into a delivery
	// vehicle for someone else's authorization code.
	redirectURI := q.Get("redirect_uri")
	if redirectURI == "" && len(client.RedirectURIs) == 1 {
		redirectURI = client.RedirectURIs[0]
	}
	if !slices.Contains(client.RedirectURIs, redirectURI) {
		s.authFailure(w, r, "invalid_request", "redirect_uri is not registered for this client")
		return
	}

	// From here the redirect URI is trusted, so failures go back to the client.
	state := q.Get("state")
	fail := func(code, description string) {
		redirectError(w, r, redirectURI, code, description, state)
	}

	if rt := q.Get("response_type"); rt != "code" {
		fail("unsupported_response_type", "only the authorization code flow is supported")
		return
	}
	if !slices.Contains(client.GrantTypes, "authorization_code") {
		fail("unauthorized_client", "this client is not registered for authorization_code")
		return
	}

	// PKCE is required of every client, not just public ones. That is the
	// OAuth 2.1 position: for a confidential client it costs one hash and it
	// removes code injection as a category.
	challenge := q.Get("code_challenge")
	method := q.Get("code_challenge_method")
	if challenge == "" {
		fail("invalid_request", "code_challenge is required (PKCE, RFC 7636)")
		return
	}
	if method != "S256" {
		fail("invalid_request", "code_challenge_method must be S256; plain is not accepted")
		return
	}

	scopes, err := resolveScopes(q.Get("scope"), client)
	if err != nil {
		fail("invalid_scope", err.Error())
		return
	}

	req := &authRequest{
		Client: client, RedirectURI: redirectURI, Scopes: scopes, State: state,
		Nonce: q.Get("nonce"), CodeChallenge: challenge, CodeChallengeMethod: method,
		Prompt: q.Get("prompt"),
	}

	claims, ok := s.currentUser(r)
	if !ok {
		// Not signed in: hand the browser to the platform login screen with
		// enough context to come straight back here afterwards.
		if req.Prompt == "none" {
			fail("login_required", "no active session and prompt=none was requested")
			return
		}
		http.Redirect(w, r, s.issuer+"/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
		return
	}

	// Checked after the user is known, because the question is about their
	// organisation: the same client is reachable for a tenant that installed the
	// app and refused for one that did not. access_denied rather than
	// unauthorized_client — the client is fine, this user's tenant is not
	// entitled to it, and that is what RFC 6749 §4.1.2.1 calls access_denied.
	if !s.allowedForTenant(ctx, claims.WorkspaceID, client.ClientID) {
		slog.Info("refused an authorization request: the user's tenant has not installed this app",
			"client_id", client.ClientID, "tenant_id", claims.WorkspaceID)
		fail("access_denied", "your organisation has not installed this application")
		return
	}

	granted, err := s.store.GetConsent(ctx, claims.UserID, client.ClientID)
	needsConsent := req.Prompt == "consent" || err != nil || !isSubset(scopes, granted)
	if needsConsent {
		if req.Prompt == "none" {
			fail("consent_required", "the user has not granted these scopes")
			return
		}
		http.Redirect(w, r, s.issuer+"/oauth/consent?"+r.URL.RawQuery, http.StatusFound)
		return
	}

	code, err := s.issueAuthCode(ctx, req, claims)
	if err != nil {
		slog.Error("failed to issue authorization code", "error", err, "client_id", client.ClientID)
		fail("server_error", "could not issue an authorization code")
		return
	}
	redirectSuccess(w, r, redirectURI, code, state)
}

// HandleClientInfo names the application behind an authorization request, for
// the sign-in screen to show while nobody is signed in yet.
//
// A person sent here from somewhere else is being asked for credentials by a
// page they did not navigate to, and the first thing that screen has to answer
// is "who is asking". The alternative — letting /oauth2/auth pass the name
// along in the redirect to /login — would put that answer in a query parameter
// anybody can write, which is a phishing kit rather than a feature: /login?
// client_name=Bank%20of%20Somewhere would render exactly like the real thing.
// Resolving it here means the name on the screen is the one that was
// registered.
//
// Unauthenticated, because the whole point is that it is read before sign-in.
// What it discloses is a registered client's display name to somebody who
// already knows its client_id — which is not a secret, travels in every
// authorization URL, and is shown on the consent screen a moment later anyway.
// An unknown and a disabled client answer identically, so this cannot be used
// to sort real client_ids from invented ones by their error.
func (s *SSOProvider) HandleClientInfo(w http.ResponseWriter, r *http.Request) {
	clientID := strings.TrimSpace(r.URL.Query().Get("client_id"))
	if clientID == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "client_id is required")
		return
	}
	client, err := s.store.GetClient(r.Context(), clientID)
	if err != nil || client.Disabled {
		writeOAuthError(w, http.StatusNotFound, "invalid_client", "unknown or disabled client")
		return
	}
	// Deliberately three fields. Everything else on a client — its redirect
	// URIs, its grants, its scopes, when its secret was rotated — is the owning
	// tenant's business and has no place on an unauthenticated endpoint.
	writeJSON(w, http.StatusOK, map[string]string{
		"client_id":   client.ClientID,
		"client_name": client.ClientName,
		"logo_uri":    client.LogoURI,
	})
}

// ConsentPrompt is what the consent screen renders.
type ConsentPrompt struct {
	ClientID    string  `json:"client_id"`
	ClientName  string  `json:"client_name"`
	ClientURI   string  `json:"client_uri,omitempty"`
	LogoURI     string  `json:"logo_uri,omitempty"`
	RedirectURI string  `json:"redirect_uri"`
	Scopes      []Scope `json:"scopes"`
	// AlreadyGranted lists scopes the user approved before, so the screen can
	// present a widening grant as what it is rather than as a fresh one.
	AlreadyGranted []string `json:"already_granted"`
}

// HandleConsentPrompt describes a pending authorization for the consent screen.
// It re-validates the query from scratch: the frontend is a renderer here, not
// a source of truth.
func (s *SSOProvider) HandleConsentPrompt(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.currentUser(r)
	if !ok {
		writeOAuthError(w, http.StatusUnauthorized, "login_required", "sign in first")
		return
	}

	req, oerr := s.parseConsentQuery(r.Context(), r.URL.Query())
	if oerr != nil {
		writeOAuthError(w, http.StatusBadRequest, oerr.Code, oerr.Description)
		return
	}

	// The consent screen describes a grant that is about to be made, so it is
	// held to the same rule as the endpoint that redirected here.
	if !s.allowedForTenant(r.Context(), claims.WorkspaceID, req.Client.ClientID) {
		writeOAuthError(w, http.StatusForbidden, "access_denied",
			"your organisation has not installed this application")
		return
	}

	// A user who has granted nothing leaves this nil, and a nil slice is sent
	// as `null` rather than `[]` — which the consent screen then asks whether
	// it includes each scope. Every first consent for a client is that case.
	granted, _ := s.store.GetConsent(r.Context(), claims.UserID, req.Client.ClientID)

	prompt := ConsentPrompt{
		ClientID: req.Client.ClientID, ClientName: req.Client.ClientName,
		ClientURI: req.Client.ClientURI, LogoURI: req.Client.LogoURI,
		RedirectURI: req.RedirectURI, AlreadyGranted: list(granted),
		Scopes: make([]Scope, 0, len(req.Scopes)),
	}
	for _, name := range req.Scopes {
		if scope, found := LookupScope(name); found {
			prompt.Scopes = append(prompt.Scopes, scope)
		}
	}
	writeJSON(w, http.StatusOK, prompt)
}

// HandleConsentDecision records an approval or refusal and returns the URL the
// browser should be sent to. Everything is re-validated server-side.
func (s *SSOProvider) HandleConsentDecision(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := s.currentUser(r)
	if !ok {
		writeOAuthError(w, http.StatusUnauthorized, "login_required", "sign in first")
		return
	}

	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "malformed form body")
		return
	}

	req, oerr := s.parseConsentQuery(ctx, r.PostForm)
	if oerr != nil {
		writeOAuthError(w, http.StatusBadRequest, oerr.Code, oerr.Description)
		return
	}

	// This endpoint mints a code of its own, so the gate at /oauth2/auth is not
	// enough: a browser can post here directly, and everything else about this
	// request is re-validated from scratch for the same reason.
	if !s.allowedForTenant(ctx, claims.WorkspaceID, req.Client.ClientID) {
		writeJSON(w, http.StatusOK, map[string]string{
			"redirect_to": errorRedirectURL(req.RedirectURI, "access_denied",
				"your organisation has not installed this application", req.State),
		})
		return
	}

	if r.PostFormValue("approved") != "true" {
		writeJSON(w, http.StatusOK, map[string]string{
			"redirect_to": errorRedirectURL(req.RedirectURI, "access_denied",
				"the user refused the request", req.State),
		})
		return
	}

	// Merge with anything granted earlier so approving a narrow request does
	// not silently withdraw a wider standing grant.
	granted, _ := s.store.GetConsent(ctx, claims.UserID, req.Client.ClientID)
	if err := s.store.SaveConsent(ctx, claims.WorkspaceID, claims.UserID, req.Client.ClientID,
		union(granted, req.Scopes)); err != nil {
		slog.Error("failed to record consent", "error", err)
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "could not record consent")
		return
	}

	code, err := s.issueAuthCode(ctx, req, claims)
	if err != nil {
		slog.Error("failed to issue authorization code", "error", err)
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "could not issue a code")
		return
	}

	target, _ := url.Parse(req.RedirectURI)
	query := target.Query()
	query.Set("code", code)
	if req.State != "" {
		query.Set("state", req.State)
	}
	target.RawQuery = query.Encode()
	writeJSON(w, http.StatusOK, map[string]string{"redirect_to": target.String()})
}

// oauthError is a code/description pair destined for the client. It is a
// carrier, not an error value: callers render Code and Description into an
// RFC 6749 §5.2 body rather than wrapping or unwrapping it.
type oauthError struct {
	Code        string
	Description string
}

// parseConsentQuery validates the subset of the authorization request that the
// consent screen round-trips.
func (s *SSOProvider) parseConsentQuery(ctx context.Context, values url.Values) (*authRequest, *oauthError) {
	// The client is read on the platform path, off the signed-in user's tenant.
	// A client belongs to the organisation that registered it, and issueAuthCode
	// below says why that is not the organisation signing in: one tenant's
	// client signs in another tenant's users, which is what federating a
	// separate deployment means. /oauth2/auth is unauthenticated and so already
	// reads it this way; these two endpoints sit behind the session middleware,
	// where row-level security scopes every read to the caller's tenant and hid
	// every client but their own — a first consent to somebody else's client
	// answered "unknown or disabled client" and no sign-in ever completed.
	client, err := s.store.GetClient(nexus.WithoutWorkspace(ctx), values.Get("client_id"))
	if err != nil || client.Disabled {
		return nil, &oauthError{"unauthorized_client", "unknown or disabled client"}
	}

	redirectURI := values.Get("redirect_uri")
	if redirectURI == "" && len(client.RedirectURIs) == 1 {
		redirectURI = client.RedirectURIs[0]
	}
	if !slices.Contains(client.RedirectURIs, redirectURI) {
		return nil, &oauthError{"invalid_request", "redirect_uri is not registered for this client"}
	}

	challenge := values.Get("code_challenge")
	if challenge == "" || values.Get("code_challenge_method") != "S256" {
		return nil, &oauthError{"invalid_request", "a S256 code_challenge is required"}
	}

	scopes, err := resolveScopes(values.Get("scope"), client)
	if err != nil {
		return nil, &oauthError{"invalid_scope", err.Error()}
	}

	return &authRequest{
		Client: client, RedirectURI: redirectURI, Scopes: scopes,
		State: values.Get("state"), Nonce: values.Get("nonce"),
		CodeChallenge: challenge, CodeChallengeMethod: "S256",
	}, nil
}

// issueAuthCode mints and stores a single-use code bound to the PKCE challenge.
func (s *SSOProvider) issueAuthCode(ctx context.Context, req *authRequest, claims auth.UserClaims) (string, error) {
	code := generateRandomString(64)
	return code, s.store.SaveAuthCode(ctx, &AuthCode{
		CodeHash: hashSecret(code),
		ClientID: req.Client.ClientID,
		// The user's tenant, not the client's: the token addresses the data
		// domain the person belongs to, which is what a resource server filters
		// by. A client registered by one tenant can sign in a user of another.
		TenantID:            claims.WorkspaceID,
		UserID:              claims.UserID,
		RedirectURI:         req.RedirectURI,
		Scopes:              req.Scopes,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
		Nonce:               req.Nonce,
		ExpiresAt:           time.Now().Add(authCodeTTL),
	})
}

// HandleTokenEndpoint implements RFC 6749 §3.2 for the three supported grants.
func (s *SSOProvider) HandleTokenEndpoint(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "malformed form body")
		return
	}

	switch r.PostFormValue("grant_type") {
	case "authorization_code":
		s.grantAuthorizationCode(w, r)
	case "refresh_token":
		s.grantRefreshToken(w, r)
	case "client_credentials":
		s.grantClientCredentials(w, r)
	case "":
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "grant_type is required")
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type",
			"supported grants are "+strings.Join(SupportedGrantTypes, ", "))
	}
}

// authenticateClient resolves the caller of the token endpoint.
//
// Confidential clients present a secret, by Basic auth or in the body. Public
// clients present only a client_id and are held up by PKCE instead — so this
// deliberately accepts a secretless public client, and equally deliberately
// refuses a secretless confidential one.
func (s *SSOProvider) authenticateClient(r *http.Request) (*Client, error) {
	clientID, clientSecret, hasBasic := r.BasicAuth()
	if hasBasic {
		// RFC 6749 §2.3.1 has the client form-urlencode both halves before
		// base64ing them, so a conformant client's secret arrives escaped and
		// has to be unescaped to match what was registered. A value that does
		// not decode is used as it stands: that is a client which skipped the
		// encoding, and refusing it here would be refusing the credential over
		// a disagreement about transport rather than about the secret.
		clientID = unescapeCredential(clientID)
		clientSecret = unescapeCredential(clientSecret)
	} else {
		clientID = r.PostFormValue("client_id")
		clientSecret = r.PostFormValue("client_secret")
	}
	if clientID == "" {
		return nil, ErrInvalidClient
	}

	client, err := s.store.GetClient(r.Context(), clientID)
	if err != nil || client.Disabled {
		return nil, ErrInvalidClient
	}

	if client.IsPublic() {
		if clientSecret != "" {
			// A public client has no secret to present; something is confused
			// about its own registration.
			return nil, ErrInvalidClient
		}
		return client, nil
	}
	return s.store.VerifyClientSecret(r.Context(), clientID, clientSecret)
}

func unescapeCredential(value string) string {
	decoded, err := url.QueryUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}

func (s *SSOProvider) grantAuthorizationCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	client, err := s.authenticateClient(r)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="oauth2"`)
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return
	}

	code := r.PostFormValue("code")
	if code == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "code is required")
		return
	}

	codeHash := hashSecret(code)
	authCode, err := s.store.ConsumeAuthCode(ctx, codeHash)
	if errors.Is(err, ErrCodeReplayed) {
		// A code offered twice means one of the two presenters stole it. There
		// is no way to tell which, so everything minted from it dies.
		if revokeErr := s.store.RevokeByAuthCode(ctx, codeHash); revokeErr != nil {
			slog.Error("failed to revoke tokens from a replayed code", "error", revokeErr)
		}
		slog.Warn("authorization code replayed", "client_id", client.ClientID)
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "authorization code already used")
		return
	}
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "unknown or expired authorization code")
		return
	}

	if authCode.ClientID != client.ClientID {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "the code was issued to another client")
		return
	}
	// RFC 6749 §4.1.3: the redirect_uri presented here must be the one the code
	// was bound to, not merely one the client registered.
	if r.PostFormValue("redirect_uri") != authCode.RedirectURI {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri does not match the authorization request")
		return
	}

	verifier := r.PostFormValue("code_verifier")
	if !verifyPKCE(verifier, authCode.CodeChallenge) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "code_verifier does not match the code_challenge")
		return
	}

	s.store.TouchClient(ctx, client.ClientID)
	s.issueTokenSet(w, r, client, authCode.TenantID, &authCode.UserID, authCode.Scopes, authCode.Nonce, &codeHash, nil)
}

func (s *SSOProvider) grantRefreshToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	client, err := s.authenticateClient(r)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="oauth2"`)
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return
	}

	presented := r.PostFormValue("refresh_token")
	if presented == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "refresh_token is required")
		return
	}

	token, err := s.store.GetToken(ctx, presented, tokenTypeRefresh)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "unknown refresh token")
		return
	}
	if token.ClientID != client.ClientID {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "the refresh token was issued to another client")
		return
	}

	// Rotation means a live refresh token is used exactly once. Seeing a
	// revoked one again is the signature of a stolen copy being replayed, so
	// the whole lineage goes — including the token the thief's victim holds,
	// which is the point: the grant is over, both parties re-authenticate.
	if token.RevokedAt != nil {
		slog.Warn("revoked refresh token replayed; revoking the family",
			"client_id", client.ClientID, "token_id", token.ID)
		if revokeErr := s.store.RevokeFamily(ctx, token.ID); revokeErr != nil {
			slog.Error("failed to revoke a refresh token family", "error", revokeErr)
		}
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "refresh token has been revoked")
		return
	}
	if time.Now().After(token.ExpiresAt) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "refresh token has expired")
		return
	}

	// A refresh may narrow the scope set but never widen it (RFC 6749 §6).
	scopes := token.Scopes
	if requested := r.PostFormValue("scope"); requested != "" {
		narrowed := strings.Fields(requested)
		if !isSubset(narrowed, token.Scopes) {
			writeOAuthError(w, http.StatusBadRequest, "invalid_scope",
				"a refresh cannot widen the granted scope")
			return
		}
		scopes = narrowed
	}

	if err := s.store.RevokeToken(ctx, presented); err != nil {
		slog.Error("failed to retire a rotated refresh token", "error", err)
	}
	s.store.TouchClient(ctx, client.ClientID)
	s.issueTokenSet(w, r, client, token.TenantID, token.UserID, scopes, "", nil, &token.ID)
}

func (s *SSOProvider) grantClientCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	client, err := s.authenticateClient(r)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="oauth2"`)
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return
	}
	if client.IsPublic() {
		writeOAuthError(w, http.StatusBadRequest, "unauthorized_client",
			"a public client cannot use client_credentials: it has no secret to prove with")
		return
	}
	if !slices.Contains(client.GrantTypes, "client_credentials") {
		writeOAuthError(w, http.StatusBadRequest, "unauthorized_client",
			"this client is not registered for client_credentials")
		return
	}

	// There is no user in this grant, so identity scopes are meaningless and
	// asking for them is a sign the caller picked the wrong flow.
	scopes, err := resolveScopes(r.PostFormValue("scope"), client)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_scope", err.Error())
		return
	}
	scopes = slices.DeleteFunc(scopes, func(s string) bool {
		return s == "openid" || s == "offline_access"
	})

	s.store.TouchClient(ctx, client.ClientID)
	s.issueTokenSet(w, r, client, client.WorkspaceID, nil, scopes, "", nil, nil)
}

// issueTokenSet mints the access token, and the id_token and refresh token when
// the granted scopes call for them.
func (s *SSOProvider) issueTokenSet(w http.ResponseWriter, r *http.Request, client *Client,
	tenantID string, userID *string, scopes []string, nonce string,
	authCodeHash *string, parentID *string) {

	ctx := r.Context()
	now := time.Now()

	accessToken := "gat_" + generateRandomString(48)
	if _, err := s.store.SaveToken(ctx, &Token{
		TokenHash: hashSecret(accessToken), TokenType: tokenTypeAccess,
		ClientID: client.ClientID, TenantID: tenantID, UserID: userID, Scopes: scopes,
		ParentID: parentID, AuthCodeHash: authCodeHash,
		ExpiresAt: now.Add(accessTokenTTL),
	}); err != nil {
		slog.Error("failed to store an access token", "error", err)
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "could not issue a token")
		return
	}

	response := map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   int(accessTokenTTL.Seconds()),
		"scope":        strings.Join(scopes, " "),
	}

	// offline_access is what turns a sign-in into a standing grant, so it is
	// what gates the refresh token rather than issuing one unconditionally.
	if userID != nil && slices.Contains(scopes, "offline_access") {
		refreshToken := "grt_" + generateRandomString(48)
		if _, err := s.store.SaveToken(ctx, &Token{
			TokenHash: hashSecret(refreshToken), TokenType: tokenTypeRefresh,
			ClientID: client.ClientID, TenantID: tenantID, UserID: userID, Scopes: scopes,
			ParentID: parentID, AuthCodeHash: authCodeHash,
			ExpiresAt: now.Add(refreshTokenTTL),
		}); err != nil {
			slog.Error("failed to store a refresh token", "error", err)
		} else {
			response["refresh_token"] = refreshToken
		}
	}

	if userID != nil && slices.Contains(scopes, "openid") {
		idToken, err := s.mintIDToken(ctx, client, tenantID, *userID, scopes, nonce, now)
		if err != nil {
			slog.Error("failed to mint an id_token", "error", err)
		} else {
			response["id_token"] = idToken
		}
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, response)
}

// userProfile is the subset of the user record the identity scopes expose.
type userProfile struct {
	Email string
	Name  string
	// GeID is the citizen's number in the Gerege register, for an account that
	// signed in with eID. Zero for everybody else, and omitted from the claims
	// rather than sent as a zero — a relying party must be able to tell "this
	// person has no Gerege number" from "this person is number nought".
	GeID int64
}

func (s *SSOProvider) loadUser(ctx context.Context, userID string) (userProfile, error) {
	var p userProfile
	var geID *int64
	err := s.store.db.QueryRow(ctx,
		`SELECT email, name, ge_id FROM registry.users WHERE id = $1`, userID).
		Scan(&p.Email, &p.Name, &geID)
	if geID != nil {
		p.GeID = *geID
	}
	return p, err
}

// tenantSlug is the human-readable name of the organisation a token was issued
// for.
//
// tenant_id has always been in the token and it is the identifier that matters,
// but it is a UUID: a third-party platform receiving a sign-in has to map it to
// the customer it knows, and every one of them would otherwise keep a table of
// UUIDs to organisation names that this platform already has. A lookup failure
// is not an error worth failing a sign-in over — the claim is simply absent.
func (s *SSOProvider) tenantSlug(ctx context.Context, tenantID string) string {
	var slug string
	if err := s.store.db.QueryRow(ctx, `SELECT slug FROM registry.tenants WHERE id = $1`, tenantID).Scan(&slug); err != nil {
		slog.Warn("could not resolve the tenant slug for a token", "error", err, "tenant_id", tenantID)
		return ""
	}
	return slug
}

// grantedRoles is what the `roles` scope adds to a token: the codes of the
// roles this person holds in the organisation the token was issued for, and
// whether they are the administrator this whole deployment was set up by.
//
// platform_admin is deliberately narrow. It is not "an admin somewhere" — every
// organisation on a deployment has an `admin` role, and a relying party that
// treated all of them as operators of the platform would be handing the keys to
// whoever signed up most recently. It is `admin` **of the first organisation**:
// the one internal/operator/tenants.Bootstrap created, which only runs on a
// deployment that has none, so there is exactly one and it belongs to whoever
// stood the deployment up.
//
// Neither is worth failing a sign-in over. A lookup that errors returns no
// roles and no claim, the same as a person who holds none — a relying party
// then sees an ordinary user, which is the safe direction to fail in.
func (s *SSOProvider) grantedRoles(ctx context.Context, tenantID, userID string) ([]string, bool) {
	rows, err := s.store.db.Query(ctx,
		`SELECT r.code
		   FROM workspace.memberships m
		   JOIN workspace.membership_roles mr ON mr.membership_id = m.id
		   JOIN workspace.roles r ON r.id = mr.role_id
		  WHERE m.tenant_id = $1::uuid AND m.user_id = $2::uuid
		  ORDER BY r.code`, tenantID, userID)
	if err != nil {
		slog.Warn("could not read the roles for a token", "error", err, "tenant_id", tenantID)
		return nil, false
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			slog.Warn("could not read a role code for a token", "error", err)
			return nil, false
		}
		roles = append(roles, code)
	}
	if err := rows.Err(); err != nil {
		slog.Warn("could not read the roles for a token", "error", err)
		return nil, false
	}

	if !slices.Contains(roles, "admin") {
		return roles, false
	}

	// `kind = 'organisation'` is load-bearing, not tidiness. Since migration
	// 00085 every person gets a personal workspace, and those are rows in this
	// same table — on a deployment where one was created before the wizard ran,
	// the oldest row is somebody's home rather than the founding organisation,
	// and nobody would ever be the platform administrator.
	//
	// Ordered by created_at with the id as the tie-break, because two
	// organisations created inside the same clock tick would otherwise make
	// this answer change between one sign-in and the next.
	var root string
	if err := s.store.db.QueryRow(ctx,
		`SELECT id::text FROM registry.tenants
		  WHERE kind = 'organisation' ORDER BY created_at, id LIMIT 1`).Scan(&root); err != nil {
		slog.Warn("could not identify the first organisation", "error", err)
		return roles, false
	}
	return roles, root == tenantID
}

// mintIDToken builds the OIDC identity assertion. Claims follow the granted
// scopes: no email scope, no email claim.
func (s *SSOProvider) mintIDToken(ctx context.Context, client *Client, tenantID, userID string,
	scopes []string, nonce string, now time.Time) (string, error) {

	key, err := s.signingKey(ctx)
	if err != nil {
		return "", err
	}

	claims := map[string]any{
		"iss":       s.issuer,
		"sub":       userID,
		"aud":       client.ClientID,
		"iat":       now.Unix(),
		"exp":       now.Add(accessTokenTTL).Unix(),
		"auth_time": now.Unix(),
		"tenant_id": tenantID,
	}
	// Which organisation this person is acting for, by name. An external app
	// signs in a user on behalf of a tenant, and needs to know which.
	if slug := s.tenantSlug(ctx, tenantID); slug != "" {
		claims["tenant_slug"] = slug
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}

	if slices.Contains(scopes, "email") || slices.Contains(scopes, "profile") {
		profile, err := s.loadUser(ctx, userID)
		if err != nil {
			return "", err
		}
		if slices.Contains(scopes, "email") {
			claims["email"] = profile.Email
			claims["email_verified"] = true
		}
		if slices.Contains(scopes, "profile") {
			claims["name"] = profile.Name
		}
		// Both, and deliberately both. The address of an eID account is derived
		// from this number, so a relying party could parse one out of the
		// other — and would then be parsing an address, which is the kind of
		// thing that survives until the day the address form changes. The
		// number is stated.
		if profile.GeID != 0 {
			claims["ge_id"] = profile.GeID
		}
	}

	if slices.Contains(scopes, "roles") {
		roles, platformAdmin := s.grantedRoles(ctx, tenantID, userID)
		// An empty array rather than an absent claim: a relying party writing
		// `contains(roles, 'admin')` against a missing key gets an error in
		// some expression languages and silence in others, and neither is
		// "this person holds no roles".
		if roles == nil {
			roles = []string{}
		}
		claims["roles"] = roles
		claims["platform_admin"] = platformAdmin
	}

	return signJWT(key.KID, key.Private, claims)
}

// HandleUserInfo is the OIDC UserInfo endpoint. It answers for the bearer of an
// access token that carries the openid scope, and returns only the claims the
// granted scopes cover.
func (s *SSOProvider) HandleUserInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="oauth2"`)
		writeOAuthError(w, http.StatusUnauthorized, "invalid_token", "a bearer access token is required")
		return
	}

	token, err := s.store.GetToken(ctx, strings.TrimSpace(header[len(prefix):]), tokenTypeAccess)
	if err != nil || token.RevokedAt != nil || time.Now().After(token.ExpiresAt) {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		writeOAuthError(w, http.StatusUnauthorized, "invalid_token", "the access token is not valid")
		return
	}
	if token.UserID == nil {
		writeOAuthError(w, http.StatusForbidden, "insufficient_scope",
			"this token represents a machine client, not a user")
		return
	}
	if !slices.Contains(token.Scopes, "openid") {
		w.Header().Set("WWW-Authenticate", `Bearer error="insufficient_scope", scope="openid"`)
		writeOAuthError(w, http.StatusForbidden, "insufficient_scope", "the openid scope is required")
		return
	}

	profile, err := s.loadUser(ctx, *token.UserID)
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "could not load the user")
		return
	}

	claims := map[string]any{"sub": *token.UserID, "tenant_id": token.TenantID}
	if slug := s.tenantSlug(ctx, token.TenantID); slug != "" {
		claims["tenant_slug"] = slug
	}
	if slices.Contains(token.Scopes, "email") {
		claims["email"] = profile.Email
		claims["email_verified"] = true
	}
	if slices.Contains(token.Scopes, "profile") {
		claims["name"] = profile.Name
	}
	if profile.GeID != 0 {
		claims["ge_id"] = profile.GeID
	}
	if slices.Contains(token.Scopes, "roles") {
		roles, platformAdmin := s.grantedRoles(ctx, token.TenantID, *token.UserID)
		if roles == nil {
			roles = []string{}
		}
		claims["roles"] = roles
		claims["platform_admin"] = platformAdmin
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, claims)
}

// HandleIntrospect implements RFC 7662. Client authentication is required: an
// open introspection endpoint lets anyone test tokens for validity.
func (s *SSOProvider) HandleIntrospect(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "malformed form body")
		return
	}
	if _, err := s.authenticateClient(r); err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="oauth2"`)
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return
	}

	inactive := map[string]any{"active": false}
	presented := r.PostFormValue("token")
	if presented == "" {
		writeJSON(w, http.StatusOK, inactive)
		return
	}

	// The hint is advisory; try the other type if it does not pan out.
	tokenType := tokenTypeAccess
	if r.PostFormValue("token_type_hint") == "refresh_token" {
		tokenType = tokenTypeRefresh
	}
	token, err := s.store.GetToken(r.Context(), presented, tokenType)
	if err != nil {
		other := tokenTypeRefresh
		if tokenType == tokenTypeRefresh {
			other = tokenTypeAccess
		}
		token, err = s.store.GetToken(r.Context(), presented, other)
	}
	if err != nil || token.RevokedAt != nil || time.Now().After(token.ExpiresAt) {
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, inactive)
		return
	}

	response := map[string]any{
		"active":     true,
		"scope":      strings.Join(token.Scopes, " "),
		"client_id":  token.ClientID,
		"token_type": "Bearer",
		"exp":        token.ExpiresAt.Unix(),
		"iss":        s.issuer,
		"tenant_id":  token.TenantID,
	}
	if token.UserID != nil {
		response["sub"] = *token.UserID
	} else {
		response["sub"] = token.ClientID
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, response)
}

// HandleRevoke implements RFC 7009, including its insistence that an unknown
// token is a success: a client cleaning up must not learn anything from the
// difference, and has nothing useful to do about it either way.
func (s *SSOProvider) HandleRevoke(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "malformed form body")
		return
	}
	client, err := s.authenticateClient(r)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="oauth2"`)
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return
	}

	presented := r.PostFormValue("token")
	if presented != "" {
		// Revoking a refresh token takes its descendants with it; revoking an
		// access token is just that one token.
		if token, lookupErr := s.store.GetToken(r.Context(), presented, tokenTypeRefresh); lookupErr == nil {
			if token.ClientID == client.ClientID {
				if err := s.store.RevokeFamily(r.Context(), token.ID); err != nil {
					slog.Error("failed to revoke a token family", "error", err)
				}
			}
		} else if err := s.store.RevokeToken(r.Context(), presented); err != nil {
			slog.Error("failed to revoke a token", "error", err)
		}
	}

	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
}

// currentUser resolves the platform session behind a browser request.
func (s *SSOProvider) currentUser(r *http.Request) (auth.UserClaims, bool) {
	if s.sessions == nil {
		return auth.UserClaims{}, false
	}
	token := auth.TokenFromRequest(r)
	if token == "" {
		return auth.UserClaims{}, false
	}
	claims, err := s.sessions.Resolve(r.Context(), token)
	if err != nil {
		return auth.UserClaims{}, false
	}
	return claims, true
}

// authFailure reports a problem that happened before a redirect_uri could be
// trusted. It renders instead of redirecting, on purpose.
func (s *SSOProvider) authFailure(w http.ResponseWriter, r *http.Request, code, description string) {
	slog.Info("rejected an authorization request", "error", code, "description", description)
	writeOAuthError(w, http.StatusBadRequest, code, description)
}

func redirectSuccess(w http.ResponseWriter, r *http.Request, redirectURI, code, state string) {
	target, err := url.Parse(redirectURI)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "unparseable redirect_uri")
		return
	}
	q := target.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	target.RawQuery = q.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func redirectError(w http.ResponseWriter, r *http.Request, redirectURI, code, description, state string) {
	http.Redirect(w, r, errorRedirectURL(redirectURI, code, description, state), http.StatusFound)
}

func errorRedirectURL(redirectURI, code, description, state string) string {
	target, err := url.Parse(redirectURI)
	if err != nil {
		return redirectURI
	}
	q := target.Query()
	q.Set("error", code)
	q.Set("error_description", description)
	if state != "" {
		q.Set("state", state)
	}
	target.RawQuery = q.Encode()
	return target.String()
}

// verifyPKCE checks a code_verifier against its S256 challenge (RFC 7636 §4.6).
func verifyPKCE(verifier, challenge string) bool {
	// RFC 7636 §4.1 fixes the verifier at 43-128 characters; a short one would
	// be brute-forceable.
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
}

// resolveScopes narrows a request to what the client is registered for,
// defaulting to the client's full set when the request names none.
func resolveScopes(requested string, client *Client) ([]string, error) {
	if strings.TrimSpace(requested) == "" {
		return client.Scopes, nil
	}
	asked := strings.Fields(requested)
	for _, scope := range asked {
		if !IsSupportedScope(scope) {
			return nil, errors.New("unknown scope: " + scope)
		}
		if !slices.Contains(client.Scopes, scope) {
			return nil, errors.New("scope not registered for this client: " + scope)
		}
	}
	return asked, nil
}

func isSubset(needles, haystack []string) bool {
	for _, n := range needles {
		if !slices.Contains(haystack, n) {
			return false
		}
	}
	return true
}

func union(a, b []string) []string {
	out := slices.Clone(a)
	for _, v := range b {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	slices.Sort(out)
	return out
}
