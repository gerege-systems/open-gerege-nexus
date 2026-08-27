package auth

import "testing"

// The address an eID account is known by, and which addresses this platform may
// rewrite.

func TestGeIDEmailIsTheNumberAtTheGeregeDomain(t *testing.T) {
	if got := GeIDEmail(10000263); got != "10000263@gemail.com" {
		t.Fatalf("GeIDEmail = %q", got)
	}
}

func TestOnlyAnInventedAddressMayBeRewritten(t *testing.T) {
	for _, invented := range []string{
		"eid+3854e516490de4f6184ac3af2d8cc8b8@identity.invalid",
		"10000263@gemail.com",
		"10000263@GEMAIL.COM",
	} {
		if !isInventedAddress(invented) {
			t.Errorf("%q should be rewritable: this platform made it up", invented)
		}
	}
	// A citizen who signed up with a mailbox of their own and later linked
	// their eID keeps the address they chose. Renaming them because of how they
	// signed in is the bug this guards against.
	for _, theirs := range []string{
		"erdenebatt@gmail.com",
		"admin@example.com",
		"person@identity.invalid.example.mn",
	} {
		if isInventedAddress(theirs) {
			t.Errorf("%q is the person's own address and must not be rewritten", theirs)
		}
	}
}
