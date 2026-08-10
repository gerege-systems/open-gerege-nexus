package ssoprovider

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These exercise the authorization server against real PostgreSQL, because the
// guarantees that matter most are enforced in SQL: single-use code redemption
// rides on an UPDATE predicate, refresh rotation on a recursive revocation, and
// tenant isolation on the WHERE clause of every portal query.
//
//	OAUTH_TEST_DATABASE_URL=postgres://... go test ./internal/platform/ssoprovider/...
//
// Without one they skip, so `go test ./...` stays green on a machine with no
// database.

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("OAUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set OAUTH_TEST_DATABASE_URL to a migrated test database to run the OAuth2 flow tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// fakeSessions stands in for the platform session store.
type fakeSessions struct{ claims auth.UserClaims }

func (f *fakeSessions) Resolve(context.Context, string) (auth.UserClaims, error) {
	return f.claims, nil
}

type fixture struct {
	provider  *SSOProvider
	pool      *pgxpool.Pool
	tenantID  string
	userID    string
	email     string
	client    *Client
	secret    string
	verifier  string
	challenge string
}

const testIssuer = "https://sso.test"

func newFixture(t *testing.T, opts ...func(*Client)) *fixture {
	t.Helper()
	ctx := context.Background()
	pool := testPool(t)

	var tenantID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO tenants (slug, name) VALUES ('oauth-' || substr(gen_random_uuid()::text, 1, 8), 'OAuth test')
		 RETURNING id::text`).Scan(&tenantID); err != nil {
		t.Fatalf("tenant: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, tenantID) })

	email := "oauth-" + NewIdentifier(8) + "@example.com"
	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, name) VALUES ($1, 'x', 'OAuth Tester') RETURNING id::text`,
		email).Scan(&userID); err != nil {
		t.Fatalf("user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })

	provider := &SSOProvider{store: NewStore(pool), issuer: testIssuer}
	provider.AttachSessions(&fakeSessions{claims: auth.UserClaims{
		UserID: userID, TenantID: tenantID, Email: email,
	}})

	secret := "sec_" + NewIdentifier(48)
	client := &Client{
		TenantID:     tenantID,
		ClientID:     "app_test_" + NewIdentifier(8),
		ClientName:   "Integration Test App",
		ClientType:   clientTypeConfidential,
		RedirectURIs: []string{"https://client.test/callback"},
		GrantTypes:   SupportedGrantTypes,
		Scopes:       []string{"openid", "profile", "email", "offline_access", "erp.read"},
	}
	for _, opt := range opts {
		opt(client)
	}

	hash := ""
	if client.ClientType == clientTypeConfidential {
		hash = HashSecret(secret)
	}
	created, err := provider.store.CreateClient(ctx, client, hash, userID)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	// A fixed PKCE pair, long enough to satisfy RFC 7636.
	verifier := "verifier-" + NewIdentifier(48)
	sum := sha256.Sum256([]byte(verifier))

	return &fixture{
		provider: provider, pool: pool, tenantID: tenantID, userID: userID, email: email,
		client: created, secret: secret, verifier: verifier,
		challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
	}
}

// authorize drives the browser endpoint and returns its status and Location.
//
// Not the *http.Response: every caller wants those two fields, and handing back
// a response nobody closes is what bodyclose exists to catch.
func (f *fixture) authorize(t *testing.T, extra url.Values) (int, string) {
	t.Helper()
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {f.client.ClientID},
		"redirect_uri":          {f.client.RedirectURIs[0]},
		"scope":                 {"openid profile email offline_access"},
		"state":                 {"state-123"},
		"code_challenge":        {f.challenge},
		"code_challenge_method": {"S256"},
		"nonce":                 {"nonce-abc"},
	}
	for k, v := range extra {
		q[k] = v
	}
	req := httptest.NewRequest(http.MethodGet, "/oauth2/auth?"+q.Encode(), nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "test-session"})
	rec := httptest.NewRecorder()
	f.provider.HandleAuthorize(rec, req)
	return rec.Code, rec.Header().Get("Location")
}

// consent approves the pending grant, as the consent screen would.
func (f *fixture) consent(t *testing.T) {
	t.Helper()
	if err := f.provider.store.SaveConsent(context.Background(), f.tenantID, f.userID,
		f.client.ClientID, f.client.Scopes); err != nil {
		t.Fatalf("consent: %v", err)
	}
}

// codeFromRedirect drives authorize through consent and extracts the code.
func (f *fixture) codeFromRedirect(t *testing.T) string {
	t.Helper()
	f.consent(t)
	status, redirect := f.authorize(t, nil)
	if status != http.StatusFound {
		t.Fatalf("expected a redirect, got %d", status)
	}
	location, err := url.Parse(redirect)
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	if got := location.Query().Get("error"); got != "" {
		t.Fatalf("authorization failed: %s — %s", got, location.Query().Get("error_description"))
	}
	if got := location.Query().Get("state"); got != "state-123" {
		t.Errorf("state was not echoed back: %q", got)
	}
	code := location.Query().Get("code")
	if code == "" {
		t.Fatal("no code in the redirect")
	}
	return code
}

// token posts to the token endpoint with client_secret_post authentication.
func (f *fixture) token(t *testing.T, form url.Values) (int, map[string]any) {
	t.Helper()
	form.Set("client_id", f.client.ClientID)
	if f.client.ClientType == clientTypeConfidential {
		form.Set("client_secret", f.secret)
	}
	req := httptest.NewRequest(http.MethodPost, "/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	f.provider.HandleTokenEndpoint(rec, req)

	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

func TestAuthorizationCodeFlowEndToEnd(t *testing.T) {
	f := newFixture(t)

	code := f.codeFromRedirect(t)

	status, body := f.token(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {f.client.RedirectURIs[0]},
		"code_verifier": {f.verifier},
	})
	if status != http.StatusOK {
		t.Fatalf("token exchange failed with %d: %v", status, body)
	}

	accessToken, _ := body["access_token"].(string)
	if accessToken == "" {
		t.Fatal("no access_token in the response")
	}
	if _, ok := body["refresh_token"].(string); !ok {
		t.Error("offline_access was granted but no refresh_token was issued")
	}

	idToken, _ := body["id_token"].(string)
	if idToken == "" {
		t.Fatal("openid was granted but no id_token was issued")
	}
	claims := verifyIDToken(t, f, idToken)
	if claims["iss"] != testIssuer {
		t.Errorf("id_token issuer is %v, want %s", claims["iss"], testIssuer)
	}
	if claims["sub"] != f.userID {
		t.Errorf("id_token subject is %v, want %s", claims["sub"], f.userID)
	}
	if claims["aud"] != f.client.ClientID {
		t.Errorf("id_token audience is %v, want %s", claims["aud"], f.client.ClientID)
	}
	if claims["nonce"] != "nonce-abc" {
		t.Errorf("the nonce was not carried into the id_token: %v", claims["nonce"])
	}
	if claims["email"] != f.email {
		t.Errorf("email claim is %v, want %s", claims["email"], f.email)
	}

	// The access token has to work at the two endpoints a relying party uses.
	t.Run("userinfo answers for the token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()
		f.provider.HandleUserInfo(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("userinfo returned %d: %s", rec.Code, rec.Body.String())
		}
		var info map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &info)
		if info["sub"] != f.userID || info["email"] != f.email {
			t.Errorf("unexpected userinfo: %v", info)
		}
	})

	t.Run("introspection reports it active", func(t *testing.T) {
		if !f.introspect(t, accessToken) {
			t.Error("a freshly issued token introspected as inactive")
		}
	})
}

func TestAuthorizationCodeIsSingleUse(t *testing.T) {
	f := newFixture(t)
	code := f.codeFromRedirect(t)

	form := func() url.Values {
		return url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"redirect_uri":  {f.client.RedirectURIs[0]},
			"code_verifier": {f.verifier},
		}
	}

	status, body := f.token(t, form())
	if status != http.StatusOK {
		t.Fatalf("first redemption failed: %v", body)
	}
	accessToken := body["access_token"].(string)

	// Replaying the code is the signature of a stolen one. RFC 6749 §4.1.2
	// wants the second attempt refused and the first attempt's tokens killed.
	status, body = f.token(t, form())
	if status == http.StatusOK {
		t.Fatal("the authorization code was accepted twice")
	}
	if body["error"] != "invalid_grant" {
		t.Errorf("expected invalid_grant, got %v", body["error"])
	}
	if f.introspect(t, accessToken) {
		t.Error("the token minted from the replayed code is still active")
	}
}

func TestRefreshRotationAndReplayRevokesTheFamily(t *testing.T) {
	f := newFixture(t)
	code := f.codeFromRedirect(t)

	_, body := f.token(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {f.client.RedirectURIs[0]},
		"code_verifier": {f.verifier},
	})
	firstRefresh, _ := body["refresh_token"].(string)
	if firstRefresh == "" {
		t.Fatal("no refresh token to rotate")
	}

	status, refreshed := f.token(t, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {firstRefresh},
	})
	if status != http.StatusOK {
		t.Fatalf("refresh failed with %d: %v", status, refreshed)
	}
	secondRefresh, _ := refreshed["refresh_token"].(string)
	secondAccess, _ := refreshed["access_token"].(string)
	if secondRefresh == "" || secondRefresh == firstRefresh {
		t.Fatal("the refresh token was not rotated")
	}
	if !f.introspect(t, secondAccess) {
		t.Fatal("the access token from the refresh is not active")
	}

	// Replaying the retired token means somebody has a copy. Both branches of
	// the family go, including the honest holder's — the grant is over.
	status, replay := f.token(t, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {firstRefresh},
	})
	if status == http.StatusOK {
		t.Fatal("a rotated refresh token was accepted a second time")
	}
	if replay["error"] != "invalid_grant" {
		t.Errorf("expected invalid_grant, got %v", replay["error"])
	}
	if f.introspect(t, secondAccess) {
		t.Error("the descendant access token survived the replay; the family was not revoked")
	}
}

func TestRefreshCannotWidenScope(t *testing.T) {
	f := newFixture(t)
	code := f.codeFromRedirect(t)
	_, body := f.token(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {f.client.RedirectURIs[0]},
		"code_verifier": {f.verifier},
	})
	refresh := body["refresh_token"].(string)

	// erp.read is registered for the client but was never granted in this
	// authorization, so a refresh must not be able to reach it.
	status, widened := f.token(t, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refresh},
		"scope":         {"openid erp.read"},
	})
	if status == http.StatusOK {
		t.Fatalf("a refresh widened the granted scope: %v", widened)
	}
	if widened["error"] != "invalid_scope" {
		t.Errorf("expected invalid_scope, got %v", widened["error"])
	}
}

func TestPKCEAndRedirectBindingAreEnforced(t *testing.T) {
	t.Run("a wrong verifier is refused", func(t *testing.T) {
		f := newFixture(t)
		code := f.codeFromRedirect(t)
		status, body := f.token(t, url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"redirect_uri":  {f.client.RedirectURIs[0]},
			"code_verifier": {"verifier-" + NewIdentifier(48)},
		})
		if status == http.StatusOK {
			t.Fatal("a code was redeemed with the wrong PKCE verifier")
		}
		if body["error"] != "invalid_grant" {
			t.Errorf("expected invalid_grant, got %v", body["error"])
		}
	})

	t.Run("a mismatched redirect_uri is refused", func(t *testing.T) {
		f := newFixture(t)
		code := f.codeFromRedirect(t)
		status, _ := f.token(t, url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"redirect_uri":  {"https://client.test/other"},
			"code_verifier": {f.verifier},
		})
		if status == http.StatusOK {
			t.Fatal("the code was redeemed against a redirect_uri it was not bound to")
		}
	})

	t.Run("an unregistered redirect_uri never reaches a redirect", func(t *testing.T) {
		f := newFixture(t)
		status, redirect := f.authorize(t, url.Values{"redirect_uri": {"https://attacker.test/steal"}})
		if status == http.StatusFound {
			t.Fatalf("the endpoint redirected to an unregistered URI: %s", redirect)
		}
	})

	t.Run("PKCE is mandatory", func(t *testing.T) {
		f := newFixture(t)
		f.consent(t)
		_, redirect := f.authorize(t, url.Values{"code_challenge": {""}})
		location, _ := url.Parse(redirect)
		if location.Query().Get("error") != "invalid_request" {
			t.Errorf("a request without a code_challenge was not refused: %s", redirect)
		}
	})
}

func TestConsentIsRequiredBeforeACodeIsIssued(t *testing.T) {
	f := newFixture(t)

	status, location := f.authorize(t, nil)
	if status != http.StatusFound {
		t.Fatalf("expected a redirect, got %d", status)
	}
	if !strings.Contains(location, "/oauth/consent") {
		t.Fatalf("expected a bounce to the consent screen, got %s", location)
	}

	// prompt=none must not silently consent on the user's behalf.
	_, denied := f.authorize(t, url.Values{"prompt": {"none"}})
	parsed, _ := url.Parse(denied)
	if parsed.Query().Get("error") != "consent_required" {
		t.Errorf("prompt=none without consent returned %q", parsed.Query().Get("error"))
	}
}

func TestClientCredentialsGrantHasNoUserAndNoIdentityScopes(t *testing.T) {
	f := newFixture(t)

	status, body := f.token(t, url.Values{
		"grant_type": {"client_credentials"},
		"scope":      {"openid erp.read"},
	})
	if status != http.StatusOK {
		t.Fatalf("client_credentials failed with %d: %v", status, body)
	}
	scope, _ := body["scope"].(string)
	if strings.Contains(scope, "openid") {
		t.Errorf("openid survived a grant with no user in it: %q", scope)
	}
	if !strings.Contains(scope, "erp.read") {
		t.Errorf("erp.read should have been granted: %q", scope)
	}
	if _, ok := body["id_token"]; ok {
		t.Error("an id_token was issued for a grant with no user")
	}

	// UserInfo has nothing to say about a machine client.
	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+body["access_token"].(string))
	rec := httptest.NewRecorder()
	f.provider.HandleUserInfo(rec, req)
	if rec.Code == http.StatusOK {
		t.Error("userinfo answered for a client_credentials token")
	}
}

func TestClientAuthenticationFailures(t *testing.T) {
	f := newFixture(t)

	t.Run("a wrong secret is refused", func(t *testing.T) {
		form := url.Values{
			"grant_type":    {"client_credentials"},
			"client_id":     {f.client.ClientID},
			"client_secret": {"sec_wrong"},
		}
		req := httptest.NewRequest(http.MethodPost, "/oauth2/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		f.provider.HandleTokenEndpoint(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("an unknown client is refused identically", func(t *testing.T) {
		// Same status and same body as a wrong secret: the endpoint must not
		// become a way to find out which client_ids exist.
		form := url.Values{
			"grant_type":    {"client_credentials"},
			"client_id":     {"app_does_not_exist"},
			"client_secret": {"sec_wrong"},
		}
		req := httptest.NewRequest(http.MethodPost, "/oauth2/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		f.provider.HandleTokenEndpoint(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("a missing secret is refused for a confidential client", func(t *testing.T) {
		// The predecessor skipped verification entirely when the secret was
		// absent, so anyone who knew a client_id could mint tokens.
		form := url.Values{
			"grant_type": {"client_credentials"},
			"client_id":  {f.client.ClientID},
		}
		req := httptest.NewRequest(http.MethodPost, "/oauth2/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		f.provider.HandleTokenEndpoint(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("a secretless request minted a token: %d %s", rec.Code, rec.Body.String())
		}
	})
}

func TestClientsAreTenantIsolated(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	var otherTenant string
	if err := f.pool.QueryRow(ctx,
		`INSERT INTO tenants (slug, name) VALUES ('oauth-other-' || substr(gen_random_uuid()::text, 1, 8), 'Other')
		 RETURNING id::text`).Scan(&otherTenant); err != nil {
		t.Fatalf("tenant: %v", err)
	}
	t.Cleanup(func() { _, _ = f.pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, otherTenant) })

	// The portal used to call a provider method that returned every client on
	// the platform, so this listing would have included the other tenant's.
	listed, err := f.provider.store.ListClients(ctx, otherTenant)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, c := range listed {
		if c.ClientID == f.client.ClientID {
			t.Fatal("a tenant can see another tenant's OAuth2 client")
		}
	}

	// Nor can it reach one by naming the client_id directly.
	if _, err := f.provider.store.GetTenantClient(ctx, otherTenant, f.client.ClientID); err == nil {
		t.Fatal("a tenant fetched another tenant's client by id")
	}
	if err := f.provider.store.DeleteClient(ctx, otherTenant, f.client.ClientID); err == nil {
		t.Fatal("a tenant deleted another tenant's client")
	}
	if err := f.provider.store.RotateClientSecret(ctx, otherTenant, f.client.ClientID, HashSecret("x")); err == nil {
		t.Fatal("a tenant rotated another tenant's client secret")
	}
}

func TestSecretRotationInvalidatesTheOldSecret(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.provider.store.VerifyClientSecret(ctx, f.client.ClientID, f.secret); err != nil {
		t.Fatalf("the original secret does not verify: %v", err)
	}

	replacement := "sec_" + NewIdentifier(48)
	if err := f.provider.store.RotateClientSecret(ctx, f.tenantID, f.client.ClientID, HashSecret(replacement)); err != nil {
		t.Fatalf("rotate: %v", err)
	}

	if _, err := f.provider.store.VerifyClientSecret(ctx, f.client.ClientID, f.secret); err == nil {
		t.Error("the old secret still works after rotation")
	}
	if _, err := f.provider.store.VerifyClientSecret(ctx, f.client.ClientID, replacement); err != nil {
		t.Errorf("the new secret does not work: %v", err)
	}
}

func TestRevocationKillsTheToken(t *testing.T) {
	f := newFixture(t)
	code := f.codeFromRedirect(t)
	_, body := f.token(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {f.client.RedirectURIs[0]},
		"code_verifier": {f.verifier},
	})
	accessToken := body["access_token"].(string)

	form := url.Values{
		"token":         {accessToken},
		"client_id":     {f.client.ClientID},
		"client_secret": {f.secret},
	}
	req := httptest.NewRequest(http.MethodPost, "/oauth2/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	f.provider.HandleRevoke(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke returned %d", rec.Code)
	}
	if f.introspect(t, accessToken) {
		t.Error("the token is still active after revocation")
	}

	// RFC 7009 §2.2: revoking something unknown is still a success.
	form.Set("token", "gat_"+NewIdentifier(48))
	req = httptest.NewRequest(http.MethodPost, "/oauth2/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	f.provider.HandleRevoke(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("revoking an unknown token returned %d, want 200", rec.Code)
	}
}

func TestExpiredRowsAreReclaimed(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// Nothing swept the in-memory token map this replaced; it only ever grew.
	_, err := f.provider.store.SaveToken(ctx, &Token{
		TokenHash: HashSecret("stale-" + NewIdentifier(16)), TokenType: tokenTypeAccess,
		ClientID: f.client.ClientID, TenantID: f.tenantID, Scopes: []string{"erp.read"},
		ExpiresAt: time.Now().Add(-48 * time.Hour),
	})
	if err != nil {
		t.Fatalf("seed stale token: %v", err)
	}

	before := f.countTokens(t)
	if _, err := f.provider.store.DeleteExpired(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if after := f.countTokens(t); after >= before {
		t.Errorf("the sweep reclaimed nothing: %d rows before, %d after", before, after)
	}
}

func (f *fixture) countTokens(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM oauth2_tokens WHERE client_id = $1`, f.client.ClientID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// introspect reports whether the provider considers a token live.
func (f *fixture) introspect(t *testing.T, token string) bool {
	t.Helper()
	form := url.Values{
		"token":         {token},
		"client_id":     {f.client.ClientID},
		"client_secret": {f.secret},
	}
	req := httptest.NewRequest(http.MethodPost, "/oauth2/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	f.provider.HandleIntrospect(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("introspection response: %v", err)
	}
	active, _ := body["active"].(bool)
	return active
}

// verifyIDToken checks the signature against the published JWKS and returns the
// claims. A client that cannot do this has no reason to trust the token.
func verifyIDToken(t *testing.T, f *fixture, idToken string) map[string]any {
	t.Helper()

	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		t.Fatalf("malformed id_token")
	}

	var header map[string]string
	headerJSON, _ := base64.RawURLEncoding.DecodeString(parts[0])
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		t.Fatalf("id_token header: %v", err)
	}

	var publicPEM string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT public_key_pem FROM oauth2_signing_keys WHERE kid = $1`, header["kid"]).Scan(&publicPEM); err != nil {
		t.Fatalf("the kid in the id_token is not a published key: %v", err)
	}
	block, _ := pem.Decode([]byte(publicPEM))
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse published key: %v", err)
	}

	signature, _ := base64.RawURLEncoding.DecodeString(parts[2])
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(parsed.(*rsa.PublicKey), crypto.SHA256, digest[:], signature); err != nil {
		t.Fatalf("the id_token does not verify against the published JWKS: %v", err)
	}

	var claims map[string]any
	claimsJSON, _ := base64.RawURLEncoding.DecodeString(parts[1])
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		t.Fatalf("id_token claims: %v", err)
	}
	return claims
}
