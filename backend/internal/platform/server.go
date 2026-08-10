/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Package platform provides the core HTTP Server orchestrator, routing table,
 * authentication middleware, and app installer wiring.
 */

package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/apps"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/ai"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/appcatalog"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/appinstaller"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/appregistry"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/cache"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/dan"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/eid"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/eidmongolia"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/emailverify"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/gerege"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/integration"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/memo"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/observability"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/rbac"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/resilience"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/ssoprovider"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"
)

// PlatformVersion is the semver the app-store manifests are validated against.
const PlatformVersion = "1.0.0"

type Server struct {
	db            *pgxpool.Pool
	installer     *appinstaller.AppInstaller
	router        *chi.Mux
	sessions      *auth.SessionStore
	loginLimiter  *security.IPRateLimiter
	pollLimiter   *security.IPRateLimiter
	aiLimiter     *security.IPRateLimiter
	verifyLimiter *security.IPRateLimiter
	// emailVerify is the shared "prove this address" service. It belongs to
	// every app module rather than to the platform's own handlers, so when a
	// module needs it, the accessor to add is one line — it is unexported now
	// only because no module has asked yet, and an exported accessor nobody
	// called read as a dependency the modules already had.
	emailVerify    *emailverify.Service
	copilotSvc     *ai.CopilotService
	forecaster     *ai.Forecaster
	eidSvc         *eid.EIDService
	danSvc         *dan.DANService
	ssoProvider    *ssoprovider.SSOProvider
	geregeSvc      *gerege.GeregeService
	integrationMgr *integration.Manager
	permissions    *rbac.SQLPermissionStore
	appGate        *memo.Cache[bool]
	bus            *cache.Bus
	sharedLogin    *security.SharedLimiter
	sharedPoll     *security.SharedLimiter
	sharedAI       *security.SharedLimiter
	sharedVerify   *security.SharedLimiter
	backgroundApps []apps.BackgroundModule
	eidMN          *eidmongolia.Service
}

// appGateTTL bounds how long the gate keeps believing an app is installed after
// somebody else's replica has uninstalled it. Installing is rare and deliberate,
// so this is about the replica that did not serve the button press.
const appGateTTL = 30 * time.Second

// appGateCacheName is what the invalidation bus knows the gate cache as.
const appGateCacheName = "appgate"

// forgetAppGate drops one tenant's cached installation answers, here and on
// every other replica.
func (s *Server) forgetAppGate(tenantID string) {
	s.bus.Invalidate(appGateCacheName, memo.Key(tenantID, ""))
}

// forgetGrants drops one tenant's cached permissions everywhere.
func (s *Server) forgetGrants(tenantID string) {
	s.bus.Invalidate(rbac.GrantCacheName, rbac.TenantPrefix(tenantID))
}

// NewServer builds the platform. bus may be a local-only one; nothing here
// requires Redis to be present.
func NewServer(db *pgxpool.Pool, catalogPath string, bus *cache.Bus) (*Server, error) {
	// #nosec G304 -- APP_CATALOG_PATH is deployment configuration read once at
	// startup. No request reaches this.
	catalogData, err := os.ReadFile(catalogPath)
	if err != nil {
		return nil, err
	}

	var rawCatalog []appcatalog.CatalogApp
	if err := json.Unmarshal(catalogData, &rawCatalog); err != nil {
		return nil, err
	}

	// Populate full manifests.
	//
	// A manifest that failed to load used to be replaced by a silent stub with
	// no dependencies, permissions or menus. Three shipped manifests were in
	// fact malformed (object instead of array for "dependencies", plain
	// strings instead of objects for "permissions") and nobody noticed: the
	// apps installed with an empty dependency graph and never contributed a
	// menu entry. Catalog integrity is now a startup error.
	catalogDir := filepath.Dir(catalogPath)
	catalog := make([]appcatalog.CatalogApp, 0, len(rawCatalog))
	for _, app := range rawCatalog {
		if !security.IsValidSlug(app.Slug) {
			return nil, fmt.Errorf("catalog app %q has an invalid slug %q", app.ID, app.Slug)
		}
		manifestPath := filepath.Join(catalogDir, "manifests", app.Slug+".json")
		manifest, err := appcatalog.LoadManifestFile(manifestPath, PlatformVersion)
		if err != nil {
			return nil, fmt.Errorf("load manifest for %s: %w", app.ID, err)
		}
		if manifest.ID != app.ID {
			return nil, fmt.Errorf("manifest %s declares id %q but the catalog entry is %q",
				manifestPath, manifest.ID, app.ID)
		}
		app.Manifest = manifest
		catalog = append(catalog, app)
	}

	installer := appinstaller.NewAppInstaller(db, catalog, PlatformVersion)

	// Keep the apps table in step with the catalog file. A missing row makes
	// installation fail on the app_installations foreign key, so this is a
	// startup concern, not a seeding concern. A cold database must not stop the
	// process from booting — /ready reports that separately.
	syncCtx, cancelSync := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelSync()
	if err := installer.SyncCatalog(syncCtx); err != nil {
		slog.Error("failed to sync app catalog into database", "error", err)
	}

	// Instantiate compile-time Go modules once. Each constructor registers the
	// module in the global app registry; calling them twice (here and again in
	// registerAppModuleRoutes) built two instances per app.
	// The integration manager is built before the modules that use it: esign
	// files finished documents through it and gov_services books meetings
	// through it, so it is a dependency of both rather than a peer.
	integrationMgr := integration.NewManager(db)

	eidMN, err := eidmongolia.New(db)
	if err != nil {
		return nil, fmt.Errorf("eID Mongolia service: %w", err)
	}

	ssoProvider := ssoprovider.NewSSOProvider(db)
	appRuntime := apps.Bootstrap(db, integrationMgr, eidMN, ssoProvider)

	s := &Server{
		db:           db,
		installer:    installer,
		router:       chi.NewRouter(),
		sessions:     auth.NewSessionStore(db, auth.DefaultSessionTTL),
		loginLimiter: newLoginLimiter(),
		pollLimiter:  newPollLimiter(),
		aiLimiter:    security.NewIPRateLimiter(rate.Limit(float64(aiRatePerMinute)/60.0), aiBurst),
		// Every send is a call to somebody else's service on a shared key, so
		// there is a cruder guard in front of the per-tenant allowance the
		// service itself applies: one per second sustained, twenty in a burst.
		verifyLimiter:  security.NewIPRateLimiter(rate.Limit(float64(verifyRatePerMinute)/60.0), verifyBurst),
		emailVerify:    emailverify.NewService(db),
		copilotSvc:     ai.NewCopilotService(db),
		forecaster:     ai.NewForecaster(db),
		eidSvc:         eid.NewEIDService(),
		danSvc:         dan.NewDANService(),
		ssoProvider:    ssoProvider,
		geregeSvc:      gerege.NewGeregeService(),
		integrationMgr: integrationMgr,
		permissions:    rbac.NewSQLPermissionStore(db),
		appGate:        memo.New[bool](appGateTTL),
		bus:            bus,
		backgroundApps: appRuntime.Background,
		eidMN:          eidMN,
	}

	// The authorization endpoint has to know who is signing in, which is the
	// platform session rather than anything OAuth owns.
	ssoProvider.AttachSessions(s.sessions)

	// Clients live in Postgres now, so the built-in one is registered once
	// rather than rebuilt into a map on every boot. A cold database must not
	// stop the process from starting: /ready reports that separately.
	ssoCtx, cancelSSO := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelSSO()
	ssoProvider.EnsureDefaultClient(ssoCtx)

	// Spent codes and dead tokens are reclaimed on a timer. The in-memory
	// token map this replaced had no eviction at all — it only ever grew.
	ssoProvider.StartJanitor(context.Background(), 15*time.Minute)

	// The bus has to know the caches before a message can arrive for one, and a
	// message can arrive as soon as the subscriber connects.
	s.bus.Register(rbac.GrantCacheName, rbac.GrantCache())
	s.bus.Register(appGateCacheName, s.appGate)

	// Deployment-wide budgets for the endpoints where a per-replica one is not
	// a budget at all. Each is nil without Redis, and a nil one allows.
	client := s.bus.Client()
	s.sharedLogin = security.NewSharedLimiter(client, "login", loginRatePerMinute, time.Minute)
	s.sharedPoll = security.NewSharedLimiter(client, "poll", pollRatePerMinute, time.Minute)
	s.sharedAI = security.NewSharedLimiter(client, "ai", aiRatePerMinute, time.Minute)
	s.sharedVerify = security.NewSharedLimiter(client, "verify", verifyRatePerMinute, time.Minute)

	s.setupRoutes()
	return s, nil
}

// StartBackgroundJobs launches the periodic work app modules need. It is
// separate from NewServer so a test can build a server without spawning
// goroutines, and it returns immediately — every job runs until ctx is
// cancelled at shutdown.
func (s *Server) StartBackgroundJobs(ctx context.Context) {
	for _, module := range s.backgroundApps {
		module.StartHousekeeping(ctx)
	}
	s.eidMN.StartHousekeeping(ctx)
	// Every sign-in writes a session row and nothing else ever removes one.
	s.sessions.StartHousekeeping(ctx)
	// Abandoned connect attempts and the delivery log are the two integration
	// tables that only ever grow.
	s.integrationMgr.StartHousekeeping(ctx)
	// Links nobody followed have to stop being reported as outstanding, and the
	// verification trail is an audit record with a retention window, not a
	// mailing list.
	s.emailVerify.StartHousekeeping(ctx)
}

func (s *Server) Router() *chi.Mux {
	return s.router
}

// InstallAppForTenant installs a catalogue app without a request behind it.
//
// It exists for the demo seeder, which needs the same dependency resolution and
// compiled-module check the store endpoint performs — writing app_installations
// rows directly would let a demo tenant claim an app whose Go module is not in
// the binary, and the shell would then render a menu leading nowhere.
func (s *Server) InstallAppForTenant(ctx context.Context, tenantID, appSlug, userID string) error {
	return s.installer.InstallApp(ctx, tenantID, appSlug, userID)
}

func (s *Server) setupRoutes() {
	r := s.router

	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(resilience.NewLoadShedder(1000).Middleware)
	r.Use(observability.MetricsMiddleware)
	r.Use(security.HeadersMiddleware)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   security.SafeCORSOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Accept-Language", "Authorization", "Content-Type", "X-Tenant-ID"},
		AllowCredentials: true,
	}))
	r.Use(security.CSRFMiddleware)

	// Infrastructure
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := s.db.Ping(r.Context()); err != nil {
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "message": "database unreachable"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})

	// Prometheus Metrics Endpoint
	r.Handle("/metrics", observability.MetricsHandler())

	// OpenID Connect Provider & OAuth2 Authorization Server.
	//
	// These sit at the root rather than under /api/, which is where the
	// specification puts them and what SSO_ISSUER advertises — the reverse
	// proxy has to route them to this service explicitly.
	r.Get("/.well-known/openid-configuration", s.ssoProvider.HandleOIDCDiscovery)
	r.Get("/.well-known/jwks.json", s.ssoProvider.HandleJWKS)
	// The authorization endpoint is a browser destination: it reads the
	// session cookie itself and answers with redirects, so it must not sit
	// behind the API's bearer-token middleware.
	r.Get("/oauth2/auth", s.ssoProvider.HandleAuthorize)
	r.Post("/oauth2/token", s.ssoProvider.HandleTokenEndpoint)
	r.Post("/oauth2/introspect", s.ssoProvider.HandleIntrospect)
	r.Post("/oauth2/revoke", s.ssoProvider.HandleRevoke)
	r.Get("/oauth2/userinfo", s.ssoProvider.HandleUserInfo)
	r.Post("/oauth2/userinfo", s.ssoProvider.HandleUserInfo)

	// Platform API
	r.Route("/api/v1", func(api chi.Router) {
		// Auth with rate limiting
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/login", s.handleLogin)
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/eid/login", s.handleEIDLogin)
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/eid/start", s.handleEIDStart)
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/eid/start-id", s.handleEIDStartByNationalID)
		// Not the login limiter: a citizen polls for as long as it takes them to
		// reach their phone, and sharing that budget with sign-in attempts made
		// a busy office throttle itself out of signing in at all.
		api.With(security.SharedRateLimitMiddleware(s.pollLimiter, s.sharedPoll)).Post("/auth/eid/poll", s.handleEIDPoll)
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/dan/login", s.handleDANLogin)
		api.Post("/auth/logout", s.handleLogout)

		// The OAuth redirect a connected provider sends the browser back to.
		// Unauthenticated on purpose — see handleIntegrationOAuthCallback: the
		// single-use state row is what carries the authority here, because a
		// cross-site redirect from Google cannot be relied on to still present
		// a SameSite=Strict session cookie.
		api.Get("/integrations/oauth/callback", s.handleIntegrationOAuthCallback)

		// Where the verification service returns somebody who has just proved
		// an address. Unauthenticated on purpose: they have not signed in, and
		// may have no account here at all. The single-use reference in the
		// query is the whole authority — see handleVerifyLanded.
		api.Get("/verify/landed", s.handleVerifyLanded)

		// Protected endpoints
		api.Group(func(pr chi.Router) {
			pr.Use(s.authMiddleware)

			pr.Get("/auth/me", s.handleMe)
			// Ending the session in front of you needs no proof beyond the
			// cookie; ending the ones you cannot see is a decision about an
			// account, so it sits behind authentication with the rest.
			pr.Post("/auth/logout-all", s.handleLogoutEverywhere)
			// Which organisations this person may act for, and moving the
			// session to one of them. Both cross tenants by definition, so
			// they run on the platform path — see handleTenants.
			pr.Get("/auth/tenants", s.handleTenants)
			pr.Post("/auth/switch-tenant", s.handleSwitchTenant)
			pr.Get("/menus", s.handleMenus)

			// Consent screen. The browser endpoint at /oauth2/auth redirects
			// here; these two describe the pending grant and record the
			// answer. Both re-validate the request against the database, so
			// the frontend is a renderer rather than a source of truth.
			pr.Get("/oauth2/consent", s.ssoProvider.HandleConsentPrompt)
			pr.Post("/oauth2/consent", s.ssoProvider.HandleConsentDecision)

			// Tenant access control. Mutations are deliberately admin-only;
			// authorization configuration can otherwise be used to self-elevate.
			pr.Route("/admin/access", func(ac chi.Router) {
				ac.Use(s.requireAdmin)
				ac.Get("/overview", s.handleAccessOverview)
				ac.Post("/roles", s.handleCreateRole)
				ac.Put("/roles/{id}", s.handleUpdateRole)
				ac.Delete("/roles/{id}", s.handleDeleteRole)
				ac.Put("/roles/{id}/permissions", s.handleSetRolePermissions)
				ac.Put("/memberships/{id}/roles", s.handleSetMembershipRoles)
			})

			// Asking the verification service to write to an address spends a
			// credential the whole platform shares, so it is a signed-in act.
			// App modules do not come through here — they hold the service and
			// call it in process.
			pr.With(security.SharedRateLimitMiddleware(s.verifyLimiter, s.sharedVerify)).Post("/verify/send", s.handleVerifySend)

			// Who has been written to is an administrative read: it is a list
			// of people's addresses and what they were asked to prove.
			pr.With(s.requireAdmin).Get("/admin/email-verification/overview", s.handleEmailVerifyOverview)

			// AI Copilot & Forecasting
			pr.With(security.SharedRateLimitMiddleware(s.aiLimiter, s.sharedAI)).Post("/ai/copilot", s.handleAICopilot)
			pr.With(security.SharedRateLimitMiddleware(s.aiLimiter, s.sharedAI)).Post("/ai/chat", s.handleAIChat)
			pr.With(security.SharedRateLimitMiddleware(s.aiLimiter, s.sharedAI)).Post("/ai/stt", s.handleAISTT)
			pr.With(security.SharedRateLimitMiddleware(s.aiLimiter, s.sharedAI)).Post("/ai/tts", s.handleAITTS)
			pr.With(security.SharedRateLimitMiddleware(s.aiLimiter, s.sharedAI)).Post("/ai/translate", s.handleAITranslate)
			pr.Get("/ai/stock-forecast", s.handleAIForecast)
			pr.With(s.requireAdmin).Get("/admin/ai/prompts", s.handleAIListPrompts)
			pr.With(s.requireAdmin).Put("/admin/ai/prompts/{key}", s.handleAIUpdatePrompt)
			pr.With(s.requireAdmin).Get("/admin/ai/knowledge", s.handleAIListKnowledge)
			pr.With(s.requireAdmin).Post("/admin/ai/knowledge", s.handleAICreateKnowledge)

			// XYP State Information Exchange System (xyp.gerege.mn)
			// XYP responses contain authoritative citizen/company data. Merely
			// belonging to a tenant is not enough authority to query that data.
			pr.With(rbac.RequirePermission(s.permissions, "xyp.citizen.read")).Post("/xyp/citizen", s.handleXYPCitizenQuery)
			pr.With(rbac.RequirePermission(s.permissions, "xyp.company.read")).Post("/xyp/company", s.handleXYPCompanyQuery)

			// External Integrations Manager (admin-only: a connector target URL
			// makes the server issue arbitrary outbound requests, and an OAuth
			// grant hands the platform an account outside it)
			pr.Route("/integrations", func(ir chi.Router) {
				ir.Use(s.requireAdmin)
				ir.Get("/", s.handleListIntegrations)
				ir.Post("/", s.handleRegisterIntegration)
				ir.Get("/providers", s.handleIntegrationProviders)
				ir.Get("/deliveries", s.handleIntegrationDeliveries)
				ir.Put("/{id}", s.handleUpdateIntegration)
				ir.Delete("/{id}", s.handleDeleteIntegration)
				ir.Post("/{id}/connect", s.handleConnectIntegration)
				ir.Post("/{id}/disconnect", s.handleDisconnectIntegration)
			})

			// Store — reads are open to any tenant member, mutations are
			// tenant-administrator only. Previously every authenticated user
			// could install, enable or disable apps for the whole tenant.
			pr.Get("/store/apps", s.handleListStoreApps)
			pr.Get("/store/apps/{slug}", s.handleGetStoreApp)
			pr.Get("/installed-apps", s.handleListInstalledApps)

			pr.Group(func(ar chi.Router) {
				ar.Use(s.requireAdmin)
				ar.Post("/store/apps/{slug}/install", s.handleInstallApp)
				ar.Post("/store/apps/{slug}/enable", s.handleEnableApp)
				ar.Post("/store/apps/{slug}/disable", s.handleDisableApp)
			})
		})
	})

	// Register compile-time Business App Routes with Tenant & App Gate protection
	s.registerAppModuleRoutes()
}

// registerAppModuleRoutes mounts every compile-time business module behind the
// tenant app gate. Billing, Documents and the Developer Portal used to be wired
// straight into the protected group, so their endpoints stayed reachable for
// tenants that had never installed the app.
func (s *Server) registerAppModuleRoutes() {
	for _, module := range appregistry.List() {
		module.RegisterRoutes(s.router, s.appGateMiddleware(module.ID()))
	}
}

// Handlers
