/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package tenant is the plane that acts for one organisation.
//
// Everything here runs on somebody's behalf: a session belongs to a person
// inside an organisation, every query is bounded by the tenant the request
// carries, and the row-level policies underneath refuse anything that is not.
// The other plane — internal/operator — acts for the deployment, on behalf of
// all of them at once, and the two do not import each other.
//
// What they share is underneath both: internal/kernel holds the pool guard, the
// caches, the settings and flag stores, and the small contracts where one plane
// has to name something the other owns. Where the planes genuinely meet, they
// meet at five tables the platform writes and a tenant reads — see
// db/migrations/ownership_test.go — and at the seam that assembles them, which
// is pkg/host.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/apps"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/appcatalog"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/async"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/cache"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/flags"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/memo"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/access"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/ai"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/appinstall"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/devices"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/devices/staffpin"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/directory"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/emailverify"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/home"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/dan"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/eid"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/eidmongolia"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/gerege"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/profile"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/reporting"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/signing"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/ssoclient"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/ssoprovider"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"
)

type Service struct {
	db        *pgxpool.Pool
	installer *appinstall.AppInstaller
	// catalogSource is where the catalogue came from and where a refresh goes.
	// In file mode it is the bundled file and nothing else; with a registry
	// configured it is that registry, its disk cache and the file behind them.
	catalogSource *appcatalog.Provider
	sessions      *auth.SessionStore
	// authn is the sign-in half of this plane: the handlers, the middleware
	// every authenticated route sits behind, and the two checks that run on
	// every request — is this organisation suspended, and is the deployment
	// read-only. It is the bottom of the plane, so everything else may reach
	// for it and it reaches for nothing here.
	authn         *auth.Handlers
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
	googleLogin *ssoclient.Client
	geregeSvc   *gerege.GeregeService
	aiSvc       *ai.Service
	staffPIN    *staffpin.Service
	// identity is who somebody is, as told by somebody else: the national eID,
	// ДАН, Google and whichever provider this deployment federates with.
	identity *identity.Handlers
	// appinstall is what an organisation has installed: the store screens, the
	// gate every module's routes sit behind, and the catalogue sync that keeps
	// the three in step.
	appinstall *appinstall.Handlers
	// access is who may do what inside one organisation, and the two links the
	// console hands out when somebody cannot get in at all.
	access *access.Handlers
	// home is a person's own workspace: the requests other organisations have
	// published into it, and the rail they publish through.
	home *home.Store
	// profile is what a person and an organisation say about themselves.
	profile *profile.Handlers
	// devices is the terminals an organisation enrols, and the sign-in a till
	// offers whoever is standing at it.
	devices     *devices.Handlers
	permissions *access.SQLPermissionStore
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
}

// appGateTTL bounds how long the gate keeps believing an app is installed after
// somebody else's replica has uninstalled it. Installing is rare and deliberate,
// so this is about the replica that did not serve the button press.
const appGateTTL = 30 * time.Second

// appGateCacheName is what the invalidation bus knows the gate cache as.
const appGateCacheName = "appgate"

// ExtraModules registers modules this binary carries beyond the platform's own.
//
// A distribution repository compiles its modules in and needs them constructed
// with the same nexus.Platform the built-in ones get, at the same moment —
// after the pool exists, before any route is mounted. It is a variadic option
// rather than a parameter so every existing caller, and every test, keeps
// compiling.
type ExtraModules func(nexus.Platform)

// Deps are what this plane is given rather than what it builds.
//
// The pool, the invalidation bus and the two stores the console edits are the
// deployment's, not this plane's: the seam builds them once and hands the same
// values to both planes, so a setting the console changes is felt by the
// running platform rather than by a second copy of it.
type Deps struct {
	DB          *pgxpool.Pool
	Bus         *cache.Bus
	Settings    *settings.Store
	Flags       *flags.Store
	CatalogPath string
	Modules     []ExtraModules
}

// New builds the tenant plane. Nothing here requires Redis: the bus may be a
// local-only one.
func New(deps Deps) (*Service, error) {
	db, bus, catalogPath, extra := deps.DB, deps.Bus, deps.CatalogPath, deps.Modules
	// Instantiate compile-time Go modules once. Each constructor registers the
	// module in the global app registry; calling them twice (here and again in
	// registerAppModuleRoutes) built two instances per app.
	eidMN, err := eidmongolia.New(db)
	if err != nil {
		return nil, fmt.Errorf("eID Mongolia service: %w", err)
	}

	ssoProvider := ssoprovider.NewSSOProvider(db)
	// The server is not built yet, so the gate is handed over as a closure over
	// the pointer that is about to be filled. Reports are listed per request,
	// long after this line.
	var server *Service

	// The three clients to the state's systems, built before the modules
	// because one of the modules is their app-facing surface. They go on the
	// server below as well; this is the same value, not a second one, so what
	// the e-Government screen reports is what the platform actually holds.
	geregeSvc, eidSvc, danSvc := gerege.NewGeregeService(), eid.NewEIDService(), dan.NewDANService()

	permissions := access.NewSQLPermissionStore(db)

	modulePlatform := appinstall.NewModulePlatform(db)

	// What the platform lends its modules. Published rather than passed, so
	// that lending one more is a line here instead of a change to a signature
	// every distribution would have to chase — see pkg/nexus/capability.go.
	// All of it before Bootstrap, which asks the registry for each in turn.
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
	// The authorization server, for whichever app administers it. That app was
	// internal/apps/sso_clients until 2026-08-25 and reached straight into
	// ssoprovider for twenty exported names; it is the App Store's now, and
	// this line is the whole of what it has instead.
	nexus.Provide[nexus.SSOClientRegistry](ssoprovider.AsClientRegistry(ssoProvider))
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
	nexus.Provide[nexus.Quota](auth.NewQuotaRail(db))
	nexus.Provide[nexus.SignatureCounter](telemetry.AsSignatureCounter())
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
	nexus.Provide[nexus.Signer](signing.Rail(eidMN))
	// The deployment's one cipher for credentials at rest, published because a
	// module that stores somebody else's OAuth token has to encrypt it and must
	// not decide how. The connectors were the first caller and were in this
	// repository; they are an app in another one now, and the key is still the
	// deployment's — see pkg/nexus/secrets.go.
	nexus.Provide[nexus.SecretSealer](security.Sealer{})
	// The PDF signing rails, built here rather than in apps.Bootstrap.
	//
	// They are what nexus.SigningRails names, and a module that signs a PDF
	// asks for that in its constructor — so the rails have to exist before any
	// module does, distribution's or this repository's. Their housekeeping is
	// appended to the runtime below, where this value is still in scope.
	esignRails := signing.New(modulePlatform, gerege.NewEsignService(), eidMN)
	// Published rather than handed to documents. The rail is the platform's —
	// ADR 0002 is about why there is exactly one — and where its routes appear
	// is the app's; a parameter made the app unable to be built anywhere else.
	//
	// One key, and the exported one. Publishing the concrete *signing.Rails
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
		return server.appinstall.InstalledAppSet(ctx, tenantID)
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
	catalogConfig := appcatalog.ConfigFromEnv(catalogPath, config.PlatformVersion)
	catalogConfig.Verify = appinstall.VerifyCatalogVersions
	catalogSource := appcatalog.NewProvider(catalogConfig)

	loadCtx, cancelLoad := context.WithTimeout(context.Background(), appinstall.CatalogLoadTimeout)
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

	installer := appinstall.NewAppInstaller(db, catalog, config.PlatformVersion)

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

	s := &Service{
		db:              db,
		installer:       installer,
		reportScheduler: reportScheduler,
		catalogSource:   catalogSource,
		sessions:        auth.NewSessionStore(db, auth.DefaultSessionTTL),
		loginLimiter:    auth.NewLoginLimiter(),
		pollLimiter:     auth.NewPollLimiter(),
		// Every send is a call to somebody else's service on a shared key, so
		// there is a cruder guard in front of the per-tenant allowance the
		// service itself applies: one per second sustained, twenty in a burst.
		verifyLimiter:  security.NewIPRateLimiter(rate.Limit(float64(auth.VerifyRatePerMinute)/60.0), auth.VerifyBurst),
		emailVerify:    emailverify.NewService(db),
		eidSvc:         eidSvc,
		danSvc:         danSvc,
		ssoProvider:    ssoProvider,
		ssoClient:      federatedSignIn,
		googleLogin:    googleLogin,
		geregeSvc:      geregeSvc,
		aiSvc:          ai.NewService(db),
		staffPIN:       staffpin.NewService(db),
		permissions:    permissions,
		appGate:        memo.New[bool](appGateTTL),
		suspended:      memo.New[bool](auth.SuspendedTTL),
		settings:       deps.Settings,
		featureFlags:   deps.Flags,
		bus:            bus,
		backgroundApps: appRuntime.Background,
		eidMN:          eidMN,
	}

	s.authn = auth.New(auth.Deps{
		DB: db, Sessions: s.sessions, Suspended: s.suspended, Bus: bus,
		Permissions: permissions,
		// Nil unless this deployment federates: a sign-out then has a second
		// half, at the provider that signed the person in.
		EndSession: endSessionURL(federatedSignIn),
	})

	// After the sign-in package: both of these ask it for a session.
	s.devices = devices.New(db, s.staffPIN, s.authn)
	s.identity = identity.New(identity.Deps{
		DB: db, Sessions: s.sessions, EID: eidSvc, DAN: danSvc,
		Google: googleLogin, SSO: federatedSignIn, Authn: s.authn,
	})
	// After identity, whose list of ways in the person's screen shows.
	s.home = home.New(db)
	// The rail a module publishes a citizen's request onto. Provided rather
	// than passed: a module that never touches a citizen should not have to
	// know this exists, and one that does asks for it by type.
	nexus.Provide[nexus.PersonFeed](home.AsPersonFeed(s.home))
	s.profile = profile.New(db, s.sessions, s.identity)
	s.access = access.New(db, bus, s.authn)
	s.appinstall = appinstall.New(appinstall.Deps{
		DB: db, Installer: installer, Catalogue: catalogSource,
		Permissions: permissions, Gate: s.appGate, Bus: bus, Authn: s.authn,
	})

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
	ssoProvider.AttachInstallGate(s.appinstall.NewExternalAppGate())

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
	s.bus.Register(access.GrantCacheName, access.GrantCache())
	s.bus.Register(appGateCacheName, s.appGate)
	s.bus.Register(auth.SuspendedCacheName, s.suspended)
	// Said once at startup and again on the console's home screen, because a
	// contradiction between two pieces of configuration is exactly the thing
	// nobody notices until it matters.
	auth.WarnAboutConflictingConfiguration()

	// Deployment-wide budgets for the endpoints where a per-replica one is not
	// a budget at all. Each is nil without Redis, and a nil one allows.
	client := s.bus.Client()
	s.sharedLogin = security.NewSharedLimiter(client, "login", auth.LoginRatePerMinute, time.Minute)
	s.sharedPoll = security.NewSharedLimiter(client, "poll", auth.PollRatePerMinute, time.Minute)
	s.sharedVerify = security.NewSharedLimiter(client, "verify", auth.VerifyRatePerMinute, time.Minute)

	return s, nil
}

// StartBackgroundJobs launches the periodic work app modules need. It is
// separate from NewServer so a test can build a server without spawning
// goroutines, and it returns immediately — every job runs until ctx is
// cancelled at shutdown.
func (s *Service) StartBackgroundJobs(ctx context.Context) {
	for _, module := range s.backgroundApps {
		module.StartHousekeeping(ctx)
	}
	// The same for a module this repository has never heard of.
	//
	// apps.Bootstrap is the platform's own list and it is empty; a
	// distribution's modules arrive through pkg/host.Options.Modules and had
	// nowhere to say that they have work of their own to run. Nothing said so
	// while every module was a screen over a request — the first one that was
	// not is the Өртөө channel, whose exchange loop is the entire product:
	// without it a child installation never asks its parent for anything, and
	// what is broken is a queue that stays full rather than a route that
	// answers 500.
	//
	// An optional interface rather than a seventh method on Module: a module
	// with no background work should not have to declare an empty one, and a
	// method every implementation stubs out is a method nobody reads.
	for _, module := range nexus.List() {
		if background, ok := module.(interface{ StartHousekeeping(context.Context) }); ok {
			background.StartHousekeeping(ctx)
		}
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
	// An impersonation whose session has expired is over; the row that says so
	// is what both the console and the organisation read.
	async.Go("impersonation-sweep", func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			s.access.EndImpersonations(ctx)
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
			s.identity.SweepExpiredBindings(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
	// Links nobody followed have to stop being reported as outstanding, and the
	// verification trail is an audit record with a retention window, not a
	// mailing list.
	s.emailVerify.StartHousekeeping(ctx)
	// Only with a registry configured; in file mode the catalogue changes when
	// the release does and there is nothing to poll.
	s.appinstall.StartCatalogSync(ctx)

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
	s.appinstall.ApplyCatalogToInstallations(sweepCtx)
}

// CatalogStatus is when the app catalogue was last fetched, whether it worked,
// and why not. The console shows it; this plane holds it.
func (s *Service) CatalogStatus() (time.Time, bool, string) { return s.appinstall.CatalogStatus() }

// SyncCatalog refreshes the catalogue from the registry, for an operator who
// has pressed the button rather than for the hourly timer.
func (s *Service) SyncCatalog(ctx context.Context) (bool, error) {
	return s.appinstall.SyncCatalog(ctx)
}

// ForgetSuspension drops one organisation's cached lifecycle state, here and
// on every replica, after the console has changed it.
func (s *Service) ForgetSuspension(tenantID string) { s.authn.ForgetSuspension(tenantID) }

// ConfigurationWarnings are this deployment's own complaints about how it is
// configured, which the console shows on its home screen.
func (s *Service) ConfigurationWarnings() []string { return auth.ConfigurationWarnings() }

// Mail is the rail the console borrows to invite an organisation's first
// administrator. Exposed rather than rebuilt there: one rail, one set of
// records, one place a send is rate limited.
func (s *Service) Mail() *emailverify.Service { return s.emailVerify }

// InstallAppForTenant installs a catalogue app without a request behind it.
//
// It exists for the demo seeder, which needs the same dependency resolution and
// compiled-module check the store endpoint performs — writing app_installations
// rows directly would let a demo tenant claim an app whose Go module is not in
// the binary, and the shell would then render a menu leading nowhere.
func (s *Service) InstallAppForTenant(ctx context.Context, tenantID, appSlug, userID string) error {
	return s.installer.InstallApp(ctx, tenantID, appSlug, userID)
}

// Routes mounts this plane's HTTP surface on the router the seam gives it.
//
// The global middleware, /health, /ready and /metrics are not here: they belong
// to the process rather than to either plane, and the console is mounted beside
// this by the same seam. See pkg/host.
func (s *Service) Routes(r chi.Router) {
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
			recovery.Get("/auth/credential", s.access.HandleCredentialCheck)
			recovery.Post("/auth/credential/redeem", s.access.HandleCredentialRedeem)
			recovery.Post("/auth/impersonation/redeem", s.access.HandleImpersonationRedeem)
		})

		// Auth with rate limiting
		// Every path by which this deployment establishes an identity of its
		// own. On a deployment that federates, requireLocalLogin closes all of
		// them and says where sign-in actually happens — see
		// sso_client_handlers.go for why that is all or nothing.
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/login", s.identity.RequireLocalLogin(s.authn.HandleLogin))
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/eid/login", s.identity.RequireLocalLogin(s.identity.HandleEIDLogin))
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/eid/start", s.identity.RequireLocalLogin(s.identity.HandleEIDStart))
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/eid/start-id", s.identity.RequireLocalLogin(s.identity.HandleEIDStartByNationalID))
		// Not the login limiter: a citizen polls for as long as it takes them to
		// reach their phone, and sharing that budget with sign-in attempts made
		// a busy office throttle itself out of signing in at all.
		api.With(security.SharedRateLimitMiddleware(s.pollLimiter, s.sharedPoll)).Post("/auth/eid/poll", s.identity.RequireLocalLogin(s.identity.HandleEIDPoll))
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/dan/login", s.identity.RequireLocalLogin(s.identity.HandleDANLogin))
		api.Post("/auth/logout", s.authn.HandleLogout)

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

		api.Get("/auth/sso/config", s.identity.HandleSSOConfig)
		// Google, when this deployment offers it. Same shape as the federated
		// pair above and public for the same reasons.
		api.Get("/auth/google/start", s.identity.HandleGoogleStart)
		// Adding Google to an account that already exists. Registered beside
		// the sign-in routes rather than in the authenticated group because it
		// is a navigation that ends at Google, and the handler resolves the
		// session itself — see handleGoogleLinkStart.
		api.Get("/auth/google/link", s.identity.HandleGoogleLinkStart)
		api.Get("/auth/google/callback", s.identity.HandleGoogleCallback)

		// Completing a first sign-in from an external provider by proving a
		// national identity. Unauthenticated for the same reason the rest of
		// this group is — nobody is signed in yet — and the binding token in
		// the request is the authority. The eID pair is budgeted like the
		// ordinary eID pair: starting pushes a notification at somebody's
		// phone, polling waits for them to reach it.
		api.Get("/auth/bind/session", s.identity.HandleBindingSession)
		api.Post("/auth/bind/consent", s.identity.HandleBindingConsent)
		api.With(security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/auth/bind/eid/start", s.identity.HandleBindingEIDStart)
		api.With(security.SharedRateLimitMiddleware(s.pollLimiter, s.sharedPoll)).Post("/auth/bind/eid/poll", s.identity.HandleBindingEIDPoll)
		api.Get("/auth/sso/start", s.identity.HandleSSOStart)
		api.Get("/auth/sso/callback", s.identity.HandleSSOCallback)
		// Device enrollment is the bootstrap: the one-time code is its authority,
		// so the device cannot already be behind session/device middleware.
		api.Post("/devices/enroll", s.devices.HandleEnrollDevice)
		api.With(s.devices.Middleware).Get("/devices/me", s.devices.HandleDeviceMe)
		api.With(s.devices.Middleware).Post("/devices/token/rotate", s.devices.HandleRotateDeviceToken)
		api.With(s.devices.Middleware, security.SharedRateLimitMiddleware(s.loginLimiter, s.sharedLogin)).Post("/devices/staff/pin", s.devices.HandleDeviceStaffPIN)
		// The till shift endpoints were here. Point of sale went to
		// pos-gerege-nexus and they did not follow — three routes over
		// pos_shifts, a table belonging to a module this binary does not have.
		// Found by db/migrations/ownership_test.go rather than by anybody
		// noticing; removed with the rest of the departed apps' remains.
		api.With(s.devices.Middleware).Post("/devices/telemetry", s.devices.HandleDeviceTelemetry)

		// Where the verification service returns somebody who has just proved
		// an address. Unauthenticated on purpose: they have not signed in, and
		// may have no account here at all. The single-use reference in the
		// query is the whole authority — see handleVerifyLanded.
		api.Get("/verify/landed", s.emailVerify.HandleVerifyLanded)

		// Protected endpoints
		api.Group(func(pr chi.Router) {
			pr.Use(s.authn.Middleware)

			pr.Get("/auth/me", s.authn.HandleMe)
			// A person's own record: which identities are linked to this
			// account and what each provider said. Inside the authenticated
			// group and answering only for the caller — see profile_handlers.go.
			// What this person asked other organisations for. In the
			// authenticated group beside /profile because it is the same kind
			// of thing — the caller's own record — and for the same reason it
			// carries no permission of its own.
			pr.Get("/me/items", s.home.HandleItems)
			pr.Get("/profile", s.profile.HandleProfile)
			pr.Post("/profile/identities/unlink", s.identity.HandleUnlinkIdentity)
			// What the signed-in person prefers, wherever they are. No
			// permission: these are the caller's own settings, and a person who
			// cannot read their own language preference has nothing to be
			// protected from. It used to be an endpoint of the organisation
			// app, which made a person's language something their employer
			// could uninstall.
			pr.Get("/profile/preferences", s.profile.HandleGetPreferences)
			pr.Put("/profile/preferences", s.profile.HandleUpdatePreferences)

			// The organisation's own legal identity. A platform route rather
			// than an app one: the control plane, the XYP rail and the SSO
			// consent screen all read it, and none of them has an opinion about
			// which apps this tenant installed. Reading is open to any member —
			// the name is on the sidebar already; writing is administrative,
			// because these fields print on documents.
			pr.Get("/tenant/profile", s.profile.HandleGetTenantProfile)
			pr.With(s.authn.RequireAdmin).Put("/tenant/profile", s.profile.HandleUpdateTenantProfile)
			// Refreshing those same fields from the register that holds them.
			// Behind the same guard as the edit, because it is one: what comes
			// back overwrites the organisation's legal identity.
			pr.With(s.authn.RequireAdmin).
				Post("/tenant/profile/sync-core", s.profile.HandleSyncTenantProfileFromCore)
			// Ending the session in front of you needs no proof beyond the
			// cookie; ending the ones you cannot see is a decision about an
			// account, so it sits behind authentication with the rest.
			pr.Post("/auth/logout-all", s.authn.HandleLogoutEverywhere)
			// Which organisations this person may act for, and moving the
			// session to one of them. Both cross tenants by definition, so
			// they run on the platform path — see handleTenants.
			pr.Get("/auth/tenants", s.authn.HandleTenants)
			pr.Post("/auth/tenants/active", s.authn.HandleSetActiveTenants)
			pr.Post("/auth/switch-tenant", s.authn.HandleSwitchTenant)
			pr.Get("/menus", s.appinstall.HandleMenus)
			pr.With(s.authn.RequireAdmin).Post("/admin/devices/enrollment-codes", s.devices.HandleCreateEnrollmentCode)
			pr.With(s.authn.RequireAdmin).Get("/admin/devices", s.devices.HandleListDevices)
			pr.With(s.authn.RequireAdmin).Put("/admin/devices/staff-pin", s.staffPIN.HandleSetPIN)
			pr.With(s.authn.RequireAdmin).Put("/admin/devices/status", s.devices.HandleUpdateDeviceStatus)
			pr.Post("/push-tokens", s.devices.HandleRegisterPushToken)

			// Consent screen. The browser endpoint at /oauth2/auth redirects
			// here; these two describe the pending grant and record the
			// answer. Both re-validate the request against the database, so
			// the frontend is a renderer rather than a source of truth.
			pr.Get("/oauth2/consent", s.ssoProvider.HandleConsentPrompt)
			pr.Post("/oauth2/consent", s.ssoProvider.HandleConsentDecision)

			// Tenant access control. Mutations are deliberately admin-only;
			// authorization configuration can otherwise be used to self-elevate.
			pr.Route("/admin/access", func(ac chi.Router) {
				ac.Use(s.authn.RequireAdmin)
				ac.Get("/overview", s.access.HandleAccessOverview)
				ac.Post("/roles", s.access.HandleCreateRole)
				ac.Put("/roles/{id}", s.access.HandleUpdateRole)
				ac.Delete("/roles/{id}", s.access.HandleDeleteRole)
				ac.Put("/roles/{id}/permissions", s.access.HandleSetRolePermissions)
				ac.Put("/memberships/{id}/roles", s.access.HandleSetMembershipRoles)
			})

			// Asking the verification service to write to an address spends a
			// credential the whole platform shares, so it is a signed-in act.
			// App modules do not come through here — they hold the service and
			// call it in process.
			pr.With(security.SharedRateLimitMiddleware(s.verifyLimiter, s.sharedVerify)).Post("/verify/send", s.emailVerify.HandleVerifySend)

			// Who has been written to is an administrative read: it is a list
			// of people's addresses and what they were asked to prove.
			pr.With(s.authn.RequireAdmin).Get("/admin/email-verification/overview", s.emailVerify.HandleOverview)

			// AI Copilot, Speech, Translation & Forecasting
			pr.Route("/ai", func(air chi.Router) {
				air.With(nexus.RequirePermission(s.permissions, "ai.read"), nexus.RateLimit(20, 10), nexus.QuotaGate("ai")).Post("/copilot", s.aiSvc.HandleAICopilot)
				air.With(nexus.RequirePermission(s.permissions, "ai.read"), nexus.RateLimit(20, 10), nexus.QuotaGate("ai")).Post("/chat", s.aiSvc.HandleAIChat)
				air.With(nexus.RequirePermission(s.permissions, "ai.read"), nexus.RateLimit(20, 10), nexus.QuotaGate("ai")).Post("/stt", s.aiSvc.HandleAISTT)
				air.With(nexus.RequirePermission(s.permissions, "ai.read"), nexus.RateLimit(20, 10), nexus.QuotaGate("ai")).Post("/tts", s.aiSvc.HandleAITTS)
				air.With(nexus.RequirePermission(s.permissions, "ai.read"), nexus.RateLimit(20, 10), nexus.QuotaGate("ai")).Post("/translate", s.aiSvc.HandleAITranslate)
				air.With(nexus.RequirePermission(s.permissions, "ai.read"), nexus.QuotaGate("ai")).Get("/stock-forecast", s.aiSvc.HandleAIForecast)
			})

			pr.Route("/admin/ai", func(aair chi.Router) {
				aair.Use(s.authn.RequireAdmin)
				aair.Get("/prompts", s.aiSvc.HandleAIListPrompts)
				aair.Put("/prompts/{key}", s.aiSvc.HandleAIUpdatePrompt)
				aair.Get("/knowledge", s.aiSvc.HandleAIListKnowledge)
				aair.Post("/knowledge", s.aiSvc.HandleAICreateKnowledge)
			})

			// Store — reads are open to any tenant member, mutations are
			// tenant-administrator only. Previously every authenticated user
			// could install, enable or disable apps for the whole tenant.
			pr.Get("/store/apps", s.appinstall.HandleListStoreApps)
			pr.Get("/store/apps/{slug}", s.appinstall.HandleGetStoreApp)
			pr.Get("/installed-apps", s.appinstall.HandleListInstalledApps)
			// What changed in an app, and what this organisation did about it.
			// A member-level read on purpose: "why did this move" is asked by
			// the people using the app, and the answer names nobody outside
			// their own tenant.
			pr.Get("/store/apps/{slug}/history", s.appinstall.HandleAppHistory)

			pr.Group(func(ar chi.Router) {
				ar.Use(s.authn.RequireAdmin)
				ar.Post("/store/apps/{slug}/install", s.appinstall.HandleInstallApp)
				ar.Post("/store/apps/{slug}/upgrade", s.appinstall.HandleUpgradeApp)
				// Whether an app follows the catalogue on its own. Reading it
				// is part of the installed-apps list; deciding it is
				// administrative, like every other store mutation.
				ar.Post("/store/apps/{slug}/auto-update", s.appinstall.HandleSetAutoUpdate)
				// Asking the registry for a new catalogue on demand. Admin-only
				// like the rest: it reaches out of this deployment and changes
				// what every tenant on it is offered.
				ar.Post("/admin/store/sync", s.appinstall.HandleSyncCatalog)
				// Where the catalogue comes from and how the last refresh
				// went. A read, but an administrative one: it names the
				// registry and reports its failures.
				ar.Get("/admin/store/status", s.appinstall.HandleCatalogStatus)
				// The whole store in one view: which versions the binary, the
				// catalogue and this tenant each hold, and where they disagree.
				// It is what makes a stale catalogue visible — from every other
				// screen a week-old one looks exactly like a current one.
				ar.Get("/admin/store/overview", s.appinstall.HandleStoreOverview)
				ar.Post("/store/apps/{slug}/enable", s.appinstall.HandleEnableApp)
				ar.Post("/store/apps/{slug}/disable", s.appinstall.HandleDisableApp)
			})
		})
	})

	// Register compile-time Business App Routes with Tenant & App Gate protection
	s.registerAppModuleRoutes(r)
}

// registerAppModuleRoutes mounts every compile-time business module behind the
// tenant app gate. Billing, Documents and the Developer Portal used to be wired
// straight into the protected group, so their endpoints stayed reachable for
// tenants that had never installed the app.
func (s *Service) registerAppModuleRoutes(r chi.Router) {
	for _, module := range nexus.List() {
		module.RegisterRoutes(r, s.appinstall.GateMiddleware(module.ID()))
	}
	s.registerRenamedRouteAliases(r)
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
func (s *Service) registerRenamedRouteAliases(r chi.Router) {
	r.Route("/api/v1/core", func(cr chi.Router) {
		// Authenticated even though the body is only a Location header. A
		// redirect that answers a stranger tells them which paths this
		// deployment serves, and it would be the one route on the platform that
		// replies to no session at all.
		cr.Use(s.authn.Middleware)
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
func SetDefaultApps(appIDs []string) { appinstall.DefaultApps = slices.Clone(appIDs) }

// endSessionURL is the federated half of signing out, as a callback the
// sign-in package can hold without naming the client.
//
// Nil when this deployment authenticates its own people, which is what
// auth.Handlers checks: there is no provider to return anybody to.
func endSessionURL(client *ssoclient.Client) auth.EndSessionURLFunc {
	if client == nil || !client.Config().Enabled() {
		return nil
	}
	return func(w http.ResponseWriter, r *http.Request) string {
		url := client.EndSessionURL(r.Context(), ssoclient.IDTokenFromRequest(r))
		ssoclient.ClearIDTokenCookie(w)
		return url
	}
}
