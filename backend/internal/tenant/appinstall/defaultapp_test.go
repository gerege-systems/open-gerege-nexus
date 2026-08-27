package appinstall_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/appinstall"
)

// A default app is what a new organisation starts with, and nothing more than
// that: the sweep installs it for a tenant that has no record of it, and an
// administrator can then remove it. Both halves are database behaviour — a
// sweep over tenants and a flag on an installation row — so they are tested
// against a real schema.
//
//	TEST_DATABASE_URL=postgres://... go test ./internal/tenant/appinstall/...
func organisationCatalogApp() catalog.CatalogApp {
	manifest := catalog.Manifest{
		ID: defaultAppID, Name: "Organisation & People", Version: "1.0.0", Platform: ">=1.0.0",
	}
	return catalog.CatalogApp{
		ID: defaultAppID, Slug: "organisation", Name: manifest.Name,
		Version: "1.0.0", Visibility: "public", Manifest: manifest,
	}
}

// The app this deployment installs for every tenant without being asked. Named
// here as a string rather than taken from the module, so this package does not
// import an app to learn one constant.
const defaultAppID = "io.gerege.nexus.organisation"

// The list is a distribution's now, set through platform.Options.DefaultApps
// rather than declared in this package. A test stands it up the same way a
// deployment does — and puts it back, because it is package-level state that
// another test would otherwise inherit.
func withDefaultApp(t *testing.T) {
	t.Helper()
	previous := appinstall.DefaultApps
	appinstall.DefaultApps = []string{defaultAppID}
	t.Cleanup(func() { appinstall.DefaultApps = previous })
}

// defaultAppModule stands in for whatever module answers for that id. Its two
// permissions are the organisation's, because the assertion downstream is that
// a default app's permissions reach the tenant's roles — the names have to be
// something, and something real reads better in a failure message.
type defaultAppModule struct{}

func (defaultAppModule) ID() string      { return defaultAppID }
func (defaultAppModule) Name() string    { return "Organisation" }
func (defaultAppModule) Version() string { return "1.0.0" }

func (defaultAppModule) Dependencies() []nexus.Dependency { return nil }

func (defaultAppModule) Permissions() []nexus.PermissionDefinition {
	return []nexus.PermissionDefinition{
		{Code: "organisation.read", Name: "Read Organisation"},
		{Code: "organisation.manage", Name: "Manage Organisation"},
	}
}

func (defaultAppModule) Menus() []nexus.MenuDefinition { return nil }

func (defaultAppModule) RegisterRoutes(chi.Router, func(http.Handler) http.Handler) {}

// newSweptTenant makes a tenant, runs the catalogue and the default-app sweep
// over it, and returns the tenant and the installer both tests then work on.
func newSweptTenant(t *testing.T) (*appinstall.AppInstaller, string) {
	t.Helper()
	pool := testPool(t)
	ctx := context.Background()

	// The module has to be registered for a module-type app to install at all;
	// in the running server this is apps.Bootstrap.
	//
	// A stub rather than the real organisation module. What is under test is the
	// installer — that a default app reaches a new tenant, with its permissions
	// — and nothing here reads a field of the module beyond the permissions it
	// declares. Importing the app for it pointed the platform at an app package,
	// which is the coupling internal/apps/boundaries_test.go exists to refuse:
	// the platform is what every deployment runs, and an app is what one product
	// ships.
	nexus.Register(defaultAppModule{})

	var tenantID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.tenants (slug, name) VALUES ('org-' || substr(gen_random_uuid()::text, 1, 8), 'Sweep')
		 RETURNING id::text`).Scan(&tenantID); err != nil {
		t.Fatalf("tenant: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1`, tenantID) })

	installer := appinstall.NewAppInstaller(pool, []catalog.CatalogApp{organisationCatalogApp()}, "1.0.0")
	// In the same order as the server: the catalogue reaches the apps table
	// first, and an installation row references it.
	if err := installer.SyncCatalog(ctx); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if err := installer.EnsureDefaultApps(ctx); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	return installer, tenantID
}

// appTable stands in for a table the default app would own.
//
// Created here rather than migrated: an app's schema is the app's, and this
// package has no app. It cascades from the tenant so the fixture's own cleanup
// takes the rows with it, and it is left in place between runs — CI runs every
// package against one database, and a table dropped at the end of one test is a
// table the next parallel package finds missing.
func appTable(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	// One transaction, because DDL here is transactional and the alternative is
	// a race: CI runs every package against one database, and a table that
	// exists for a moment with a tenant_id and no policy is what
	// TestEveryTenantTableHasForcedRLS would catch in the package next door.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS default_app_probe (
		     tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		     code      TEXT NOT NULL,
		     name      TEXT NOT NULL,
		     PRIMARY KEY (tenant_id, code)
		 )`,
		// The platform's own policy. A fixture that skipped the rule the schema
		// is held to would not behave like the thing it stands in for.
		`ALTER TABLE default_app_probe ENABLE ROW LEVEL SECURITY`,
		`ALTER TABLE default_app_probe FORCE ROW LEVEL SECURITY`,
		`DROP POLICY IF EXISTS tenant_isolation ON default_app_probe`,
		`CREATE POLICY tenant_isolation ON default_app_probe TO gerege_nexus_tenant
			USING (tenant_id IS NULL OR tenant_id = ANY (COALESCE(
				NULLIF(current_setting('app.allowed_tenants', true), '')::uuid[],
				ARRAY[NULLIF(current_setting('app.current_tenant', true), '')::uuid])))
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)`,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON default_app_probe TO gerege_nexus_tenant`,
	} {
		if _, err := tx.Exec(ctx, statement); err != nil {
			t.Fatalf("prepare the fixture table: %v", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit the fixture table: %v", err)
	}
}

func TestEveryTenantGetsTheDefaultAppWithoutAnybodyInstallingIt(t *testing.T) {
	withDefaultApp(t)
	pool := testPool(t)
	ctx := context.Background()
	installer, tenantID := newSweptTenant(t)

	var installed string
	if err := pool.QueryRow(ctx,
		`SELECT installed_version FROM tenant.app_installations WHERE tenant_id = $1 AND app_id = $2`,
		tenantID, defaultAppID).Scan(&installed); err != nil {
		t.Fatalf("the default app was not installed for a tenant that lacked it: %v", err)
	}
	if installed != "1.0.0" {
		t.Fatalf("installed version %q", installed)
	}

	// And running again is a no-op rather than a second row or an error: this
	// runs at every boot and after every catalogue sync.
	if err := installer.EnsureDefaultApps(ctx); err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM tenant.app_installations WHERE tenant_id = $1 AND app_id = $2`,
		tenantID, defaultAppID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one installation, got %d", count)
	}
}

// Removing the organisation app is allowed, survives the sweep that installed
// it, and takes nothing with it.
//
// The three claims are one test because they are one behaviour: "uninstall" on
// this platform means the gate closes, and it has to still mean that after the
// next catalogue refresh — otherwise a tenant that removed an app would find it
// back within the hour, which is indistinguishable from the removal having
// silently failed.
func TestTheDefaultAppCanBeRemovedAndStaysRemovedWithoutLosingData(t *testing.T) {
	withDefaultApp(t)
	pool := testPool(t)
	ctx := context.Background()
	installer, tenantID := newSweptTenant(t)

	// Something in the app's own tables, so "the data survived" is a row and
	// not an assumption. The table is this test's own: it used to write a
	// department, and `departments` left with the organisation app on
	// 2026-08-23. A platform test that reaches for an app's table is a platform
	// test that stops compiling every time an app moves — and the claim here is
	// about the installer, which does not care whose table it is.
	appTable(t, pool)
	if _, err := pool.Exec(ctx,
		`INSERT INTO default_app_probe (tenant_id, code, name) VALUES ($1, 'hq', 'Төв оффис')`,
		tenantID); err != nil {
		t.Fatalf("probe row: %v", err)
	}

	if err := installer.DisableApp(ctx, tenantID, "organisation", "someone"); err != nil {
		t.Fatalf("the default app refused to be disabled: %v", err)
	}

	var enabled bool
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT enabled, status FROM tenant.app_installations WHERE tenant_id = $1 AND app_id = $2`,
		tenantID, defaultAppID).Scan(&enabled, &status); err != nil {
		t.Fatalf("the installation row was deleted rather than disabled: %v", err)
	}
	if enabled || status != "disabled" {
		t.Fatalf("after disabling: enabled=%v status=%q", enabled, status)
	}

	// The sweep runs at every boot and after every catalogue sync. It must not
	// undo somebody's decision.
	if err := installer.EnsureDefaultApps(ctx); err != nil {
		t.Fatalf("sweep after removal: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT enabled FROM tenant.app_installations WHERE tenant_id = $1 AND app_id = $2`,
		tenantID, defaultAppID).Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("the sweep put back an app the tenant had removed")
	}

	// Nothing was dropped: turning it back on finds the department still there.
	if err := installer.EnableApp(ctx, tenantID, "organisation", "someone"); err != nil {
		t.Fatalf("re-enable: %v", err)
	}
	var name string
	if err := pool.QueryRow(ctx,
		`SELECT name FROM default_app_probe WHERE tenant_id = $1 AND code = 'hq'`, tenantID).Scan(&name); err != nil {
		t.Fatalf("the row did not survive the removal: %v", err)
	}
	if name != "Төв оффис" {
		t.Fatalf("the row came back as %q", name)
	}
}
