/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
	withdrawn[nexus.RateLimiter]()
	withdrawn[nexus.MeetingBooker]()
	withdrawn[nexus.Link]()
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
		provided[nexus.StateRails](t)
		provided[nexus.RateLimiter](t)
		provided[nexus.MeetingBooker](t)
		provided[nexus.Link](t)
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
