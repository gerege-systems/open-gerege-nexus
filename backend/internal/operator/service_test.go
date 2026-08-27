/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package operator

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestLegacyConsoleRouteMovesToVersionedPlatformAPI(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("CONTROL_PLANE_HOST", "")

	router := chi.NewRouter()
	New(nil, ConsoleDeps{}).Routes(router)

	tests := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{name: "prefix", method: http.MethodGet, path: "/cp/api", want: "/api/platform/v1"},
		{name: "path and query", method: http.MethodPost, path: "/cp/api/tenants/acme/suspend?reason=test", want: "/api/platform/v1/tenants/acme/suspend?reason=test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"reason":"test"}`))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusPermanentRedirect {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusPermanentRedirect)
			}
			if got := rec.Header().Get("Location"); got != tt.want {
				t.Fatalf("Location = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLegacyConsoleRouteRemainsBehindHostGate(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("CONTROL_PLANE_HOST", "cp.nexus.test")

	router := chi.NewRouter()
	New(nil, ConsoleDeps{}).Routes(router)
	req := httptest.NewRequest(http.MethodGet, "https://public.nexus.test/cp/api/tenants", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Fatalf("public host disclosed legacy route in Location %q", got)
	}
}
