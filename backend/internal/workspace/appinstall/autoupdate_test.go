package appinstall_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/appinstall"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/jackc/pgx/v5/pgxpool"
)

// What auto-update must never do is widen what an app may do without anybody
// agreeing to it. These run against a real schema because the decision reads
// the manifest history in app_versions and writes a pin — neither of which
// exists in Go.
//
//	TEST_DATABASE_URL=postgres://... go test ./internal/workspace/appinstall/...
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a migrated database to run the auto-update tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// externalApp builds a catalogue entry for a third-party platform.
func externalApp(version string, permissions []string, scopes []string, launch string) catalog.CatalogApp {
	manifest := catalog.Manifest{
		ID: "mn.test.hrms", Name: "Test HRMS", Version: version, Type: catalog.TypeExternal,
		External: &catalog.ExternalSpec{LaunchURL: launch, Scopes: scopes, Embed: "new_tab"},
	}
	for _, code := range permissions {
		manifest.Permissions = append(manifest.Permissions,
			nexus.PermissionDefinition{Code: code, Name: code})
	}
	return catalog.CatalogApp{
		ID: manifest.ID, Slug: "test-hrms", Name: manifest.Name,
		Version: version, Visibility: "public", Manifest: manifest,
	}
}

// fixture puts one tenant with one installed external app into the database.
func fixture(t *testing.T, installed catalog.CatalogApp) (*pgxpool.Pool, string) {
	t.Helper()
	pool := testPool(t)
	ctx := context.Background()

	var tenantID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.tenants (slug, name) VALUES ('au-' || substr(gen_random_uuid()::text, 1, 8), 'Auto update')
		 RETURNING id::text`).Scan(&tenantID); err != nil {
		t.Fatalf("tenant: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1`, tenantID) })
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.apps WHERE id = $1`, installed.ID)
	})

	// The installed version, in the catalogue table and the version history —
	// which is what a later version is compared against.
	if _, err := pool.Exec(ctx,
		`INSERT INTO registry.apps (id, slug, name, visibility) VALUES ($1, $2, $3, 'public')
		 ON CONFLICT (id) DO NOTHING`, installed.ID, installed.Slug, installed.Name); err != nil {
		t.Fatalf("app: %v", err)
	}
	manifest, err := json.Marshal(installed.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO registry.app_versions (app_id, version, manifest) VALUES ($1, $2, $3)
		 ON CONFLICT (app_id, version) DO NOTHING`, installed.ID, installed.Version, manifest); err != nil {
		t.Fatalf("version: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace.app_installations (tenant_id, app_id, installed_version, status, enabled)
		 VALUES ($1, $2, $3, 'installed', TRUE)`, tenantID, installed.ID, installed.Version); err != nil {
		t.Fatalf("installation: %v", err)
	}
	return pool, tenantID
}

func installationState(t *testing.T, pool *pgxpool.Pool, tenantID, appID string) (version, pinned string) {
	t.Helper()
	if err := pool.QueryRow(context.Background(),
		`SELECT installed_version, COALESCE(pinned_version, '') FROM workspace.app_installations
		  WHERE tenant_id = $1 AND app_id = $2`, tenantID, appID).Scan(&version, &pinned); err != nil {
		t.Fatalf("read installation: %v", err)
	}
	return version, pinned
}

func TestAWiderVersionIsHeldForAnAdministrator(t *testing.T) {
	installed := externalApp("1.0.0", []string{"hrms.read"}, []string{"openid"}, "https://hrms.test/sso")
	pool, tenantID := fixture(t, installed)

	// The same app, now asking to read the organisation's ERP data.
	wider := externalApp("1.1.0", []string{"hrms.read"}, []string{"openid", "erp.read"}, "https://hrms.test/sso")
	installer := appinstall.NewAppInstaller(pool, []catalog.CatalogApp{wider}, "1.0.0")

	result, err := installer.AutoUpdate(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(result.Upgraded) != 0 {
		t.Fatalf("a version asking for more must not be applied silently; upgraded %+v", result.Upgraded)
	}
	if len(result.Held) != 1 {
		t.Fatalf("expected one held installation, got %+v", result.Held)
	}
	if got := result.Held[0].Added; len(got) != 1 || got[0] != "scope:erp.read" {
		t.Fatalf("expected the added scope to be named, got %v", got)
	}

	version, pinned := installationState(t, pool, tenantID, installed.ID)
	if version != "1.0.0" {
		t.Fatalf("the installation moved to %s", version)
	}
	// Pinned where it is, so the next sweep does not ask the same question
	// again — the administrator's answer is what moves it.
	if pinned != "1.0.0" {
		t.Fatalf("expected the installation to be pinned at 1.0.0, got %q", pinned)
	}
}

func TestAMetadataOnlyVersionIsAppliedOnItsOwn(t *testing.T) {
	installed := externalApp("1.0.0", []string{"hrms.read"}, []string{"openid"}, "https://hrms.test/sso")
	pool, tenantID := fixture(t, installed)

	// A new name and a new path under the same host: nothing to approve.
	same := externalApp("1.1.0", []string{"hrms.read"}, []string{"openid"}, "https://hrms.test/sso/v2")
	installer := appinstall.NewAppInstaller(pool, []catalog.CatalogApp{same}, "1.0.0")

	result, err := installer.AutoUpdate(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(result.Held) != 0 {
		t.Fatalf("nothing was widened; expected no hold, got %+v", result.Held)
	}
	if len(result.Upgraded) != 1 {
		t.Fatalf("expected the installation to follow the catalogue, got %+v", result.Upgraded)
	}

	version, pinned := installationState(t, pool, tenantID, installed.ID)
	if version != "1.1.0" {
		t.Fatalf("expected 1.1.0, got %s", version)
	}
	if pinned != "" {
		t.Fatal("an unremarkable update must not pin anything")
	}
}

func TestMovingToAnotherHostIsHeld(t *testing.T) {
	installed := externalApp("1.0.0", []string{"hrms.read"}, []string{"openid"}, "https://hrms.test/sso")
	pool, _ := fixture(t, installed)

	// Somewhere else entirely is not the same application, whatever the
	// manifest calls itself.
	moved := externalApp("1.1.0", []string{"hrms.read"}, []string{"openid"}, "https://elsewhere.example/sso")
	installer := appinstall.NewAppInstaller(pool, []catalog.CatalogApp{moved}, "1.0.0")

	result, err := installer.AutoUpdate(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(result.Held) != 1 {
		t.Fatalf("expected the host change to be held, got %+v", result.Held)
	}
}

func TestAnInstallationWithAutoUpdateOffIsLeftAlone(t *testing.T) {
	installed := externalApp("1.0.0", []string{"hrms.read"}, []string{"openid"}, "https://hrms.test/sso")
	pool, tenantID := fixture(t, installed)

	if _, err := pool.Exec(context.Background(),
		`UPDATE workspace.app_installations SET auto_update = FALSE WHERE tenant_id = $1`, tenantID); err != nil {
		t.Fatal(err)
	}

	newer := externalApp("1.1.0", []string{"hrms.read"}, []string{"openid"}, "https://hrms.test/sso")
	installer := appinstall.NewAppInstaller(pool, []catalog.CatalogApp{newer}, "1.0.0")

	result, err := installer.AutoUpdate(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(result.Upgraded) != 0 || len(result.Held) != 0 {
		t.Fatalf("an administrator turned this off; nothing should have happened: %+v", result)
	}
	if version, _ := installationState(t, pool, tenantID, installed.ID); version != "1.0.0" {
		t.Fatalf("expected the installation to stay at 1.0.0, got %s", version)
	}
}

func TestTurningAutoUpdateBackOnClearsThePin(t *testing.T) {
	installed := externalApp("1.0.0", []string{"hrms.read"}, []string{"openid"}, "https://hrms.test/sso")
	pool, tenantID := fixture(t, installed)

	wider := externalApp("1.1.0", []string{"hrms.read"}, []string{"openid", "erp.read"}, "https://hrms.test/sso")
	installer := appinstall.NewAppInstaller(pool, []catalog.CatalogApp{wider}, "1.0.0")
	if _, err := installer.AutoUpdate(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if _, pinned := installationState(t, pool, tenantID, installed.ID); pinned == "" {
		t.Fatal("expected the sweep to pin the installation")
	}

	// The two switches are the same decision seen from either end, so one
	// must not leave the other's state behind.
	if err := installer.SetAutoUpdate(context.Background(), tenantID, "test-hrms", true); err != nil {
		t.Fatalf("set auto-update: %v", err)
	}
	if _, pinned := installationState(t, pool, tenantID, installed.ID); pinned != "" {
		t.Fatalf("expected the pin to be cleared, got %q", pinned)
	}
}
