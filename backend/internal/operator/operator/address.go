/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package operator

import (
	"log/slog"
	"net/netip"
	"os"
	"regexp"
	"strings"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
)

// Which addresses may reach the console, and when that question is asked at all.
//
// It used to be nginx's, and only nginx's: cp.nexus.gerege.mn shipped with a
// snippet that allowed a list of CIDRs and denied everybody else, so an
// attacker on the public internet never reached this process. That is a strong
// boundary and it has one property that turned out to matter more: it cannot
// see the platform's own configuration. A deployment that is **public** — one
// that provisions an account for anybody who can prove an identity — was still
// being told which offices its operators may sit in, and changing that meant a
// secret, a deploy, and an nginx reload.
//
// So the decision moved to where the configuration lives:
//
//	access mode private → the address must be on the list, when there is one
//	access mode public  → no address restriction at all
//
// The cost is stated plainly: on a public deployment an unauthenticated
// stranger now reaches the console's sign-in instead of being refused a layer
// earlier. What stands behind it is what always stood behind it — an operator
// account that does not exist unless somebody made it from the host, a password,
// a confirmed second factor, and a step-up for anything that changes something.
// The address list was the outermost of five, and it is the only one that
// disagreed with the platform's own answer to "who is this deployment for".

// AllowedCIDRsEnv is where the list comes from. Empty means no address
// restriction, which is also what "public" means at runtime.
const AllowedCIDRsEnv = "CONTROL_PLANE_ALLOWED_CIDRS"

// openValues are the ways an operator says "no address restriction" out loud.
//
// A word rather than 0.0.0.0/0. The deploy refuses the default route on
// purpose — a mistyped prefix must not silently open the console — and a word
// that nothing else could be mistaken for is a decision rather than a typo.
var openValues = map[string]bool{"open": true, "any": true, "all": true, "none": true}

var cidrSeparator = regexp.MustCompile(`[\s,]+`)

// parseAllowedCIDRs reads the environment's list.
//
// A token that is not an address is dropped with a warning rather than failing
// the boot: the list is a restriction, and a deployment that cannot start
// because one entry has a typo is a deployment whose operators cannot fix the
// typo. The remaining entries still apply, and the warning says which did not.
func parseAllowedCIDRs(raw string) []netip.Prefix {
	raw = strings.TrimSpace(raw)
	if raw == "" || openValues[strings.ToLower(raw)] {
		return nil
	}
	var allowed []netip.Prefix
	for _, token := range cidrSeparator.Split(raw, -1) {
		if token = strings.TrimSpace(token); token == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(token); err == nil {
			allowed = append(allowed, prefix.Masked())
			continue
		}
		// A bare address is a /32 or a /128, which is how an operator writes
		// "this one machine".
		if addr, err := netip.ParseAddr(token); err == nil {
			allowed = append(allowed, netip.PrefixFrom(addr, addr.BitLen()))
			continue
		}
		slog.Warn("control plane: ignoring an address that is not a CIDR",
			"env", AllowedCIDRsEnv, "value", token)
	}
	return allowed
}

// addressAllowed reports whether this caller may reach the console.
//
// True whenever the platform is public, and true on a private deployment that
// named no addresses: an empty list is "not configured", not "nobody".
func (c *Console) addressAllowed(clientIP string) bool {
	if len(c.allowedCIDRs) == 0 {
		return true
	}
	if settings.Get(settings.AccessMode) != settings.AccessPrivate {
		return true
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(clientIP))
	if err != nil {
		// No address to judge — behind a proxy that sends none, or a malformed
		// header. A private deployment with an allowlist refuses: this is the
		// one place where "unknown" must not read as "allowed".
		slog.Warn("control plane: refusing a request with no usable client address",
			"client_ip", clientIP)
		return false
	}
	// IPv4 arriving as ::ffff:a.b.c.d would otherwise miss every IPv4 prefix.
	if addr.Is4In6() {
		addr = addr.Unmap()
	}
	for _, prefix := range c.allowedCIDRs {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// allowedCIDRsFromEnv is what New reads at construction.
func allowedCIDRsFromEnv() []netip.Prefix {
	return parseAllowedCIDRs(os.Getenv(AllowedCIDRsEnv))
}
