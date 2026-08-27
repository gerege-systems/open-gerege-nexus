package operator

import (
	"net/netip"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
)

// When the console asks about an address, and when it does not.

func TestAPublicPlatformDoesNotRestrictByAddress(t *testing.T) {
	t.Setenv("SEED_DEMO_DATA", "false") // unrelated, but keeps the settings quiet
	console := &Console{allowedCIDRs: parseAllowedCIDRs("66.181.179.88/32")}

	// The list is configured, and the platform is public: it is not consulted.
	// This is the whole point — a deployment that provisions accounts for
	// anybody should not also be telling its operators which office to sit in.
	t.Setenv("PLATFORM_ACCESS_MODE", settings.AccessPublic)
	if !console.addressAllowed("203.0.113.7") {
		t.Error("a public platform refused an address")
	}
}

func TestAPrivatePlatformKeepsItsAddressList(t *testing.T) {
	t.Setenv("PLATFORM_ACCESS_MODE", settings.AccessPrivate)
	console := &Console{allowedCIDRs: parseAllowedCIDRs("66.181.179.88/32, 10.1.0.0/24")}

	for _, allowed := range []string{"66.181.179.88", "10.1.0.4", "10.1.0.255"} {
		if !console.addressAllowed(allowed) {
			t.Errorf("%s should be allowed", allowed)
		}
	}
	for _, refused := range []string{"66.181.179.89", "10.1.1.4", "203.0.113.7", ""} {
		if console.addressAllowed(refused) {
			t.Errorf("%s should be refused", refused)
		}
	}
}

// An empty list is "not configured", not "nobody" — otherwise turning the
// platform private would lock every operator out of the console at once.
func TestAPrivatePlatformWithNoListIsNotClosed(t *testing.T) {
	t.Setenv("PLATFORM_ACCESS_MODE", settings.AccessPrivate)
	console := &Console{allowedCIDRs: parseAllowedCIDRs("")}
	if !console.addressAllowed("203.0.113.7") {
		t.Error("an unconfigured list refused an address")
	}
}

func TestTheListIsWrittenTheWayAnOperatorWouldWriteIt(t *testing.T) {
	got := parseAllowedCIDRs("66.181.179.88, 10.1.0.0/24\n2001:db8::/48  not-an-address")
	want := []netip.Prefix{
		netip.MustParsePrefix("66.181.179.88/32"),
		netip.MustParsePrefix("10.1.0.0/24"),
		netip.MustParsePrefix("2001:db8::/48"),
	}
	if len(got) != len(want) {
		t.Fatalf("parsed %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d is %v, want %v", i, got[i], want[i])
		}
	}

	// The words that mean "no restriction". Written out rather than 0.0.0.0/0,
	// which the deploy refuses so that a mistyped prefix cannot open the door.
	for _, open := range []string{"open", "any", "ALL", "none", "  "} {
		if parsed := parseAllowedCIDRs(open); len(parsed) != 0 {
			t.Errorf("%q parsed as a restriction: %v", open, parsed)
		}
	}
}

// IPv4 arriving over an IPv6 socket is the same address, and a console that
// refused it would refuse whoever is behind a dual-stack proxy.
func TestAnIPv4AddressInIPv6FormIsTheSameAddress(t *testing.T) {
	t.Setenv("PLATFORM_ACCESS_MODE", settings.AccessPrivate)
	console := &Console{allowedCIDRs: parseAllowedCIDRs("66.181.179.88/32")}
	if !console.addressAllowed("::ffff:66.181.179.88") {
		t.Error("the mapped form of an allowed address was refused")
	}
}
