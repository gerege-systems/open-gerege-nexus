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
