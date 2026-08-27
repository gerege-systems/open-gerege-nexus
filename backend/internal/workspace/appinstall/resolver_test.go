package appinstall_test

import (
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/appinstall"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

func TestDependencyGraph_ResolutionAndCycleDetection(t *testing.T) {
	contacts := catalog.Manifest{
		ID:           "io.gerege.nexus.contacts",
		Name:         "Contacts",
		Version:      "1.0.0",
		Dependencies: nil,
	}

	products := catalog.Manifest{
		ID:           "io.gerege.nexus.products",
		Name:         "Products",
		Version:      "1.0.0",
		Dependencies: nil,
	}

	inventory := catalog.Manifest{
		ID:      "io.gerege.nexus.inventory",
		Name:    "Inventory",
		Version: "1.0.0",
		Dependencies: []nexus.Dependency{
			{ID: "io.gerege.nexus.contacts", VersionConstraint: "^1.0.0"},
			{ID: "io.gerege.nexus.products", VersionConstraint: "^1.0.0"},
		},
	}

	t.Run("Happy path: Inventory resolves Contacts and Products first", func(t *testing.T) {
		g := appinstall.NewDependencyGraph([]catalog.Manifest{contacts, products, inventory})
		order, err := g.ResolveInstallOrder("io.gerege.nexus.inventory")
		if err != nil {
			t.Fatalf("expected resolution to succeed, got: %v", err)
		}
		if len(order) != 3 {
			t.Fatalf("expected 3 apps, got %d", len(order))
		}
		if order[len(order)-1] != "io.gerege.nexus.inventory" {
			t.Errorf("inventory must be last, got %v", order)
		}
	})

	t.Run("Missing dependency fails resolution", func(t *testing.T) {
		g := appinstall.NewDependencyGraph([]catalog.Manifest{inventory}) // missing contacts & products
		_, err := g.ResolveInstallOrder("io.gerege.nexus.inventory")
		if err == nil {
			t.Fatal("expected error due to missing dependencies, got nil")
		}
	})

	t.Run("Cycle detection fails resolution", func(t *testing.T) {
		appA := catalog.Manifest{
			ID:           "app.a",
			Dependencies: []nexus.Dependency{{ID: "app.b"}},
		}
		appB := catalog.Manifest{
			ID:           "app.b",
			Dependencies: []nexus.Dependency{{ID: "app.a"}},
		}
		g := appinstall.NewDependencyGraph([]catalog.Manifest{appA, appB})
		_, err := g.ResolveInstallOrder("app.a")
		if err == nil {
			t.Fatal("expected cycle detection error, got nil")
		}
	})
}
