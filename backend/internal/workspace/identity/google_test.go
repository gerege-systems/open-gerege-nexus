package identity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/ssoclient"
)

func googleServer(t *testing.T, mutate ...func(*ssoclient.Config)) *Handlers {
	t.Helper()
	cfg := ssoclient.Config{
		EnvPrefix:   "GOOGLE_LOGIN",
		EnvClientID: "GOOGLE_LOGIN_CLIENT_ID",
		Issuer:      ssoclient.GoogleIssuer,
		ClientID:    "123.apps.googleusercontent.com",
		RedirectURI: "https://nexus.gerege.mn" + ssoclient.GoogleCallbackPath,
	}
	for _, m := range mutate {
		m(&cfg)
	}
	return New(Deps{Google: ssoclient.New(cfg)})
}

func ssoConfigOf(t *testing.T, server *Handlers) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	server.HandleSSOConfig(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/config", nil))
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body
}

func TestGoogleIsOffUntilItIsConfigured(t *testing.T) {
	body := ssoConfigOf(t, New(Deps{}))
	google, _ := body["google"].(map[string]any)
	if google == nil || google["enabled"] != false {
		t.Fatalf("google = %v, want it reported as unavailable", body["google"])
	}

	// And the endpoints say so rather than starting a flow against a provider
	// that was never configured.
	for name, handler := range map[string]http.HandlerFunc{
		"start":    New(Deps{}).HandleGoogleStart,
		"callback": New(Deps{}).HandleGoogleCallback,
	} {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/"+name, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", name, rec.Code)
		}
	}
}

func TestGoogleIsOfferedWhenConfigured(t *testing.T) {
	body := ssoConfigOf(t, googleServer(t))
	google, _ := body["google"].(map[string]any)
	if google["enabled"] != true {
		t.Fatalf("google = %v, want it offered", body["google"])
	}
	if google["start_url"] == "" || google["start_url"] == nil {
		t.Error("the screen is told Google is available but not where to send anybody")
	}
}

// A deployment that federates has handed the question of who somebody is to its
// provider. Google is one of this platform's own answers to that question, so
// it closes with the rest — otherwise the front door nobody manages is back.
func TestGoogleClosesWhenTheDeploymentFederates(t *testing.T) {
	server := googleServer(t)
	server.ssoClient = ssoclient.New(ssoclient.Config{
		Issuer: "https://nexus.gerege.mn", ClientID: "aimag",
		RedirectURI: "https://aimag.gerege.mn" + ssoclient.CallbackPath,
	})

	if google, _ := ssoConfigOf(t, server)["google"].(map[string]any); google["enabled"] != false {
		t.Error("Google is still offered on a deployment that federates")
	}

	rec := httptest.NewRecorder()
	server.HandleGoogleStart(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/start", nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 with the federated sign-in address", rec.Code)
	}

	// Unless the operator kept the local paths open on purpose.
	server.ssoClient = ssoclient.New(ssoclient.Config{
		Issuer: "https://nexus.gerege.mn", ClientID: "aimag", LocalLogin: true,
		RedirectURI: "https://aimag.gerege.mn" + ssoclient.CallbackPath,
	})
	if google, _ := ssoConfigOf(t, server)["google"].(map[string]any); google["enabled"] != true {
		t.Error("SSO_CLIENT_LOCAL_LOGIN did not keep the Google button")
	}
}
