package operator

import (
	"strings"
	"testing"
	"time"
)

// rfc6238Secret is the ASCII string "12345678901234567890" in base32 — the
// shared secret every test vector in RFC 6238 Appendix B is computed against.
// Checking against the specification rather than against our own output is the
// point: a home-grown implementation that agrees with itself would pass a test
// written from its own results and still be unable to read a code from anyone's
// telephone.
const rfc6238Secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func TestVerifyTOTPAcceptsTheSpecificationsVectors(t *testing.T) {
	// Appendix B gives eight digits; an authenticator shows the last six, which
	// is what this platform asks for.
	cases := []struct {
		at   int64
		code string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}
	for _, c := range cases {
		step, ok := verifyTOTP(rfc6238Secret, c.code, time.Unix(c.at, 0))
		if !ok {
			t.Errorf("code %s at %d was refused", c.code, c.at)
			continue
		}
		if want := c.at / TOTPPeriod; step != want {
			t.Errorf("code %s at %d reported step %d, want %d", c.code, c.at, step, want)
		}
	}
}

func TestVerifyTOTPRefusesTheWrongCode(t *testing.T) {
	if _, ok := verifyTOTP(rfc6238Secret, "000000", time.Unix(59, 0)); ok {
		t.Fatal("a wrong code was accepted")
	}
	if _, ok := verifyTOTP(rfc6238Secret, "", time.Unix(59, 0)); ok {
		t.Fatal("an empty code was accepted")
	}
	if _, ok := verifyTOTP("", "287082", time.Unix(59, 0)); ok {
		t.Fatal("an account with no secret accepted a code")
	}
}

// The previous step is accepted, and it reports the step it actually belongs
// to. Both halves matter: the tolerance is what makes the console usable with a
// telephone whose clock has drifted, and the reported step is what stops that
// tolerance from becoming a replay window — sign-in refuses a step it has
// already seen.
func TestVerifyTOTPToleratesOneStepAndNamesIt(t *testing.T) {
	const previous = "287082" // step 1
	step, ok := verifyTOTP(rfc6238Secret, previous, time.Unix(89, 0))
	if !ok {
		t.Fatal("the previous step's code was refused")
	}
	if step != 1 {
		t.Fatalf("the previous step's code reported step %d, want 1", step)
	}

	// Two steps away is outside the window.
	if _, ok := verifyTOTP(rfc6238Secret, previous, time.Unix(119, 0)); ok {
		t.Fatal("a code two steps old was accepted")
	}
}

func TestNewTOTPSecretProducesAUsableEnrolment(t *testing.T) {
	secret, uri, err := NewTOTPSecret("operator@example.mn")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(secret) != 32 {
		t.Fatalf("a 20-byte secret should encode to 32 base32 characters, got %d", len(secret))
	}
	// The URI has to carry the secret an authenticator will read back, or the
	// QR code enrols something the account cannot verify.
	if want := "secret=" + secret; !strings.Contains(uri, want) {
		t.Fatalf("the enrolment URI does not carry the secret: %s", uri)
	}
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Fatalf("the enrolment URI is not an otpauth one: %s", uri)
	}
}

// A host that runs several of these products gave every one of their consoles
// the same authenticator entry, because the issuer was written into the image.
// A phone showing four identical rows cannot tell you which code opens which
// door.
func TestTheEnrolmentIsNamedAfterTheDeployment(t *testing.T) {
	t.Setenv("BRAND_NAME", "Gerege Salus")

	_, uri, err := NewTOTPSecret("operator@example.mn")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.Contains(uri, "issuer=Gerege%20Salus%20Control%20Plane") {
		t.Errorf("the issuer does not follow BRAND_NAME: %s", uri)
	}
	if strings.Contains(uri, "Gerege%20Nexus") {
		t.Errorf("the built-in name is still in the URI: %s", uri)
	}

	// url.Values.Encode writes a space as `+`, and authenticator applications
	// print it literally — the phone read "Gerege+Salus+Control+Plane".
	if strings.Contains(uri, "+") {
		t.Errorf("a space is encoded as + and will be shown as one: %s", uri)
	}
}
