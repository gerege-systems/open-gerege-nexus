/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package setup

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/geregecore"
	"github.com/go-chi/chi/v5"
)

// The two guards on the route that can open the console, both of which have to
// hold before anything is written and neither of which needs a database — which
// is the point: a deployment that cannot serve a console, or a request that
// does not carry the wizard's token, is refused before the first query.

func routerWithToken(token string) chi.Router {
	s := New(nil)
	s.token = token
	r := chi.NewRouter()
	s.Routes(r)
	return r
}

func post(t *testing.T, r chi.Router, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Setup-Token", token)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// Without the token the console routes are not merely refused, they are absent:
// 404 rather than 401, so a stranger is not told there is a token to guess.
func TestTheConsoleRoutesAreInvisibleWithoutTheToken(t *testing.T) {
	r := routerWithToken("the-real-token")
	for _, path := range []string{"/api/v1/setup/operator", "/api/v1/setup/operator/confirm"} {
		if got := post(t, r, path, "", `{}`).Code; got != http.StatusNotFound {
			t.Errorf("%s without a token answered %d, want 404", path, got)
		}
		if got := post(t, r, path, "a-guess", `{}`).Code; got != http.StatusNotFound {
			t.Errorf("%s with a wrong token answered %d, want 404", path, got)
		}
	}
}

// A deployment with no CONTROL_PLANE_HOST has nowhere to serve a console, so an
// operator made here could never sign in. The refusal comes before the database
// is touched, which is why this test can run without one — and why the nil pool
// above is a check rather than a shortcut: a handler that queried first would
// panic here instead of answering 409.
func TestTheConsoleCannotBeOpenedWithoutAnAddress(t *testing.T) {
	t.Setenv("CONTROL_PLANE_HOST", "")
	r := routerWithToken("the-real-token")
	rec := post(t, r, "/api/v1/setup/operator", "the-real-token",
		`{"email":"ops@example.mn","name":"Ops","password":"correct-horse-battery"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("answered %d, want 409: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "CONTROL_PLANE_HOST") {
		t.Errorf("the refusal does not name the setting that is missing: %s", rec.Body.String())
	}
}

// The first account is the deployment's own door, not a citizen.
//
// The wizard used to look a person up in the Gerege Core directory and put
// their name on it, which made a real person permanently answerable for an
// account nobody signs in as after the first hour — and left them in the user
// list as the one entry with no eID and no organisation but the first. The name
// is now fixed and the lookup is gone; the address stays, because a password
// reset has to reach somebody.
func TestTheFirstAccountIsNamedAfterItsRoleNotAPerson(t *testing.T) {
	if SuperAdminName != "Super Admin" {
		t.Errorf("the first account is called %q", SuperAdminName)
	}

	// A caller cannot choose the name: the field is not on the request at all,
	// so an old client sending one is ignored rather than obeyed.
	var completion Completion
	if _, ok := any(completion.Admin).(struct {
		Email string `json:"email"`
	}); !ok {
		t.Errorf("the admin step accepts more than an address: %#v", completion.Admin)
	}
}

// The directory lookup for a person is gone from the wizard. It is still
// available to the console (internal/operator/tenants), where an operator is
// asking about somebody real.
func TestTheWizardNoLongerLooksPeopleUp(t *testing.T) {
	r := routerWithToken("open-sesame")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/person",
		strings.NewReader(`{"registration_number":"УБ12345678"}`))
	req.Header.Set("X-Setup-Token", "open-sesame")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("the person lookup answered %d; it should not exist", rec.Code)
	}
}

// A registration number the directory does not know must not answer 404.
//
// 404 is requireToken's, and it means "there is no wizard here" — so the
// browser reads one as a dead token and throws the operator back to the token
// screen. A number that is merely wrong has to be distinguishable from a token
// that is merely gone, or the wizard sends people hunting for a new link when
// what they need is to retype seven digits.
func TestADirectoryMissIsNotTheGatesFourOhFour(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Service{}).failLookup(rec, geregecore.ErrNotFound)

	if rec.Code == http.StatusNotFound {
		t.Fatal("a directory miss answered 404, which the wizard reads as a stale setup token")
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("a directory miss answered %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}
