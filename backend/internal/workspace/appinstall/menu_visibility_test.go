package appinstall

import (
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

func TestAMenuEntryIsHiddenBehindItsOwnPermission(t *testing.T) {
	menus := []nexus.MenuDefinition{
		{ID: "fuel_stations", AppID: "fuel"},
		{ID: "fuel_oversight", AppID: "fuel", Permission: "fuel.oversight"},
		{ID: "hr_people", AppID: "hr"},
	}
	appPermission := func(appID string) string {
		if appID == "hr" {
			return "hr.read"
		}
		return ""
	}

	ids := func(items []nexus.MenuDefinition) []string {
		out := make([]string, 0, len(items))
		for _, item := range items {
			out = append(out, item.ID)
		}
		return out
	}

	got := ids(visibleMenus(append([]nexus.MenuDefinition(nil), menus...), map[string]bool{}, appPermission))
	if len(got) != 1 || got[0] != "fuel_stations" {
		t.Fatalf("with no grants saw %v, want only fuel_stations", got)
	}

	got = ids(visibleMenus(append([]nexus.MenuDefinition(nil), menus...),
		map[string]bool{"fuel.oversight": true, "hr.read": true}, appPermission))
	if len(got) != 3 {
		t.Fatalf("with every grant saw %v, want all three", got)
	}
}
