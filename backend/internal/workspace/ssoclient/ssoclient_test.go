package ssoclient

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// A provider stood up in-process, so the whole relying-party flow can be
// exercised for real: discovery, the authorization request, the code exchange
// and the signature over the id_token it answers with.
type fakeProvider struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	kid    string

	// What the token endpoint will mint, and what it saw when it did.
	claims       map[string]any
	sawVerifier  string
	sawBasicAuth string
}

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	p := &fakeProvider{key: key, kid: "test-key"}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"issuer":                 p.server.URL,
			"authorization_endpoint": p.server.URL + "/oauth2/auth",
			"token_endpoint":         p.server.URL + "/oauth2/token",
			"userinfo_endpoint":      p.server.URL + "/oauth2/userinfo",
			"jwks_uri":               p.server.URL + "/.well-known/jwks.json",
			"end_session_endpoint":   p.server.URL + "/oauth2/logout",
		})
	})
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"keys": []any{map[string]any{
			"kty": "RSA", "use": "sig", "alg": "RS256", "kid": p.kid,
			"n": b64(key.N.Bytes()),
			"e": b64(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		p.sawVerifier = r.PostFormValue("code_verifier")
		p.sawBasicAuth = r.Header.Get("Authorization")
		writeJSON(w, map[string]any{
			"access_token": "opaque", "token_type": "Bearer",
			"id_token": p.sign(t, p.claims),
		})
	})
	p.server = httptest.NewServer(mux)
	t.Cleanup(p.server.Close)
	return p
}

// sign mints an RS256 id_token over claims, filling in the ones every valid
// token has so a test only has to state the one it is about.
func (p *fakeProvider) sign(t *testing.T, claims map[string]any) string {
	t.Helper()
	full := map[string]any{
		"iss": p.server.URL,
		"sub": "provider-subject-1",
		"aud": "client-under-test",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	for key, value := range claims {
		if value == nil {
			delete(full, key)
			continue
		}
		full[key] = value
	}

	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": p.kid})
	payload, _ := json.Marshal(full)
	signingInput := b64(header) + "." + b64(payload)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signingInput + "." + b64(signature)
}

func (p *fakeProvider) client(mutate ...func(*Config)) *Client {
	cfg := Config{
		Issuer:      p.server.URL,
		ClientID:    "client-under-test",
		Scopes:      []string{"openid", "profile", "email"},
		RedirectURI: "https://client.example" + CallbackPath,
	}
	for _, m := range mutate {
		m(&cfg)
	}
	return New(cfg)
}

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func b64(data []byte) string { return base64.RawURLEncoding.EncodeToString(data) }

func TestBeginAuthorizationCarriesPKCEAndState(t *testing.T) {
	provider := newFakeProvider(t)
	client := provider.client()

	request, err := client.BeginAuthorization(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	parsed, err := url.Parse(request.URL)
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	q := parsed.Query()

	if got := q.Get("response_type"); got != "code" {
		t.Errorf("response_type = %q, want code", got)
	}
	if got := q.Get("code_challenge_method"); got != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", got)
	}
	if q.Get("state") != request.State || q.Get("nonce") != request.Nonce {
		t.Error("the request URL does not carry the state and nonce the caller was handed to keep")
	}
	// The challenge must be the hash of the verifier, not the verifier — sending
	// the verifier itself would make PKCE a formality.
	sum := sha256.Sum256([]byte(request.CodeVerifier))
	if want := b64(sum[:]); q.Get("code_challenge") != want {
		t.Errorf("code_challenge = %q, want the S256 hash of the verifier", q.Get("code_challenge"))
	}
	if len(request.CodeVerifier) < 43 {
		t.Errorf("code_verifier is %d characters; RFC 7636 requires at least 43", len(request.CodeVerifier))
	}
}

func TestExchangeReturnsVerifiedIdentity(t *testing.T) {
	provider := newFakeProvider(t)
	provider.claims = map[string]any{
		"nonce": "nonce-1", "email": "person@example.mn",
		"email_verified": true, "name": "Бат", "tenant_slug": "demo",
	}
	client := provider.client()

	identity, err := client.Exchange(context.Background(), "code-1", strings.Repeat("v", 43), "nonce-1")
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if identity.Subject != "provider-subject-1" {
		t.Errorf("subject = %q", identity.Subject)
	}
	if identity.Email != "person@example.mn" || !identity.EmailVerified {
		t.Errorf("email = %q verified = %v", identity.Email, identity.EmailVerified)
	}
	if identity.Name != "Бат" || identity.TenantSlug != "demo" {
		t.Errorf("name = %q tenant_slug = %q", identity.Name, identity.TenantSlug)
	}
	if identity.IDToken == "" {
		t.Error("the raw id_token was not kept; signing out cannot present a hint without it")
	}
	if provider.sawVerifier != strings.Repeat("v", 43) {
		t.Errorf("the token endpoint saw code_verifier %q", provider.sawVerifier)
	}
	// A public client presents no secret. Sending one would be a credential
	// leaked to a provider that never issued it.
	if provider.sawBasicAuth != "" {
		t.Errorf("a public client authenticated with %q", provider.sawBasicAuth)
	}
}

func TestExchangeAuthenticatesAConfidentialClient(t *testing.T) {
	provider := newFakeProvider(t)
	provider.claims = map[string]any{"nonce": "nonce-1"}
	client := provider.client(func(c *Config) { c.ClientSecret = "s3cret" })

	if _, err := client.Exchange(context.Background(), "code-1", strings.Repeat("v", 43), "nonce-1"); err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if !strings.HasPrefix(provider.sawBasicAuth, "Basic ") {
		t.Fatalf("the token endpoint saw Authorization %q, want HTTP Basic", provider.sawBasicAuth)
	}
}

// Each of these is a token that verifies cryptographically and must still be
// refused, because a signature only says who minted it — not that it was minted
// for us, for this sign-in, or recently.
func TestExchangeRefusesTokensThatDoNotAnswerThisRequest(t *testing.T) {
	cases := []struct {
		name   string
		claims map[string]any
		nonce  string
		want   string
	}{
		{
			name:   "a token for another client of the same provider",
			claims: map[string]any{"aud": "somebody-elses-client", "nonce": "nonce-1"},
			nonce:  "nonce-1",
			want:   "not issued for this client",
		},
		{
			name:   "a token answering a different sign-in",
			claims: map[string]any{"nonce": "a-different-nonce"},
			nonce:  "nonce-1",
			want:   "different sign-in request",
		},
		{
			name:   "an expired token",
			claims: map[string]any{"nonce": "nonce-1", "exp": time.Now().Add(-time.Hour).Unix()},
			nonce:  "nonce-1",
			want:   "expired",
		},
		{
			name:   "a token naming a different issuer",
			claims: map[string]any{"nonce": "nonce-1", "iss": "https://somewhere.else"},
			nonce:  "nonce-1",
			want:   "issued by",
		},
		{
			name:   "a token authorised for another party",
			claims: map[string]any{"nonce": "nonce-1", "azp": "somebody-elses-client"},
			nonce:  "nonce-1",
			want:   "different client",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := newFakeProvider(t)
			provider.claims = tc.claims
			client := provider.client()

			_, err := client.Exchange(context.Background(), "code-1", strings.Repeat("v", 43), tc.nonce)
			if err == nil {
				t.Fatal("the exchange succeeded; it must not")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

// A signature from a key the provider never published is the substitution
// attack the JWKS exists to prevent.
func TestExchangeRefusesAForeignSignature(t *testing.T) {
	provider := newFakeProvider(t)
	provider.claims = map[string]any{"nonce": "nonce-1"}

	forged, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	provider.key = forged // the JWKS still publishes the original

	client := provider.client()
	_, err = client.Exchange(context.Background(), "code-1", strings.Repeat("v", 43), "nonce-1")
	if err == nil || !strings.Contains(err.Error(), "does not verify") {
		t.Fatalf("error = %v, want a refusal to verify the signature", err)
	}
}

// The issuer in a discovery document has to be the one that was asked for.
// Without the check, a redirect to somebody else's document would hand this
// client an authorization endpoint of the attacker's choosing.
func TestDiscoveryRefusesAMismatchedIssuer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"issuer":                 "https://not-the-one-we-asked-for",
			"authorization_endpoint": "https://evil.example/auth",
			"token_endpoint":         "https://evil.example/token",
			"jwks_uri":               "https://evil.example/jwks",
		})
	}))
	defer server.Close()

	client := New(Config{Issuer: server.URL, ClientID: "c", RedirectURI: "https://client.example" + CallbackPath})
	if _, err := client.BeginAuthorization(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "names issuer") {
		t.Fatalf("error = %v, want the issuer mismatch to be refused", err)
	}
}

func TestEndSessionURLCarriesWhatTheProviderNeeds(t *testing.T) {
	provider := newFakeProvider(t)
	client := provider.client(func(c *Config) {
		c.PostLogoutRedirectURI = "https://client.example/"
	})

	target := client.EndSessionURL(context.Background(), "the-id-token")
	parsed, err := url.Parse(target)
	if err != nil {
		t.Fatalf("parse end session URL: %v", err)
	}
	q := parsed.Query()
	if q.Get("client_id") != "client-under-test" {
		t.Errorf("client_id = %q", q.Get("client_id"))
	}
	if q.Get("id_token_hint") != "the-id-token" {
		t.Errorf("id_token_hint = %q", q.Get("id_token_hint"))
	}
	if q.Get("post_logout_redirect_uri") != "https://client.example/" {
		t.Errorf("post_logout_redirect_uri = %q", q.Get("post_logout_redirect_uri"))
	}
}

// A provider without an end_session_endpoint gets an empty string rather than a
// URL to nowhere, so the caller finishes the sign-out locally.
func TestEndSessionURLIsEmptyWithoutTheEndpoint(t *testing.T) {
	// The document has to name its own origin as the issuer or discovery
	// refuses it, and the handler only learns that origin from the request.
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"issuer": "http://" + r.Host, "authorization_endpoint": "http://" + r.Host + "/auth",
			"token_endpoint": "http://" + r.Host + "/token", "jwks_uri": "http://" + r.Host + "/jwks",
		})
	}))
	defer echo.Close()

	client := New(Config{Issuer: echo.URL, ClientID: "c", RedirectURI: "https://client.example" + CallbackPath})
	if got := client.EndSessionURL(context.Background(), "hint"); got != "" {
		t.Errorf("EndSessionURL = %q, want empty for a provider that offers no logout endpoint", got)
	}
}

// A provider that already has a session with the browser, for somebody who has
// already granted these scopes, may answer without showing anything at all.
// The browser leaves and comes straight back, which reads as a button that did
// nothing — and picks an account on a shared machine without asking. So the
// request has to say what it wants.
func TestAuthParamsReachTheAuthorizationRequest(t *testing.T) {
	provider := newFakeProvider(t)
	client := provider.client(func(cfg *Config) {
		cfg.AuthParams = map[string]string{"prompt": "select_account", "access_type": "online"}
	})

	request, err := client.BeginAuthorization(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	q, err := url.Parse(request.URL)
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	if got := q.Query().Get("prompt"); got != "select_account" {
		t.Errorf("prompt = %q, want select_account", got)
	}
	if got := q.Query().Get("access_type"); got != "online" {
		t.Errorf("access_type = %q, want online", got)
	}
}

// Configuration may add to the request; it may not rewrite what the protocol
// just generated. A deployment that could set state or code_challenge could
// turn its own PKCE off by writing a value it does not hold the verifier for.
func TestAuthParamsCannotOverrideProtocolParameters(t *testing.T) {
	provider := newFakeProvider(t)
	client := provider.client(func(cfg *Config) {
		cfg.AuthParams = map[string]string{
			"state":                 "attacker-chosen",
			"code_challenge":        "attacker-chosen",
			"code_challenge_method": "plain",
			"redirect_uri":          "https://elsewhere.invalid/callback",
		}
	})

	request, err := client.BeginAuthorization(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	parsed, err := url.Parse(request.URL)
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	q := parsed.Query()
	if q.Get("state") == "attacker-chosen" || q.Get("state") != request.State {
		t.Error("configuration overwrote the generated state")
	}
	if q.Get("code_challenge") == "attacker-chosen" {
		t.Error("configuration overwrote the PKCE challenge")
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Error("configuration downgraded the PKCE method")
	}
	if q.Get("redirect_uri") == "https://elsewhere.invalid/callback" {
		t.Error("configuration redirected the callback elsewhere")
	}
}

// The Google configuration is where the choice is actually made, so assert it
// there too: a future edit that drops the map would otherwise leave both tests
// above passing and the behaviour gone.
func TestGoogleAsksForTheAccountChooser(t *testing.T) {
	t.Setenv("GOOGLE_LOGIN_CLIENT_ID", "id.apps.googleusercontent.com")
	t.Setenv("SSO_ISSUER", "https://nexus.example.mn")

	cfg := GoogleConfigFromEnv()
	if cfg.AuthParams["prompt"] != "select_account" {
		t.Errorf("prompt = %q, want select_account", cfg.AuthParams["prompt"])
	}
	if cfg.AuthParams["access_type"] != "online" {
		t.Errorf("access_type = %q, want online", cfg.AuthParams["access_type"])
	}
}
