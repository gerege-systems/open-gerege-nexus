/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package appcatalog_test

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/appcatalog"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"
)

// The order the shell draws the app rail in, and which app it draws as itself.
//
// Both used to be constants in frontend/components/Layout.tsx: a list of nine
// app ids in the order they should appear, and one id named as the app rendered
// inside the platform's own menu group. Six of the nine had already left this
// repository and the list still named them, which is what a hard-coded list of
// app ids does — it cannot be wrong loudly.
//
// They are manifest fields now, so this is where the answer is checked. The
// expectation below is what the shell drew on the day the constants were
// removed; a manifest changing it is allowed and has to change this line too,
// which is the point.
func TestTheAppRailIsOrderedByTheManifests(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	apps, err := appcatalog.LoadFile(filepath.Join(root, "catalog", "apps.json"), "")
	if err != nil {
		t.Fatalf("load the bundled catalogue: %v", err)
	}

	// Exactly the sort the shell applies: by declared order, then by id among
	// the apps that declared none. 999 is the same "no opinion" position the
	// list it replaced used.
	const unordered = 999
	type app struct {
		id     string
		order  int
		chrome bool
	}
	list := make([]app, 0, len(apps))
	for _, entry := range apps {
		order := entry.Manifest.Order
		if order == 0 {
			order = unordered
		}
		list = append(list, app{entry.ID, order, entry.Manifest.Chrome})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].order != list[j].order {
			return list[i].order < list[j].order
		}
		return list[i].id < list[j].id
	})

	var rail []string
	var chrome []string
	for _, entry := range list {
		if entry.chrome {
			chrome = append(chrome, entry.id)
			continue
		}
		rail = append(rail, entry.id)
	}

	// Empty since 2026-08-25, when sso_clients left for the App Store and this
	// repository stopped shipping apps of its own. The rail a deployment
	// actually draws is whatever its catalogue carries; what is asserted here
	// is that the core contributes nothing to it.
	wantRail := []string{
		// No order declared, so id order among themselves — which is where the
		// list this replaced also put them, by falling through to 999. egov,
		// documents and the organisation were here until they moved to
		// client-gerege-nexus on 2026-08-23.
		// Өртөө and reports were here until 2026-08-23, when both left for
		// client-gerege-nexus. The channel one ran on and the engine the other
		// ran on did not: a deployment can be on the ring with no task board,
		// and can mail a schedule with no screen to make one.
	}
	if len(rail) != len(wantRail) {
		t.Fatalf("the rail holds %v, want %v", rail, wantRail)
	}
	for i := range rail {
		if rail[i] != wantRail[i] {
			t.Errorf("rail position %d is %s, want %s (full order: %v)", i, rail[i], wantRail[i], rail)
		}
	}

	// The assistant is chrome, and it is the only thing that is. The
	// organisation was — the shell drew its screens as part of itself rather
	// than as a tile on the rail — and it is a distribution's app now.
	//
	// The assistant claims it for a different reason: it has no screens at all.
	// It is reached from the shell's chat affordance, so a tile on the rail
	// would be a tile that opens nothing. The flag is what keeps an app out of
	// the rail without pretending it is not installed.
	// All three for the same reason, and it is not the organisation's: none has
	// a screen of its own. The assistant is reached from the shell's chat
	// affordance, the connectors from /settings/integrations, and a staff PIN
	// is set from the member's row in Access control — all three shell screens.
	// A tile on the rail would open nothing in every case.
	wantChrome := []string{}
	if len(chrome) != len(wantChrome) {
		t.Fatalf("the shell draws %v as part of itself, want %v", chrome, wantChrome)
	}
	for i := range chrome {
		if chrome[i] != wantChrome[i] {
			t.Errorf("chrome position %d is %s, want %s", i, chrome[i], wantChrome[i])
		}
	}
}

// An external app cannot claim to be part of this shell.
//
// It runs somewhere else and is reached by handing the user over, so drawing it
// as the platform's own navigation would be a claim the platform cannot keep.
// Refused at validation rather than ignored at render, because a manifest that
// asks for something it does not get is a manifest whose author believes
// something untrue.
func TestAnExternalAppMayNotBeShellChrome(t *testing.T) {
	manifest := catalog.Manifest{
		ID: "com.example.thing", Name: "Thing", Version: "1.0.0",
		Type:     catalog.TypeExternal,
		External: &catalog.ExternalSpec{LaunchURL: "https://example.com/app"},
		Chrome:   true,
	}
	if err := catalog.ValidateManifest(manifest, ""); err == nil {
		t.Error("an external app was accepted as shell chrome")
	}
}
