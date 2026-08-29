/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Package ssoprovider implements the platform's OAuth2 Authorization Server and
 * OpenID Connect Provider.
 *
 * Supported today, and advertised in the discovery document only because it is
 * supported: authorization_code with mandatory PKCE, refresh_token with
 * rotation and replay detection, and client_credentials for machine callers.
 * Access tokens are opaque and validated at /oauth2/introspect; id_tokens are
 * RS256 JWTs verifiable against /.well-known/jwks.json.
 */

package ssoprovider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Lifetimes. Access tokens are short because they cannot be revoked mid-flight
// by a resource server that caches introspection results; refresh tokens carry
// the long-lived grant and rotate on every use.
const (
	accessTokenTTL  = 1 * time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour
	authCodeTTL     = 5 * time.Minute
)

// ErrInvalidClient covers every client authentication failure. Callers must not
// distinguish "unknown client" from "wrong secret", or the endpoint becomes a
// client_id oracle.
var ErrInvalidClient = errors.New("invalid_client")

// Scope is a permission a client can ask a user to grant. Description and
// DescriptionMN are what the consent screen actually renders, so a user is
// approving something they can read rather than a bare identifier.
type Scope struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	DescriptionMN string `json:"description_mn"`
	// Sensitive scopes are called out on the consent screen.
	Sensitive bool `json:"sensitive"`
}

// SupportedScopes is the whole vocabulary. A client may not be registered with,
// nor request, anything outside it.
var SupportedScopes = []Scope{
	{Name: "openid", Description: "Sign you in and confirm your identity", DescriptionMN: "Таныг нэвтрүүлж, хэн болохыг баталгаажуулна"},
	{Name: "profile", Description: "Read your name and account details", DescriptionMN: "Таны нэр болон бүртгэлийн мэдээллийг харна"},
	{Name: "email", Description: "Read your email address", DescriptionMN: "Таны и-мэйл хаягийг харна"},
	{Name: "phone", Description: "Read your phone number", DescriptionMN: "Таны утасны дугаарыг харна"},
	{Name: "offline_access", Description: "Stay signed in when you are away", DescriptionMN: "Та байхгүй үед ч холболтоо хадгална"},
	// What the person may do here, so a relying party can decide what to show
	// them without keeping its own copy of this platform's org chart. Grafana
	// is the first consumer: it maps `platform_admin` onto its own server
	// administrator, which is how the person who set this deployment up ends up
	// owning the dashboards without a second account being created anywhere.
	//
	// Sensitive, because an application that knows a person is an
	// administrator knows which account is worth attacking.
	{Name: "roles", Description: "Read your roles in this organisation", DescriptionMN: "Танай байгууллага дахь таны эрхийг харна", Sensitive: true},
	{Name: "erp.read", Description: "Read your organisation's ERP data", DescriptionMN: "Танай байгууллагын ERP өгөгдлийг унших", Sensitive: true},
	{Name: "erp.write", Description: "Create and change your organisation's ERP data", DescriptionMN: "Танай байгууллагын ERP өгөгдлийг үүсгэх, өөрчлөх", Sensitive: true},
}

// SupportedGrantTypes is what the token endpoint actually implements.
var SupportedGrantTypes = []string{"authorization_code", "refresh_token", "client_credentials"}

func scopeNames() []string {
	names := make([]string, 0, len(SupportedScopes))
	for _, s := range SupportedScopes {
		names = append(names, s.Name)
	}
	return names
}

// IsSupportedScope reports whether a scope is in the vocabulary above.
func IsSupportedScope(name string) bool {
	return slices.ContainsFunc(SupportedScopes, func(s Scope) bool { return s.Name == name })
}

// LookupScope returns the descriptions for a scope name.
func LookupScope(name string) (Scope, bool) {
	for _, s := range SupportedScopes {
		if s.Name == name {
			return s, true
		}
	}
	return Scope{}, false
}

// SSOProvider is the authorization server.
type SSOProvider struct {
	store  *Store
	issuer string

	// sessions resolves the platform session behind a browser hitting the
	// authorization endpoint. Wired by AttachSessions after construction,
	// because the session store and the provider are both built by the Server.
	sessions SessionResolver

	// endSessions ends that session when a relying party sends the browser to
	// the logout endpoint. See AttachSessionEnder.
	endSessions SessionEnder

	// installs answers whether the signing-in user's tenant has installed the
	// app a client belongs to. Nil means nothing is gated. See InstallGate.
	installs InstallGate

	keyMu sync.RWMutex
	key   *signingKey
}

// NewSSOProvider builds the provider over a database. The previous constructor
// took no arguments and kept clients and tokens in Go maps, which meant every
// deploy silently deregistered every integration.
func NewSSOProvider(db *pgxpool.Pool) *SSOProvider {
	issuer := strings.TrimSuffix(os.Getenv("SSO_ISSUER"), "/")
	if issuer == "" {
		issuer = strings.TrimSuffix(os.Getenv("PUBLIC_ORIGIN"), "/")
	}
	if issuer == "" {
		issuer = "http://localhost:8080"
		slog.Warn("SSO_ISSUER is not set; falling back to localhost, which no external client can reach",
			"issuer", issuer)
	}

	return &SSOProvider{store: NewStore(db), issuer: issuer}
}

// Issuer is the origin this provider names itself by in tokens and discovery.
func (s *SSOProvider) Issuer() string { return s.issuer }

// Store exposes persistence to the developer portal module, which manages
// clients on a tenant's behalf.
func (s *SSOProvider) Store() *Store { return s.store }

// signingKey returns the active key, loading it once and caching it.
func (s *SSOProvider) signingKey(ctx context.Context) (*signingKey, error) {
	s.keyMu.RLock()
	cached := s.key
	s.keyMu.RUnlock()
	if cached != nil {
		return cached, nil
	}
	s.keyMu.Lock()
	defer s.keyMu.Unlock()
	if s.key != nil {
		return s.key, nil
	}
	key, err := s.store.ActiveSigningKey(ctx)
	if err != nil {
		return nil, err
	}
	s.key = key
	return key, nil
}

// EnsureDefaultClient registers the platform's own first-party client from
// SSO_DEFAULT_CLIENT_SECRET.
//
// Clients are tenant-owned now, so this one needs an owner: the tenant named by
// SSO_DEFAULT_CLIENT_TENANT (slug, default "demo"). If that tenant does not
// exist the client is skipped rather than created ownerless — an ownerless row
// would be invisible to every developer portal and impossible to rotate.
func (s *SSOProvider) EnsureDefaultClient(ctx context.Context) {
	secret := os.Getenv("SSO_DEFAULT_CLIENT_SECRET")
	if secret == "" {
		slog.Info("SSO_DEFAULT_CLIENT_SECRET is not set; skipping the built-in client")
		return
	}

	slug := os.Getenv("SSO_DEFAULT_CLIENT_TENANT")
	if slug == "" {
		slug = "demo"
	}

	var tenantID string
	if err := s.store.db.QueryRow(ctx,
		`SELECT id::text FROM registry.tenants WHERE slug = $1`, slug).Scan(&tenantID); err != nil {
		slog.Info("skipping the built-in SSO client: its owning tenant does not exist",
			"tenant_slug", slug)
		return
	}

	const clientID = "gerege-dev-portal"
	if existing, err := s.store.GetClient(ctx, clientID); err == nil {
		// Keep the secret in step with the environment so rotating the
		// deployment secret actually rotates the credential.
		if err := s.store.RotateClientSecret(ctx, existing.WorkspaceID, clientID, hashSecret(secret)); err != nil {
			slog.Error("failed to sync the built-in SSO client secret", "error", err)
		}
		return
	}

	_, err := s.store.CreateClient(ctx, &Client{
		WorkspaceID:  tenantID,
		ClientID:     clientID,
		ClientName:   "Gerege Developer Portal",
		ClientType:   clientTypeConfidential,
		RedirectURIs: []string{s.issuer + "/oauth/callback"},
		// Where a sign-out started by this client returns the person. Without a
		// registered address the logout endpoint refuses to forward them, which
		// for the platform's own client would mean its own logout dead-ends.
		PostLogoutRedirectURIs: []string{s.issuer + "/"},
		GrantTypes:             SupportedGrantTypes,
		// offline_access belongs here because the grant list above includes
		// refresh_token, and offline_access is the only scope that produces a
		// refresh token — registering one without the other left the client
		// holding a grant it could never exercise.
		Scopes: []string{"openid", "profile", "email", "offline_access", "erp.read", "erp.write"},
	}, hashSecret(secret), "")
	if err != nil {
		slog.Error("failed to register the built-in SSO client", "error", err)
		return
	}
	slog.Info("registered the built-in SSO client", "client_id", clientID, "tenant_slug", slug)
}

// EnsureConsoleClient registers the developer console at developer.gerege.mn.
//
// The console is a public client with PKCE: it ships no secret, because a
// secret baked into a container image anybody can pull is not a secret. That is
// exactly the case PKCE exists for, and this provider requires PKCE of every
// client anyway.
//
// It is registered from configuration rather than by hand for the same reason
// the built-in client is: a client that has to exist for a deployment to work
// should not depend on somebody remembering to create it, and a redirect URI
// typed into a form is a redirect URI that can be typed wrongly.
//
// Nothing happens unless DEVELOPER_CONSOLE_ORIGIN is set, so a deployment
// without a console is untouched.
func (s *SSOProvider) EnsureConsoleClient(ctx context.Context) {
	origin := strings.TrimSuffix(strings.TrimSpace(os.Getenv("DEVELOPER_CONSOLE_ORIGIN")), "/")
	if origin == "" {
		return
	}
	clientID := strings.TrimSpace(os.Getenv("DEVELOPER_CONSOLE_CLIENT_ID"))
	if clientID == "" {
		clientID = "gerege-developer-console"
	}

	slug := os.Getenv("SSO_DEFAULT_CLIENT_TENANT")
	if slug == "" {
		slug = "demo"
	}
	var tenantID string
	if err := s.store.db.QueryRow(ctx,
		`SELECT id::text FROM registry.tenants WHERE slug = $1`, slug).Scan(&tenantID); err != nil {
		slog.Info("skipping the developer console client: its owning tenant does not exist",
			"tenant_slug", slug)
		return
	}

	redirect := origin + "/auth/callback"
	postLogout := origin + "/"
	if existing, err := s.store.GetClient(ctx, clientID); err == nil {
		// The origin can move — a deployment renames a host, a staging console
		// appears — and the redirect URI has to follow it or every sign-in
		// fails with a mismatch nobody can fix from the console itself. The
		// post-logout address moves with it for the same reason.
		if !slices.Contains(existing.RedirectURIs, redirect) ||
			!slices.Contains(existing.PostLogoutRedirectURIs, postLogout) {
			updated := *existing
			updated.ClientURI = origin
			updated.RedirectURIs = []string{redirect}
			updated.PostLogoutRedirectURIs = []string{postLogout}
			if _, err := s.store.UpdateClient(ctx, existing.WorkspaceID, &updated); err != nil {
				slog.Error("could not point the developer console client at its origin",
					"error", err, "redirect_uri", redirect)
				return
			}
			slog.Info("updated the developer console redirect URI", "redirect_uri", redirect)
		}
		return
	}

	if _, err := s.store.CreateClient(ctx, &Client{
		WorkspaceID:            tenantID,
		ClientID:               clientID,
		ClientName:             "Gerege Developer Console",
		ClientURI:              origin,
		ClientType:             clientTypePublic,
		RedirectURIs:           []string{redirect},
		PostLogoutRedirectURIs: []string{postLogout},
		// No refresh_token: the console holds an identity for an hour and asks
		// the person to sign in again, which for something opened a few times a
		// week is a better trade than storing a long-lived credential.
		GrantTypes: []string{"authorization_code"},
		Scopes:     []string{"openid", "profile", "email"},
	}, "", ""); err != nil {
		slog.Error("failed to register the developer console client", "error", err)
		return
	}
	slog.Info("registered the developer console client", "client_id", clientID, "origin", origin)
}

// StartJanitor sweeps spent codes and dead tokens until ctx is cancelled.
// Without it the tables grow without bound, which is the durable version of the
// unbounded token map this provider used to keep in memory.
func (s *SSOProvider) StartJanitor(ctx context.Context, every time.Duration) {
	go func() {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				removed, err := s.store.DeleteExpired(sweepCtx)
				cancel()
				if err != nil {
					slog.Warn("oauth2 janitor sweep failed", "error", err)
					continue
				}
				if removed > 0 {
					slog.Info("oauth2 janitor reclaimed expired rows", "rows", removed)
				}
			}
		}
	}()
}

// HandleOIDCDiscovery advertises exactly what is implemented.
//
// The earlier document listed an empty response_types_supported and only
// client_credentials, which told conformant clients that interactive sign-in
// was impossible. It is now implemented, so it is now advertised.
func (s *SSOProvider) HandleOIDCDiscovery(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                s.issuer,
		"authorization_endpoint":                s.issuer + "/oauth2/auth",
		"token_endpoint":                        s.issuer + "/oauth2/token",
		"userinfo_endpoint":                     s.issuer + "/oauth2/userinfo",
		"jwks_uri":                              s.issuer + "/.well-known/jwks.json",
		"introspection_endpoint":                s.issuer + "/oauth2/introspect",
		"revocation_endpoint":                   s.issuer + "/oauth2/revoke",
		"end_session_endpoint":                  s.issuer + "/oauth2/logout",
		"response_types_supported":              []string{"code"},
		"response_modes_supported":              []string{"query"},
		"grant_types_supported":                 SupportedGrantTypes,
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post", "none"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      scopeNames(),
		"claims_supported": []string{
			"iss", "sub", "aud", "exp", "iat", "nonce", "auth_time",
			"email", "email_verified", "name", "tenant_id", "tenant_slug",
			// The citizen's number in the Gerege register, for an account that
			// signed in with eID. Advertised so a relying party can discover it
			// rather than finding it by accident in a token.
			"ge_id",
		},
		"service_documentation": s.issuer + "/developer/apps",
	})
}

// HandleJWKS publishes the public halves of the id_token signing keys.
func (s *SSOProvider) HandleJWKS(w http.ResponseWriter, r *http.Request) {
	keys, err := s.store.PublicKeys(r.Context())
	if err != nil {
		slog.Error("failed to load JWKS", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"keys": []any{}})
		return
	}
	// Ensure a key exists even on a provider that has not signed anything yet,
	// so a client integrating against us never fetches an empty set.
	if len(keys) == 0 {
		if _, err := s.signingKey(r.Context()); err == nil {
			keys, _ = s.store.PublicKeys(r.Context())
		}
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

// HandleScopes lists the scope vocabulary with human descriptions, for the
// consent screen and the developer portal's scope picker.
func (s *SSOProvider) HandleScopes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"scopes": SupportedScopes})
}

// NewIdentifier returns n hex characters of crypto/rand output, for callers
// outside the package that need to mint a client_id or a secret.
func NewIdentifier(n int) string { return generateRandomString(n) }

// HashSecret digests a client secret the way the store expects to find it.
// Exported so the developer portal can hand over a hash and never a secret.
func HashSecret(secret string) string { return hashSecret(secret) }

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeOAuthError(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, map[string]string{"error": code, "error_description": description})
}

// generateRandomString returns n hex characters of crypto/rand output.
//
// The original implementation drew n bytes and then truncated the 2n-character
// hex string back to n, silently halving the entropy of every secret it made.
func generateRandomString(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand does not fail on supported platforms; a predictable
		// fallback here would be worse than stopping.
		panic("ssoprovider: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)[:n]
}
