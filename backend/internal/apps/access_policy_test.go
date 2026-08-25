package apps

import (
	"os"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// What every module compiled into this repository declares to nexus.AccessPolicy.
//
// Empty since 2026-08-25, when sso_clients — the last one — left for the App
// Store. That is the finished state of the split rather than a gap, and the
// table stays because the next module to arrive here has to be classified
// before anybody notices it was not.
var corePolicies = map[string]struct {
	module         nexus.Module
	menu, prefix   string
	whyNoRouteGate string
}{}

func TestEveryCoreModuleDeclaresTheAccessPolicyWeThinkItDoes(t *testing.T) {
	for name, want := range corePolicies {
		t.Run(name, func(t *testing.T) {
			if got := nexus.MenuPermissionOf(want.module); got != want.menu {
				t.Errorf("menu permission: got %q, want %q", got, want.menu)
			}
			if got := nexus.RoutePermissionPrefixOf(want.module); got != want.prefix {
				t.Errorf("route prefix: got %q, want %q", got, want.prefix)
			}
			if want.prefix == "" && want.whyNoRouteGate == "" {
				t.Errorf("a module that declines route gating needs a reason recorded here")
			}
		})
	}
}

// The modules that deliberately declare nothing.
var policylessModules = map[string]nexus.Module{}

// Directories under internal/apps that hold no module at all.
//
// There is one entry's worth of history and no entries. esign was here: the PDF
// rails of the documents app, registering nothing with nexus, listed rather than
// deleted because the directory was still under internal/apps and the count
// below reads directories. It is internal/tenant/signing now — a package that
// answers none of nexus.Module's methods does not belong in the tree of things
// that do — so the classification has nothing to say about it.
//
// The map stays for the next package that ends up in the same position, because
// absent-because-considered and absent-because-forgotten look identical in a
// table.
var nonModulePackages = map[string]string{}

func TestTheModulesWithNoPolicyAreTheOnesWeMeant(t *testing.T) {
	for name, mod := range policylessModules {
		t.Run(name, func(t *testing.T) {
			if got := nexus.MenuPermissionOf(mod); got != "" {
				t.Errorf("expected no menu permission, got %q", got)
			}
			if got := nexus.RoutePermissionPrefixOf(mod); got != "" {
				t.Errorf("expected no route prefix, got %q", got)
			}
		})
	}
}

// The count is the point of this one, and it reads the directories rather than
// trusting a number somebody typed.
//
// The first version compared two hand-written constants. It would have passed
// happily while three modules were deleted from the tree, because both
// constants get edited in the same breath — a test that can only catch somebody
// editing one of its own numbers and not the other is not watching anything.
//
// What it is for: adding a module and never deciding its access policy. The
// module works, its routes mount, and nothing asks whether anyone should be
// able to reach them. That mistake leaves no other trace.
func TestEveryModuleInThisRepositoryIsClassified(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read internal/apps: %v", err)
	}

	unseen := map[string]bool{}
	for name := range corePolicies {
		unseen[name] = true
	}
	for name := range policylessModules {
		unseen[name] = true
	}
	for name := range nonModulePackages {
		unseen[name] = true
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !unseen[entry.Name()] {
			t.Errorf("internal/apps/%s is not classified — add it to corePolicies, "+
				"to policylessModules or to nonModulePackages, having decided "+
				"what gates it", entry.Name())
		}
		delete(unseen, entry.Name())
	}
	for name := range unseen {
		t.Errorf("%s is classified but no longer exists; drop it from the table", name)
	}
}
