/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package platform is the plane that acts for the whole deployment.
//
// One organisation is never what a request here is about. An operator suspends
// a tenant, reads the audit trail of every tenant, changes a setting every
// tenant is served under, counts what all of them used — work done on behalf of
// the deployment rather than on anybody's behalf inside it. The other plane,
// internal/workspace, is the opposite of that, and neither imports the other.
//
// The distinction the design rests on (docs/CONTROL_PLANE.md §1) is the one
// every multi-tenant platform eventually draws: the data plane is where
// organisations do their work, the control plane is where the platform is
// operated. They share a binary here, because a second Go process would double
// the deployment and the monitoring for a console two people use — but they
// share nothing else:
//
//	tenant user   → users / sessions      → gerege_nexus_tenant, one organisation
//	operator      → operator_accounts /
//	                operator_sessions     → gerege_nexus_operator, read-only
//
// Separate accounts, separate sessions, separate cookie, separate database
// role, separate audit table. A tenant administrator's account being taken does
// not reach anything in this plane, and an operator's account being taken
// reaches only what migration 00049 named — which is a list of SELECTs.
//
// Three rules hold everywhere in here, and each has a home:
//
//   - The console answers on one hostname only. operator.HostGate, and nginx in
//     front of it with an address allowlist.
//   - Every write is audited, in the same transaction as the write.
//     operator.Do, and operator.RequireAudit above it — a write whose audit row
//     did not land does not reach the caller as a success.
//   - Nothing runs as the login role. Every query is made with a context marked
//     by dbguard.AsOperator, which is operator.Scoped.
//
// This file is the plane's composition root: it builds the screens, each of
// which is its own package, and mounts them under one route table. The screens
// stand on internal/operator/operator and never on each other's routes.
package operator

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/credentials"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/flags"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/mailrail"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/announce"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/approvals"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/backup"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/catalog"
	platformflags "github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/flags"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/metering"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/observability"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	platformsettings "github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/settings"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/support"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/tenants"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"
)

// ConsoleDeps are the tenant plane's own services the console borrows: the
// installer, so a new organisation's apps are installed by the same code path
// the store uses, and the mail rail, so its first administrator can be invited.
//
// All may be nil: a deployment with no mail configured still runs a console, it
// just cannot invite anybody, and it says so on the screen rather than in the
// log.
type ConsoleDeps struct {
	Installer tenants.Installer
	Mail      mailrail.Sender
	// Settings and Flags are what the console edits. Held as the deployment's
	// own stores rather than rebuilt here, so a change the console makes is
	// felt by the running platform rather than by a second copy of it.
	Settings *settings.Store
	Flags    *flags.Store
	// Credentials are the keys this deployment reaches other systems with,
	// sealed. Held here for the same reason as the two above: a key set in the
	// console has to be the key the running platform presents.
	Credentials *credentials.Store
	// TenantChanged is called after the console changes an organisation's
	// lifecycle, so the other plane can drop what it has cached about it — on
	// every replica, through the invalidation bus, rather than after each of
	// them has waited out its own copy.
	//
	// A callback rather than the console reaching into those caches: this plane
	// must not know that they exist, and the other must not have to expose
	// them.
	TenantChanged func(tenantID string)
	// Warnings are the deployment's own complaints about its configuration. A
	// callback for the same reason: the answer lives in the other plane.
	Warnings func() []string
	// CatalogStatus is when the app catalogue was last fetched, whether it
	// worked, and why not. The other plane holds it in memory; this one shows
	// it.
	CatalogStatus func() (at time.Time, ok bool, detail string)
	// SyncCatalog triggers a manual refresh of the app catalogue.
	SyncCatalog func(ctx context.Context) (changed bool, err error)
	// PlatformVersion is the semver this binary claims, stamped at build time.
	PlatformVersion string
}

// Service is this plane: the screens, and the one route table they are mounted
// under.
type Service struct {
	db *pgxpool.Pool
	op *operator.Console

	tenants       *tenants.Service
	approvals     *approvals.Service
	support       *support.Service
	audit         *audit.Service
	settings      *platformsettings.Service
	flags         *platformflags.Service
	announce      *announce.Service
	observability *observability.Service
	backup        *backup.Service
	metering      *metering.Service
	catalog       *catalog.Service
}

// New builds the plane. It performs no I/O: a deployment without the migrations
// still constructs, and its routes refuse at the door.
//
// The order is the dependency order rather than a preference: the core first,
// then the screens that only need it, then the four that borrow another screen's
// answer — the tenant detail page shows an audit trail, creating an organisation
// invites its first administrator, the usage chart is drawn against a quota, and
// the front page reads what was last backed up.
func New(db *pgxpool.Pool, deps ConsoleDeps) *Service {
	op := operator.New(db)

	auditScreen := audit.New(op, audit.Deps{DB: db})
	supportScreen := support.New(op, support.Deps{DB: db, Mail: deps.Mail})
	backupScreen := backup.New(op, backup.Deps{DB: db})
	tenantScreen := tenants.New(op, tenants.Deps{
		DB: db, Installer: deps.Installer, Support: supportScreen, Audit: auditScreen,
		TenantChanged: deps.TenantChanged,
	})
	observabilityScreen := observability.New(op, observability.Deps{
		DB: db, Backup: backupScreen, Warnings: deps.Warnings,
		CatalogStatus: deps.CatalogStatus, PlatformVersion: deps.PlatformVersion,
	})

	return &Service{
		db: db, op: op,
		tenants:   tenantScreen,
		approvals: approvals.New(op, approvals.Deps{DB: db, TenantChanged: deps.TenantChanged}),
		support:   supportScreen,
		audit:     auditScreen,
		settings: platformsettings.New(op, platformsettings.Deps{
			DB: db, Settings: deps.Settings, Flags: deps.Flags, Warnings: deps.Warnings,
			Credentials: deps.Credentials,
		}),
		flags:         platformflags.New(op, platformflags.Deps{Flags: deps.Flags}),
		announce:      announce.New(op, announce.Deps{DB: db}),
		observability: observabilityScreen,
		backup:        backupScreen,
		metering:      metering.NewScreen(op, metering.Deps{DB: db, Tenants: tenantScreen}),
		catalog: catalog.New(op, catalog.Deps{
			Observability: observabilityScreen, SyncCatalog: deps.SyncCatalog,
		}),
	}
}

// loginRate is what one address may spend on sign-in attempts: a burst of five,
// then one every twelve seconds. The console has a handful of accounts and no
// public sign-up, so a budget this tight costs a real operator nothing and
// makes an address that is guessing wait.
var loginRate = rate.Every(12e9)

// Routes mounts the console at its own prefix.
//
// Mounted unconditionally, and closed by its own first middleware rather than
// by leaving the routes off: a route table that changes shape with the
// environment is one where "is the console reachable" has a different answer in
// production from the one the tests exercise.
func (s *Service) Routes(r chi.Router) {
	r.Route("/api/platform/v1", s.console)

	// Keep the old address for one release, but only as a redirect. Mounting the
	// console twice would duplicate its security boundary and let the two route
	// trees drift. HostGate still wraps the compatibility address: even a
	// Location header must not disclose the console on a public hostname.
	legacy := s.op.HostGate(http.HandlerFunc(movedConsoleRoute))
	r.Handle("/cp/api", legacy)
	r.Handle("/cp/api/*", legacy)
}

// movedConsoleRoute preserves the remainder of the old path and its query.
// StatusPermanentRedirect is 308, so methods and request bodies survive.
//
// DEPRECATED: remove in vNEXT together with the /cp/api compatibility route.
func movedConsoleRoute(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/cp/api")
	target := "/api/platform/v1" + suffix
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusPermanentRedirect)
}

// console is everything under that prefix.
//
// HostGate first, so a request that did not arrive on the console's hostname is
// refused before anything else looks at it — including the rate limiter, which
// otherwise spends its budget on the traffic that is not for it.
//
// RequireAudit next, above authentication rather than below it, so that it
// covers the sign-in route too — beginning a session is a write, and it is one
// of the writes an operator audit exists to hold.
func (s *Service) console(r chi.Router) {
	r.Use(s.op.HostGate)
	r.Use(s.op.RequireAudit)

	r.Group(func(anon chi.Router) {
		anon.Use(security.RateLimitMiddleware(security.NewIPRateLimiter(loginRate, 5)))
		anon.Post("/session", s.op.HandleLogin)
	})

	r.Group(func(signedIn chi.Router) {
		signedIn.Use(s.op.RequireOperator)

		signedIn.Get("/me", s.op.HandleMe)
		signedIn.Delete("/session", s.op.HandleLogout)
		signedIn.Post("/step-up", s.op.HandleStepUp)

		// One group per screen, each declaring the capability its own routes
		// ask for. The order is the order they were mounted in when this was
		// one method: nothing routes by declaration order — the patterns are
		// distinct — but a golden route table is easier to read against its
		// history when the history is the only thing that changed.
		s.tenants.Routes(signedIn)
		s.audit.Routes(signedIn)
		s.approvals.Routes(signedIn)
		s.support.Routes(signedIn)
		s.settings.Routes(signedIn)
		s.flags.Routes(signedIn)
		s.metering.Routes(signedIn)
		s.observability.Routes(signedIn)
		s.catalog.Routes(signedIn)
		s.backup.Routes(signedIn)
		s.announce.Routes(signedIn)
	})
}

// StartBackgroundJobs runs this plane's periodic work: the daily meter, the
// operator session purge, and the removal of organisations whose grace period
// has ended. It returns immediately; every job runs until ctx is cancelled at
// shutdown.
func (s *Service) StartBackgroundJobs(ctx context.Context) {
	// Yesterday's usage, every night: what the console charts and what the AI
	// limit is enforced against.
	metering.NewCollector(s.db).Start(ctx)
	s.op.Sessions().StartHousekeeping(ctx)
	s.tenants.StartDeletionSweep(ctx)
}

// Enabled reports whether the console answers at all on this deployment.
func (s *Service) Enabled() bool { return s.op.Enabled() }
