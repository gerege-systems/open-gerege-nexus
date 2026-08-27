/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package workspace

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/cache"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/flags"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// A distribution's module is built after the platform has published what a
// module may ask for.
//
// This is the ordering bug the e-Government move found. ExtraModules used to be
// called at the top of NewServer, before a single Provide — so a module that
// asked for the state registry in its constructor, the way every module in this
// repository does, got nothing. Nothing failed: nexus.Capability returns a zero
// value and an error, the module logged a warning and served a degraded screen,
// and the deployment had the rail all along.
//
// A golden route file cannot see this and neither can a build: the signature is
// identical either way. So the property is asserted directly — from inside an
// ExtraModules callback, which is the only place it can be observed.
func TestADistributionsModuleIsBuiltAfterTheCapabilitiesExist(t *testing.T) {
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to a migrated test database to run this test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	t.Setenv("APP_CATALOG_URL", "")

	// Emptied first, or this test cannot fail.
	//
	// nexus's registry is package-level and nothing resets it between tests, so
	// by the time this one runs an earlier NewServer in the same binary —
	// app_gate_test.go's, sorted first — has already provided every one of
	// these. Asking inside the callback would then answer from that server
	// however this one is ordered: the whole package passed with the loop back
	// at the top of NewServer, while the same test alone failed on ten
	// capabilities. Withdrawing makes the answer this NewServer's.
	//
	// Not restored: NewServer provides all of them again a few lines below, and
	// from the server this test built rather than the one an earlier test
	// abandoned to a closed pool.
	withdrawn[nexus.StateRegistry]()
	withdrawn[nexus.AuditReader]()
	withdrawn[nexus.Directory]()
	withdrawn[nexus.ReportEngine]()
	withdrawn[nexus.EIDSigner]()
	withdrawn[nexus.DANAuthenticator]()
	withdrawn[nexus.Signer]()
	withdrawn[nexus.SecretSealer]()
	withdrawn[nexus.RateLimiter]()
	withdrawn[nexus.SigningRails]()
	withdrawn[nexus.Quota]()

	// Everything a module carried by another repository reaches for today,
	// asserted inside the callback because that is the moment in question:
	// asking again afterwards would pass however NewServer is ordered.
	//
	// nexus.StateRails is checked but not withdrawn above: it is a func type,
	// and Provide only deletes a capability when the value boxes to a nil
	// interface, which a nil func does not. Withdrawing it would store a nil
	// func for the next caller to invoke.
	var called bool
	register := func(nexus.Platform) {
		called = true
		provided[nexus.StateRegistry](t)
		provided[nexus.AuditReader](t)
		provided[nexus.Directory](t)
		provided[nexus.ReportEngine](t)
		provided[nexus.EIDSigner](t)
		provided[nexus.DANAuthenticator](t)
		provided[nexus.Signer](t)
		provided[nexus.SecretSealer](t)
		provided[nexus.StateRails](t)
		provided[nexus.RateLimiter](t)
		provided[nexus.SigningRails](t)
		// The assistant is an app now and asks for this in its constructor; a
		// distribution's assistant would ask at the same moment.
		provided[nexus.Quota](t)
	}

	if _, err := New(Deps{
		DB: pool, Bus: cache.NewBus(ctx, nil),
		Settings:    settings.NewStore(pool),
		Flags:       flags.NewStore(pool),
		CatalogPath: filepath.FromSlash("../../../catalog/apps.json"),
		Modules:     []ExtraModules{register},
	}); err != nil {
		t.Fatalf("build the server: %v", err)
	}
	if !called {
		t.Fatal("the ExtraModules callback was never called")
	}
}

// withdrawn removes a capability, so that a later Capability call answers about
// what has been provided since rather than about what some other test left in
// the registry. Interface types only — see the note at the call site.
func withdrawn[T any]() {
	var none T
	nexus.Provide(none)
}

// A module's background work starts after its tables exist.
//
// StartBackgroundJobs starts the periodic work of every registered module, and
// the same function applies the catalogue — which is what runs a module's own
// migrations (AppInstaller.MigrateModules). Started in the wrong order, a
// module whose tables arrived with this release sweeps a schema that is not
// there yet.
//
// That is not hypothetical: client.gerege.mn's v1.15.0 rollout logged
// `relation "urtuu_peers" does not exist` six times in the 600ms before the
// catalogue sweep created them. Accurate, self-healing on the next tick, and
// indistinguishable in a log from the deployment having failed.
//
// Nothing about the signatures records the order, so a module observes it from
// inside its own StartHousekeeping: it asks whether the table its migration
// creates is there yet.
func TestAModulesHousekeepingStartsAfterItsMigrations(t *testing.T) {
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to a migrated test database to run this test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	t.Setenv("APP_CATALOG_URL", "")

	// A clean slate, so the answer is about this run rather than about a
	// previous one.
	//
	// The history is dropped by its qualified name on purpose: goose is handed
	// `public.goose_db_version_<slug>` and the search_path has had no public in
	// it since migration 00080, so an unqualified DROP here silently removes
	// nothing — leaving a history that says version 1 is applied and a table
	// that this cleanup removed. The second run of this test then sees no
	// table and blames the ordering.
	if err := dropProbeTables(ctx, pool); err != nil {
		t.Fatalf("clear the probe: %v", err)
	}

	module := &sweepOrderModule{db: pool}
	server, err := New(Deps{
		DB: pool, Bus: cache.NewBus(ctx, nil),
		Settings:    settings.NewStore(pool),
		Flags:       flags.NewStore(pool),
		CatalogPath: filepath.FromSlash("../../../catalog/apps.json"),
		Modules: []ExtraModules{func(nexus.Platform) {
			nexus.Register(module)
			nexus.Migrations(module.ID(), probeSchema())
		}},
	})
	if err != nil {
		t.Fatalf("build the server: %v", err)
	}
	t.Cleanup(func() { _ = dropProbeTables(ctx, pool) })

	jobs, stop := context.WithCancel(ctx)
	defer stop()
	server.StartBackgroundJobs(jobs)

	if !module.started {
		t.Fatal("the module's StartHousekeeping was never called")
	}
	if !module.tableExisted {
		t.Error("housekeeping started before the catalogue sweep applied the module's " +
			"migrations, so its first sweep runs against tables that do not exist yet")
	}
}

// dropProbeTables removes both halves of the probe: the table its migration
// creates, and the history that would otherwise say the migration has run.
func dropProbeTables(ctx context.Context, pool *pgxpool.Pool) error {
	for _, stmt := range []string{
		`DROP TABLE IF EXISTS sweep_order_probe`,
		`DROP TABLE IF EXISTS public.goose_db_version_sweep_order_probe`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// probeSchema is one migration creating one table.
func probeSchema() fs.FS {
	const up = `-- +goose Up
CREATE TABLE IF NOT EXISTS sweep_order_probe (id INT PRIMARY KEY);
-- +goose Down
DROP TABLE IF EXISTS sweep_order_probe;
`
	return fstest.MapFS{"00001_probe.sql": &fstest.MapFile{Data: []byte(up)}}
}

// sweepOrderModule is a module that only remembers what it saw.
type sweepOrderModule struct {
	db           *pgxpool.Pool
	started      bool
	tableExisted bool
}

func (m *sweepOrderModule) ID() string                       { return "io.gerege.nexus.sweep_order_probe" }
func (m *sweepOrderModule) Name() string                     { return "Sweep order probe" }
func (m *sweepOrderModule) Version() string                  { return "1.0.0" }
func (m *sweepOrderModule) Dependencies() []nexus.Dependency { return nil }
func (m *sweepOrderModule) Permissions() []nexus.PermissionDefinition {
	return nil
}
func (m *sweepOrderModule) Menus() []nexus.MenuDefinition                              { return nil }
func (m *sweepOrderModule) RegisterRoutes(chi.Router, func(http.Handler) http.Handler) {}

// StartHousekeeping asks the one question this test exists for.
func (m *sweepOrderModule) StartHousekeeping(ctx context.Context) {
	m.started = true
	var name *string
	if err := m.db.QueryRow(ctx, `SELECT to_regclass('sweep_order_probe')::text`).Scan(&name); err != nil {
		return
	}
	m.tableExisted = name != nil
}
