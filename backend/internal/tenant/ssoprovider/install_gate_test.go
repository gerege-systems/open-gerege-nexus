package ssoprovider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// The other half of an external app's install gate: what the authorization
// endpoint does with the answer. The decision itself is tested without a
// database in internal/operator; this is the endpoint, against real rows.
//
//	OAUTH_TEST_DATABASE_URL=postgres://... go test ./internal/tenant/ssoprovider/...

// stubGate answers whatever it is told to.
type stubGate struct {
	allow bool
	err   error
}

func (g stubGate) AllowClient(context.Context, string, string) (bool, error) {
	return g.allow, g.err
}

func TestAuthorizeRefusesAClientTheTenantHasNotInstalled(t *testing.T) {
	f := newFixture(t)
	f.consent(t)
	f.provider.AttachInstallGate(stubGate{allow: false})

	status, redirect := f.authorize(t, nil)
	if status != http.StatusFound {
		t.Fatalf("expected a redirect back to the client, got %d", status)
	}
	location, err := url.Parse(redirect)
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	// access_denied, not unauthorized_client: the client is fine, this user's
	// organisation is not entitled to it.
	if got := location.Query().Get("error"); got != "access_denied" {
		t.Fatalf("expected access_denied, got %q", got)
	}
	if got := location.Query().Get("code"); got != "" {
		t.Fatal("a refused authorization must not carry a code")
	}
	if got := location.Query().Get("state"); got != "state-123" {
		t.Errorf("state must be echoed on the error too; got %q", got)
	}
}

func TestAGateThatCannotAnswerRefuses(t *testing.T) {
	f := newFixture(t)
	f.consent(t)
	// "Allowed" here would hand somebody's third-party HR system a user their
	// employer never onboarded, because a database was briefly unreachable.
	f.provider.AttachInstallGate(stubGate{allow: true, err: errors.New("database unreachable")})

	_, redirect := f.authorize(t, nil)
	location, err := url.Parse(redirect)
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	if got := location.Query().Get("error"); got != "access_denied" {
		t.Fatalf("expected access_denied when the gate fails, got %q", got)
	}
}

func TestConsentCannotBeUsedToWalkAroundTheGate(t *testing.T) {
	f := newFixture(t)
	f.provider.AttachInstallGate(stubGate{allow: false})

	// The consent endpoint mints a code of its own, so a browser posting
	// straight to it would otherwise never meet the gate at /oauth2/auth.
	form := url.Values{
		"client_id":             {f.client.ClientID},
		"redirect_uri":          {f.client.RedirectURIs[0]},
		"scope":                 {"openid profile"},
		"state":                 {"state-123"},
		"code_challenge":        {f.challenge},
		"code_challenge_method": {"S256"},
		"approved":              {"true"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/oauth2/consent", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "test-session"})
	rec := httptest.NewRecorder()
	f.provider.HandleConsentDecision(rec, req)

	var body struct {
		RedirectTo string `json:"redirect_to"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(body.RedirectTo, "error=access_denied") {
		t.Fatalf("expected the consent decision to be refused; got %q", body.RedirectTo)
	}
	if strings.Contains(body.RedirectTo, "code=") {
		t.Fatal("a refused consent must not mint a code")
	}
}

func TestAnInstalledAppSignsInAndTheTokenNamesTheTenant(t *testing.T) {
	f := newFixture(t)
	f.provider.AttachInstallGate(stubGate{allow: true})

	status, tokens := f.token(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {f.codeFromRedirect(t)},
		"redirect_uri":  {f.client.RedirectURIs[0]},
		"code_verifier": {f.verifier},
	})
	if status != http.StatusOK {
		t.Fatalf("token exchange failed with %d: %v", status, tokens)
	}
	idToken, _ := tokens["id_token"].(string)
	if idToken == "" {
		t.Fatal("expected an id_token for the openid scope")
	}

	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		t.Fatalf("expected a JWT, got %q", idToken)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}

	// A third-party platform is signing somebody in on behalf of an
	// organisation and has to be told which one — by id, which is what matters,
	// and by slug, so it does not have to keep a table of this platform's UUIDs.
	if claims["tenant_id"] != f.tenantID {
		t.Fatalf("expected tenant_id %q, got %v", f.tenantID, claims["tenant_id"])
	}
	slug, _ := claims["tenant_slug"].(string)
	if slug == "" {
		t.Fatal("expected a tenant_slug claim")
	}

	var expected string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT slug FROM registry.tenants WHERE id = $1`, f.tenantID).Scan(&expected); err != nil {
		t.Fatalf("read the tenant slug: %v", err)
	}
	if slug != expected {
		t.Fatalf("expected tenant_slug %q, got %q", expected, slug)
	}
}
