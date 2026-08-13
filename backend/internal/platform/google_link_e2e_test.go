/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Connecting Google to an account, from the button to the row in the database.
 *
 * The unit tests around this each proved one link in the chain and none of them
 * proved the chain. What was shipped several times over was a flow that could
 * only be judged by pressing the button on a deployment, which is the slowest
 * and least reliable way to find out whether it works.
 *
 * So this stands a fake OpenID provider up in-process and walks the whole
 * thing: press the button, follow the redirect, come back with the code the
 * provider issued, and then read the profile the way the screen reads it. The
 * provider is fake; every line of ours between the two is real, including the
 * session, the cookies, the id_token verification and the SQL.
 *
 *	AUTH_TEST_DATABASE_URL=postgres://... go test ./internal/platform/...
 */

package platform

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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/ssoclient"
)

// ---------------------------------------------------------------- the provider

// fakeGoogle is an OpenID provider that behaves like Google for the parts this
// flow touches: it publishes a discovery document and a key, and it trades an
// authorization code for an id_token it signs.
type fakeGoogle struct {
	server  *httptest.Server
	key     *rsa.PrivateKey
	subject string
	email   string
	// nonce is whatever the last authorization request asked for. The id_token
	// has to echo it back or verification refuses the token, which is the point
	// of the nonce.
	nonce string
	// sawPrompt records what the authorization request asked the provider to
	// do. A flow that never asks is one the provider may answer in silence.
	sawPrompt string
	// sawVerifier is the PKCE verifier presented at the token endpoint.
	sawVerifier string
}

// signIDToken produces an RS256 id_token by hand. The platform verifies these
// without a JWT library, so the test does not add one either — a dependency
// that exists only in tests can hide a bug in the code that has to do it the
// hard way.
func (p *fakeGoogle) signIDToken(claims map[string]any) string {
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": "fake"})
	payload, _ := json.Marshal(claims)
	signingInput := rawB64(header) + "." + rawB64(payload)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, digest[:])
	if err != nil {
		panic("sign id_token: " + err.Error())
	}
	return signingInput + "." + rawB64(signature)
}

func newFakeGoogle(t *testing.T, subject, email string) *fakeGoogle {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	p := &fakeGoogle{key: key, subject: subject, email: email}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, map[string]any{
			"issuer":                 p.server.URL,
			"authorization_endpoint": p.server.URL + "/o/oauth2/v2/auth",
			"token_endpoint":         p.server.URL + "/token",
			"jwks_uri":               p.server.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, map[string]any{"keys": []any{map[string]any{
			"kty": "RSA", "use": "sig", "alg": "RS256", "kid": "fake",
			"n": rawB64(key.N.Bytes()),
			"e": rawB64(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		// The verifier arrives here, which is the half of PKCE the provider
		// checks; recording it lets a test assert the exchange was bound to
		// the request that started it.
		p.sawVerifier = r.PostFormValue("code_verifier")
		writeTestJSON(w, map[string]any{
			"access_token": "fake-access", "token_type": "Bearer",
			"id_token": p.signIDToken(map[string]any{
				"iss":            p.server.URL,
				"aud":            "test-client",
				"sub":            p.subject,
				"email":          p.email,
				"email_verified": true,
				"name":           "Erdenebat Tsenddorj",
				"picture":        "https://lh3.example.invalid/a/photo",
				"nonce":          p.nonce,
				"iat":            time.Now().Unix(),
				"exp":            time.Now().Add(5 * time.Minute).Unix(),
			}),
		})
	})

	p.server = httptest.NewServer(mux)
	t.Cleanup(p.server.Close)
	return p
}

// client builds the ssoclient this platform would hold for Google.
//
// Derived from GoogleConfigFromEnv rather than written out here, and only the
// three fields that must point at the fake are replaced. The first version of
// this test spelled the configuration out, which meant it asserted its own
// literal back to itself: deleting prompt=select_account from the real
// configuration left it green. A test that cannot fail when the code is wrong
// is worse than no test, because it is also a claim.
func (p *fakeGoogle) client(t *testing.T, redirectURI string) *ssoclient.Client {
	t.Helper()
	t.Setenv("GOOGLE_LOGIN_CLIENT_ID", "test-client")
	t.Setenv("GOOGLE_LOGIN_CLIENT_SECRET", "test-secret")
	t.Setenv("SSO_ISSUER", "https://nexus.test.invalid")

	cfg := ssoclient.GoogleConfigFromEnv()
	cfg.Issuer = p.server.URL
	cfg.RedirectURI = redirectURI
	return ssoclient.New(cfg)
}

func writeTestJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func rawB64(data []byte) string { return base64.RawURLEncoding.EncodeToString(data) }

// ------------------------------------------------------------------ the fixture

type linkFixture struct {
	server  *Server
	pool    *pgxpool.Pool
	google  *fakeGoogle
	userID  string
	session string
}

func newLinkFixture(t *testing.T) *linkFixture {
	t.Helper()
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to a migrated test database to run the Google link tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping: %v", err)
	}
	t.Cleanup(pool.Close)

	google := newFakeGoogle(t, "google-subject-"+uuid.NewString(), "person+"+uuid.NewString()+"@identity.invalid")
	fixture := &linkFixture{
		pool:   pool,
		google: google,
		server: &Server{
			db:          pool,
			sessions:    auth.NewSessionStore(pool, auth.DefaultSessionTTL),
			googleLogin: google.client(t, "https://nexus.test.invalid/api/v1/auth/google/callback"),
		},
	}
	fixture.userID, fixture.session = fixture.newSignedInPerson(t)
	return fixture
}

// newSignedInPerson creates an account with an eID identity already attached —
// the state somebody is in when they press the button — and a live session.
func (f *linkFixture) newSignedInPerson(t *testing.T) (userID, token string) {
	t.Helper()
	ctx := context.Background()
	if err := f.pool.QueryRow(ctx,
		`INSERT INTO users(email, password_hash, name) VALUES($1,'x','link probe') RETURNING id::text`,
		"link+"+uuid.NewString()+"@identity.invalid").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { _, _ = f.pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID) })

	if _, err := f.pool.Exec(ctx,
		`INSERT INTO user_eid_identities(user_id, person_etsi, given_name, surname, claims)
		 VALUES($1,$2,'Эрдэнэбат','Цэнддорж','{}'::jsonb)`, userID, "ETSI-"+uuid.NewString()); err != nil {
		t.Fatalf("insert eid identity: %v", err)
	}

	var tenantID string
	if err := f.pool.QueryRow(ctx, `SELECT id::text FROM tenants ORDER BY created_at LIMIT 1`).Scan(&tenantID); err != nil {
		t.Fatalf("read a tenant: %v", err)
	}
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO memberships(tenant_id, user_id) VALUES($1,$2) ON CONFLICT DO NOTHING`,
		tenantID, userID); err != nil {
		t.Fatalf("insert membership: %v", err)
	}

	token, _, err := f.sessionFor(userID, tenantID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return userID, token
}

func (f *linkFixture) sessionFor(userID, tenantID string) (string, time.Time, error) {
	return f.server.sessions.Create(context.Background(), userID, tenantID, "eid", "go-test", "127.0.0.1")
}

// press is the button. It returns where the browser is sent and the cookies it
// was handed, because the flow lives in those cookies and the next step needs
// them exactly as a browser would present them.
func (f *linkFixture) press(t *testing.T, sessionToken string) (location string, jar []*http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "https://nexus.test.invalid/api/v1/auth/google/link", nil)
	if sessionToken != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionToken})
	}
	rec := httptest.NewRecorder()
	f.server.handleGoogleLinkStart(rec, req)
	return rec.Header().Get("Location"), rec.Result().Cookies()
}

// returnFromGoogle replays what the provider does to the browser: a GET back to
// the callback carrying the code and the state, with the flow cookie still set.
func (f *linkFixture) returnFromGoogle(t *testing.T, authorizeURL string, jar []*http.Cookie, sessionToken string) string {
	t.Helper()
	parsed, err := url.Parse(authorizeURL)
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	q := parsed.Query()
	f.google.nonce = q.Get("nonce")
	f.google.sawPrompt = q.Get("prompt")

	target := "https://nexus.test.invalid/api/v1/auth/google/callback?code=fake-code&state=" + url.QueryEscape(q.Get("state"))
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for _, cookie := range jar {
		req.AddCookie(cookie)
	}
	if sessionToken != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionToken})
	}
	rec := httptest.NewRecorder()
	f.server.handleGoogleCallback(rec, req)
	return rec.Header().Get("Location")
}

// ---------------------------------------------------------------------- tests

// The whole thing, in the order a person does it.
func TestLinkingGoogleFromTheProfileEndToEnd(t *testing.T) {
	f := newLinkFixture(t)

	authorizeURL, jar := f.press(t, f.session)
	if !strings.HasPrefix(authorizeURL, f.google.server.URL) {
		t.Fatalf("the button did not send the browser to the provider: %q", authorizeURL)
	}
	if len(jar) == 0 {
		t.Fatal("no flow cookie was set, so the callback would have nothing to check the state against")
	}

	landing := f.returnFromGoogle(t, authorizeURL, jar, f.session)
	if strings.Contains(landing, "link_error") {
		t.Fatalf("the link failed: %s", landing)
	}
	if !strings.HasSuffix(landing, "/profile") {
		t.Errorf("landed at %q, want the profile", landing)
	}

	// The provider must have been asked to show the chooser. Without this the
	// trip can complete with nothing on screen, which is what made an earlier
	// version of this look like a button that did nothing.
	if f.google.sawPrompt != "select_account" {
		t.Errorf("prompt = %q, want select_account", f.google.sawPrompt)
	}

	// And the row exists, under the right person, with what Google said.
	identities := f.server.linkedIdentities(context.Background(), f.userID)
	if len(identities) != 2 {
		t.Fatalf("identities = %d, want 2 (eID and Google)", len(identities))
	}
	var google *linkedIdentity
	for i := range identities {
		if identities[i].Kind == "sso" {
			google = &identities[i]
		}
	}
	if google == nil {
		t.Fatal("no Google identity was recorded")
	}
	if google.Subject != f.google.subject {
		t.Errorf("subject = %q, want %q", google.Subject, f.google.subject)
	}
	if google.Email != f.google.email {
		t.Errorf("email = %q, want %q", google.Email, f.google.email)
	}
	if google.Claims["picture"] != "https://lh3.example.invalid/a/photo" {
		t.Errorf("the picture claim was not kept: %v", google.Claims["picture"])
	}
	if google.Claims["email_verified"] != true {
		t.Errorf("email_verified was not kept: %v", google.Claims["email_verified"])
	}
	// Two identities, so either may now go.
	for _, identity := range identities {
		if !identity.Removable {
			t.Errorf("%s is not removable despite there being two", identity.Kind)
		}
	}
}

// Pressing it twice is something people do. It must refresh rather than fail.
func TestLinkingTheSameGoogleTwiceIsNotAnError(t *testing.T) {
	f := newLinkFixture(t)

	for attempt := 1; attempt <= 2; attempt++ {
		authorizeURL, jar := f.press(t, f.session)
		if landing := f.returnFromGoogle(t, authorizeURL, jar, f.session); strings.Contains(landing, "link_error") {
			t.Fatalf("attempt %d failed: %s", attempt, landing)
		}
	}

	if got := len(f.server.linkedIdentities(context.Background(), f.userID)); got != 2 {
		t.Errorf("identities = %d after linking twice, want 2", got)
	}
}

// The guard that matters most: one person's Google may not be taken by another.
func TestAGoogleAccountCannotBeTakenFromAnotherPerson(t *testing.T) {
	f := newLinkFixture(t)

	authorizeURL, jar := f.press(t, f.session)
	if landing := f.returnFromGoogle(t, authorizeURL, jar, f.session); strings.Contains(landing, "link_error") {
		t.Fatalf("the first link failed: %s", landing)
	}

	// Somebody else, able to authenticate at Google as the same account.
	otherID, otherSession := f.newSignedInPerson(t)
	authorizeURL, jar = f.press(t, otherSession)
	landing := f.returnFromGoogle(t, authorizeURL, jar, otherSession)
	if !strings.Contains(landing, "already_linked_elsewhere") {
		t.Fatalf("the second account was allowed to take it: %s", landing)
	}

	var owner string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT user_id::text FROM user_sso_identities WHERE subject = $1`, f.google.subject).Scan(&owner); err != nil {
		t.Fatalf("read the identity's owner: %v", err)
	}
	if owner != f.userID {
		t.Errorf("owner = %s, want the original person %s", owner, f.userID)
	}
	if got := len(f.server.linkedIdentities(context.Background(), otherID)); got != 1 {
		t.Errorf("the other person ended up with %d identities, want only their eID", got)
	}
}

// Signed out mid-flow: nothing is linked, and the reason is said out loud.
func TestALinkWithoutASessionLinksNothing(t *testing.T) {
	f := newLinkFixture(t)

	if landing, _ := f.press(t, ""); !strings.Contains(landing, "link_error=session_expired") {
		t.Errorf("pressing it signed out went to %q, want a session_expired reason", landing)
	}

	// And a session that ends between the button and Google's answer.
	authorizeURL, jar := f.press(t, f.session)
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE sessions SET revoked_at = NOW() WHERE user_id = $1`, f.userID); err != nil {
		t.Fatalf("revoke sessions: %v", err)
	}
	landing := f.returnFromGoogle(t, authorizeURL, jar, f.session)
	if !strings.Contains(landing, "link_error=session_expired") {
		t.Errorf("returning with a dead session went to %q, want a session_expired reason", landing)
	}
	if got := len(f.server.linkedIdentities(context.Background(), f.userID)); got != 1 {
		t.Errorf("identities = %d, want the eID alone — nothing should have been linked", got)
	}
}

// A callback whose state does not match the cookie is somebody else's request.
func TestACallbackWithTheWrongStateIsRefused(t *testing.T) {
	f := newLinkFixture(t)

	authorizeURL, jar := f.press(t, f.session)
	parsed, _ := url.Parse(authorizeURL)
	f.google.nonce = parsed.Query().Get("nonce")

	req := httptest.NewRequest(http.MethodGet,
		"https://nexus.test.invalid/api/v1/auth/google/callback?code=fake-code&state=not-the-one", nil)
	for _, cookie := range jar {
		req.AddCookie(cookie)
	}
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: f.session})
	rec := httptest.NewRecorder()
	f.server.handleGoogleCallback(rec, req)

	if got := len(f.server.linkedIdentities(context.Background(), f.userID)); got != 1 {
		t.Errorf("a mismatched state linked something: identities = %d", got)
	}
}

// Unlinking, and the floor under it.
func TestUnlinkingGoogleLeavesTheEIDPinned(t *testing.T) {
	f := newLinkFixture(t)

	authorizeURL, jar := f.press(t, f.session)
	if landing := f.returnFromGoogle(t, authorizeURL, jar, f.session); strings.Contains(landing, "link_error") {
		t.Fatalf("link failed: %s", landing)
	}

	body := strings.NewReader(`{"kind":"sso","issuer":"` + f.google.server.URL + `","subject":"` + f.google.subject + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile/identities/unlink", body)
	req = req.WithContext(auth.WithUserContext(req.Context(), auth.UserClaims{UserID: f.userID}))
	rec := httptest.NewRecorder()
	f.server.handleUnlinkIdentity(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unlink returned %d: %s", rec.Code, rec.Body.String())
	}
	identities := f.server.linkedIdentities(context.Background(), f.userID)
	if len(identities) != 1 || identities[0].Kind != "eid" {
		t.Fatalf("after unlinking Google, identities = %+v", identities)
	}
	if identities[0].Removable {
		t.Error("the last identity is still offered as removable")
	}

	// And the server refuses even if the screen asks anyway.
	body = strings.NewReader(`{"kind":"eid","subject":"` + identities[0].Subject + `"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/profile/identities/unlink", body)
	req = req.WithContext(auth.WithUserContext(req.Context(), auth.UserClaims{UserID: f.userID}))
	rec = httptest.NewRecorder()
	f.server.handleUnlinkIdentity(rec, req)
	if rec.Code != http.StatusConflict {
		t.Errorf("removing the last identity returned %d, want 409", rec.Code)
	}
}
