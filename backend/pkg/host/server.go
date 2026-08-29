/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Where the two planes become one process.
 *
 * internal/workspace acts for one organisation and internal/operator acts for the
 * deployment; neither imports the other, and neither owns the router. This
 * does: it builds what they share, hands the same values to both, mounts them
 * side by side, and answers the three routes that belong to the process rather
 * than to either plane.
 *
 * That the console is a route table on this router rather than a second binary
 * is the arrangement docs/CONTROL_PLANE.md describes and this file implements.
 * It is also the one place a deployment could decide to mount only one of them,
 * which is what makes that a configuration question rather than a fork.
 */

package host

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/cache"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/credentials"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/flags"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/resilience"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
	core "github.com/gerege-systems/open-gerege-nexus/backend/internal/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/setup"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/verifications"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/person"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The two caches the bus carries that belong to neither plane on its own. The
// names are this seam's arrangement, which is why they are declared where the
// stores are handed out rather than where they are read.
const (
	settingsCacheName    = "settings"
	flagsCacheName       = "flags"
	credentialsCacheName = "credentials"
)

// server is the assembled process: both planes and the router they share.
type server struct {
	router      *chi.Mux
	workspace   *workspace.Service
	platform    *core.Service
	settings    *settings.Store
	flags       *flags.Store
	credentials *credentials.Store
	// setup is the first-run wizard. It belongs to neither plane: it runs
	// before there is an organisation for the tenant plane to act for, and it
	// is not the console — a deployment with no console at all still has to be
	// able to open its first organisation.
	setup *setup.Service
}

// newServer builds what the planes share, then each plane, then the router.
//
// The order is the dependency order and not a preference: the stores exist
// before either plane is given one, the tenant plane exists before the console
// can borrow the installer and the mail rail from it, and the router exists
// last because mounting is the only thing left to do.
func newServer(db *pgxpool.Pool, catalogPath string, bus *cache.Bus, extra ...workspace.ExtraModules) (*server, error) {
	// What the console edits and the request path reads. One copy, handed to
	// both planes: a second would mean a setting changed in the console is felt
	// by half the process.
	settingsStore := settings.NewStore(db)
	flagsStore := flags.NewStore(db)
	// The keys, sealed. A third store rather than a third kind of setting: see
	// internal/kernel/credentials for why the line between them is drawn in
	// code rather than in a comment.
	credentialStore := credentials.NewStore(db)

	tenantPlane, err := workspace.New(workspace.Deps{
		DB: db, Bus: bus, Settings: settingsStore, Flags: flagsStore,
		CatalogPath: catalogPath, Modules: extra,
	})
	if err != nil {
		return nil, err
	}

	// The console borrows two of the tenant plane's own services: the
	// installer, so a new organisation's apps are installed by the same code
	// path the store uses, and the mail rail, so its first administrator can be
	// invited. It borrows them through this struct rather than by importing the
	// plane — nothing else of it is reachable from the console.
	platformPlane := core.New(db, core.ConsoleDeps{
		Installer: tenantPlane, Mail: tenantPlane.Mail(),
		TenantChanged: tenantPlane.ForgetSuspension,
		Settings:      settingsStore, Flags: flagsStore, Credentials: credentialStore,
		Warnings: tenantPlane.ConfigurationWarnings, CatalogStatus: tenantPlane.CatalogStatus,
		VerificationHealth: func(ctx context.Context) verifications.Health {
			configured, reachable, detail, provider, admin := tenantPlane.MailVerificationHealth(ctx)
			return verifications.Health{
				Configured: configured, Reachable: reachable, Detail: detail,
				ProviderURL: provider, AdminURL: admin,
			}
		},
		SyncCatalog:     tenantPlane.SyncCatalog,
		PlatformVersion: config.PlatformVersion,
	})

	// The process-wide stores the scattered consumers read: the idle timeout in
	// auth, the catalogue's interval, the copilot's model, and every
	// flags.Enabled in every module. Installed before the first request rather
	// than lazily, so no request is served with "no store, use the environment"
	// while a value sits in the database.
	settings.UseStore(settingsStore)
	flags.UseStore(flagsStore)
	credentials.UseStore(credentialStore)

	// A first read, so the platform starts with what the console last decided
	// rather than with the defaults for the first thirty seconds. A failure is
	// not fatal — the environment and the defaults are still there — but it is
	// worth saying, because a deployment that logs this every start has a
	// database the settings never load from.
	settingsCtx, cancelSettings := context.WithTimeout(context.Background(), 10*time.Second)
	if err := settingsStore.Load(settingsCtx); err != nil {
		slog.Warn("could not read the platform settings at startup", "error", err)
	}
	if err := flagsStore.Load(settingsCtx); err != nil {
		slog.Warn("could not read the feature flags at startup", "error", err)
	}
	if err := credentialStore.Load(settingsCtx); err != nil {
		slog.Warn("could not read the platform credentials at startup", "error", err)
	}
	cancelSettings()

	// The bus has to know the caches before a message can arrive for one, and a
	// message can arrive as soon as the subscriber connects.
	bus.Register(settingsCacheName, settingsStore)
	bus.Register(flagsCacheName, flagsStore)
	bus.Register(credentialsCacheName, credentialStore)
	settingsStore.OnChange(func() { bus.Invalidate(settingsCacheName, "") })
	flagsStore.OnChange(func() { bus.Invalidate(flagsCacheName, "") })
	credentialStore.OnChange(func() { bus.Invalidate(credentialsCacheName, "") })

	setupWizard := setup.New(db)

	// The person's own tree. Built here because it is nobody's subpackage: see
	// the note beside its Routes call below.
	personPlane := person.New(db)
	// The rail a module publishes a citizen's request onto. Provided rather
	// than passed: a module that never touches a citizen should not have to
	// know this exists, and one that does asks for it by type.
	nexus.Provide[nexus.PersonFeed](person.AsPersonFeed(personPlane))

	return &server{
		router:      newRouter(db, tenantPlane, platformPlane, personPlane, setupWizard),
		workspace:   tenantPlane,
		platform:    platformPlane,
		settings:    settingsStore,
		flags:       flagsStore,
		credentials: credentialStore,
		setup:       setupWizard,
	}, nil
}

// newRouter is the process's HTTP surface: what every request passes through,
// the three routes that belong to no plane, and then the planes themselves.
func newRouter(db *pgxpool.Pool, tenantPlane *workspace.Service, platformPlane *core.Service, personPlane *person.Store, wizard *setup.Service) *chi.Mux {
	r := chi.NewRouter()

	// First, so everything below it — the access log, every slog line a handler
	// writes, and the X-Request-Id header the caller gets back — names the same
	// request. It is also what makes a log line joinable to a trace once
	// tracing is on.
	r.Use(chimiddleware.RequestID)
	// Before the logger, so a log line can name the trace it belongs to. It is
	// a no-op wrapper when OTEL_EXPORTER_OTLP_ENDPOINT is unset.
	r.Use(telemetry.TracingMiddleware)
	r.Use(telemetry.RequestLogger)
	// Not chi's Recoverer: that one prints a stack trace to stdout and nothing
	// else. This one logs it with the request id and the tenant, and reports it
	// to GlitchTip when SENTRY_DSN is set.
	r.Use(telemetry.RecoveryMiddleware)
	r.Use(resilience.NewLoadShedder(1000).Middleware)
	r.Use(telemetry.MetricsMiddleware)
	r.Use(security.HeadersMiddleware)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: security.SafeCORSOrigins(),
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		// X-Setup-Token is the first-run wizard's, and it has to be here for the
		// same reason X-Tenant-ID is: a header the preflight does not name is a
		// request the browser never sends. In production the browser app and
		// the API share an origin and none of this is consulted; in
		// development they are :3000 and :8080, which is exactly where a
		// deployment is stood up for the first time.
		AllowedHeaders:   []string{"Accept", "Accept-Language", "Authorization", "Content-Type", "X-Tenant-ID", "X-Setup-Token"},
		AllowCredentials: true,
	}))
	r.Use(security.CSRFMiddleware)

	// Infrastructure
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// The platform version is part of the health answer because it is what
		// an app store has to know about this instance: which manifests apply
		// to it, and whether an operator's rollout actually landed.
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "platform_version": config.PlatformVersion})
	})
	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "message": "database unreachable"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})

	// Prometheus Metrics Endpoint
	r.Handle("/metrics", telemetry.MetricsHandler())

	// The operator console first, in the order it was mounted in when both
	// planes were one package. Nothing routes by declaration order here — the
	// patterns are distinct — but a golden route table is easier to read
	// against its history when the history is the only thing that changed.
	platformPlane.Routes(r)
	tenantPlane.Routes(r)
	// The person's own tree, mounted here rather than by either plane.
	//
	// internal/person answers for one subject — the human being — and neither
	// existing plane is that subject: workspace answers for an organisation
	// and operator answers for the deployment. Its rows live in the person's
	// own workspace and its queries run as the workspace role, so it is not a
	// third database boundary; it is a third *domain*, and the personal side is
	// the one with room to grow. Keeping it out of internal/workspace now is
	// cheaper than taking it out later, when it is four packages deep in a tree
	// named for organisations.
	//
	// internal/planes_test.go holds the rule that makes that separation real: a
	// plane may not import another. This is the file allowed to name them all,
	// which is why the session middleware is handed across here.
	personPlane.Routes(r, tenantPlane.AuthMiddleware())
	// Last, and unconditionally: the wizard answers 404 to everything until it
	// is armed, so mounting it on a deployment that was set up years ago costs
	// one route table entry and nothing else.
	wizard.Routes(r)

	return r
}

// Router is the assembled handler, for the HTTP server and for the tests that
// walk the route table.
func (s *server) Router() *chi.Mux { return s.router }

// StartBackgroundJobs starts both planes' periodic work and the refresh timers
// of the stores they share.
func (s *server) StartBackgroundJobs(ctx context.Context) {
	// What the console can change without a deployment. Both refresh on their
	// own timer as well as on the bus, so a deployment without Redis is at most
	// thirty seconds behind.
	s.settings.StartRefresh(ctx)
	s.flags.StartRefresh(ctx)
	s.credentials.StartRefresh(ctx)
	s.workspace.StartBackgroundJobs(ctx)
	s.platform.StartBackgroundJobs(ctx)
}
