/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
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

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/auth"
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

	granted, _ := s.store.GetConsent(r.Context(), claims.UserID, req.Client.ClientID)

	prompt := ConsentPrompt{
		ClientID: req.Client.ClientID, ClientName: req.Client.ClientName,
		ClientURI: req.Client.ClientURI, LogoURI: req.Client.LogoURI,
		RedirectURI: req.RedirectURI, AlreadyGranted: granted,
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
	if err := s.store.SaveConsent(ctx, claims.TenantID, claims.UserID, req.Client.ClientID,
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

// oauthError is a code/description pair destined for the client.
type oauthError struct {
	Code        string
	Description string
}

func (e *oauthError) Error() string { return e.Code + ": " + e.Description }

// parseConsentQuery validates the subset of the authorization request that the
// consent screen round-trips.
func (s *SSOProvider) parseConsentQuery(ctx context.Context, values url.Values) (*authRequest, *oauthError) {
	client, err := s.store.GetClient(ctx, values.Get("client_id"))
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
		TenantID:            claims.TenantID,
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
	if !hasBasic {
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
	s.issueTokenSet(w, r, client, client.TenantID, nil, scopes, "", nil, nil)
}

// issueTokenSet mints the access token, and the id_token and refresh token when
// the granted scopes call for them.
func (s *SSOProvider) issueTokenSet(w http.ResponseWriter, r *http.Request, client *Client,
	tenantID string, userID *string, scopes []string, nonce string,
	authCodeHash *string, parentID *string) {

	ctx := r.Context()
	now := time.Now()

	accessToken := "gat_" + generateRandomString(48)
	stored, err := s.store.SaveToken(ctx, &Token{
		TokenHash: hashSecret(accessToken), TokenType: tokenTypeAccess,
		ClientID: client.ClientID, TenantID: tenantID, UserID: userID, Scopes: scopes,
		ParentID: parentID, AuthCodeHash: authCodeHash,
		ExpiresAt: now.Add(accessTokenTTL),
	})
	if err != nil {
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

	_ = stored
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, response)
}

// userProfile is the subset of the user record the identity scopes expose.
type userProfile struct {
	Email string
	Name  string
}

func (s *SSOProvider) loadUser(ctx context.Context, userID string) (userProfile, error) {
	var p userProfile
	err := s.store.db.QueryRow(ctx, `SELECT email, name FROM users WHERE id = $1`, userID).
		Scan(&p.Email, &p.Name)
	return p, err
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
	if slices.Contains(token.Scopes, "email") {
		claims["email"] = profile.Email
		claims["email_verified"] = true
	}
	if slices.Contains(token.Scopes, "profile") {
		claims["name"] = profile.Name
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
