/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package host

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/cache"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// What the assembled router promises before any handler runs.
//
// route_policy_test.go asks whether a route is gated. This file asks about the
// three things that sit above every route and belong to no plane: the hardening
// headers, the CSRF gate, and the hostname the operator console answers on.
// Each is a middleware somebody could drop from newRouter without a single
// route disappearing — and the route table would still be the one on record.
//
// The console's host gate has its own unit test in internal/operator/operator.
// The claim here is different: that the gate is mounted in the router this
// process actually serves.

// assembledRouter builds the real router, with the environment set before it is
// built. Both the headers middleware and the console's host gate read their
// environment once, at construction, so a t.Setenv after this returns changes
// nothing.
func assembledRouter(t *testing.T, env map[string]string) chi.Routes {
	t.Helper()
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to a migrated test database to run the hardening tests")
	}
	for key, value := range env {
		t.Setenv(key, value)
	}
	t.Setenv("APP_CATALOG_URL", "")

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	server, err := newServer(pool, filepath.FromSlash("../../../catalog/apps.json"),
		cache.NewBus(context.Background(), nil))
	if err != nil {
		t.Fatalf("build the server: %v", err)
	}
	return server.router
}

func answer(t *testing.T, router chi.Routes, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	func() {
		// A handler reached without the state it needs panics; the recovery
		// middleware turns that into a 500, which is still an answer these
		// tests can read.
		defer func() { _ = recover() }()
		router.(http.Handler).ServeHTTP(rec, req)
	}()
	return rec
}

// Every answer carries the headers, including the ones nobody looks at until an
// audit asks. Asserted on /health because it is the one route that answers
// without a database, a session or a tenant — so a failure here is the
// middleware, not the handler.
func TestEveryAnswerCarriesTheHardeningHeaders(t *testing.T) {
	router := assembledRouter(t, map[string]string{"ENVIRONMENT": "development"})
	rec := answer(t, router, httptest.NewRequest(http.MethodGet, "/health", nil))

	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s is %q, want %q", header, got, want)
		}
	}
	// The policy's shape matters more than its exact text: no inline script,
	// nothing from another origin by default.
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("the content security policy does not confine scripts to this origin: %q", csp)
	}
	if strings.Contains(csp, "unsafe-eval") || strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Errorf("the content security policy admits inline or evaluated script: %q", csp)
	}
	if rec.Header().Get("Permissions-Policy") == "" {
		t.Error("no Permissions-Policy: the camera, microphone and geolocation are left to the browser's default")
	}
	// HSTS is a production-only header: sending it from a development
	// deployment on http would pin somebody's browser to a scheme this
	// deployment does not serve.
	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("a development deployment sent HSTS: %q", got)
	}
}

func TestAProductionDeploymentSendsHSTS(t *testing.T) {
	router := assembledRouter(t, map[string]string{"ENVIRONMENT": "production"})
	rec := answer(t, router, httptest.NewRequest(http.MethodGet, "/health", nil))
	if got := rec.Header().Get("Strict-Transport-Security"); !strings.Contains(got, "max-age=") {
		t.Errorf("production did not send HSTS: %q", got)
	}
}

// The gate that makes the cookie safe to hold. A write that carries a session
// cookie and no evidence of where it came from is refused before it reaches a
// handler; the same write from our own page is not.
func TestACookieCarryingWriteNeedsProofOfWhereItCameFrom(t *testing.T) {
	router := assembledRouter(t, map[string]string{"ENVIRONMENT": "development"})

	withCookie := func(header, value string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: security.TenantSessionCookie, Value: "a-session"})
		if header != "" {
			req.Header.Set(header, value)
		}
		return answer(t, router, req)
	}

	if rec := withCookie("", ""); rec.Code != http.StatusForbidden {
		t.Errorf("a cookie-carrying write with no Origin and no Sec-Fetch-Site answered %d, want 403", rec.Code)
	}
	if rec := withCookie("Sec-Fetch-Site", "cross-site"); rec.Code != http.StatusForbidden {
		t.Errorf("a cross-site write answered %d, want 403", rec.Code)
	}
	if rec := withCookie("Origin", "https://attacker.example"); rec.Code != http.StatusForbidden {
		t.Errorf("a write from an origin nobody allowed answered %d, want 403", rec.Code)
	}
	// Our own page says so, and the request goes through to the handler —
	// whatever the handler then makes of a session that does not exist.
	if rec := withCookie("Sec-Fetch-Site", "same-origin"); rec.Code == http.StatusForbidden {
		t.Errorf("a write from our own page was refused as cross-site: %s", rec.Body.String())
	}

	// A request with no cookie never enters the gate: a client holding a bearer
	// token, or a sign-in, must not need a browser's headers.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"","password":""}`))
	req.Header.Set("Content-Type", "application/json")
	if rec := answer(t, router, req); rec.Code == http.StatusForbidden {
		t.Errorf("a request with no cookie was refused by the CSRF gate: %s", rec.Body.String())
	}
}

// The console's second gate, in the router this process serves.
//
// nginx's address allowlist is the first, and neither trusts the other: a
// request that arrives on the tenant's hostname is answered 404 — not 403,
// which would confirm that something is there.
func TestTheConsoleAnswersOnlyOnItsOwnHost(t *testing.T) {
	router := assembledRouter(t, map[string]string{
		"ENVIRONMENT":        "production",
		"CONTROL_PLANE_HOST": "admin.test",
	})

	elsewhere := httptest.NewRequest(http.MethodGet, "/api/platform/v1/tenants", nil)
	elsewhere.Host = "nexus.test"
	if rec := answer(t, router, elsewhere); rec.Code != http.StatusNotFound {
		t.Errorf("the console answered %d on the tenant's hostname, want 404", rec.Code)
	}

	// On its own hostname it is reached — and then refuses for the ordinary
	// reason, which is that nobody is signed in. 404 there would mean the gate
	// is refusing everybody, which is the failure this test would otherwise
	// pass through.
	own := httptest.NewRequest(http.MethodGet, "/api/platform/v1/tenants", nil)
	own.Host = "admin.test"
	if rec := answer(t, router, own); rec.Code == http.StatusNotFound {
		t.Errorf("the console answered 404 on its own hostname: %s", rec.Body.String())
	}
}

// Health is what an orchestrator reads, and it names the build so a rollout can
// be told from a restart.
func TestHealthNamesTheBuild(t *testing.T) {
	router := assembledRouter(t, map[string]string{"ENVIRONMENT": "development"})
	rec := answer(t, router, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/health answered %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("the health answer is not JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("the health answer does not say ok: %+v", body)
	}
	if _, named := body["platform_version"]; !named {
		t.Errorf("the health answer does not name the build: %+v", body)
	}
}
