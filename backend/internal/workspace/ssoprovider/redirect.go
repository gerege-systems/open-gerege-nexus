/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package ssoprovider

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

// ValidateRedirectURI enforces both transport security and the operator's host
// allowlist. OAUTH_REDIRECT_HOSTS is a comma-separated list of exact hostnames;
// subdomains do not inherit trust. Local loopback callbacks remain available
// for installed development clients.
//
// Unchanged from where it used to live at the top of ssoprovider.go; it moved
// here when that file became the authorization server rather than a client map.
// It is the deployment-wide floor, applied on top of the per-client exact match
// that sso_clients.validateRedirectURI enforces at registration and
// HandleAuthorize enforces again on every request.
func ValidateRedirectURI(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return errors.New("invalid redirect URI")
	}
	host := strings.ToLower(u.Hostname())
	loopback := host == "localhost" || host == "127.0.0.1" || host == "::1"
	if u.Scheme != "https" && (u.Scheme != "http" || !loopback) {
		return errors.New("redirect URI must use HTTPS")
	}
	if loopback {
		return nil
	}
	allowed := os.Getenv("OAUTH_REDIRECT_HOSTS")
	if strings.TrimSpace(allowed) == "" {
		allowed = "nexus.gerege.mn"
	}
	for _, candidate := range strings.Split(allowed, ",") {
		if strings.EqualFold(strings.TrimSpace(candidate), host) {
			return nil
		}
	}
	return errors.New("redirect URI host is not allowed")
}
