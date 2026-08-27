/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * RP-initiated logout (OpenID Connect RP-Initiated Logout 1.0).
 *
 * The discovery document has advertised end_session_endpoint since it was
 * written, and nothing served it: a relying party that ended its own session
 * and sent the person here — which is what a conformant client does — landed on
 * a 404 while staying signed in at the provider. That is worse than not
 * advertising it at all, because the next click on "sign in" walks straight
 * back into the still-live session and looks like the logout was ignored.
 */

package ssoprovider

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
)

// Why an id_token_hint was not believed. They are never shown to the caller —
// the hint is optional, so a bad one costs the caller nothing but a log line.
var (
	errMalformedToken       = errors.New("id_token_hint is not a compact JWS")
	errUnsupportedAlgorithm = errors.New("id_token_hint is not a kid-bearing RS256 token")
	errUnknownKey           = errors.New("id_token_hint names a key this provider never published")
	errBadSignature         = errors.New("id_token_hint signature does not verify")
)

// SessionEnder ends the platform session a browser is holding.
//
// It is separate from SessionResolver because the two are needed by different
// endpoints and a provider wired with only the first is still a working
// authorization server — it simply cannot honour a logout, and says so in the
// log rather than pretending it did.
type SessionEnder interface {
	Revoke(ctx context.Context, token string) error
}

// AttachSessionEnder wires session revocation into the logout endpoint.
func (s *SSOProvider) AttachSessionEnder(ender SessionEnder) { s.endSessions = ender }

// HandleEndSession implements RP-initiated logout.
//
// The contract is narrow on purpose: it ends the *provider's* session for the
// browser making the request, and then returns the person to the relying party
// that sent them, if that relying party registered somewhere to return them to.
// It does not revoke the client's tokens — those have their own endpoint
// (RFC 7009) and their own short lifetime, and a logout that silently killed a
// backend integration's refresh token would break a machine grant every time a
// person closed a tab.
//
// Both GET and POST are served. The specification allows either, and a client
// redirecting a browser can only produce the first.
func (s *SSOProvider) HandleEndSession(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "malformed request")
		return
	}
	ctx := r.Context()

	// End the session before anything else can fail. Whatever happens to the
	// redirect afterwards, the person asked to be signed out and must be.
	s.endBrowserSession(r)
	auth.ClearSessionCookie(w)

	client := s.logoutClient(ctx, r.Form.Get("client_id"), r.Form.Get("id_token_hint"))
	requested := strings.TrimSpace(r.Form.Get("post_logout_redirect_uri"))
	if requested == "" {
		// Nothing to return to. The person is signed out and lands on the
		// platform they signed out of, which is the only page we know is ours.
		http.Redirect(w, r, s.issuer+"/", http.StatusFound)
		return
	}

	// An unregistered return address is refused rather than followed. This is
	// the whole security question the endpoint poses: a logout URL is one a
	// client hands out freely, so anybody can put a post_logout_redirect_uri on
	// it, and following one unchecked would make the provider forward browsers
	// wherever a phishing page asked.
	if client == nil || !slices.Contains(client.PostLogoutRedirectURIs, requested) {
		slog.Info("refused a post-logout redirect that no client has registered",
			"post_logout_redirect_uri", requested, "client_known", client != nil)
		writeOAuthError(w, http.StatusBadRequest, "invalid_request",
			"post_logout_redirect_uri is not registered for this client")
		return
	}

	target := requested
	if state := r.Form.Get("state"); state != "" {
		if parsed, err := url.Parse(requested); err == nil {
			q := parsed.Query()
			q.Set("state", state)
			parsed.RawQuery = q.Encode()
			target = parsed.String()
		}
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// endBrowserSession revokes the platform session the request carries.
func (s *SSOProvider) endBrowserSession(r *http.Request) {
	token := auth.TokenFromRequest(r)
	if token == "" {
		return
	}
	if s.endSessions == nil {
		slog.Warn("a logout request arrived but no session store is wired to end it")
		return
	}
	if err := s.endSessions.Revoke(r.Context(), token); err != nil {
		slog.Error("failed to end a session on an RP-initiated logout", "error", err)
	}
}

// logoutClient works out which relying party is asking.
//
// client_id is taken at face value — naming a client is not a permission, and
// what the caller can do with the answer is bounded by that client's registered
// return addresses. An id_token_hint is only believed if it verifies against a
// key this provider published; an unverifiable one is ignored rather than
// refused, because the person is already signed out by the time it is read and
// failing here would strand them on an error page.
func (s *SSOProvider) logoutClient(ctx context.Context, clientID, idTokenHint string) *Client {
	if clientID == "" && idTokenHint != "" {
		if claims, err := s.verifyOwnIDToken(ctx, idTokenHint); err == nil {
			if aud, ok := claims["aud"].(string); ok {
				clientID = aud
			}
		} else {
			slog.Info("ignored an id_token_hint that did not verify", "error", err)
		}
	}
	if clientID == "" {
		return nil
	}
	client, err := s.store.GetClient(ctx, clientID)
	if err != nil {
		return nil
	}
	return client
}

// verifyOwnIDToken checks a compact RS256 JWS this provider signed and returns
// its claims. It is the only place the provider reads a JWT, so it is
// deliberately strict: RS256 only, kid required, signature checked before a
// single claim is looked at. Expiry is not enforced — a hint's job is to say
// who was signed in, and an id_token that has just expired still says it.
func (s *SSOProvider) verifyOwnIDToken(ctx context.Context, token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errMalformedToken
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errMalformedToken
	}
	var header struct {
		Alg string `json:"alg"`
		KID string `json:"kid"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, errMalformedToken
	}
	if header.Alg != "RS256" || header.KID == "" {
		return nil, errUnsupportedAlgorithm
	}

	keys, err := s.store.VerificationKeys(ctx)
	if err != nil {
		return nil, err
	}
	key, ok := keys[header.KID]
	if !ok {
		return nil, errUnknownKey
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errMalformedToken
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
		return nil, errBadSignature
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errMalformedToken
	}
	var claims map[string]any
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, errMalformedToken
	}
	return claims, nil
}
