/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package ssoclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Identity is who the provider says somebody is.
//
// Subject is the only field that is guaranteed and the only one that is a
// stable name for this person: an email address changes, a display name
// changes, the subject does not. Everything else is what the granted scopes
// happened to carry.
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	// TenantSlug and TenantID are what a Gerege Nexus provider says about which
	// organisation the person was acting for. Another provider will not send
	// them, and this client works without them.
	TenantSlug string
	TenantID   string

	// IDToken is the raw token, kept so it can be presented back to the
	// provider as an id_token_hint when this person signs out.
	IDToken string

	// Claims is everything the provider actually said, verified, before this
	// struct picked the handful of fields a sign-in needs. It is kept because
	// a person is entitled to see what was handed over about them, and because
	// a provider that starts sending a new claim should not need a schema
	// change here before anybody can see it.
	Claims map[string]any
}

// ErrNoIDToken means the provider answered the token endpoint without one,
// which makes the exchange OAuth2 rather than OpenID Connect and leaves nothing
// signed to identify anybody by.
var ErrNoIDToken = errors.New("the SSO provider returned no id_token")

// Exchange redeems an authorization code and returns the verified identity.
//
// The verifier is the PKCE proof minted when the request began, and the nonce
// is what binds the returned id_token to that same request. Both are the
// caller's to have kept; this function is where they are spent.
func (c *Client) Exchange(ctx context.Context, code, codeVerifier, nonce string) (*Identity, error) {
	meta, err := c.discovery.get(ctx)
	if err != nil {
		return nil, err
	}

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.cfg.RedirectURI},
		"code_verifier": {codeVerifier},
		"client_id":     {c.cfg.ClientID},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, meta.TokenEndpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if c.cfg.Confidential() {
		// HTTP Basic rather than the form body: RFC 6749 §2.3.1 says a client
		// that can use it should, and it keeps the secret out of anything that
		// logs request bodies.
		req.SetBasicAuth(url.QueryEscape(c.cfg.ClientID), url.QueryEscape(c.cfg.ClientSecret))
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reach the SSO provider's token endpoint: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read the SSO provider's answer: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		// The OAuth2 error code, when there is one, is the useful half of a
		// failed exchange — "invalid_grant" and "invalid_client" send an
		// operator to two completely different places.
		var oauthErr struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &oauthErr)
		if oauthErr.Error != "" {
			return nil, fmt.Errorf("the SSO provider refused the code exchange: %s (%s)",
				oauthErr.Error, oauthErr.Description)
		}
		return nil, fmt.Errorf("the SSO provider answered %d to the code exchange", res.StatusCode)
	}

	var tokens struct {
		IDToken     string `json:"id_token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("parse the SSO provider's token response: %w", err)
	}
	if tokens.IDToken == "" {
		return nil, ErrNoIDToken
	}

	claims, err := c.verifyIDToken(ctx, meta, tokens.IDToken, nonce)
	if err != nil {
		return nil, err
	}

	identity := &Identity{
		Claims:        claims,
		Subject:       stringClaim(claims, "sub"),
		Email:         stringClaim(claims, "email"),
		EmailVerified: boolClaim(claims, "email_verified"),
		Name:          stringClaim(claims, "name"),
		TenantSlug:    stringClaim(claims, "tenant_slug"),
		TenantID:      stringClaim(claims, "tenant_id"),
		IDToken:       tokens.IDToken,
	}
	if identity.Subject == "" {
		return nil, errors.New("the SSO provider's id_token carries no subject")
	}

	// A provider may hold the profile claims back from the id_token and put
	// them only at UserInfo; asking there costs one request and is the
	// difference between a provisioned account with a name and one called after
	// an opaque subject identifier.
	if identity.Email == "" || identity.Name == "" {
		c.fillFromUserInfo(ctx, meta, tokens.AccessToken, identity)
	}
	return identity, nil
}

// fillFromUserInfo tops up the claims the id_token did not carry. It is best
// effort on purpose: the identity is already proved and verified by this point,
// and a UserInfo endpoint that is slow or absent must not fail a sign-in that
// has otherwise succeeded.
func (c *Client) fillFromUserInfo(ctx context.Context, meta *Metadata, accessToken string, identity *Identity) {
	if meta.UserInfoEndpoint == "" || accessToken == "" {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, meta.UserInfoEndpoint, nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return
	}
	var claims map[string]any
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&claims); err != nil {
		return
	}
	// The subject has to agree. A UserInfo response describing somebody else is
	// either a provider bug or a mix-up attack, and either way the safe reading
	// is that it says nothing about the person who just signed in.
	if stringClaim(claims, "sub") != identity.Subject {
		return
	}
	// Merged under a key of its own: these came from a different endpoint and
	// flattening them together would lose which of the two said what.
	if identity.Claims != nil {
		identity.Claims["userinfo"] = claims
	}
	if identity.Email == "" {
		identity.Email = stringClaim(claims, "email")
		identity.EmailVerified = identity.EmailVerified || boolClaim(claims, "email_verified")
	}
	if identity.Name == "" {
		identity.Name = stringClaim(claims, "name")
	}
	if identity.TenantSlug == "" {
		identity.TenantSlug = stringClaim(claims, "tenant_slug")
	}
}

func stringClaim(claims map[string]any, name string) string {
	if value, ok := claims[name].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func boolClaim(claims map[string]any, name string) bool {
	value, _ := claims[name].(bool)
	return value
}
