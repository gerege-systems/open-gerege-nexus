package ssoclient

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigFromEnvIsOffByDefault(t *testing.T) {
	t.Setenv("SSO_CLIENT_ISSUER", "")
	if ConfigFromEnv().Enabled() {
		t.Fatal("client mode is on without an issuer; every existing deployment would change behaviour")
	}
}

func TestConfigFromEnvDerivesTheCallbackFromThisDeployment(t *testing.T) {
	t.Setenv("SSO_CLIENT_ISSUER", "https://nexus.gerege.mn/")
	t.Setenv("SSO_CLIENT_ID", "regional-office")
	t.Setenv("SSO_ISSUER", "https://aimag.gerege.mn")

	cfg := ConfigFromEnv()
	if !cfg.Enabled() {
		t.Fatal("client mode is off with an issuer set")
	}
	// The trailing slash on the issuer must not survive into the URLs built
	// from it, or discovery is fetched from a double-slashed path.
	if cfg.Issuer != "https://nexus.gerege.mn" {
		t.Errorf("issuer = %q", cfg.Issuer)
	}
	if want := "https://aimag.gerege.mn" + CallbackPath; cfg.RedirectURI != want {
		t.Errorf("redirect_uri = %q, want %q", cfg.RedirectURI, want)
	}
	if cfg.PostLogoutRedirectURI != "https://aimag.gerege.mn/" {
		t.Errorf("post_logout_redirect_uri = %q", cfg.PostLogoutRedirectURI)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestConfigAlwaysAsksForOpenID(t *testing.T) {
	t.Setenv("SSO_CLIENT_ISSUER", "https://nexus.gerege.mn")
	t.Setenv("SSO_CLIENT_ID", "c")
	t.Setenv("SSO_CLIENT_SCOPES", "profile email")

	// Without openid the provider answers with no id_token at all, and there is
	// nothing signed to identify anybody by.
	if scopes := ConfigFromEnv().Scopes; !contains(scopes, "openid") {
		t.Errorf("scopes = %v, want openid to have been added", scopes)
	}
}

func TestValidateRejectsAHalfWrittenConfiguration(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "a named provider with no client id",
			cfg:  Config{Issuer: "https://nexus.gerege.mn", RedirectURI: "https://a.mn" + CallbackPath},
			want: "SSO_CLIENT_ID is required",
		},
		{
			name: "a provider reached over plain HTTP",
			cfg:  Config{Issuer: "http://nexus.gerege.mn", ClientID: "c", RedirectURI: "https://a.mn" + CallbackPath},
			want: "must use HTTPS",
		},
		{
			name: "no callback and nothing to derive one from",
			cfg:  Config{Issuer: "https://nexus.gerege.mn", ClientID: "c"},
			want: "SSO_CLIENT_REDIRECT_URI",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if err == nil {
				t.Fatal("the configuration was accepted; it cannot work")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

// Loopback over plain HTTP is allowed so two instances can be run against each
// other on one machine.
func TestValidateAllowsALoopbackProvider(t *testing.T) {
	cfg := Config{Issuer: "http://localhost:8080", ClientID: "c", RedirectURI: "http://localhost:8081" + CallbackPath}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestDisplayNameFallsBackToTheHost(t *testing.T) {
	if got := (Config{Issuer: "https://nexus.gerege.mn"}).DisplayName(); got != "nexus.gerege.mn" {
		t.Errorf("DisplayName = %q", got)
	}
	if got := (Config{Issuer: "https://nexus.gerege.mn", ProviderName: "Гэрэгэ"}).DisplayName(); got != "Гэрэгэ" {
		t.Errorf("DisplayName = %q", got)
	}
}

func TestFlowRoundTripsAndRefusesAMismatchedState(t *testing.T) {
	recorder := httptest.NewRecorder()
	SetFlowCookie(recorder, FederationFlow, Flow{State: "state-1", Nonce: "nonce-1", CodeVerifier: "verifier-1", Next: "/documents"})

	cookie := findCookie(t, recorder, FederationFlow.Name)
	if !cookie.HttpOnly {
		t.Error("the flow cookie is readable by script")
	}
	// Strict would not be sent on the callback, which arrives as a top-level
	// navigation from the provider's origin — the flow would lose its own state
	// on the one request that needs it.
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
	}
	if cookie.Path != FederationFlow.Path {
		t.Errorf("path = %q, want the cookie confined to the callback", cookie.Path)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)

	if _, err := ReadFlow(httptest.NewRecorder(), request, FederationFlow, "a-different-state"); !errors.Is(err, ErrStateMismatch) {
		t.Fatalf("err = %v, want ErrStateMismatch", err)
	}

	// Re-add: ReadFlow above consumed nothing from the request, but a fresh one
	// keeps the two reads independent.
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)
	flow, err := ReadFlow(httptest.NewRecorder(), request, FederationFlow, "state-1")
	if err != nil {
		t.Fatalf("ReadFlow: %v", err)
	}
	if flow.Nonce != "nonce-1" || flow.CodeVerifier != "verifier-1" || flow.Next != "/documents" {
		t.Errorf("flow = %+v", flow)
	}
}

// A flow is single-use: the cookie goes whatever the outcome, so a code kept
// from a failed attempt cannot be presented again against the same state.
func TestReadFlowAlwaysClearsTheCookie(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	if _, err := ReadFlow(recorder, request, FederationFlow, "state-1"); !errors.Is(err, ErrNoFlow) {
		t.Fatalf("err = %v, want ErrNoFlow", err)
	}
	if cookie := findCookie(t, recorder, FederationFlow.Name); cookie.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want the cookie expired", cookie.MaxAge)
	}
}

func TestSafeNextRefusesAnythingThatCanLeaveTheSite(t *testing.T) {
	for _, raw := range []string{
		"", "https://evil.example", "//evil.example", "/\\evil.example",
		"/safe\\..\\evil.example", "/profile\nLocation: https://evil.example", "evil.example",
	} {
		if got := SafeNext(raw, "/apps"); got != "/apps" {
			t.Errorf("SafeNext(%q) = %q, want the fallback", raw, got)
		}
	}
	if got := SafeNext("/documents?id=4", "/apps"); got != "/documents?id=4" {
		t.Errorf("SafeNext dropped a legitimate path: %q", got)
	}
}

func findCookie(t *testing.T, recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("no %q cookie was set", name)
	return nil
}
