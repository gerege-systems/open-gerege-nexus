/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Package ssoclient makes this platform a relying party of another OpenID
 * Connect provider — including another Gerege Nexus.
 *
 * The platform has always been an authorization server (see ssoprovider). It
 * could hand identities out and never take one in, so a group running several
 * deployments had one sign-in per deployment and no way to make one of them the
 * source of truth. This is the other half: set SSO_CLIENT_ISSUER and this
 * deployment stops authenticating people itself and starts asking the provider
 * named there who they are.
 *
 * The two halves are independent. A deployment can be a provider, a client, or
 * both — a regional instance that federates upward to a national one and still
 * issues identities to the apps installed on it is the case this shape exists
 * for.
 *
 * This package is the protocol and nothing else: configuration, discovery, the
 * authorization request, the code exchange, and id_token verification. What to
 * do with the verified identity — which local account it is, which tenant it
 * joins, what session it gets — belongs to the platform package, next to every
 * other sign-in path.
 */

package ssoclient

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config is a deployment's relying-party configuration, read from the
// environment. Zero value means this deployment is not a client of anything,
// which is what every existing deployment stays without touching its config.
type Config struct {
	// EnvPrefix is what the variables behind this configuration are called, so
	// a refusal names the one the operator has to fix. "SSO_CLIENT" for the
	// deployment-wide federation, "GOOGLE_LOGIN" for the Google button.
	//
	// EnvClientID is separate because the two families do not agree on it: the
	// federation's is SSO_CLIENT_ID, not SSO_CLIENT_CLIENT_ID. Deriving it
	// would have produced a variable name that does not exist, in the one
	// message whose whole job is to name the variable to go and set.
	EnvPrefix   string
	EnvClientID string

	// Issuer is the provider's issuer URL — the origin its discovery document
	// is published under. Setting it is what turns client mode on.
	Issuer string
	// ProviderName is what the sign-in screen calls the provider. It is
	// cosmetic; an unset one falls back to the issuer's hostname, which is
	// always true even if it is not pretty.
	ProviderName string

	ClientID     string
	ClientSecret string
	Scopes       []string

	// AuthParams are extra query parameters put on the authorization request.
	//
	// They exist because "ask the provider" is not one behaviour. A provider
	// that already has a session with this browser, for a person who has
	// already granted these scopes, is entitled to answer immediately and send
	// the browser back without showing anything — which is correct for a
	// silent re-authentication and wrong for somebody who pressed a button and
	// expects to be asked. prompt=select_account is how that is said in the
	// protocol, and it belongs to the deployment's configuration of a provider
	// rather than to this package, which does not know which case it is in.
	AuthParams map[string]string

	// RedirectURI is this deployment's callback, registered at the provider.
	RedirectURI string
	// PostLogoutRedirectURI is where the provider returns somebody after they
	// sign out, also registered there.
	PostLogoutRedirectURI string

	// TenantSlug is the organisation a person who has never signed in here
	// before is provisioned into. Empty means no provisioning: an identity the
	// provider vouches for but this deployment has no account for is refused,
	// which is the right default for a deployment whose membership is managed
	// deliberately.
	TenantSlug string

	// LocalLogin keeps this deployment's own password and eID sign-in working
	// alongside the provider's. It is off by default, because the point of
	// federating is that there is one place to prove who you are — but an
	// operator locked out by an unreachable provider needs a documented way
	// back in, and this is it.
	LocalLogin bool
}

// Enabled reports whether this deployment is a client of a provider.
//
// The issuer alone decides it, and a missing client id is a startup error
// rather than a quiet return to local sign-in: an operator who has named a
// provider has said what they want, and a deployment that answered a
// half-finished configuration by carrying on with its own login screen would be
// federated in the config file and not in fact.
func (c Config) Enabled() bool { return c.Issuer != "" }

// Confidential reports whether this client authenticates with a secret. A
// deployment is a server, so it can hold one — but a provider may still have
// registered it as public, and then PKCE alone is what proves the exchange.
func (c Config) Confidential() bool { return c.ClientSecret != "" }

// DisplayName is what to put on the sign-in button.
func (c Config) DisplayName() string {
	if c.ProviderName != "" {
		return c.ProviderName
	}
	if u, err := url.Parse(c.Issuer); err == nil && u.Host != "" {
		return u.Host
	}
	return c.Issuer
}

// ConfigFromEnv reads the relying-party configuration.
//
// Defaults are derived from what the deployment already knows about itself so
// that the ordinary case is two variables — the issuer and the client id. The
// callback and the post-logout address default to this deployment's own
// origin, because that is what they have to be; naming them separately is for
// development, where the API and the browser app sit on different ports.
func ConfigFromEnv() Config {
	issuer := trimSlash(os.Getenv("SSO_CLIENT_ISSUER"))
	if issuer == "" {
		return Config{}
	}

	// Our own origin, the same way the provider half works it out.
	origin := trimSlash(os.Getenv("SSO_ISSUER"))
	if origin == "" {
		origin = trimSlash(os.Getenv("PUBLIC_ORIGIN"))
	}

	cfg := Config{
		EnvPrefix:             "SSO_CLIENT",
		EnvClientID:           "SSO_CLIENT_ID",
		Issuer:                issuer,
		ProviderName:          strings.TrimSpace(os.Getenv("SSO_CLIENT_PROVIDER_NAME")),
		ClientID:              strings.TrimSpace(os.Getenv("SSO_CLIENT_ID")),
		ClientSecret:          strings.TrimSpace(os.Getenv("SSO_CLIENT_SECRET")),
		RedirectURI:           trimSlash(os.Getenv("SSO_CLIENT_REDIRECT_URI")),
		PostLogoutRedirectURI: strings.TrimSpace(os.Getenv("SSO_CLIENT_POST_LOGOUT_REDIRECT_URI")),
		TenantSlug:            strings.TrimSpace(os.Getenv("SSO_CLIENT_TENANT")),
		LocalLogin:            strings.EqualFold(strings.TrimSpace(os.Getenv("SSO_CLIENT_LOCAL_LOGIN")), "true"),
	}

	if cfg.RedirectURI == "" && origin != "" {
		cfg.RedirectURI = origin + CallbackPath
	}
	if cfg.PostLogoutRedirectURI == "" && origin != "" {
		cfg.PostLogoutRedirectURI = origin + "/"
	}

	// openid is not optional — it is what makes this OpenID Connect rather than
	// bare OAuth2, and without it no id_token comes back at all.
	cfg.Scopes = strings.Fields(os.Getenv("SSO_CLIENT_SCOPES"))
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"openid", "profile", "email"}
	}
	if !contains(cfg.Scopes, "openid") {
		cfg.Scopes = append([]string{"openid"}, cfg.Scopes...)
	}
	return cfg
}

// CallbackPath is where the provider returns the browser. It is a constant
// rather than configuration because the route serving it is one this platform
// registers itself; SSO_CLIENT_REDIRECT_URI changes the origin in front of it,
// never the path.
const CallbackPath = "/api/v1/auth/sso/callback"

// Validate reports a configuration that cannot work, so a deployment says so at
// startup rather than at the first person's first sign-in.
func (c Config) Validate() error {
	if !c.Enabled() {
		return nil
	}
	// A Config built by hand — in a test, or by a caller that has not said which
	// family it belongs to — is the deployment-wide federation, so it is named
	// as one. Both defaults move together deliberately: SSO_CLIENT's client id
	// variable is SSO_CLIENT_ID, and defaulting only the prefix would name
	// SSO_CLIENT_CLIENT_ID, which does not exist.
	prefix, clientIDVar := c.EnvPrefix, c.EnvClientID
	if prefix == "" {
		prefix = "SSO_CLIENT"
		if clientIDVar == "" {
			clientIDVar = "SSO_CLIENT_ID"
		}
	}
	u, err := url.Parse(c.Issuer)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return errors.New(prefix + "_ISSUER must be an absolute URL")
	}
	if u.Scheme != "https" && !isLoopback(u.Hostname()) {
		return errors.New(prefix + "_ISSUER must use HTTPS unless it is on the loopback interface")
	}
	if clientIDVar == "" {
		clientIDVar = prefix + "_CLIENT_ID"
	}
	if c.ClientID == "" {
		return errors.New(clientIDVar + " is required when " + prefix + "_ISSUER names a provider")
	}
	if c.RedirectURI == "" {
		return errors.New(prefix + "_REDIRECT_URI could not be derived; set it, or set PUBLIC_ORIGIN")
	}
	if _, err := url.Parse(c.RedirectURI); err != nil {
		return errors.New(prefix + "_REDIRECT_URI must be an absolute URL")
	}
	return nil
}

// Client is a configured relying party. It holds the discovery cache, so one is
// built per process rather than per request.
type Client struct {
	cfg  Config
	http *http.Client

	discovery *discoveryCache
	keys      *keyCache
}

// New builds a relying party over cfg. It performs no network access: discovery
// happens on first use and is retried on the next one, so a provider that is
// briefly unreachable delays a sign-in rather than preventing the platform from
// starting.
func New(cfg Config) *Client {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	return &Client{
		cfg:       cfg,
		http:      httpClient,
		discovery: newDiscoveryCache(cfg.Issuer, httpClient),
		keys:      newKeyCache(httpClient),
	}
}

// Config returns the configuration this client was built from.
func (c *Client) Config() Config { return c.cfg }

// AuthorizationRequest is one pending sign-in: the URL to send the browser to,
// and the three secrets that have to survive until the callback comes back.
type AuthorizationRequest struct {
	URL          string
	State        string
	Nonce        string
	CodeVerifier string
}

// BeginAuthorization builds an authorization request with PKCE.
//
// State, nonce and verifier are minted here and handed back for the caller to
// park somewhere the callback can read: state is what makes the callback
// answerable only to a request this deployment started, nonce ties the id_token
// to it, and the verifier is what proves at the token endpoint that the code
// was redeemed by whoever asked for it.
func (c *Client) BeginAuthorization(ctx context.Context) (*AuthorizationRequest, error) {
	meta, err := c.discovery.get(ctx)
	if err != nil {
		return nil, err
	}

	req := &AuthorizationRequest{
		State:        randomToken(),
		Nonce:        randomToken(),
		CodeVerifier: randomToken() + randomToken(),
	}

	challenge := sha256.Sum256([]byte(req.CodeVerifier))
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {c.cfg.ClientID},
		"redirect_uri":          {c.cfg.RedirectURI},
		"scope":                 {strings.Join(c.cfg.Scopes, " ")},
		"state":                 {req.State},
		"nonce":                 {req.Nonce},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
	}
	// Added after the protocol's own parameters and never over them: a
	// deployment may want to ask the provider for a different prompt, not to
	// rewrite the state or the PKCE challenge this package just generated.
	for key, value := range c.cfg.AuthParams {
		if _, reserved := q[key]; !reserved && value != "" {
			q.Set(key, value)
		}
	}

	target, err := url.Parse(meta.AuthorizationEndpoint)
	if err != nil {
		return nil, fmt.Errorf("provider advertises an unparseable authorization_endpoint: %w", err)
	}
	// Merged rather than replaced: an authorization_endpoint carrying its own
	// query is unusual but legal, and dropping it would break such a provider.
	existing := target.Query()
	for key, values := range q {
		existing[key] = values
	}
	target.RawQuery = existing.Encode()

	req.URL = target.String()
	return req, nil
}

// EndSessionURL is where to send the browser to sign out at the provider.
//
// The empty string means the provider does not offer RP-initiated logout, and
// the caller should finish the local sign-out on its own rather than sending
// somebody to a URL that does not exist.
func (c *Client) EndSessionURL(ctx context.Context, idTokenHint string) string {
	meta, err := c.discovery.get(ctx)
	if err != nil || meta.EndSessionEndpoint == "" {
		return ""
	}
	target, err := url.Parse(meta.EndSessionEndpoint)
	if err != nil {
		return ""
	}
	q := target.Query()
	// client_id as well as the hint: a provider that cannot verify the hint —
	// one that has rotated its keys, say — still needs to know whose
	// post_logout_redirect_uri it is being asked to match.
	q.Set("client_id", c.cfg.ClientID)
	if idTokenHint != "" {
		q.Set("id_token_hint", idTokenHint)
	}
	if c.cfg.PostLogoutRedirectURI != "" {
		q.Set("post_logout_redirect_uri", c.cfg.PostLogoutRedirectURI)
	}
	target.RawQuery = q.Encode()
	return target.String()
}

// randomToken returns 32 bytes of crypto/rand as base64url — 43 characters,
// which is exactly the minimum RFC 7636 allows for a PKCE verifier and ample
// for a state or a nonce.
func randomToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand does not fail on a supported platform, and a predictable
		// state or verifier would silently remove the protection it exists for.
		panic("ssoclient: crypto/rand unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func trimSlash(raw string) string { return strings.TrimRight(strings.TrimSpace(raw), "/") }

func contains(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
