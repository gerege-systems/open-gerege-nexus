/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The HTTP surface of this binary, written down.
 *
 * setupRoutes is three hundred and sixty lines long and mounts close to a
 * hundred routes. Adding one more is a single line in the middle of it, and
 * nothing in a review distinguishes that line from the twenty around it — which
 * is how the core grows: not by anybody deciding to widen it, but by every app
 * that needs an endpoint finding the cheapest place to put one, here.
 *
 * So the route table is a golden file. Adding a route is allowed and often
 * right — this is not a freeze — but it cannot happen unnoticed, and the diff
 * in a review is the plain statement of what the platform's surface gained or
 * lost.
 *
 *	go test ./pkg/host -run TestTheRouteTable -update
 *
 * App module routes are in the file too, behind an "[app] " marker. They are
 * mounted by registerAppModuleRoutes rather than written into setupRoutes, so
 * a change to them means an app changed and a change to the unmarked lines
 * means the platform did. Two different facts; the marker keeps the diff from
 * saying both at once.
 */

package host

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/go-chi/chi/v5"
)

var updateRoutes = flag.Bool("update", false, "rewrite testdata/routes.txt from the current router")

func TestTheRouteTableIsTheOneOnRecord(t *testing.T) {
	got := renderRoutes(t, routerUnderTest(t))
	golden := filepath.Join("testdata", "routes.txt")

	if *updateRoutes {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Log("wrote", golden)
		return
	}

	want, err := os.ReadFile(golden) // #nosec G304 -- a fixed path beside the test
	if err != nil {
		t.Fatalf("read the recorded route table: %v", err)
	}
	if got != string(want) {
		t.Errorf(`the route table of this binary is not what is on record.

Every line here is a piece of the platform's public surface. If the change is
deliberate, re-record it with

    go test ./pkg/host -run TestTheRouteTable -update

and say in the commit message what the surface gained and why it belongs in the
core rather than in an app. If it is not deliberate, this is the accident the
file exists to catch.

Lines marked "[app] " come from registerAppModuleRoutes and move when an app
does. Unmarked lines are setupRoutes — the platform itself.

%s`, diffRoutes(string(want), got))
	}
}

// renderRoutes prints every route the router serves, one per line, sorted.
//
// The pattern is printed as chi holds it, trailing "/*" and all: a mount point
// admitting a whole subtree is a different fact from a leaf, and flattening the
// two would hide the wider one.
func renderRoutes(t *testing.T, router chi.Routes) string {
	t.Helper()

	app := appModulePatterns(t)

	var lines []string
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		line := method + " " + route
		if app[line] {
			line = "[app] " + line
		}
		lines = append(lines, line)
		return nil
	})
	if err != nil {
		t.Fatalf("walk routes: %v", err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n"
}

// appModulePatterns is what registerAppModuleRoutes would mount, on its own.
//
// The compiled modules are asked to register against an empty router instead of
// being subtracted from the real one afterwards, because they are handed the
// root router and mount absolute paths: what they claim is the same either way,
// and asking them directly cannot go stale when setupRoutes changes.
func appModulePatterns(t *testing.T) map[string]bool {
	t.Helper()

	bare := chi.NewRouter()
	passthrough := func(next http.Handler) http.Handler { return next }
	for _, module := range nexus.List() {
		module.RegisterRoutes(bare, passthrough)
	}

	found := map[string]bool{}
	err := chi.Walk(bare, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		found[method+" "+route] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk the app module routes: %v", err)
	}
	return found
}

// diffRoutes lists the lines each side has and the other does not.
func diffRoutes(want, got string) string {
	inWant := map[string]bool{}
	for _, line := range strings.Split(want, "\n") {
		inWant[line] = true
	}
	inGot := map[string]bool{}
	for _, line := range strings.Split(got, "\n") {
		inGot[line] = true
	}

	var b strings.Builder
	for _, line := range strings.Split(got, "\n") {
		if line != "" && !inWant[line] {
			fmt.Fprintf(&b, "+ %s\n", line)
		}
	}
	for _, line := range strings.Split(want, "\n") {
		if line != "" && !inGot[line] {
			fmt.Fprintf(&b, "- %s\n", line)
		}
	}
	return b.String()
}
