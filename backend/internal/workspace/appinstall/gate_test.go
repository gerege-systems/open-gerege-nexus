/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Which permission a module's route needs, and what happens when it names none.
 *
 * These were beside the role tests while both were in one package. What they
 * are about is the gate a module's routes sit behind.
 */

package appinstall

import (
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// gatedModule and selfGatedModule stand in for the two answers a real module
// can give. They are fakes rather than the real contacts and gov_services
// modules because what is under test here is the platform's half — that it asks
// the module and derives the verb correctly. What each real module answers is
// asserted where the modules live, in internal/apps.
type gatedModule struct{ nexus.Module }

func (gatedModule) ID() string                    { return "io.gerege.test.gated" }
func (gatedModule) MenuPermission() string        { return "gated.read" }
func (gatedModule) RoutePermissionPrefix() string { return "gated" }

type selfGatedModule struct{ nexus.Module }

func (selfGatedModule) ID() string                    { return "io.gerege.test.selfgated" }
func (selfGatedModule) MenuPermission() string        { return "selfgated.read" }
func (selfGatedModule) RoutePermissionPrefix() string { return "" }

// withModules points the lookup at a fixed set for the duration of one test.
//
// Not nexus.Register: the registry is global and has no remove, so a fake left
// in it is found by the next test that builds a Server, which mounts the routes
// of everything registered and dereferences the fake's embedded nil Module. The
// first version of this file did exactly that, and it passed locally — the test
// it broke needs a database and had quietly skipped.
func withModules(t *testing.T, mods ...nexus.Module) {
	t.Helper()
	byID := make(map[string]nexus.Module, len(mods))
	for _, m := range mods {
		byID[m.ID()] = m
	}
	previous := lookupModule
	lookupModule = func(id string) (nexus.Module, bool) {
		m, ok := byID[id]
		return m, ok
	}
	t.Cleanup(func() { lookupModule = previous })
}

func TestAppRequestPermission(t *testing.T) {
	withModules(t, gatedModule{}, selfGatedModule{})

	if got := appRequestPermission("io.gerege.test.gated", "GET", "/x"); got != "gated.read" {
		t.Fatalf("got %q", got)
	}
	if got := appRequestPermission("io.gerege.test.gated", "HEAD", "/x"); got != "gated.read" {
		t.Fatalf("HEAD reads: got %q", got)
	}
	if got := appRequestPermission("io.gerege.test.gated", "POST", "/x"); got != "gated.manage" {
		t.Fatalf("got %q", got)
	}
	// An empty prefix is a decision, not a gap: the module gates its own
	// routes because the verb cannot express its rule.
	if got := appRequestPermission("io.gerege.test.selfgated", "POST", "/x"); got != "" {
		t.Fatalf("a module that declines route gating must not get one: %q", got)
	}
	// A module nobody registered is not a module with no permissions — it is
	// not in this binary at all, and its routes are not mounted either.
	if got := appRequestPermission("io.gerege.test.absent", "POST", "/x"); got != "" {
		t.Fatalf("got %q", got)
	}
}

// A module that predates nexus.AccessPolicy — it implements Module and nothing
// more. It must keep working, ungated, rather than fail to compile or panic:
// the interface is optional, and "optional" has to be true for a distribution
// pinned to an older SDK.
type policylessModule struct{ nexus.Module }

func (policylessModule) ID() string { return "io.gerege.test.policyless" }

func TestAModuleWithoutAnAccessPolicyIsNotGated(t *testing.T) {
	withModules(t, policylessModule{})
	if got := appRequestPermission("io.gerege.test.policyless", "POST", "/x"); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := appReadPermission("io.gerege.test.policyless"); got != "" {
		t.Fatalf("got %q", got)
	}
}

// The claim this file used to make about documents, esign and gov_services —
// that none of them accepts a platform-derived permission, because each checks
// at route registration and in its handlers — is now asserted against the real
// modules in internal/apps/access_policy_test.go. It belongs there: it is a
// statement about what those modules declare, and the platform's side of it is
// covered above with fakes.
