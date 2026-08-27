/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Signing in with Google for the first time, and the second.
 *
 * The other end-to-end test covers adding Google from the profile, where the
 * person is already signed in. This covers the flow that starts at the sign-in
 * screen: Google answers, nobody here recognises the account, so the identity
 * is parked and eID is asked to say who this is. What the two share is the
 * thing worth proving — that afterwards the Google identity is written down,
 * under the same issuer and subject the next sign-in will look for.
 *
 * If it is not, the symptom is not an error. It is eID being asked again every
 * single time, and a profile that says Google is not connected while the
 * person is looking at the Google account they just used.
 *
 *	AUTH_TEST_DATABASE_URL=postgres://... go test ./internal/operator/...
 */

package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/identity/eid"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/identity/eidmongolia"
)

// bindFixture is the link fixture without the person: nobody is signed in when
// this flow starts, which is the whole difference.
type bindFixture struct {
	*linkFixture
}

func newBindFixture(t *testing.T) *bindFixture {
	t.Helper()
	// The eID service reads this at construction, so it has to be set before
	// the server is built.
	t.Setenv("EID_MOCK_MODE", "true")
	t.Setenv("EID_RP_SECRET", "test-linking-key")

	f := &bindFixture{linkFixture: newLinkFixture(t)}
	f.server.eidSvc = eid.NewEIDService()

	// The person this flow will land on already exists and already signs in
	// with eID — which is the case worth testing, because it is the one people
	// are actually in. newLinkFixture made them; this points their eID identity
	// at whoever the mock provider will say they are, so the flow resolves to
	// them rather than provisioning somebody new.
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE platform.user_eid_identities SET person_etsi = $2 WHERE user_id = $1`,
		f.userID, eidmongolia.PersonEtsi("CID-AA90010111")); err != nil {
		t.Fatalf("point the eID identity at the mock: %v", err)
	}
	return f
}

// signInWithGoogle walks the sign-in start and callback, returning where the
// browser is sent afterwards.
func (f *bindFixture) signInWithGoogle(t *testing.T) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "https://nexus.test.invalid/api/v1/auth/google/start", nil)
	rec := httptest.NewRecorder()
	f.server.HandleGoogleStart(rec, req)

	authorizeURL := rec.Header().Get("Location")
	if !strings.HasPrefix(authorizeURL, f.google.server.URL) {
		t.Fatalf("sign-in did not reach the provider: %q", authorizeURL)
	}
	return f.returnFromGoogle(t, authorizeURL, rec.Result().Cookies(), "")
}

// bindingTokenFrom pulls the token out of the /login/bind redirect.
func bindingTokenFrom(t *testing.T, landing string) string {
	t.Helper()
	parsed, err := url.Parse(landing)
	if err != nil {
		t.Fatalf("parse the landing URL %q: %v", landing, err)
	}
	if !strings.HasSuffix(parsed.Path, "/login/bind") {
		t.Fatalf("landed at %q, want the binding screen", landing)
	}
	token := parsed.Query().Get("b")
	if token == "" {
		t.Fatalf("the binding screen was given no token: %q", landing)
	}
	return token
}

// completeEID consents and then drives the eID step to completion. The mock
// provider reports RUNNING for its first moment, which is what the real one
// does while somebody reaches for their phone.
func (f *bindFixture) completeEID(t *testing.T, token string) *httptest.ResponseRecorder {
	t.Helper()

	consent := httptest.NewRequest(http.MethodPost, "/api/v1/auth/bind/consent",
		strings.NewReader(`{"binding":"`+token+`"}`))
	rec := httptest.NewRecorder()
	f.server.HandleBindingConsent(rec, consent)
	if rec.Code != http.StatusOK {
		t.Fatalf("consent returned %d: %s", rec.Code, rec.Body.String())
	}

	start := httptest.NewRequest(http.MethodPost, "/api/v1/auth/bind/eid/start",
		strings.NewReader(`{"binding":"`+token+`"}`))
	rec = httptest.NewRecorder()
	f.server.HandleBindingEIDStart(rec, start)
	if rec.Code != http.StatusOK {
		t.Fatalf("eID start returned %d: %s", rec.Code, rec.Body.String())
	}
	var started struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil || started.SessionID == "" {
		t.Fatalf("no eID session came back: %s", rec.Body.String())
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		poll := httptest.NewRequest(http.MethodPost, "/api/v1/auth/bind/eid/poll",
			strings.NewReader(`{"binding":"`+token+`","session_id":"`+started.SessionID+`"}`))
		rec = httptest.NewRecorder()
		f.server.HandleBindingEIDPoll(rec, poll)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"RUNNING"`) {
			return rec
		}
		if time.Now().After(deadline) {
			t.Fatal("the eID step never left RUNNING")
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// The first sign-in: parked, verified with eID, and written down.
func TestAFirstGoogleSignInIsBoundByEID(t *testing.T) {
	f := newBindFixture(t)

	landing := f.signInWithGoogle(t)
	token := bindingTokenFrom(t, landing)

	rec := f.completeEID(t, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("the eID step failed with %d: %s", rec.Code, rec.Body.String())
	}

	// Whoever eID said this is now owns the Google identity. Found by issuer
	// and subject, because that is how the next sign-in will look for it.
	var userID string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT user_id::text FROM platform.user_sso_identities WHERE issuer = $1 AND subject = $2`,
		f.google.server.URL, f.google.subject).Scan(&userID); err != nil {
		t.Fatalf("the Google identity was not written down: %v", err)
	}
	if userID != f.userID {
		t.Fatalf("Google was attached to %s, want the eID account %s", userID, f.userID)
	}

	identities := f.server.LinkedIdentities(context.Background(), userID)
	if len(identities) != 2 {
		t.Fatalf("identities = %d, want the eID and the Google that started this: %+v", len(identities), identities)
	}
	var sawGoogle bool
	for _, identity := range identities {
		if identity.Kind == "sso" {
			sawGoogle = true
			if identity.Email != f.google.email {
				t.Errorf("email = %q, want %q", identity.Email, f.google.email)
			}
			if identity.Claims["picture"] == nil {
				t.Error("what Google said was not kept")
			}
		}
	}
	if !sawGoogle {
		t.Error("the profile would show no Google account")
	}
}

// The second sign-in is the one that was broken: it must go straight through
// rather than asking for eID again.
func TestASecondGoogleSignInDoesNotAskForEIDAgain(t *testing.T) {
	f := newBindFixture(t)

	token := bindingTokenFrom(t, f.signInWithGoogle(t))
	if rec := f.completeEID(t, token); rec.Code != http.StatusOK {
		t.Fatalf("the first sign-in failed with %d: %s", rec.Code, rec.Body.String())
	}
	var userID string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT user_id::text FROM platform.user_sso_identities WHERE issuer = $1 AND subject = $2`,
		f.google.server.URL, f.google.subject).Scan(&userID); err != nil {
		t.Fatalf("the first sign-in linked nothing: %v", err)
	}

	// Same Google account, a fresh visit.
	landing := f.signInWithGoogle(t)
	if strings.Contains(landing, "/login/bind") {
		t.Fatalf("the second sign-in was sent to eID again: %s", landing)
	}
	if strings.Contains(landing, "sso_error") {
		t.Fatalf("the second sign-in failed: %s", landing)
	}

	// And it did not quietly make a second account.
	var accounts int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM platform.user_sso_identities WHERE subject = $1`, f.google.subject).Scan(&accounts); err != nil {
		t.Fatalf("count identities: %v", err)
	}
	if accounts != 1 {
		t.Errorf("the subject is recorded %d times, want once", accounts)
	}
}

// Consent is not a formality: the eID step must refuse until it is given.
func TestTheEIDStepRefusesUntilConsentIsGiven(t *testing.T) {
	f := newBindFixture(t)
	token := bindingTokenFrom(t, f.signInWithGoogle(t))

	start := httptest.NewRequest(http.MethodPost, "/api/v1/auth/bind/eid/start",
		strings.NewReader(`{"binding":"`+token+`"}`))
	rec := httptest.NewRecorder()
	f.server.HandleBindingEIDStart(rec, start)
	if rec.Code == http.StatusOK {
		t.Error("eID was started before the person agreed to what would be shared")
	}
}

// A linked identity that the sign-in path cannot see is worse than no link at
// all: the person is sent through eID again, every time, while their profile
// shows the provider as connected. This asks what happens when the membership
// the lookup joins against is not there.
func TestASignInFindsALinkedIdentityWithoutAMembership(t *testing.T) {
	f := newBindFixture(t)

	// Link it the ordinary way first, then take the membership away — the
	// state somebody is in after being removed from an organisation, or
	// provisioned into none.
	token := bindingTokenFrom(t, f.signInWithGoogle(t))
	if rec := f.completeEID(t, token); rec.Code != http.StatusOK {
		t.Fatalf("the first sign-in failed with %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := f.pool.Exec(context.Background(),
		`DELETE FROM tenant.memberships WHERE user_id = $1`, f.userID); err != nil {
		t.Fatalf("remove the membership: %v", err)
	}

	// The identity is still there, and the profile still shows it.
	identities := f.server.LinkedIdentities(context.Background(), f.userID)
	if len(identities) != 2 {
		t.Fatalf("the profile lost an identity along with the membership: %+v", identities)
	}

	landing := f.signInWithGoogle(t)
	if strings.Contains(landing, "/login/bind") {
		t.Fatalf("a linked Google account was sent through eID again because the membership was missing: %s", landing)
	}
	// It cannot sign in either — there is nothing to sign in to — but it has to
	// say so rather than pretend the account is new.
	if !strings.Contains(landing, "no_organisation") {
		t.Errorf("landed at %q, want a reason naming the missing organisation", landing)
	}
}
