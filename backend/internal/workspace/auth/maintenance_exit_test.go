/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Maintenance must not trap anybody in the organisation it is on.
//
// Reading works, signing out works — and moving to another organisation has to
// work too, because it is the other way out. It did not: switching is a POST,
// the read-only gate refused every POST, and the administrator who had just
// turned Maintenance on could reach none of their other organisations until
// somebody turned it off again.
func TestMaintenanceLetsSomebodyLeave(t *testing.T) {
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to a migrated test database")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	store := settings.NewStore(pool)
	settings.UseStore(store)
	t.Cleanup(func() { settings.UseStore(nil) })
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO registry.platform_settings (key, value) VALUES ($1, 'true')
		 ON CONFLICT (key) DO UPDATE SET value = 'true'`, settings.Maintenance); err != nil {
		t.Fatalf("turn Maintenance on: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM registry.platform_settings WHERE key = $1`, settings.Maintenance)
	})
	if err := store.Load(context.Background()); err != nil {
		t.Fatalf("load the settings: %v", err)
	}

	handlers := New(Deps{DB: pool})
	for _, probe := range []struct {
		path    string
		refused bool
	}{
		{"/api/v1/auth/switch-tenant", false},
		{"/api/v1/auth/logout", false},
		{"/api/v1/organisation", true},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, probe.path, nil)
		if got := handlers.RefuseIfReadOnly(recorder, request, ""); got != probe.refused {
			t.Errorf("POST %s refused=%v, want %v", probe.path, got, probe.refused)
		}
	}
}
