/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The access mode, as the federated sign-in paths see it.
 *
 * The eID half of these is beside the mode itself in internal/tenant/auth;
 * these three go through resolveOrProvisionSSOUser and the sign-in
 * configuration screen, and travel with sso_client_handlers.go.
 */

package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/auth"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/ssoclient"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ssoModeServer(t *testing.T, mode string) (*Handlers, *settings.Store, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to a migrated test database to run the access mode tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	store := settings.NewStore(pool)
	settings.UseStore(store)
	t.Cleanup(func() { settings.UseStore(nil) })

	setSSOMode(t, pool, store, mode)
	// Provisioning through SSO resolves a tenant and checks the quota, both of
	// which are the sign-in package's.
	return New(Deps{DB: pool, Authn: auth.New(auth.Deps{DB: pool})}), store, pool
}

// setSSOMode writes the access mode and reloads, which is what the console's
// own write does after it commits.
func setSSOMode(t *testing.T, pool *pgxpool.Pool, store *settings.Store, mode string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO registry.platform_settings (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		settings.AccessMode, mode); err != nil {
		t.Fatalf("write the access mode: %v", err)
	}
	if err := store.Load(context.Background()); err != nil {
		t.Fatalf("reload the settings: %v", err)
	}
}

func TestAPrivatePlatformProvisionsNobodyThroughSSO(t *testing.T) {
	server, _, pool := ssoModeServer(t, settings.AccessPrivate)

	slug := fmt.Sprintf("sso-jit-%d", time.Now().UnixNano())
	var tenantID string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO registry.tenants (slug, name) VALUES ($1, $1) RETURNING id::text`, slug).
		Scan(&tenantID); err != nil {
		t.Fatalf("create the provisioning organisation: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1::uuid`, tenantID)
	})

	config := ssoclient.Config{Issuer: "https://provider.example", TenantSlug: slug}
	identity := &ssoclient.Identity{
		Subject: fmt.Sprintf("subject-%d", time.Now().UnixNano()),
		Email:   fmt.Sprintf("stranger-%d@example.mn", time.Now().UnixNano()),
		Name:    "A Stranger",
	}

	if _, _, err := server.resolveOrProvisionSSOUser(context.Background(), config, identity); err == nil {
		t.Fatal("a private platform provisioned an account through a federated provider")
	}

	var people int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM tenant.memberships WHERE tenant_id = $1::uuid`, tenantID).Scan(&people); err != nil {
		t.Fatalf("count the members: %v", err)
	}
	if people != 0 {
		t.Fatalf("%d accounts were created while the platform was private", people)
	}
}

// Switching to public takes effect on the next request, with no restart — the
// property that makes this a setting rather than an environment variable.
func TestSwitchingToPublicOpensProvisioningWithoutARestart(t *testing.T) {
	server, store, pool := ssoModeServer(t, settings.AccessPrivate)

	slug := fmt.Sprintf("open-%d", time.Now().UnixNano())
	var tenantID string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO registry.tenants (slug, name) VALUES ($1, $1) RETURNING id::text`, slug).
		Scan(&tenantID); err != nil {
		t.Fatalf("create the provisioning organisation: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1::uuid`, tenantID)
	})

	config := ssoclient.Config{Issuer: "https://provider.example", TenantSlug: slug}
	identity := &ssoclient.Identity{
		Subject: fmt.Sprintf("subject-%d", time.Now().UnixNano()),
		Email:   fmt.Sprintf("newcomer-%d@example.mn", time.Now().UnixNano()),
		Name:    "A Newcomer",
	}

	if _, _, err := server.resolveOrProvisionSSOUser(context.Background(), config, identity); err == nil {
		t.Fatal("the platform was open while private")
	}

	// The same process, the same objects, one row changed.
	setSSOMode(t, pool, store, settings.AccessPublic)

	userID, gotTenant, err := server.resolveOrProvisionSSOUser(context.Background(), config, identity)
	if err != nil {
		t.Fatalf("a public platform refused to provision: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id = $1::uuid`, userID)
	})
	if gotTenant != tenantID {
		t.Fatalf("the account landed in %s, want %s", gotTenant, tenantID)
	}
}

// The sign-in screen has to know, or it offers a way in that this deployment
// will refuse.
func TestTheSignInConfigurationReportsTheMode(t *testing.T) {
	server, store, pool := ssoModeServer(t, settings.AccessPrivate)

	read := func() string {
		recorder := httptest.NewRecorder()
		server.HandleSSOConfig(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/config", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("the configuration endpoint answered %d", recorder.Code)
		}
		var body struct {
			AccessMode string `json:"access_mode"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return body.AccessMode
	}

	if got := read(); got != settings.AccessPrivate {
		t.Fatalf("the configuration reports %q", got)
	}
	setSSOMode(t, pool, store, settings.AccessPublic)
	if got := read(); got != settings.AccessPublic {
		t.Fatalf("the configuration still reports %q after the switch", got)
	}
}
