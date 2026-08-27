package emailverify_test

import (
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/emailverify"
)

// The link goes out in a mail carrying this platform's name, and /verify/landed
// forwards whoever clicks it. Without a host allowlist any signed-in member of
// any tenant could pick that destination — which is the open redirector a
// phishing link wants to borrow, wearing a government hostname.
func TestARedirectMustPointSomewhereTheOperatorNamed(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("PUBLIC_ORIGIN", "https://nexus.gerege.mn")
	t.Setenv("EMAIL_VERIFY_REDIRECT_HOSTS", "portal.example.mn, second.example.mn")

	allowed := []string{
		"https://nexus.gerege.mn/verified",
		"https://portal.example.mn/done?ref=1",
		"https://second.example.mn/",
	}
	for _, raw := range allowed {
		if _, err := emailverify.ValidateRedirect(raw); err != nil {
			t.Errorf("%s was refused: %v", raw, err)
		}
	}

	refused := []string{
		"https://phishing.example/login",
		// A subdomain does not inherit trust: the neighbours under gerege.mn are
		// other products, not this one.
		"https://evil.nexus.gerege.mn/",
		"https://nexus.gerege.mn.evil.example/",
		"http://nexus.gerege.mn/verified", // HTTPS is still required
		"https://localhost/verified",      // loopback is a development-only allowance
	}
	for _, raw := range refused {
		if _, err := emailverify.ValidateRedirect(raw); err == nil {
			t.Errorf("%s was accepted", raw)
		}
	}
}

// An empty destination stays legitimate: the platform answers the click itself.
func TestNoRedirectIsStillAllowed(t *testing.T) {
	t.Setenv("PUBLIC_ORIGIN", "https://nexus.gerege.mn")
	got, err := emailverify.ValidateRedirect("   ")
	if err != nil || got != "" {
		t.Fatalf("empty redirect: got %q, %v", got, err)
	}
}

// Outside production a developer has to be able to point a link at the app they
// are running, or the flow cannot be exercised locally at all.
func TestLoopbackIsAllowedOutsideProduction(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("PUBLIC_ORIGIN", "https://nexus.gerege.mn")
	for _, raw := range []string{"http://localhost:3000/done", "https://127.0.0.1:3000/done"} {
		if _, err := emailverify.ValidateRedirect(raw); err != nil {
			t.Errorf("%s was refused in development: %v", raw, err)
		}
	}
}

// The refusal has to say which variable to change, or an operator is left
// guessing why a link they configured does not work.
func TestTheRefusalNamesTheVariableToChange(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("PUBLIC_ORIGIN", "https://nexus.gerege.mn")
	_, err := emailverify.ValidateRedirect("https://elsewhere.example/x")
	if err == nil {
		t.Fatal("an unlisted host was accepted")
	}
	if !strings.Contains(err.Error(), "EMAIL_VERIFY_REDIRECT_HOSTS") {
		t.Errorf("the refusal does not name the variable: %v", err)
	}
}
