package ssoclient

import (
	"strings"
	"testing"
)

func TestGoogleConfigIsOffUntilAClientIDIsSet(t *testing.T) {
	t.Setenv("GOOGLE_LOGIN_CLIENT_ID", "")
	if GoogleConfigFromEnv().Enabled() {
		t.Fatal("Google sign-in is on without configuration")
	}
}

// The Drive and Meet connectors already use GOOGLE_OAUTH_CLIENT_ID. Inheriting
// a sign-in path from those would mean enabling a document connector quietly
// opened a new front door.
func TestGoogleLoginDoesNotInheritTheConnectorCredentials(t *testing.T) {
	t.Setenv("GOOGLE_LOGIN_CLIENT_ID", "")
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "connector.apps.googleusercontent.com")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "connector-secret")

	if GoogleConfigFromEnv().Enabled() {
		t.Fatal("configuring the Drive/Meet connector switched Google sign-in on")
	}
}

func TestGoogleConfigDerivesItsCallbackAndAsksOnlyWhoYouAre(t *testing.T) {
	t.Setenv("GOOGLE_LOGIN_CLIENT_ID", "123.apps.googleusercontent.com")
	t.Setenv("GOOGLE_LOGIN_CLIENT_SECRET", "s3cret")
	t.Setenv("SSO_ISSUER", "https://nexus.gerege.mn")

	cfg := GoogleConfigFromEnv()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Issuer != GoogleIssuer {
		t.Errorf("issuer = %q, want it fixed at Google's", cfg.Issuer)
	}
	if want := "https://nexus.gerege.mn" + GoogleCallbackPath; cfg.RedirectURI != want {
		t.Errorf("redirect_uri = %q, want %q", cfg.RedirectURI, want)
	}
	// A sign-in screen asking for anything beyond identity is a consent prompt
	// that argues against itself.
	for _, scope := range cfg.Scopes {
		if scope != "openid" && scope != "email" && scope != "profile" {
			t.Errorf("scope %q is more than signing in needs", scope)
		}
	}
}

// The message has to name the variable the operator actually sets, or it sends
// them looking for one that does not exist.
func TestGoogleValidationNamesGoogleVariables(t *testing.T) {
	cfg := Config{
		EnvPrefix: "GOOGLE_LOGIN", EnvClientID: "GOOGLE_LOGIN_CLIENT_ID",
		Issuer: GoogleIssuer, RedirectURI: "https://a.mn" + GoogleCallbackPath,
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "GOOGLE_LOGIN_CLIENT_ID") {
		t.Fatalf("error = %v, want it to name GOOGLE_LOGIN_CLIENT_ID", err)
	}
}

func TestAllowedDomainsGateTheAddress(t *testing.T) {
	t.Setenv("GOOGLE_LOGIN_ALLOWED_DOMAINS", " @gerege.mn , DGOV.MN ")
	domains := GoogleAllowedDomains()

	for _, allowed := range []string{"bat@gerege.mn", "person@dgov.mn", "MIXED@Gerege.mn"} {
		if !EmailInDomains(allowed, domains) {
			t.Errorf("%q was refused", allowed)
		}
	}
	for _, refused := range []string{"someone@gmail.com", "attacker@notgerege.mn", "gerege.mn", ""} {
		if EmailInDomains(refused, domains) {
			t.Errorf("%q was admitted", refused)
		}
	}
	// No list is not a check. It is safe only because provisioning is off by
	// default, which is what GoogleAllowedDomains' comment says.
	if !EmailInDomains("anyone@anywhere.example", nil) {
		t.Error("an empty list should not refuse anybody")
	}
}
