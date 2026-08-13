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
	"sync"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/apps"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/ai"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/appcatalog"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/appinstaller"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/appregistry"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/async"
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
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/ssoclient"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/ssoprovider"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"
)

// PlatformVersion is the semver the app-store manifests are validated against.
//
// It is a var rather than a const so a release build can stamp the version it
// actually is:
//
//	go build -ldflags "-X github.com/gerege-systems/open-gerege-nexus/backend/internal/platform.PlatformVersion=1.2.0"
//
// A manifest names the platform it needs (`"platform": ">=1.1.0"`), and a store
// that separates from this binary has to be told which platform is asking. A
// constant would have every deployment claim 1.0.0 for ever, so every app would
// look compatible with every instance. The default stays 1.0.0, which is what an
// unstamped build has always reported; whatever is injected must be valid
// semver, because manifest validation parses it.
var PlatformVersion = "1.0.0"

type Server struct {
	db        *pgxpool.Pool
	installer *appinstaller.AppInstaller
	// catalogSource is where the catalogue came from and where a refresh goes.
	// In file mode it is the bundled file and nothing else; with a registry
	// configured it is that registry, its disk cache and the file behind them.
	catalogSource *appcatalog.Provider
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
	emailVerify *emailverify.Service
	copilotSvc  *ai.CopilotService
	forecaster  *ai.Forecaster
	eidSvc      *eid.EIDService
	danSvc      *dan.DANService
	ssoProvider *ssoprovider.SSOProvider
	// ssoClient is the other half: the provider this deployment signs its own
	// people in through, when it has one. Nil means it authenticates them
	// itself, which is every deployment that has not set SSO_CLIENT_ISSUER.
	ssoClient *ssoclient.Client
	// googleLogin is a button on this platform's own sign-in screen rather than
	// a replacement for it — see google_login_handlers.go for why the two are
	// separate despite sharing every line of protocol.
	googleLogin    *ssoclient.Client
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

	// The last thing the catalogue sync did. An administrator pressing "check
	// for updates" gets an answer; the hourly one leaves only a log line, and a
	// registry that has been failing for a week is exactly the thing nobody
	// notices. See handleCatalogStatus.
	syncMu      sync.RWMutex
	lastSyncAt  time.Time
	lastSyncOK  bool
	lastSyncErr string
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
	// The server is not built yet, so the gate is handed over as a closure over
	// the pointer that is about to be filled. Reports are listed per request,
	// long after this line.
	var server *Server
	appRuntime := apps.Bootstrap(db, integrationMgr, eidMN, ssoProvider,
		func(ctx context.Context, tenantID string) (map[string]bool, error) {
			return server.installedAppSet(ctx, tenantID)
		})

	// The relying-party half. A deployment that names a provider but cannot
	// reach it is a deployment nobody can sign in to, so a configuration that
	// cannot work is a startup failure rather than a surprise at the first
	// sign-in — but the provider being *reachable* is not checked here, because
	// that is a running condition and not a configuration one.
	ssoClientConfig := ssoclient.ConfigFromEnv()
	if err := ssoClientConfig.Validate(); err != nil {
		return nil, fmt.Errorf("SSO client configuration: %w", err)
	}
	var federatedSignIn *ssoclient.Client
	if ssoClientConfig.Enabled() {
		federatedSignIn = ssoclient.New(ssoClientConfig)
		slog.Info("this deployment signs people in through an SSO provider",
			"issuer", ssoClientConfig.Issuer,
			"client_id", ssoClientConfig.ClientID,
			"redirect_uri", ssoClientConfig.RedirectURI,
			"local_login", ssoClientConfig.LocalLogin)
	}

	// Modules first, catalogue second: the catalogue is held against the module
	// registry as it is loaded, and Bootstrap is what fills that registry.
	catalogConfig := appcatalog.ConfigFromEnv(catalogPath, PlatformVersion)
	catalogConfig.Verify = verifyCatalogVersions
	catalogSource := appcatalog.NewProvider(catalogConfig)

	loadCtx, cancelLoad := context.WithTimeout(context.Background(), catalogLoadTimeout)
	defer cancelLoad()
	catalog, err := catalogSource.Load(loadCtx)
	if err != nil {
		return nil, err
	}
	if catalogSource.Remote() {
		// Configured, not necessarily reached: Load says in its own log line
		// which source actually answered, and boot carries on either way.
		slog.Info("app catalog registry is configured",
			"apps", len(catalog), "sync_interval", catalogSource.SyncInterval().String())
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

	// Google sign-in. Off unless GOOGLE_LOGIN_CLIENT_ID is set, and validated
	// here for the same reason the federation is: a button that cannot work
	// should fail at startup, not under somebody's finger.
	googleConfig := ssoclient.GoogleConfigFromEnv()
	if err := googleConfig.Validate(); err != nil {
		return nil, fmt.Errorf("google sign-in configuration: %w", err)
	}
	var googleLogin *ssoclient.Client
	if googleConfig.Enabled() {
		googleLogin = ssoclient.New(googleConfig)
		slog.Info("Google sign-in is enabled",
			"client_id", googleConfig.ClientID,
			"redirect_uri", googleConfig.RedirectURI,
			"provisioning_tenant", googleConfig.TenantSlug,
			"allowed_domains", ssoclient.GoogleAllowedDomains())
	}

	s := &Server{
		db:            db,
		installer:     installer,
		catalogSource: catalogSource,
		router:        chi.NewRouter(),
		sessions:      auth.NewSessionStore(db, auth.DefaultSessionTTL),
		loginLimiter:  newLoginLimiter(),
		pollLimiter:   newPollLimiter(),
		aiLimiter:     security.NewIPRateLimiter(rate.Limit(float64(aiRatePerMinute)/60.0), aiBurst),
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
		ssoClient:      federatedSignIn,
		googleLogin:    googleLogin,
		geregeSvc:      gerege.NewGeregeService(),
		integrationMgr: integrationMgr,
		permissions:    rbac.NewSQLPermissionStore(db),
		appGate:        memo.New[bool](appGateTTL),
		bus:            bus,
		backgroundApps: appRuntime.Background,
		eidMN:          eidMN,
	}

	// And now the closure above has something to call.
	server = s

	// The authorization endpoint has to know who is signing in, which is the
	// platform session rather than anything OAuth owns.
	ssoProvider.AttachSessions(s.sessions)

	// And how to end one, for a relying party that sends somebody here to sign
	// out. Without this the logout endpoint could only clear the cookie in
	// front of it and would leave the session it names alive.
	ssoProvider.AttachSessionEnder(s.sessions)

	// And whether the organisation they are signing in for has installed the
	// app behind the client. For an external app that is the only gate there
	// is: nothing of it runs here for appGateMiddleware to stand in front of.
	ssoProvider.AttachInstallGate(s.newExternalAppGate())

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

// catalogLoadTimeout bounds the catalogue fetch that boot waits on. A registry
// that is merely slow must not hold a deployment open; the fallbacks are there
// precisely so this can give up.
const catalogLoadTimeout = 20 * time.Second

// verifyCatalogVersions holds a catalogue against the modules compiled into this
// binary.
//
// The two drifted apart unnoticed — esign shipped 2.0.0 as a module and 1.0.0 in
// the catalogue, and the developer portal did the same — because nothing ever
// compared them. Once a registry outside this repository publishes versions, a
// number the store advertises but the binary does not have is an upgrade that
// silently does nothing.
//
// It runs against every candidate catalogue, so what it means depends on where
// the catalogue came from: the bundled file failing it is a startup error, the
// same way its manifests failing validation is, while a registry answer failing
// it is discarded in favour of the cache or the file. The catalogue/manifest
// half of the comparison lives in appcatalog.ValidateCatalog, which every source
// goes through.
//
// An app with no compiled module is not an error. External apps have none by
// definition.
func verifyCatalogVersions(catalog []appcatalog.CatalogApp) error {
	present := make(map[string]bool, len(catalog))
	for _, app := range catalog {
		present[app.ID] = true
		mod, ok := appregistry.Get(app.ID)
		if !ok {
			continue
		}
		if mod.Version() != app.Version {
			return fmt.Errorf("module %s is compiled at version %q but the catalog declares %q",
				app.ID, mod.Version(), app.Version)
		}
	}

	// A catalogue without the platform's own apps is not a catalogue this build
	// should run on — it is one that predates it. The version check above
	// cannot see that: it skips ids with no compiled module, and after a rename
	// every renamed app looks exactly like a third party's, so an entire stale
	// catalogue passes without a word.
	//
	// That is not hypothetical. A cache written before the ids were renamed was
	// accepted whole, the core app was absent from it, no tenant got the
	// screens, and every app in the store offered an install that failed on a
	// foreign key. The bundled file always matches the binary, so refusing here
	// is what reaches it.
	for _, appID := range appinstaller.CoreApps {
		if !present[appID] {
			return fmt.Errorf("catalog does not carry the platform's own app %s", appID)
		}
	}
	return nil
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
	// Abandoned identity bindings hold verified claims about somebody, so they
	// do not get to sit in the table after they stop being redeemable.
	async.Go("identity-binding-sweep", func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			s.sweepExpiredBindings(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
	// Abandoned connect attempts and the delivery log are the two integration
	// tables that only ever grow.
	s.integrationMgr.StartHousekeeping(ctx)
	// Links nobody followed have to stop being reported as outstanding, and the
	// verification trail is an audit record with a retention window, not a
	// mailing list.
	s.emailVerify.StartHousekeeping(ctx)
	// Only with a registry configured; in file mode the catalogue changes when
	// the release does and there is nothing to poll.
	s.startCatalogSync(ctx)

	// The publishing console's OAuth2 client, when a deployment has one.
	//
	// Here rather than in NewServer because it needs its owning tenant to
	// exist, and on a cold database that tenant is created by the seeder — which
	// runs after the server is built and before this. Registering it from
	// configuration at all is the same argument as for the built-in client: a
	// client a console cannot work without should not depend on somebody
	// remembering to create it, and a redirect URI typed into a form is a
	// redirect URI that can be typed wrongly.
	ensureCtx, cancelEnsure := context.WithTimeout(ctx, 10*time.Second)
	defer cancelEnsure()
	s.ssoProvider.EnsureConsoleClient(ensureCtx)

	// And the catalogue in hand is applied once at startup, in either mode. A
	// release that carries a new module version is a catalogue change as much
	// as a publication is, and a file-mode instance only ever sees one at a
	// deploy — where nothing else would notice it.
	sweepCtx, cancelSweep := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelSweep()
	s.applyCatalogToInstallations(sweepCtx)
}

// startCatalogSync keeps this instance's catalogue in step with the registry.
//
// It is a poll rather than a push because an instance may be behind anything —
// a firewall, a home connection, an air gap that was opened for an hour — and
// the registry knowing how to reach every one of them is a coupling this
// architecture spent its effort avoiding. A failed round is a warning: the
// catalogue in hand keeps serving, and installed apps do not depend on this at
// all.
func (s *Server) startCatalogSync(ctx context.Context) {
	if !s.catalogSource.Remote() {
		return
	}
	interval := s.catalogSource.SyncInterval()
	async.Go("catalog-sync", func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				syncCtx, cancel := context.WithTimeout(ctx, catalogLoadTimeout)
				changed, err := s.syncCatalogFromRegistry(syncCtx)
				cancel()
				switch {
				case err != nil:
					slog.Warn("catalog: registry sync failed; keeping the catalogue in hand", "error", err)
				case changed:
					slog.Info("catalog: updated from the registry", "apps", len(s.installer.GetCatalog()))
				}
			}
		}
	})
}

// recordSync remembers how the last attempt went.
func (s *Server) recordSync(err error) {
	s.syncMu.Lock()
	s.lastSyncAt, s.lastSyncOK = time.Now(), err == nil
	s.lastSyncErr = ""
	if err != nil {
		s.lastSyncErr = err.Error()
	}
	s.syncMu.Unlock()
}

// syncCatalogFromRegistry fetches, accepts and publishes a new catalogue.
//
// The order matters: the apps table has to carry a row before an installation
// can reference it, and every replica's app gate has to stop answering from a
// catalogue that no longer exists. The gate is dropped for every tenant rather
// than one, because a catalogue change is not a tenant's act.
func (s *Server) syncCatalogFromRegistry(ctx context.Context) (changed bool, err error) {
	defer func() { s.recordSync(err) }()

	catalog, changed, err := s.catalogSource.Refresh(ctx)
	if err != nil {
		return false, err
	}

	if changed {
		s.installer.SetCatalog(catalog)
		if err := s.installer.SyncCatalog(ctx); err != nil {
			return true, fmt.Errorf("sync the new catalogue into the database: %w", err)
		}
		s.bus.Invalidate(appGateCacheName, "")
	}

	// After every sync, not only after a change.
	//
	// A catalogue that has not moved can still be ahead of an installation —
	// one made before the version was published, or one whose upgrade failed
	// the first time — and an instance that only ever swept on change would
	// leave those behind for ever, with an update the store offers and nothing
	// takes. The sweep is a no-op where there is nothing to do.
	s.applyCatalogToInstallations(ctx)
	return changed, nil
}

// applyCatalogToInstallations carries tenants forward to the catalogue in hand.
//
// Its failures are logged rather than returned: this runs on a timer and at
// startup, where there is nobody to hand an error to, and an installation left
// where it is is the safe outcome — the store still offers the update.
func (s *Server) applyCatalogToInstallations(ctx context.Context) {
	// The platform's own apps first: a tenant without them has no organisation
	// screen, and no way to install one — the store is behind the app that is
	// missing.
	if err := s.installer.EnsureCoreApps(ctx); err != nil {
		slog.Error("catalog: could not install the core apps", "error", err)
	}

	swept, err := s.installer.AutoUpdate(ctx)
	if err != nil {
		slog.Error("catalog: could not apply the catalogue to installations", "error", err)
		return
	}
	if len(swept.Upgraded) == 0 && len(swept.Held) == 0 {
		return
	}
	slog.Info("catalog: installations followed the catalogue",
		"upgraded", len(swept.Upgraded), "held_for_approval", len(swept.Held))
	// Their menus and gates were decided by the version that just moved.
	s.bus.Invalidate(appGateCacheName, "")
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

	// First, so everything below it — the access log, every slog line a handler
	// writes, and the X-Request-Id header the caller gets back — names the same
	// request. It is also what makes a log line joinable to a trace once
	// tracing is on.
	r.Use(chimiddleware.RequestID)
	// Before the logger, so a log line can name the trace it belongs to. It is
	// a no-op wrapper when OTEL_EXPORTER_OTLP_ENDPOINT is unset.
	r.Use(observability.TracingMiddleware)
	r.Use(observability.RequestLogger)
	// Not chi's Recoverer: that one prints a stack trace to stdout and nothing
	// else. This one logs it with the request id and the tenant, and reports it
	// to GlitchTip when SENTRY_DSN is set.
	r.Use(observability.RecoveryMiddleware)
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
		// The platform version is part of the health answer because it is what
		// an app store has to know about this instance: which manifests apply
		// to it, and whether an operator's rollout actually landed.
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "platform_version": PlatformVersion})
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
	// RP-initiated logout, which the discovery document has advertised since it
	// was written. Like the authorization endpoint it is a browser destination:
	// it reads the session cookie and answers with a redirect.
	r.Get("/oauth2/logout", s.ssoProvider.HandleEndSession)
	r.Post("/oauth2/logout", s.ssoProvider.HandleEndSession)

	// Platform API
	r.Route("/api/v1", func(api chi.Router) {
		// Auth with rate limiting
		// Every path by which this deployment establishes an identity of its
		// own. On a deployment that federates, requireLocalLogin closes all of
		// them and says where sign-in actually happens — see
		// sso_client_handlers.go for why that is all or nothing.
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/login", s.requireLocalLogin(s.handleLogin))
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/eid/login", s.requireLocalLogin(s.handleEIDLogin))
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/eid/start", s.requireLocalLogin(s.handleEIDStart))
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/eid/start-id", s.requireLocalLogin(s.handleEIDStartByNationalID))
		// Not the login limiter: a citizen polls for as long as it takes them to
		// reach their phone, and sharing that budget with sign-in attempts made
		// a busy office throttle itself out of signing in at all.
		api.With(security.SharedRateLimitMiddleware(s.pollLimiter, s.sharedPoll)).Post("/auth/eid/poll", s.requireLocalLogin(s.handleEIDPoll))
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/dan/login", s.requireLocalLogin(s.handleDANLogin))
		api.Post("/auth/logout", s.handleLogout)

		// Signing in through the provider this deployment is a client of.
		//
		// All three are unauthenticated, and each carries its own authority:
		// the config endpoint says nothing secret, the start endpoint mints the
		// state it later requires, and the callback is answerable only to a
		// browser holding the cookie that start set. They are registered
		// whether or not client mode is on — the handlers answer 404 when it is
		// off, which is a truthful answer that does not depend on the routing
		// table having been built differently.
		//
		// Deliberately not on the login budget. None of them checks a
		// credential — the guessing this deployment has to ration happens at
		// the provider, and starting a sign-in here costs a cookie and a
		// redirect. Sharing the sign-in budget would have meant an office
		// behind one address running itself out of sign-ins by clicking a
		// button that only ever redirects.
		// Who is asking. The sign-in screen shows the name of the application
		// that sent somebody here, and it has to resolve it server-side —
		// see HandleClientInfo for why passing it in the redirect would be a
		// phishing kit rather than a feature.
		api.Get("/oauth2/client-info", s.ssoProvider.HandleClientInfo)

		api.Get("/auth/sso/config", s.handleSSOConfig)
		// Google, when this deployment offers it. Same shape as the federated
		// pair above and public for the same reasons.
		api.Get("/auth/google/start", s.handleGoogleStart)
		api.Get("/auth/google/callback", s.handleGoogleCallback)

		// Completing a first sign-in from an external provider by proving a
		// national identity. Unauthenticated for the same reason the rest of
		// this group is — nobody is signed in yet — and the binding token in
		// the request is the authority. The eID pair is budgeted like the
		// ordinary eID pair: starting pushes a notification at somebody's
		// phone, polling waits for them to reach it.
		api.Get("/auth/bind/session", s.handleBindingSession)
		api.Post("/auth/bind/consent", s.handleBindingConsent)
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/bind/eid/start", s.handleBindingEIDStart)
		api.With(security.SharedRateLimitMiddleware(s.pollLimiter, s.sharedPoll)).Post("/auth/bind/eid/poll", s.handleBindingEIDPoll)
		api.Get("/auth/sso/start", s.handleSSOStart)
		api.Get("/auth/sso/callback", s.handleSSOCallback)
		// Device enrollment is the bootstrap: the one-time code is its authority,
		// so the device cannot already be behind session/device middleware.
		api.Post("/devices/enroll", s.handleEnrollDevice)
		api.With(s.deviceMiddleware).Get("/devices/me", s.handleDeviceMe)
		api.With(s.deviceMiddleware).Post("/devices/token/rotate", s.handleRotateDeviceToken)
		api.With(s.deviceMiddleware, security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/devices/staff/pin", s.handleDeviceStaffPIN)
		api.With(s.deviceMiddleware).Get("/devices/shifts/current", s.handleCurrentShift)
		api.With(s.deviceMiddleware, s.authMiddleware).Post("/devices/shifts/open", s.handleOpenShift)
		api.With(s.deviceMiddleware, s.authMiddleware).Post("/devices/shifts/close", s.handleCloseShift)
		api.With(s.deviceMiddleware).Post("/devices/telemetry", s.handleDeviceTelemetry)

		// The OAuth redirect a connected provider sends the browser back to.
		// Unauthenticated on purpose — see handleIntegrationOAuthCallback: the
		// single-use state row is what carries the authority here, because a
		// cross-site redirect from Google cannot be relied on to present a
		// session cookie at all.
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
			// A person's own record: which identities are linked to this
			// account and what each provider said. Inside the authenticated
			// group and answering only for the caller — see profile_handlers.go.
			pr.Get("/profile", s.handleProfile)
			pr.Post("/profile/identities/unlink", s.handleUnlinkIdentity)
			// Ending the session in front of you needs no proof beyond the
			// cookie; ending the ones you cannot see is a decision about an
			// account, so it sits behind authentication with the rest.
			pr.Post("/auth/logout-all", s.handleLogoutEverywhere)
			// Which organisations this person may act for, and moving the
			// session to one of them. Both cross tenants by definition, so
			// they run on the platform path — see handleTenants.
			pr.Get("/auth/tenants", s.handleTenants)
			pr.Post("/auth/tenants/active", s.handleSetActiveTenants)
			pr.Post("/auth/switch-tenant", s.handleSwitchTenant)
			pr.Get("/menus", s.handleMenus)
			pr.With(s.requireAdmin).Post("/admin/devices/enrollment-codes", s.handleCreateEnrollmentCode)
			pr.With(s.requireAdmin).Get("/admin/devices", s.handleListDevices)
			pr.With(s.requireAdmin).Put("/admin/devices/staff-pin", s.handleSetStaffPIN)
			pr.With(s.requireAdmin).Put("/admin/devices/status", s.handleUpdateDeviceStatus)
			pr.Post("/push-tokens", s.handleRegisterPushToken)

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
			// What changed in an app, and what this organisation did about it.
			// A member-level read on purpose: "why did this move" is asked by
			// the people using the app, and the answer names nobody outside
			// their own tenant.
			pr.Get("/store/apps/{slug}/history", s.handleAppHistory)

			pr.Group(func(ar chi.Router) {
				ar.Use(s.requireAdmin)
				ar.Post("/store/apps/{slug}/install", s.handleInstallApp)
				ar.Post("/store/apps/{slug}/upgrade", s.handleUpgradeApp)
				// Whether an app follows the catalogue on its own. Reading it
				// is part of the installed-apps list; deciding it is
				// administrative, like every other store mutation.
				ar.Post("/store/apps/{slug}/auto-update", s.handleSetAutoUpdate)
				// Asking the registry for a new catalogue on demand. Admin-only
				// like the rest: it reaches out of this deployment and changes
				// what every tenant on it is offered.
				ar.Post("/admin/store/sync", s.handleSyncCatalog)
				// Where the catalogue comes from and how the last refresh
				// went. A read, but an administrative one: it names the
				// registry and reports its failures.
				ar.Get("/admin/store/status", s.handleCatalogStatus)
				// The whole store in one view: which versions the binary, the
				// catalogue and this tenant each hold, and where they disagree.
				// It is what makes a stale catalogue visible — from every other
				// screen a week-old one looks exactly like a current one.
				ar.Get("/admin/store/overview", s.handleStoreOverview)
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
