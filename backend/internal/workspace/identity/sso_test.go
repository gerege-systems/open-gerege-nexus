package identity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/ssoclient"
)

// A Server needs a database for almost everything, but not for the question
// these cover: whether this deployment authenticates people itself, which is
// decided by configuration alone.
func federatedServer(t *testing.T, mutate ...func(*ssoclient.Config)) *Handlers {
	t.Helper()
	cfg := ssoclient.Config{
		Issuer:      "https://nexus.gerege.mn",
		ClientID:    "aimag-office",
		RedirectURI: "https://aimag.gerege.mn" + ssoclient.CallbackPath,
	}
	for _, m := range mutate {
		m(&cfg)
	}
	return New(Deps{SSO: ssoclient.New(cfg)})
}

func TestLocalLoginIsClosedOnAFederatedDeployment(t *testing.T) {
	server := federatedServer(t)
	if server.LocalLoginAllowed() {
		t.Fatal("local sign-in is still open; a federated deployment would have two front doors")
	}

	reached := false
	handler := server.RequireLocalLogin(func(w http.ResponseWriter, r *http.Request) { reached = true })

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))

	if reached {
		t.Fatal("the password handler ran on a deployment that does not authenticate anybody")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}

	// Not a bare refusal: a native client or a stale tab has to be told where
	// sign-in actually happens, or the only signal is an endpoint that stopped
	// working.
	var body struct {
		Code     string `json:"code"`
		StartURL string `json:"start_url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Code != "sso_required" {
		t.Errorf("code = %q, want sso_required", body.Code)
	}
	if body.StartURL == "" {
		t.Error("the refusal does not say where to sign in")
	}
}

// The break-glass. An operator locked out by an unreachable provider needs a
// documented way back in, and this is it.
func TestLocalLoginStaysOpenWhenTheOperatorKeepsIt(t *testing.T) {
	server := federatedServer(t, func(c *ssoclient.Config) { c.LocalLogin = true })
	if !server.LocalLoginAllowed() {
		t.Fatal("SSO_CLIENT_LOCAL_LOGIN did not keep the local paths open")
	}

	reached := false
	handler := server.RequireLocalLogin(func(w http.ResponseWriter, r *http.Request) { reached = true })
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))
	if !reached {
		t.Error("the password handler was refused despite the local login escape hatch")
	}
}

// Every deployment that has not named a provider must be untouched by all of
// this: its own sign-in paths answer exactly as before.
func TestLocalLoginIsUntouchedWithoutAProvider(t *testing.T) {
	server := New(Deps{})
	if server.SsoClientEnabled() {
		t.Fatal("client mode is on without configuration")
	}
	if !server.LocalLoginAllowed() {
		t.Fatal("local sign-in was closed on a deployment that federates nothing")
	}

	rec := httptest.NewRecorder()
	server.HandleSSOConfig(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/config", nil))

	var body struct {
		Enabled    bool `json:"enabled"`
		LocalLogin bool `json:"local_login"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Enabled || !body.LocalLogin {
		t.Errorf("config = %+v, want the login screen to keep its own forms", body)
	}
}

func TestSSOConfigDescribesTheProviderToTheLoginScreen(t *testing.T) {
	server := federatedServer(t, func(c *ssoclient.Config) { c.ProviderName = "Гэрэгэ Нексус" })

	rec := httptest.NewRecorder()
	server.HandleSSOConfig(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/config", nil))

	var body struct {
		Enabled      bool   `json:"enabled"`
		ProviderName string `json:"provider_name"`
		StartURL     string `json:"start_url"`
		LocalLogin   bool   `json:"local_login"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Enabled || body.ProviderName != "Гэрэгэ Нексус" || body.StartURL == "" || body.LocalLogin {
		t.Errorf("config = %+v", body)
	}
}

// The start and callback endpoints are registered whether or not client mode is
// on, so they have to answer truthfully when it is off rather than begin a flow
// against a provider that was never configured.
func TestSSOEndpointsAreInertWithoutAProvider(t *testing.T) {
	server := New(Deps{})
	for name, handler := range map[string]http.HandlerFunc{
		"start":    server.HandleSSOStart,
		"callback": server.HandleSSOCallback,
	} {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/"+name, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", name, rec.Code)
		}
	}
}
