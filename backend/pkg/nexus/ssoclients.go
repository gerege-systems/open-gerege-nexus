/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import (
	"context"
	"errors"
	"time"
)

// The authorization server, as the app that administers it sees it.
//
// The provider stays the platform's. Issuing codes, exchanging tokens, signing
// id_tokens, holding the clients and enforcing what a redirect may be are one
// authorization server this deployment shares, and none of it is an app's
// decision — CORE_BOUNDARY_PLAN §4.2, the same rule that keeps the report
// engine in the core and puts its screens in a distribution.
//
// What an app decides is what a registration has to look like before the
// provider is allowed to hold it, and which of them a tenant may see. That is
// io.gerege.nexus.sso_clients, and since 2026-08-25 it lives in
// github.com/gerege-systems/appstore-gerege-nexus rather than here.
//
// It was calling ssoprovider directly — twenty exported names under internal/,
// so the screens could not have been built anywhere else. Sixteen methods here
// instead, named for what the app does. The provider's own shape is not
// committed to semver by any of it: what crosses this boundary is a client
// registration and what it is doing, not a *Store.
type SSOClientRegistry interface {
	// Issuer is this deployment's OIDC issuer — the origin an integrator's
	// client library is configured with, and the prefix of every endpoint the
	// app hands them.
	Issuer() string

	// SupportedScopes and SupportedGrantTypes are the vocabulary a client may
	// be registered with. The app renders them in its picker so the screen and
	// the consent page cannot drift apart.
	SupportedScopes() []SSOScope
	SupportedGrantTypes() []string
	IsSupportedScope(name string) bool

	// AllowedRedirect reports whether this deployment's operator permits a
	// redirect URI, and says why not when it does not.
	//
	// The provider's rule, not the app's: an unchecked return address turns
	// the authorization endpoint into an open redirector, so the answer has to
	// be the same one HandleAuthorize will give later.
	AllowedRedirect(raw string) error

	// NewIdentifier is the platform's randomness — the only source in this
	// deployment that an audit has looked at. A module generating a client_id
	// or a secret from its own is generating it from one that nobody has.
	NewIdentifier(n int) string

	ListClients(ctx context.Context, tenantID string) ([]SSOClient, error)

	// GetClient answers ErrSSOClientNotFound when this tenant does not own a
	// client by that id, which is also the answer when somebody else does.
	GetClient(ctx context.Context, tenantID, clientID string) (SSOClient, error)

	// CreateClient registers a client and returns it. The secret is passed in
	// plaintext and stored as a digest; a public client is created with an
	// empty one. What comes back carries the plaintext exactly once — this is
	// the only call that can, because the database cannot reproduce it.
	CreateClient(ctx context.Context, tenantID string, c SSOClient, secret, createdBy string) (SSOClient, error)

	UpdateClient(ctx context.Context, tenantID string, c SSOClient) (SSOClient, error)
	DeleteClient(ctx context.Context, tenantID, clientID string) error

	// RotateClientSecret replaces the digest, invalidating the old secret.
	RotateClientSecret(ctx context.Context, tenantID, clientID, secret string) error

	// ClientActivity is what this tenant's clients are actually doing: live
	// tokens, standing consents, and when each credential was last exchanged.
	ClientActivity(ctx context.Context, tenantID string) ([]SSOClientActivity, error)

	// Consents lists the standing grants users have given this tenant's
	// clients, most recent first, up to limit.
	Consents(ctx context.Context, tenantID string, limit int) ([]SSOConsent, error)

	// RevokeClientTokens invalidates every live token a client holds without
	// deleting the registration, and reports how many it killed.
	RevokeClientTokens(ctx context.Context, tenantID, clientID string) (int64, error)

	// WithdrawConsent removes one user's standing grant to one client.
	WithdrawConsent(ctx context.Context, tenantID, clientID, userID string) error

	// SigningKeys is the public half of what the JWKS publishes, so an
	// integrator can see which kid to pin. No private material crosses here,
	// and none is selected on the way.
	SigningKeys(ctx context.Context) ([]SSOSigningKey, error)
}

// ErrSSOClientNotFound is what every lookup answers when it finds nothing, so a
// caller never has to tell "absent" from "belongs to somebody else".
var ErrSSOClientNotFound = errors.New("nexus: no such SSO client")

// SSOScope is one permission a client can ask a user to grant. Description and
// DescriptionMN are what the consent screen renders, so what somebody approves
// is something they can read.
type SSOScope struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	DescriptionMN string `json:"description_mn"`
	Sensitive     bool   `json:"sensitive"`
}

// SSOClient is a registered OAuth2 client.
//
// The JSON is the wire format the shell's screens have always read; it is part
// of this contract for that reason, and not only because the app happens to
// marshal the struct.
type SSOClient struct {
	ID           string   `json:"id"`
	TenantID     string   `json:"tenant_id"`
	ClientID     string   `json:"client_id"`
	ClientName   string   `json:"client_name"`
	ClientURI    string   `json:"client_uri,omitempty"`
	LogoURI      string   `json:"logo_uri,omitempty"`
	ClientType   string   `json:"client_type"`
	RedirectURIs []string `json:"redirect_uris"`
	GrantTypes   []string `json:"grant_types"`
	Scopes       []string `json:"scopes"`
	Disabled     bool     `json:"disabled"`
	// PostLogoutRedirectURIs is where this client may send somebody after
	// signing them out here. Matched exactly, like RedirectURIs.
	PostLogoutRedirectURIs []string `json:"post_logout_redirect_uris"`

	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	SecretRotatedAt *time.Time `json:"secret_rotated_at,omitempty"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`

	// Secret carries the plaintext exactly once, in the answer to the call
	// that created or rotated it.
	Secret string `json:"client_secret,omitempty"`
}

// SSOClientActivity is one client's live footprint.
type SSOClientActivity struct {
	ClientID       string     `json:"client_id"`
	ClientName     string     `json:"client_name"`
	ClientType     string     `json:"client_type"`
	Disabled       bool       `json:"disabled"`
	ActiveAccess   int        `json:"active_access_tokens"`
	ActiveRefresh  int        `json:"active_refresh_tokens"`
	ConsentedUsers int        `json:"consented_users"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
}

// SSOConsent is one user's standing grant to one client.
type SSOConsent struct {
	ClientID   string    `json:"client_id"`
	ClientName string    `json:"client_name"`
	UserID     string    `json:"user_id"`
	UserEmail  string    `json:"user_email"`
	UserName   string    `json:"user_name"`
	Scopes     []string  `json:"scopes"`
	GrantedAt  time.Time `json:"granted_at"`
}

// SSOSigningKey describes a published key without any private material.
type SSOSigningKey struct {
	KID       string     `json:"kid"`
	Algorithm string     `json:"algorithm"`
	Active    bool       `json:"active"`
	CreatedAt time.Time  `json:"created_at"`
	RetiredAt *time.Time `json:"retired_at,omitempty"`
}
