/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Signing in with Google.
 *
 * Google is an ordinary OpenID Connect provider, so it needs no second
 * implementation: the discovery, PKCE, code exchange and id_token verification
 * in this package are the same ones the deployment-wide federation uses. What
 * is different is what it *means* — Google is one button among several on this
 * platform's own sign-in screen, not a replacement for it — and that difference
 * lives in the platform package, next to the other sign-in paths.
 */

package ssoclient

import (
	"os"
	"strings"
)

// GoogleIssuer is Google's OpenID Connect issuer. It is fixed rather than
// configurable: a deployment that could be pointed at a different "Google"
// could be pointed at somebody else's.
const GoogleIssuer = "https://accounts.google.com"

// GoogleCallbackPath is where Google returns the browser. Like CallbackPath it
// is a constant, because the route serving it is one this platform registers.
const GoogleCallbackPath = "/api/v1/auth/google/callback"

// GoogleConfigFromEnv reads the Google sign-in configuration.
//
// GOOGLE_LOGIN_CLIENT_ID is the switch. It is deliberately separate from the
// GOOGLE_OAUTH_CLIENT_ID that the Drive and Meet connectors use, even though a
// deployment will usually register both against the same Google project: those
// credentials are configured to reach somebody's files, and inheriting a
// sign-in path from them would mean enabling a document connector quietly
// opened a new front door. They may hold the same value — that is the
// operator's explicit act, and the login redirect URI has to be registered at
// Google either way.
func GoogleConfigFromEnv() Config {
	clientID := strings.TrimSpace(os.Getenv("GOOGLE_LOGIN_CLIENT_ID"))
	if clientID == "" {
		return Config{}
	}

	origin := trimSlash(os.Getenv("SSO_ISSUER"))
	if origin == "" {
		origin = trimSlash(os.Getenv("PUBLIC_ORIGIN"))
	}

	cfg := Config{
		EnvPrefix:    "GOOGLE_LOGIN",
		EnvClientID:  "GOOGLE_LOGIN_CLIENT_ID",
		Issuer:       GoogleIssuer,
		ProviderName: "Google",
		ClientID:     clientID,
		ClientSecret: strings.TrimSpace(os.Getenv("GOOGLE_LOGIN_CLIENT_SECRET")),
		RedirectURI:  trimSlash(os.Getenv("GOOGLE_LOGIN_REDIRECT_URI")),
		TenantSlug:   strings.TrimSpace(os.Getenv("GOOGLE_LOGIN_TENANT")),
		// Exactly what a sign-in needs and nothing more. Google shows the person
		// the scopes it is being asked for, and asking for anything beyond who
		// they are — on a screen whose whole purpose is to establish who they
		// are — is a consent prompt that argues against itself.
		Scopes: []string{"openid", "email", "profile"},
		AuthParams: map[string]string{
			// Always show the account chooser. Without it Google is free to
			// answer from the browser's existing session, and for somebody who
			// has already granted these scopes it does: the browser leaves and
			// returns with no screen in between. That is indistinguishable
			// from a button that did nothing, and it silently picks an account
			// on a shared machine instead of asking which one.
			"prompt": "select_account",
			// Identity only. A refresh token is for acting on somebody's
			// behalf later, which this never does — it reads who they are once
			// and writes it down.
			"access_type": "online",
		},
	}
	if cfg.RedirectURI == "" && origin != "" {
		cfg.RedirectURI = origin + GoogleCallbackPath
	}
	return cfg
}

// GoogleAllowedDomains is the operator's list of email domains that may sign in
// with Google, lower-cased and comma-separated in GOOGLE_LOGIN_ALLOWED_DOMAINS.
//
// Empty means the domain is not the check. That is not as loose as it sounds:
// provisioning is off unless GOOGLE_LOGIN_TENANT names an organisation, so
// without a domain list a Google identity still only reaches an account that
// already exists here. Set both and the list becomes what stops "anyone with a
// Google account" from becoming "anyone with an account here".
func GoogleAllowedDomains() []string {
	raw := strings.Split(os.Getenv("GOOGLE_LOGIN_ALLOWED_DOMAINS"), ",")
	domains := make([]string, 0, len(raw))
	for _, candidate := range raw {
		// Trim the spaces before the "@", not after: "  @gerege.mn" does not
		// start with an "@" until the padding is gone, and a domain that kept
		// one matches no address ever issued.
		if candidate = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(candidate), "@")); candidate != "" {
			domains = append(domains, candidate)
		}
	}
	return domains
}

// EmailInDomains reports whether an address belongs to one of the domains. An
// empty list admits everything; the caller decides whether that is safe, and
// GoogleAllowedDomains says why it usually is.
func EmailInDomains(email string, domains []string) bool {
	if len(domains) == 0 {
		return true
	}
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	host := strings.ToLower(email[at+1:])
	for _, domain := range domains {
		if host == domain {
			return true
		}
	}
	return false
}
