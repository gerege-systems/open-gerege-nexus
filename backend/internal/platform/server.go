/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Package platform provides the core HTTP Server orchestrator, routing table,
 * authentication middleware, and app installer wiring.
 */

package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/apps"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/appcatalog"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/appinstaller"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/async"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/cache"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/controlplane"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/dan"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/directory"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/eid"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/eidmongolia"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/emailverify"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/esign"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/flags"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/gerege"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/integration"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/memo"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/metering"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/observability"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/rbac"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/reporting"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/resilience"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/settings"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/ssoclient"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/ssoprovider"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/urtuu"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
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
var PlatformVersion = "1.1.0"

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
	verifyLimiter *security.IPRateLimiter
	// emailVerify is the shared "prove this address" service. It belongs to
	// every app module rather than to the platform's own handlers, so when a
	// module needs it, the accessor to add is one line — it is unexported now
	// only because no module has asked yet, and an exported accessor nobody
	// called read as a dependency the modules already had.
	emailVerify *emailverify.Service
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
	// urtuuLink is the Өртөө channel: the links to other installations and the
	// queues in both directions. A platform service rather than part of the
	// Өртөө app, because the channel is infrastructure any module may reach for
	// and the task board is a product a tenant chooses to install.
	urtuuLink   *urtuu.Service
	permissions *rbac.SQLPermissionStore
	appGate     *memo.Cache[bool]
	// settings and featureFlags are the two things the console can change
	// without a deployment. Both are memory with a timer behind them; both are
	// on the invalidation bus so a change is felt on every replica at once.
	settings     *settings.Store
	featureFlags *flags.Store
	// suspended answers "is this organisation closed" without a query per
	// request. Registered on the invalidation bus, so resuming one takes
	// effect everywhere rather than after every replica's own TTL.
	suspended       *memo.Cache[bool]
	bus             *cache.Bus
	sharedLogin     *security.SharedLimiter
	sharedPoll      *security.SharedLimiter
	sharedVerify    *security.SharedLimiter
	backgroundApps  []apps.BackgroundModule
	reportScheduler *reporting.Scheduler
	eidMN           *eidmongolia.Service
	// cp is the operator console. It is a field on this server rather than a
	// process of its own for the reason the plan gives: one binary. What keeps
	// it separate is everything else — its own hostname, accounts, sessions,
	// cookie, database role and audit table.
	cp *controlplane.Service

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

// The other two things the bus carries. Named here rather than in their own
// packages because the names are this server's arrangement, not theirs.
const (
	settingsCacheName = "settings"
	flagsCacheName    = "flags"
)

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
// ExtraModules registers modules this binary carries beyond the platform's own.
//
// A distribution repository compiles its modules in and needs them constructed
// with the same nexus.Platform the built-in ones get, at the same moment —
// after the pool exists, before any route is mounted. It is a variadic option
// rather than a parameter so every existing caller, and every test, keeps
// compiling.
type ExtraModules func(nexus.Platform)

func NewServer(db *pgxpool.Pool, catalogPath string, bus *cache.Bus, extra ...ExtraModules) (*Server, error) {
	// Instantiate compile-time Go modules once. Each constructor registers the
	// module in the global app registry; calling them twice (here and again in
	// registerAppModuleRoutes) built two instances per app.
	// The integration manager is built before the modules that use it: esign
	// files finished documents through it and gov_services books meetings
	// through it, so it is a dependency of both rather than a peer.
	integrationMgr := integration.NewManager(db)
	// The booking contract, published rather than handed to one module. It was
	// declared with an adapter and no way to get one; a module that books an
	// appointment now asks nexus.Meetings() for it. See pkg/nexus/meetings.go.
	nexus.Provide[nexus.MeetingBooker](integration.AsMeetingBooker(integrationMgr))

	eidMN, err := eidmongolia.New(db)
	if err != nil {
		return nil, fmt.Errorf("eID Mongolia service: %w", err)
	}

	ssoProvider := ssoprovider.NewSSOProvider(db)
	// The server is not built yet, so the gate is handed over as a closure over
	// the pointer that is about to be filled. Reports are listed per request,
	// long after this line.
	var server *Server

	// The three clients to the state's systems, built before the modules
	// because one of the modules is their app-facing surface. They go on the
	// server below as well; this is the same value, not a second one, so what
	// the e-Government screen reports is what the platform actually holds.
	geregeSvc, eidSvc, danSvc := gerege.NewGeregeService(), eid.NewEIDService(), dan.NewDANService()

	permissions := rbac.NewSQLPermissionStore(db)
	// The Өртөө channel, built before the modules because the Өртөө app is
	// handed it: the app registers the readers for the task envelopes, and a
	// reader registered after the exchange loop had started would have let a
	// round of arrivals sit unread.
	//
	// Switched off — with a log line — on every deployment that has not been
	// given a signing key, which is all of them until somebody establishes a
	// link. The app is still constructed; see apps.Bootstrap.
	urtuuLink := urtuu.New(db, permissions)
	// Published for every module, not handed to one. The Өртөө app is its first
	// caller and should not be its only one: any module with something to say to
	// another installation asks nexus.Ring() for it. See pkg/nexus/link.go.
	nexus.Provide[nexus.Link](urtuuLink)
	// The reading half of the same channel: who is on the other end of a link,
	// what a request code means, whether it has been announced there. Published
	// because the task board asked those questions by joining the channel's own
	// tables — the coupling ADR 0004 named as what kept it in this repository.
	nexus.Provide[nexus.PeerDirectory](urtuu.AsPeerDirectory(urtuuLink))

	modulePlatform := newModulePlatform(db)

	// What the platform lends its modules. Published rather than passed, so
	// that lending one more is a line here instead of a change to a signature
	// every distribution would have to chase — see pkg/nexus/capability.go.
	// All of it before Bootstrap, which asks the registry for each in turn.
	nexus.Provide(integrationMgr)
	nexus.Provide(eidMN)
	nexus.Provide(ssoProvider)
	nexus.Provide(geregeSvc)
	// The state's registers and the audit trail, as contracts rather than as
	// this repository's types. Both were reached for by internal/apps/egov —
	// one through *gerege.GeregeService in a struct field, the other through
	// its own SQL over audit_events — and both were the reason that module
	// could not be compiled anywhere else.
	nexus.Provide[nexus.StateRegistry](gerege.AsStateRegistry(geregeSvc))
	nexus.Provide[nexus.AuditReader](audit.AsReader(db))
	// The identity rails a module signs with, and the two platform services a
	// module cannot build for itself: a rate limiter whose budget is shared
	// across the deployment, and the signature counter, whose registry stays
	// here so that no module can declare a metric with a tenant label.
	nexus.Provide[nexus.EIDSigner](eid.AsSigner(eidSvc))
	nexus.Provide[nexus.DANAuthenticator](dan.AsAuthenticator(danSvc))
	nexus.Provide[nexus.RateLimiter](security.AsRateLimiter(bus.Client()))
	// The monthly allowance the control plane sold, as middleware a module can
	// ask for by name. The assistant is the one metered kind today and it is an
	// app now, so without this the app would be enforcing a number nobody sold
	// — or nothing at all.
	nexus.Provide[nexus.Quota](quotaRail{db: db})
	nexus.Provide[nexus.SignatureCounter](observability.AsSignatureCounter())
	// The report engine, as six methods rather than as fifteen package
	// functions. See pkg/nexus/reportengine.go for why the three-step ones are
	// one call.
	reportEngine := reporting.NewEngine(db)
	nexus.Provide[nexus.ReportEngine](reporting.AsEngine(reportEngine))
	// The two records a reports screen shows and this platform keeps: a
	// schedule is mailed by the sweep below with nobody present, and a grant is
	// what lets one organisation's report read another's rows. Both were
	// written by the app with its own SQL, which is what kept it here.
	nexus.Provide[nexus.ReportSchedules](reporting.AsSchedules(db))
	nexus.Provide[nexus.ReportGrants](reporting.AsGrants(db))
	// Who belongs to an organisation. The platform's most careful tables, and
	// the ones a module was reading with its own SQL until migration 00076 and
	// pkg/nexus/directory.go between them made that unnecessary.
	nexus.Provide[nexus.Directory](directory.New(db))
	// What this deployment is wired to. Read per call rather than captured as a
	// snapshot, and assembled here because this is the only place all three
	// clients are in scope. The shape is staterail's, not the app's: the
	// platform may not import an app to name a type it builds.
	nexus.Provide[nexus.StateRails](func() []nexus.StateRail {
		return []nexus.StateRail{
			{ID: "xyp", Name: "ХУР", Mode: geregeSvc.Mode(), Endpoint: geregeSvc.Endpoint()},
			{ID: "eid", Name: "eID Mongolia", Mode: eidSvc.Mode(), Endpoint: eidSvc.Endpoint()},
			{ID: "dan", Name: "ДАН", Mode: danSvc.Mode(), Endpoint: danSvc.Endpoint()},
		}
	})
	// The signing rail, as the SDK publishes it: a document that carries a file
	// is signed over that file's digest through this. Provided even when the
	// installation has no eID registration — it answers Enabled() false, which
	// is the state a module is meant to ask about rather than a nil it has to
	// guard.
	nexus.Provide[nexus.Signer](Signing(eidMN))
	// The PDF signing rails, built here rather than in apps.Bootstrap.
	//
	// They are what nexus.SigningRails names, and a module that signs a PDF
	// asks for that in its constructor — so the rails have to exist before any
	// module does, distribution's or this repository's. Their housekeeping is
	// appended to the runtime below, where this value is still in scope.
	esignRails := esign.New(modulePlatform, gerege.NewEsignService(), eidMN, integrationMgr)
	// Published rather than handed to documents. The rail is the platform's —
	// ADR 0002 is about why there is exactly one — and where its routes appear
	// is the app's; a parameter made the app unable to be built anywhere else.
	//
	// One key, and the exported one. Publishing the concrete *esign.Rails
	// beside it bought nothing: the only reader was apps.Bootstrap, thirty
	// lines down the same function, and it keyed a background loop on a type
	// from internal/ that no distribution can name or replace — so a Provide
	// deleted by accident became a nil *Rails that boots clean and panics into
	// a recovered goroutine five minutes later.
	nexus.Provide[nexus.SigningRails](esignRails)
	// A closure over the server pointer that is about to be filled, so the
	// named type is what carries it: an unnamed func would key the registry on
	// a shape every other callback of the same signature shares.
	//
	// The nil check is not decoration. Distribution modules are constructed
	// below, while this pointer is still nil, and this capability is the one
	// they can now resolve before it means anything — an error is the answer
	// the SDK promises for that, not a panic inside NewServer.
	nexus.Provide[nexus.InstalledApps](func(ctx context.Context, tenantID string) (map[string]bool, error) {
		if server == nil {
			return nil, errors.New("the platform is still starting; ask which apps a tenant has after it has")
		}
		return server.installedAppSet(ctx, tenantID)
	})

	// A distribution's modules register themselves here, beside the platform's
	// own, so appregistry sees one list and the store, the menu and the gate
	// cannot tell the two apart — which is the point of the SDK.
	//
	// After every Provide above and before Bootstrap below, and both halves of
	// that matter. This loop used to run first, which meant a module carried by
	// a distribution was constructed before the platform had published anything
	// — so client-gerege-nexus's e-Government module asked for the state
	// registry, got nothing, logged a warning and served a degraded screen on a
	// deployment that had the rail all along. Nothing failed; the app was
	// simply built too early. Before Bootstrap, because the reports app is
	// constructed last on purpose: a module that registers a report after it
	// would be missing from the first listing.
	for _, register := range extra {
		if register != nil {
			register(modulePlatform)
		}
	}

	appRuntime := apps.Bootstrap(modulePlatform)
	// The rails' sweep of abandoned signing ceremonies, on the same list as
	// every module's housekeeping. Appended here rather than inside Bootstrap
	// because this is where the value lives, and a value handed over is one
	// the compiler checks.
	appRuntime.Background = append(appRuntime.Background, esignRails)
	// The schedule sweep, on the same list. It ran because the reports app
	// happened to start it, which made a screen responsible for a deployment's
	// housekeeping — and meant a deployment that removed the app quietly
	// stopped mailing the schedules it still had. The schedules are this
	// platform's rows and the mail is its rail, so the sweep is its own.
	reportScheduler := reporting.NewScheduler(reportEngine, reporting.NewSMTPDeliverer())

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
		db:              db,
		installer:       installer,
		reportScheduler: reportScheduler,
		catalogSource:   catalogSource,
		router:          chi.NewRouter(),
		sessions:        auth.NewSessionStore(db, auth.DefaultSessionTTL),
		loginLimiter:    newLoginLimiter(),
		pollLimiter:     newPollLimiter(),
		// Every send is a call to somebody else's service on a shared key, so
		// there is a cruder guard in front of the per-tenant allowance the
		// service itself applies: one per second sustained, twenty in a burst.
		verifyLimiter:  security.NewIPRateLimiter(rate.Limit(float64(verifyRatePerMinute)/60.0), verifyBurst),
		emailVerify:    emailverify.NewService(db),
		eidSvc:         eidSvc,
		danSvc:         danSvc,
		ssoProvider:    ssoProvider,
		ssoClient:      federatedSignIn,
		googleLogin:    googleLogin,
		geregeSvc:      geregeSvc,
		integrationMgr: integrationMgr,
		urtuuLink:      urtuuLink,
		permissions:    permissions,
		appGate:        memo.New[bool](appGateTTL),
		suspended:      memo.New[bool](suspendedTTL),
		settings:       settings.NewStore(db),
		featureFlags:   flags.NewStore(db),
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
	s.bus.Register(suspendedCacheName, s.suspended)
	s.bus.Register(settingsCacheName, s.settings)
	s.bus.Register(flagsCacheName, s.featureFlags)
	s.settings.OnChange(func() { s.bus.Invalidate(settingsCacheName, "") })
	s.featureFlags.OnChange(func() { s.bus.Invalidate(flagsCacheName, "") })

	// The process-wide stores the scattered consumers read: the idle timeout in
	// auth, the catalogue's interval, the copilot's model, and every
	// flags.Enabled in every module. Installed before the first request rather
	// than lazily, so no request is served with "no store, use the environment"
	// while a value sits in the database.
	settings.UseStore(s.settings)
	flags.UseStore(s.featureFlags)

	// A first read, so the platform starts with what the console last decided
	// rather than with the defaults for the first thirty seconds. A failure is
	// not fatal — the environment and the defaults are still there — but it is
	// worth saying, because a deployment that logs this every start has a
	// database the settings never load from.
	settingsCtx, cancelSettings := context.WithTimeout(context.Background(), 10*time.Second)
	if err := s.settings.Load(settingsCtx); err != nil {
		slog.Warn("could not read the platform settings at startup", "error", err)
	}
	if err := s.featureFlags.Load(settingsCtx); err != nil {
		slog.Warn("could not read the feature flags at startup", "error", err)
	}
	cancelSettings()

	// Said once at startup and again on the console's home screen, because a
	// contradiction between two pieces of configuration is exactly the thing
	// nobody notices until it matters.
	warnAboutConflictingConfiguration()

	// Deployment-wide budgets for the endpoints where a per-replica one is not
	// a budget at all. Each is nil without Redis, and a nil one allows.
	client := s.bus.Client()
	s.sharedLogin = security.NewSharedLimiter(client, "login", loginRatePerMinute, time.Minute)
	s.sharedPoll = security.NewSharedLimiter(client, "poll", pollRatePerMinute, time.Minute)
	s.sharedVerify = security.NewSharedLimiter(client, "verify", verifyRatePerMinute, time.Minute)

	// The console borrows two of the platform's own services: the installer,
	// so a new organisation's apps are installed by the same code path the
	// store uses, and the mail rail, so its first administrator can be
	// invited. Nothing else of the platform is reachable from it.
	s.cp = controlplane.New(db, controlplane.Deps{
		Installer: s, Mail: s.emailVerify, TenantChanged: s.forgetSuspension,
		Settings: s.settings, Flags: s.featureFlags,
		Warnings: ConfigurationWarnings, CatalogStatus: s.catalogSyncStatus,
		SyncCatalog:     s.syncCatalogFromRegistry,
		PlatformVersion: PlatformVersion,
	})

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
// half of the comparison lives in catalog.ValidateCatalog, which every source
// goes through.
//
// An app with no compiled module is not an error. External apps have none by
// definition.
func verifyCatalogVersions(catalog []catalog.CatalogApp) error {
	present := make(map[string]bool, len(catalog))
	for _, app := range catalog {
		present[app.ID] = true
		mod, ok := nexus.Get(app.ID)
		if !ok {
			continue
		}
		if mod.Version() != app.Version {
			return fmt.Errorf("module %s is compiled at version %q but the catalog declares %q",
				app.ID, mod.Version(), app.Version)
		}
	}

	// A catalogue without the platform's own default apps is not a catalogue
	// this build should run on — it is one that predates it. The version check
	// above cannot see that: it skips ids with no compiled module, and after a
	// rename every renamed app looks exactly like a third party's, so an entire
	// stale catalogue passes without a word.
	//
	// That is not hypothetical. A cache written before the ids were renamed was
	// accepted whole, the organisation app was absent from it, no tenant got
	// the screens, and every app in the store offered an install that failed on
	// a foreign key. The bundled file always matches the binary, so refusing
	// here is what reaches it.
	//
	// This is a staleness check, not a claim that the app is mandatory: a
	// tenant may remove it, and that removal is a row in app_installations
	// rather than an absence from the catalogue.
	for _, appID := range appinstaller.DefaultApps {
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
	// The schedule sweep. It ran because the reports app happened to start it
	// until 2026-08-23, which made a screen responsible for a deployment's
	// housekeeping — and meant a deployment that removed the app quietly
	// stopped mailing the schedules it still had. The schedules are this
	// platform's rows and the mail is its rail.
	s.reportScheduler.Start(ctx)
	// Every sign-in writes a session row and nothing else ever removes one.
	s.sessions.StartHousekeeping(ctx)
	// Yesterday's usage, every night: what the console charts and what the AI
	// limit is enforced against.
	metering.New(s.db).Start(ctx)
	// What the console can change without a deployment. Both refresh on their
	// own timer as well as on the bus, so a deployment without Redis is at
	// most thirty seconds behind.
	s.settings.StartRefresh(ctx)
	s.featureFlags.StartRefresh(ctx)
	// The console's sessions are a separate table with the same problem, and
	// the organisations whose grace period has ended are removed by the same
	// call.
	s.cp.StartHousekeeping(ctx)
	// An impersonation whose session has expired is over; the row that says so
	// is what both the console and the organisation read.
	async.Go("impersonation-sweep", func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			s.endImpersonations(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
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
	// The Өртөө exchange: one loop keeping every link this installation is the
	// child on in conversation with its parent, and the sweep behind it. Both
	// return immediately and do nothing at all where Өртөө is unconfigured.
	s.urtuuLink.StartHousekeeping(ctx)
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
	async.Go("catalog-sync", func() {
		// The interval is read on every round rather than captured, because it
		// is a platform setting now: an operator who slows the polling down
		// during a registry incident should not have to restart the platform
		// for it to take effect. A change is felt after the current wait, which
		// is the most anybody could reasonably expect of a poll.
		ticker := time.NewTicker(s.catalogInterval())
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ticker.Reset(s.catalogInterval())
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

// catalogInterval is how long to wait before asking the registry again.
//
// The setting, then whatever the catalogue source worked out from the
// environment at startup. A minute is the floor for the reason source.go gives:
// below that this stops being a poll and becomes a load generator pointed at
// somebody else's registry.
func (s *Server) catalogInterval() time.Duration {
	interval := settings.Duration(settings.CatalogSyncInterval)
	if interval <= 0 {
		interval = s.catalogSource.SyncInterval()
	}
	if interval < time.Minute {
		interval = time.Minute
	}
	return interval
}

// catalogSyncStatus is what the console shows about the catalogue: when it was
// last fetched, whether that worked, and why not.
func (s *Server) catalogSyncStatus() (time.Time, bool, string) {
	s.syncMu.RLock()
	defer s.syncMu.RUnlock()
	// A deployment in file mode has never synced and never will, which is not
	// a failure — it is what "no registry configured" looks like.
	if s.lastSyncAt.IsZero() && !s.catalogSource.Remote() {
		return time.Time{}, true, "the catalogue comes from the bundled file"
	}
	return s.lastSyncAt, s.lastSyncOK, s.lastSyncErr
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
	// The modules' own schemas first, and on every sweep rather than only at
	// the install that first needed them: a module can gain a schema — or have
	// one moved into it out of db/migrations — long after its app was
	// installed, and the install path is not reached again for an app that is
	// already there. See MigrateModules for what that cost the first time.
	//
	// Logged and not fatal, like the catalogue sync above it. A database that
	// is not up yet must not stop the process from booting; this runs again on
	// the next sweep, and the error says what is broken until it does.
	if err := s.installer.MigrateModules(ctx); err != nil {
		slog.Error("catalog: a module's own schema could not be applied — its routes will fail until it is",
			"error", err)
	}

	// The distribution's default apps next: a tenant that never got one has no
	// way to install it either, because on a deployment where the store itself
	// sits behind an app the missing app is what would have carried it.
	// A no-op where the list is empty, which is every deployment that has not
	// set platform.Options.DefaultApps — this repository's own included.
	if err := s.installer.EnsureDefaultApps(ctx); err != nil {
		slog.Error("catalog: could not install the default apps", "error", err)
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

	// The operator console (docs/CONTROL_PLANE_PLAN.md).
	//
	// Mounted unconditionally, and closed by its own first middleware rather
	// than by leaving the routes off: a route table that changes shape with the
	// environment is one where "is the console reachable" has a different
	// answer in production from the one the tests exercise. HostGate answers
	// 404 for every request that did not arrive on the console's hostname,
	// which on this deployment is every request that is not an operator's.
	r.Route("/cp/api", s.cp.Routes)

	// OpenID Connect Provider & OAuth2 Authorization Server.
	//
	// These sit at the root rather than under /api/, which is where the
	// specification puts them and what SSO_ISSUER advertises — the reverse
	// proxy has to route them to this service explicitly.
	r.Get("/.well-known/openid-configuration", s.ssoProvider.HandleOIDCDiscovery)
	r.Get("/.well-known/jwks.json", s.ssoProvider.HandleJWKS)

	// This installation's Өртөө identity: the public key a subordinate
	// installation verifies its parent's envelopes with. At the root and public
	// by necessity — it is read before any relationship exists, by a server
	// that has nothing to authenticate with yet. It carries no secret and
	// nothing tenant-scoped, and answers 404 where Өртөө is not configured.
	r.Get("/.well-known/urtuu.json", s.urtuuLink.HandleWellKnown)
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
		// The two journeys that begin in the operator console and finish here,
		// on the hostname the person actually uses. Both are unauthenticated
		// by necessity — somebody choosing a password has no session, and an
		// operator's console session means nothing on this host — and both
		// are single-use tokens with short lives (see access_recovery.go).
		//
		// Rate limited with the sign-in budget: they are credential
		// endpoints, and a token that can be guessed quickly is a token that
		// can be guessed.
		api.Group(func(recovery chi.Router) {
			recovery.Use(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin))
			recovery.Get("/auth/credential", s.handleCredentialCheck)
			recovery.Post("/auth/credential/redeem", s.handleCredentialRedeem)
			recovery.Post("/auth/impersonation/redeem", s.handleImpersonationRedeem)
			// Redeeming an Өртөө invitation. Session-less for the same reason
			// the three above are — the caller is another installation, not a
			// person — and on the sign-in budget for the same reason too: a
			// single-use code that can be guessed quickly is a code that can be
			// guessed. See internal/platform/urtuu/peers.go.
			recovery.Post("/urtuu/peers/redeem", s.urtuuLink.HandleRedeem)
		})

		// The exchange itself. Authenticated by the link's bearer token, which
		// authMiddleware knows nothing about, so these sit outside it — and
		// every envelope inside is checked again against the peer's public key,
		// because a token says who is speaking and only a signature says who
		// wrote what was said.
		//
		// Deliberately not on the poll limiter: the caller is a server holding
		// a credential this deployment issued, not somebody's browser, and one
		// child catching up after a week of downtime must not throttle another
		// out of the channel. What bounds it is batchLimit and the long-poll
		// window on the other side of the call.
		api.Get("/urtuu/exchange/pull", s.urtuuLink.HandlePull)
		api.Post("/urtuu/exchange/push", s.urtuuLink.HandlePush)
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
		// Adding Google to an account that already exists. Registered beside
		// the sign-in routes rather than in the authenticated group because it
		// is a navigation that ends at Google, and the handler resolves the
		// session itself — see handleGoogleLinkStart.
		api.Get("/auth/google/link", s.handleGoogleLinkStart)
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
		// The till shift endpoints were here. Point of sale went to
		// pos-gerege-nexus and they did not follow — three routes over
		// pos_shifts, a table belonging to a module this binary does not have.
		// Found by db/migrations/ownership_test.go rather than by anybody
		// noticing; removed with the rest of the departed apps' remains.
		api.With(s.deviceMiddleware).Post("/devices/telemetry", s.handleDeviceTelemetry)

		// The OAuth redirect a connected provider sends the browser back to.
		// Unauthenticated on purpose — see handleIntegrationOAuthCallback: the
		// single-use state row is what carries the authority here, because a
		// cross-site redirect from Google cannot be relied on to present a
		// session cookie at all.
		// The connector OAuth callback was here until 2026-08-23 — an
		// unauthenticated route, because the provider sends the
		// administrator's browser back with no session of ours. It is
		// internal/apps/integrations' now, registered by that module outside
		// its own gate for the same reason.

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
			// What the signed-in person prefers, wherever they are. No
			// permission: these are the caller's own settings, and a person who
			// cannot read their own language preference has nothing to be
			// protected from. It used to be an endpoint of the organisation
			// app, which made a person's language something their employer
			// could uninstall.
			pr.Get("/profile/preferences", s.handleGetPreferences)
			pr.Put("/profile/preferences", s.handleUpdatePreferences)

			// The organisation's own legal identity. A platform route rather
			// than an app one: the control plane, the XYP rail and the SSO
			// consent screen all read it, and none of them has an opinion about
			// which apps this tenant installed. Reading is open to any member —
			// the name is on the sidebar already; writing is administrative,
			// because these fields print on documents.
			pr.Get("/tenant/profile", s.handleGetTenantProfile)
			pr.With(s.requireAdmin).Put("/tenant/profile", s.handleUpdateTenantProfile)
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
			// Setting a member's staff PIN was here until 2026-08-23. It is
			// internal/apps/staffpin's route now — the credential is a
			// product's, the sign-in it feeds is the platform's, and the two
			// halves are split along that line rather than along this file.
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

			// Settings → Өртөө: the links this organisation has to other
			// installations. A platform screen rather than an app one, because
			// a channel established by an administrator has to outlive any app
			// being uninstalled — the tasks in flight over it do not stop
			// existing because somebody removed the board they are shown on.
			s.urtuuLink.TenantRoutes(pr)

			// Asking the verification service to write to an address spends a
			// credential the whole platform shares, so it is a signed-in act.
			// App modules do not come through here — they hold the service and
			// call it in process.
			pr.With(security.SharedRateLimitMiddleware(s.verifyLimiter, s.sharedVerify)).Post("/verify/send", s.handleVerifySend)

			// Who has been written to is an administrative read: it is a list
			// of people's addresses and what they were asked to prove.
			pr.With(s.requireAdmin).Get("/admin/email-verification/overview", s.handleEmailVerifyOverview)

			// AI Copilot & Forecasting
			// The assistant's ten routes were here until 2026-08-23. They are
			// internal/apps/ai's now — mounted by the module, behind the app
			// gate, so a deployment that does not want an assistant can remove
			// one instead of serving it. What stayed is underneath: the shared
			// rate limiter and the monthly allowance, published as
			// nexus.RateLimiter and nexus.Quota.

			// The connector administration — eight admin-only routes — was
			// here until 2026-08-23. Administering a rail is an app; the rail
			// itself stayed, because the signing rails file documents through
			// it and nexus.MeetingBooker is its adapter.

			pr.Group(func(ar chi.Router) {
				ar.Use(s.requireAdmin)
				// Store — reads AND mutations are tenant-administrator only.
				// Which apps a deployment could install is platform
				// administration, not something a member browses: the shell's
				// rail runs on /menus, so an ordinary member loses nothing.
				// (Reads were member-level until 2026-08-27.)
				ar.Get("/store/apps", s.handleListStoreApps)
				ar.Get("/store/apps/{slug}", s.handleGetStoreApp)
				ar.Get("/installed-apps", s.handleListInstalledApps)
				ar.Get("/store/apps/{slug}/history", s.handleAppHistory)
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
	for _, module := range nexus.List() {
		module.RegisterRoutes(s.router, s.appGateMiddleware(module.ID()))
	}
	s.registerRenamedRouteAliases()
}

// registerRenamedRouteAliases keeps the pre-rename URLs answering for one
// release.
//
// /api/v1/core was the organisation app before it was called that, and the tree
// under it has since been split in two: the legal profile and a person's
// preferences became platform routes, and departments and people stayed with
// the app — which has since left this repository, so only the platform half is
// still redirected. A redirect rather than a second mount, because a duplicated
// route table would mean duplicating the middleware decision as well, and
// getting that wrong here is how a deprecated path quietly becomes the one
// without the permission check.
//
// 308 and not 302: the method and body survive it, so a PUT stays a PUT.
//
// This is registered on the root router rather than inside the /api/v1 group
// because the app modules mount there too, and a subrouter mounted at
// /api/v1/core has to be the one thing chi routes that prefix to.
//
// DEPRECATED: remove in vNEXT, together with the redirects themselves.
func (s *Server) registerRenamedRouteAliases() {
	s.router.Route("/api/v1/core", func(cr chi.Router) {
		// Authenticated even though the body is only a Location header. A
		// redirect that answers a stranger tells them which paths this
		// deployment serves, and it would be the one route on the platform that
		// replies to no session at all.
		cr.Use(s.authMiddleware)
		cr.Handle("/organisation", movedTo("/api/v1/tenant/profile"))
		cr.Handle("/me/preferences", movedTo("/api/v1/profile/preferences"))
		// /departments and /people were redirected under /api/v1/organisation
		// until 2026-08-23, when the app that served that prefix left for
		// client-gerege-nexus. A redirect outliving its target is worse than no
		// redirect: 308 preserves the method and the body, so a PUT followed
		// the Location header into a 404 and lost the write, and the client was
		// told to cache the dead address permanently. A distribution that
		// carries the app can re-register the pair beside its own routes.
	})
}

// movedTo redirects one exact path to another, query string included.
func movedTo(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, withQuery(target, r), http.StatusPermanentRedirect)
	}
}

func withQuery(path string, r *http.Request) string {
	if r.URL.RawQuery == "" {
		return path
	}
	return path + "?" + r.URL.RawQuery
}

// Handlers

// SetDefaultApps records which apps every organisation gets without asking.
//
// The distribution's decision, arriving through platform.Options.DefaultApps.
// Call it before NewServer: the catalogue check reads the list while the server
// is built, and the sweep in appinstaller reads it afterwards on a timer.
//
// Cloned rather than kept. The caller's slice is usually a literal and never
// touched again, but a header shared with something that appends later would be
// read by that timer with no synchronisation at all, and the list decides what
// gets installed into every tenant.
func SetDefaultApps(appIDs []string) { appinstaller.DefaultApps = slices.Clone(appIDs) }
