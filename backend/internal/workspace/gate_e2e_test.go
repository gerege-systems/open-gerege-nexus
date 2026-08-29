/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The app gate, tested through the app that made it worth testing.
 *
 * Every compiled module is mounted behind GateMiddleware, and until now
 * nothing asserted what it does — the gate was one line in registerAppModuleRoutes
 * and the closest thing to a test was the route-policy sweep, which only asks
 * whether a stranger is refused. The e-Government app is the one where the
 * answer matters most: its endpoints read the national registry, they used to
 * be platform routes reachable by any tenant holding the permission, and the
 * move behind the gate is the thing that changed.
 *
 *	AUTH_TEST_DATABASE_URL=postgres://... go test ./internal/operator/...
 */

package workspace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/cache"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/flags"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/access"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type gateFixture struct {
	server *Service
	// The plane's own routes, without the process-wide middleware the seam
	// adds. Every request below is a GET, so the chain in pkg/host would
	// change nothing about what is being asserted here — which is which
	// requests the app gate lets through.
	router       chi.Router
	pool         *pgxpool.Pool
	tenantID     string
	userID       string
	membershipID string
	token        string
}

func newGateFixture(t *testing.T) *gateFixture {
	t.Helper()
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to a migrated test database to run the app gate tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	t.Setenv("APP_CATALOG_URL", "")
	server, err := New(Deps{
		DB: pool, Bus: cache.NewBus(ctx, nil),
		Settings:    settings.NewStore(pool),
		Flags:       flags.NewStore(pool),
		CatalogPath: filepath.FromSlash("../../../catalog/apps.json"),
	})
	if err != nil {
		t.Fatalf("build the server: %v", err)
	}

	f := &gateFixture{server: server, pool: pool}
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.tenants (slug, name) VALUES ('gate-' || substr(gen_random_uuid()::text, 1, 8), 'Gate test')
		 RETURNING id::text`).Scan(&f.tenantID); err != nil {
		t.Fatalf("tenant: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1`, f.tenantID)
	})

	// An administrator, so nothing below is refused for want of a permission —
	// what is under test is the installation check, and a 403 that could be
	// either would prove nothing.
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name, is_admin)
		 VALUES ('gate-' || substr(gen_random_uuid()::text, 1, 8) || '@example.com', 'x', 'Gate Admin', TRUE)
		 RETURNING id::text`).Scan(&f.userID); err != nil {
		t.Fatalf("user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id = $1`, f.userID) })

	if err := pool.QueryRow(ctx,
		`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1, $2) RETURNING id::text`,
		f.tenantID, f.userID).Scan(&f.membershipID); err != nil {
		t.Fatalf("membership: %v", err)
	}

	// `is_admin` on the session is the tenant's admin role, not the global flag
	// on the user row — see SessionStore.Resolve. Without the role the caller is
	// an ordinary member and every assertion below would be reading a permission
	// refusal as an app-gate refusal.
	if _, err := pool.Exec(ctx,
		`WITH r AS (
		     INSERT INTO workspace.roles (tenant_id, code, name) VALUES ($1, 'admin', 'Administrator')
		     ON CONFLICT (tenant_id, code) DO UPDATE SET active = TRUE
		     RETURNING id
		 )
		 INSERT INTO workspace.membership_roles (membership_id, role_id)
		 SELECT $2::uuid, r.id FROM r ON CONFLICT DO NOTHING`,
		f.tenantID, f.membershipID); err != nil {
		t.Fatalf("admin role: %v", err)
	}

	token, _, err := server.sessions.Create(ctx, f.userID, f.tenantID, "password", "go-test", "127.0.0.1")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	f.token = token
	f.router = chi.NewRouter()
	server.Routes(f.router)
	return f
}

func (f *gateFixture) do(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: f.token})
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

// install writes the installation row directly.
//
// Not through the installer: that grants permissions and resolves dependencies,
// which is a different mechanism with its own tests, and here it would put
// three more moving parts between the assertion and the thing being asserted.
func (f *gateFixture) install(t *testing.T, appID string) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(),
		`INSERT INTO workspace.app_installations (tenant_id, app_id, installed_version, status, enabled)
		 VALUES ($1, $2, '1.0.0', 'installed', TRUE)
		 ON CONFLICT (tenant_id, app_id) DO UPDATE SET enabled = TRUE, status = 'installed'`,
		f.tenantID, appID); err != nil {
		t.Fatalf("install %s: %v", appID, err)
	}
	// The gate caches its answer for thirty seconds, which is longer than this
	// test takes.
	f.server.appinstall.ForgetGate(f.tenantID)
}

// An app's routes are behind its installation, and another app's are not.
//
// Both halves are the point. The first is what the app gate is for: a tenant
// that has not installed an app must be refused its endpoints, whatever
// permission the caller holds. The second is what makes the gate safe — one
// app being absent must not make another unusable.
//
// This was written about the e-Government link, whose routes had been platform
// routes before it became an app. That module moved to client-gerege-nexus on
// 2026-08-23; documents, the organisation, Өртөө's task board and the reports
// app followed it the same day, and sso_clients left for the App Store on
// 2026-08-25. The claim has now outlived every one of its subjects, because
// this repository carries no app at all — which is the argument for keeping it,
// and the reason it now brings its own.
//
// The probe below is a module in the same sense any distribution's is: it
// registers with nexus, it is mounted by registerAppModuleRoutes, and the gate
// cannot tell it from the real thing. Asserting the gate through an app that
// happened to still be here was always incidental; a real one is what is no
// longer available.
type gateProbeModule struct{ id, prefix string }

func (m gateProbeModule) ID() string                    { return m.id }
func (gateProbeModule) Name() string                    { return "Gate probe" }
func (gateProbeModule) Version() string                 { return "1.0.0" }
func (gateProbeModule) MenuPermission() string          { return "" }
func (m gateProbeModule) RoutePermissionPrefix() string { return "" }

func (gateProbeModule) Dependencies() []nexus.Dependency          { return nil }
func (gateProbeModule) Permissions() []nexus.PermissionDefinition { return nil }
func (gateProbeModule) Menus() []nexus.MenuDefinition             { return nil }

func (m gateProbeModule) RegisterRoutes(r chi.Router, gate func(http.Handler) http.Handler) {
	r.Route("/api/v1/"+m.prefix, func(pr chi.Router) {
		pr.Use(gate)
		pr.Get("/probe", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	})
}

func TestAnAppsRoutesAreBehindItsInstallationAndAnothersAreNot(t *testing.T) {
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	appID := "io.gerege.test.gate." + suffix
	slug := "gate-" + suffix
	// Registered before the fixture, because the fixture is what builds the
	// router: a module registered after it would never be mounted.
	nexus.Register(gateProbeModule{id: appID, prefix: slug})

	f := newGateFixture(t)
	if _, err := f.pool.Exec(context.Background(),
		`INSERT INTO registry.apps (id, slug, name) VALUES ($1, $2, 'Gate probe')`,
		appID, slug); err != nil {
		t.Fatalf("app: %v", err)
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM registry.apps WHERE id = $1`, appID)
	})

	// Nothing installed yet: the app's routes are refused.
	if res := f.do(t, http.MethodGet, "/api/v1/"+slug+"/probe", ""); res.Code != http.StatusForbidden {
		t.Fatalf("the app answered %d without being installed; expected 403: %s",
			res.Code, res.Body.String())
	}

	// Platform routes are unaffected by whether optional apps are installed.
	// Any core read does; this one used to be the assistant's prompts, which
	// moved to the console with the screen that edited them.
	res := f.do(t, http.MethodGet, "/api/v1/installed-apps", "")
	if res.Code != http.StatusOK {
		t.Fatalf("a platform core route answered %d: %s", res.Code, res.Body.String())
	}

	// And with the app installed the gate opens.
	f.install(t, appID)
	res = f.do(t, http.MethodGet, "/api/v1/"+slug+"/probe", "")
	if res.Code == http.StatusForbidden {
		t.Fatalf("the app was refused after installation: %s", res.Body.String())
	}
	if res.Code == http.StatusNotFound {
		t.Fatalf("the app answered 404; this test asserts nothing unless the route is served")
	}
}

// A data-path request must keep serving the last known control decisions when
// their source tables cannot be read. Each cache is warmed first; the locks
// below then turn an accidental query into a deadline rather than allowing a
// fast database to hide it.
func TestARequestUsesCachedControlDecisionsWhenTheirTablesAreUnavailable(t *testing.T) {
	f := newGateFixture(t)
	ctx := context.Background()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	appID := "io.gerege.test.dependency." + suffix
	permission := "dependency." + suffix + ".read"
	settingKey := "dependency.test." + suffix
	roleCode := "dependency_" + suffix

	settings.Register(settings.Spec{
		Key: settingKey, Kind: settings.KindString, Default: "default",
		Description: "test-only control/data dependency probe",
	})
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO registry.platform_settings (key, value) VALUES ($1, 'cached')`, settingKey); err != nil {
		t.Fatalf("setting: %v", err)
	}
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO registry.feature_flags (key, description, owner, kind, enabled, rollout)
		 VALUES ($1, 'test-only control/data dependency probe', 'test', 'kill_switch', FALSE, 100)`,
		flags.ModuleKillSwitch(appID)); err != nil {
		t.Fatalf("flag: %v", err)
	}
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO registry.apps (id, slug, name) VALUES ($1, $2, 'Dependency probe')`,
		appID, "dependency-"+suffix); err != nil {
		t.Fatalf("app: %v", err)
	}
	f.install(t, appID)

	// This user was an administrator only to keep the older gate fixture
	// focused. Make it an ordinary member here so RequirePermission must use
	// the memoized grant rather than taking the administrator shortcut.
	if _, err := f.pool.Exec(ctx,
		`DELETE FROM workspace.membership_roles WHERE membership_id = $1`, f.membershipID); err != nil {
		t.Fatalf("remove fixture role: %v", err)
	}
	var roleID string
	if err := f.pool.QueryRow(ctx,
		`INSERT INTO workspace.roles (tenant_id, code, name) VALUES ($1, $2, 'Dependency probe') RETURNING id::text`,
		f.tenantID, roleCode).Scan(&roleID); err != nil {
		t.Fatalf("role: %v", err)
	}
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO registry.permissions (code, name) VALUES ($1, 'Dependency probe')`, permission); err != nil {
		t.Fatalf("permission: %v", err)
	}
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO workspace.membership_roles (membership_id, role_id) VALUES ($1, $2)`,
		f.membershipID, roleID); err != nil {
		t.Fatalf("membership role: %v", err)
	}
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO workspace.role_permissions (role_id, permission_id)
		 SELECT $1, id FROM registry.permissions WHERE code = $2`, roleID, permission); err != nil {
		t.Fatalf("role permission: %v", err)
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM workspace.roles WHERE id = $1`, roleID)
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM registry.permissions WHERE code = $1`, permission)
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM registry.feature_flags WHERE key = $1`, flags.ModuleKillSwitch(appID))
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM registry.platform_settings WHERE key = $1`, settingKey)
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM registry.apps WHERE id = $1`, appID)
	})

	oldSettings, oldFlags := settings.Default, flags.Default
	settings.UseStore(f.server.settings)
	flags.UseStore(f.server.featureFlags)
	t.Cleanup(func() {
		settings.UseStore(oldSettings)
		flags.UseStore(oldFlags)
	})
	if err := f.server.settings.Load(ctx); err != nil {
		t.Fatalf("warm settings: %v", err)
	}
	if err := f.server.featureFlags.Load(ctx); err != nil {
		t.Fatalf("warm flags: %v", err)
	}
	if suspended, _ := f.server.authn.TenantSuspended(ctx, f.tenantID); suspended {
		t.Fatal("fixture tenant is suspended")
	}
	if installed, err := f.server.appinstall.AppInstalled(ctx, f.tenantID, appID); err != nil || !installed {
		t.Fatalf("warm app gate: installed=%v error=%v", installed, err)
	}
	access.InvalidateTenant(f.tenantID)
	if permissions, err := f.server.permissions.GetUserPermissions(ctx, f.tenantID, f.userID); err != nil || !permissions[permission] {
		t.Fatalf("warm permission cache: permissions=%v error=%v", permissions, err)
	}

	blocker, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin blocker: %v", err)
	}
	t.Cleanup(func() { _ = blocker.Rollback(context.Background()) })
	if _, err := blocker.Exec(ctx, `LOCK TABLE
		registry.platform_settings,
		registry.feature_flags,
		registry.feature_flag_overrides,
		registry.tenants,
		workspace.app_installations,
		workspace.role_permissions,
		registry.permissions
		IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatalf("lock control tables: %v", err)
	}

	// Prove the tables really are unavailable from the pool used by handlers.
	probeCtx, cancelProbe := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancelProbe()
	if _, err := f.pool.Exec(probeCtx, `SELECT 1 FROM registry.platform_settings LIMIT 1`); err == nil {
		t.Fatal("control table remained readable while the blocker held its lock")
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := settings.Get(settingKey); got != "cached" {
			t.Errorf("cached setting = %q, want cached", got)
		}
		if flags.Enabled(r.Context(), flags.ModuleKillSwitch(appID)) {
			t.Error("cached kill switch unexpectedly enabled the refusal")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	permissionGate := nexus.RequirePermission(f.server.permissions, permission)(handler)
	requestGate := f.server.appinstall.GateMiddleware(appID)(permissionGate)

	reqCtx, cancelRequest := context.WithTimeout(ctx, 2*time.Second)
	defer cancelRequest()
	req := httptest.NewRequest(http.MethodGet, "/dependency-probe", nil).WithContext(reqCtx)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: f.token})
	rec := httptest.NewRecorder()
	requestGate.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("cached request answered %d, want 204: %s", rec.Code, rec.Body.String())
	}
}
