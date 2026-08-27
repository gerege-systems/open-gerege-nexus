package menu_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/menu"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/go-chi/chi/v5"
)

// A module used to have to appear in blueprints.go before any of its menus were
// shown — including the ones it declares itself. That is a silent failure: the
// app installs, its screens work, and nothing in the sidebar points at them.
// This is the module that has no blueprint, standing in for the next one.
type blueprintlessModule struct{}

func (blueprintlessModule) ID() string                       { return "io.gerege.nexus.noblueprint" }
func (blueprintlessModule) Name() string                     { return "No Blueprint" }
func (blueprintlessModule) Version() string                  { return "1.0.0" }
func (blueprintlessModule) Dependencies() []nexus.Dependency { return nil }
func (blueprintlessModule) Permissions() []nexus.PermissionDefinition {
	return nil
}
func (blueprintlessModule) Menus() []nexus.MenuDefinition {
	// Declared out of order on purpose: what comes back must follow Order, not
	// the order they were written in.
	return []nexus.MenuDefinition{
		{ID: "noblueprint_third", Label: "Third", Path: "/noblueprint/c", Icon: "box", Order: 7},
		{ID: "noblueprint_home", Label: "Home", Path: "/noblueprint", Icon: "box", Order: 5},
		{ID: "noblueprint_second", Label: "Second", Path: "/noblueprint/b", Icon: "box", Order: 6},
	}
}
func (blueprintlessModule) RegisterRoutes(chi.Router, func(http.Handler) http.Handler) {}

type enabledStore struct{ ids []string }

func (s enabledStore) GetEnabledAppIDsForTenant(context.Context, string) ([]string, error) {
	return s.ids, nil
}
func (s enabledStore) GetCatalog() []catalog.CatalogApp { return nil }

func TestAModuleWithoutABlueprintStillContributesItsOwnScreens(t *testing.T) {
	mod := blueprintlessModule{}
	nexus.Register(mod)

	menus, err := menu.GetTenantMenus(context.Background(),
		enabledStore{ids: []string{mod.ID()}}, "tenant", "en")
	if err != nil {
		t.Fatalf("menus: %v", err)
	}

	var found *nexus.MenuDefinition
	for i := range menus {
		if menus[i].ID == "noblueprint_home" {
			found = &menus[i]
		}
	}
	if found == nil {
		t.Fatalf("the module's own screen is missing from the sidebar: %+v", menus)
	}
	// Hung under the app's Modules group, with the slug taken from the app id,
	// exactly as a module that does have a blueprint would be.
	if found.ParentID != "noblueprint_modules" {
		t.Fatalf("expected the entry under the app's Modules group, got parent %q", found.ParentID)
	}
	if found.AppName != mod.Name() {
		t.Fatalf("expected the entry to name its app, got %q", found.AppName)
	}
}

// A module says what order its screens read in, and that used to be thrown
// away: every entry was rewritten to the same order and then sorted with an
// unstable sort, so core's three screens came out as organisation, people,
// departments — or any other arrangement, changing between builds.
func TestAModulesMenusKeepTheOrderItDeclared(t *testing.T) {
	mod := blueprintlessModule{}
	nexus.Register(mod)

	menus, err := menu.GetTenantMenus(context.Background(),
		enabledStore{ids: []string{mod.ID()}}, "tenant", "en")
	if err != nil {
		t.Fatalf("menus: %v", err)
	}

	var got []string
	for _, item := range menus {
		if item.AppID == mod.ID() && item.ParentID != "" {
			got = append(got, item.ID)
		}
	}
	want := []string{"noblueprint_home", "noblueprint_second", "noblueprint_third"}
	if len(got) != len(want) {
		t.Fatalf("expected %d entries, got %v", len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("the module's order was not kept:\n got %v\nwant %v", got, want)
		}
	}
}
