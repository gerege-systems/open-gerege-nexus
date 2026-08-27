/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Booting Gerege Nexus.
 *
 * This is the whole of what it takes to start the platform: configuration,
 * tracing, the pool and the isolation guard installed on it, the invalidation
 * bus, the server, the background sweeps, the listener and a graceful stop.
 *
 * It is a public package because a distribution repository — a product built on
 * this platform with its own Go modules — has to start the same platform, and
 * `internal/operator` is closed to it by the language. Without this the SDK let
 * somebody write a module and gave them nowhere to run it, which is a fork with
 * extra steps.
 *
 * A distribution's whole main.go is:
 *
 *	func main() {
 *	    if err := platform.Run(platform.Options{
 *	        Modules: func(p nexus.Platform) {
 *	            registry.New(p)
 *	            publisher.New(p)
 *	        },
 *	    }); err != nil {
 *	        os.Exit(1)
 *	    }
 *	}
 *
 * The platform's own cmd/api is the same call with no Modules, which is the
 * point: this repository's binary is built the way every distribution's is.
 */

package platform

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/async"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/cache"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/dbguard"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/identity/eid"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/reporting"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Response deadline. /auth/eid/poll blocks for eid.PollWindow while the relying
// party waits for the citizen to approve, so this has to outlast it with room
// for the round trip. At 15s it did not: Go passed the write deadline while the
// handler was still waiting, closed the connection without a response, and
// nginx turned that into a 502 for every check the citizen did not answer
// within 15 seconds — which read as a slow, flaky sign-in.
//
// It is not only sign-in any more: /documents/{id}/sign/eid/poll waits on the same
// relying-party call for the same window, so a signature ceremony would have been
// cut off in the same way and lost the approval a citizen had just given.
var writeTimeout = eid.PollWindow + 15*time.Second

// Slowloris is held off by the header deadline rather than by writeTimeout, so
// a long poll does not have to buy a client the right to dribble out a request.
const (
	readTimeout       = 15 * time.Second
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 60 * time.Second
)

// Options is what a distribution varies. Everything else is read from the
// environment, because a deployment configures a platform the same way whoever
// built it does.
// Options is what a distribution varies. Everything else is read from the
// environment, because a deployment configures a platform the same way whoever
// built it does.
type Options struct {
	// Modules registers the caller's own app modules. It is handed the same
	// nexus.Platform the built-in modules get, after the pool exists and before
	// any route is mounted — a module that registers later would not be served.
	Modules func(nexus.Platform)

	// DefaultApps are installed for every organisation without anybody asking.
	//
	// A distribution's decision, not the platform's. The list used to be a
	// package variable in internal/tenant/appinstall naming this
	// repository's own apps; when the first of them moved out
	// (client-gerege-nexus, 2026-08-23) the behaviour moved with it and had
	// nowhere to be declared — a deployment could carry an app that every
	// tenant should have and no way to say so.
	//
	// A default app must be one this binary carries: the catalogue check at
	// startup refuses an id with no module behind it, which is what turns a
	// stale catalogue into a failure to start rather than a store full of
	// installs that fail on a foreign key.
	//
	// Empty is an ordinary answer. A deployment where every app is chosen is a
	// deployment that installs nothing by default.
	DefaultApps []string
}

// Run starts the platform and blocks until it is asked to stop.
// Run starts the platform and blocks until it is asked to stop.
func Run(opts Options) error {
	// JSON, as before, but wrapped so that request_id and tenant_id ride along
	// on every line written while a request is being served. Without that, a
	// log line in Loki can only be found by its text, and the four lines one
	// failing request produced are scattered among everyone else's.
	telemetry.SetupLogging(slog.LevelInfo)
	if err := config.ValidateProduction(); err != nil {
		return fmt.Errorf("invalid production configuration: %w", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgrespassword@localhost:5432/platform_db?sslmode=disable"
	}

	catalogPath := resolveCatalogPath(os.Getenv("APP_CATALOG_PATH"))

	ctx := context.Background()

	// Initialize OpenTelemetry distributed tracing
	shutdownTracing, err := telemetry.SetupTracing(ctx, "gerege-nexus", os.Getenv("ENVIRONMENT"))
	if err != nil {
		slog.Error("failed to setup tracing", "error", err)
	} else {
		defer func() {
			_ = shutdownTracing(ctx)
		}()
	}

	// Panics and unhandled errors, grouped by what broke rather than scattered
	// through the log. Off without SENTRY_DSN.
	shutdownErrors, err := telemetry.SetupErrorTracking("gerege-nexus", os.Getenv("ENVIRONMENT"))
	if err != nil {
		slog.Error("failed to setup error tracking", "error", err)
	} else {
		defer func() {
			_ = shutdownErrors(ctx)
		}()
	}

	// Connect to shared-schema PostgreSQL database pool
	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return fmt.Errorf("failed to parse db url: %w", err)
	}
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 5
	poolConfig.MaxConnIdleTime = 15 * time.Minute

	// Bind every pooled connection to the tenant its caller is acting for, so
	// the row-level policies from migration 00029 apply. Installed before the
	// pool exists because it is a property of how connections are handed out;
	// it stays dormant until Probe confirms the database can enforce it.
	guard := &dbguard.Guard{}
	guard.Install(poolConfig)

	// A span per query, so a slow request shows which statement it waited on
	// rather than a gap between two spans. Also dormant without tracing.
	telemetry.InstrumentPool(poolConfig)

	db, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres pool: %w", err)
	}
	defer db.Close()

	// Saturation: how full the pool is, and how often a caller had to wait for
	// it. This is the half of the golden signals /metrics could not answer, and
	// it is the first thing to look at when latency rises without traffic
	// rising with it.
	telemetry.RegisterPoolCollector(db)

	// Where audit rows go. Before this the trail was stdout only — unsearchable
	// and gone with the container.
	audit.UseDatabase(db)
	// And the same recorder for anything written against the public SDK. An
	// app module cannot import internal/tenant/audit, so it calls
	// nexus.Audit; without this line those events would be logged and dropped
	// while the platform's own reached the table, which is the worst of the
	// three possible states because the trail would look complete.
	nexus.Provide[nexus.AuditSink](audit.Record)
	// Reports arrive the same way and for the same reason: a module declares
	// one against the SDK's contract, and the engine that runs it is inside the
	// platform. Registrations made before this point are buffered and delivered
	// here, so a distribution's main() may construct its modules in whatever
	// order it likes.
	nexus.Provide[nexus.ReportSink](reporting.Register)

	databaseReachable := db.Ping(ctx) == nil
	if !databaseReachable {
		slog.Warn("database unreachable on startup; continuing, /ready reports it")
	} else {
		// Before anything else touches the pool: a probe that ran alongside
		// live queries could leave a connection carrying a role nothing had
		// asked for.
		probeCtx, cancelProbe := context.WithTimeout(ctx, 10*time.Second)
		err := guard.Probe(probeCtx, db)
		cancelProbe()
		if err != nil {
			// The guard could not be switched on safely. That is a deployment
			// fault — the migrations are present but the login role cannot
			// reach the app role — and running on one layer while the schema
			// says there are two is the state worth refusing to start in.
			return fmt.Errorf("failed to enable row-level tenant isolation: %w", err)
		}
	}

	// Redis, when configured, is what makes a permission revoked on one replica
	// stop being honoured by the others, and what makes a rate limit a budget
	// for the deployment rather than one per process. Absent, both fall back to
	// what this platform has always done.
	redisClient := cache.Dial(os.Getenv("REDIS_URL"))
	if redisClient != nil {
		defer func() { _ = redisClient.Close() }()
	}
	busCtx, stopBus := context.WithCancel(context.Background())
	defer stopBus()
	bus := cache.NewBus(busCtx, redisClient)
	slog.Info("cache invalidation configured", "shared", bus.Redis())

	// Initialize Platform Server
	// Before the server is built: NewServer checks the catalogue against this
	// list and refuses to start when an id has no module behind it, which is
	// the check that catches a stale catalogue.
	tenant.SetDefaultApps(opts.DefaultApps)

	srv, err := newServer(db, catalogPath, bus, tenant.ExtraModules(opts.Modules))
	if err != nil {
		return fmt.Errorf("failed to initialize platform server: %w", err)
	}

	// Restores the documented demo login (admin@example.com) and the two
	// organisations it belongs to. Skipped in production unless SEED_DEMO_DATA
	// is set explicitly.
	//
	// After the server rather than before it: seeding now installs apps for
	// those tenants, and an installation row references the apps table, which
	// NewServer is what fills from the catalogue file.
	if databaseReachable {
		seedInitialData(ctx, db, srv.tenant)
		// Says, once, how to get in — the wizard's address with its token, and
		// the command that does the same thing from a terminal. Silent on a
		// deployment that already has an organisation.
		srv.setup.Arm(ctx)
	}

	// Background jobs run until this context is cancelled during shutdown, so
	// a sweep in flight is not left holding a database connection.
	jobsCtx, stopJobs := context.WithCancel(context.Background())
	defer stopJobs()
	if !databaseReachable {
		// The probe above had nothing to talk to. Without this the process
		// would serve every request for the rest of its life with tenant
		// isolation switched off, because the one thing that switches it on
		// already ran and failed.
		async.Go("dbguard-probe-retry", func() {
			guard.ProbeUntilEnabled(jobsCtx, db, 15*time.Second)
		})
	}
	srv.StartBackgroundJobs(jobsCtx)

	httpSrv := &http.Server{
		Addr:              ":" + port,
		Handler:           srv.Router(),
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	// A listener that dies takes the process with it, but through the same exit
	// as a signal does rather than through os.Exit from inside a goroutine —
	// which is what this used to do, and which skipped every deferred close on
	// the way out: the pool, the Redis client, the tracer's flush.
	listenErr := make(chan error, 1)
	async.Go("http-server", func() {
		slog.Info("starting Gerege Nexus API server", "port", port)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErr <- err
		}
	})

	// Graceful shutdown handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	var runErr error
	select {
	case <-stop:
	case err := <-listenErr:
		runErr = fmt.Errorf("server listener: %w", err)
		slog.Error("the listener stopped", "error", err)
	}

	slog.Info("shutting down HTTP API server gracefully...")
	stopJobs()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced server shutdown", "error", err)
	} else {
		slog.Info("server shutdown complete cleanly")
	}
	return runErr
}
