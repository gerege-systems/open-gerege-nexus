/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package apps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
)

// Nothing appears in the navigation unless there is something behind it.
//
// This was internal/workspace/menu's, over a table of screens still to be built
// that only this repository's apps could appear in. The table went on
// 2026-08-23 — a module declares its own entries now, in whichever of its two
// groups it means — and the assertion came here, where the modules are.
//
// It reaches into the frontend on purpose. A menu is declared in Go and
// rendered from a Next.js page, so the drift it catches is exactly the kind no
// single-language test can see.
func TestEveryMenuEntryHasARealPage(t *testing.T) {
	root := repoRoot(t)

	for name, module := range everyModule {
		for _, item := range module.Menus() {
			if item.ExternalURL != "" {
				continue // somewhere else entirely; this repository draws nothing
			}
			if item.Path == "" {
				t.Errorf("%s declares menu %q with no path", name, item.ID)
				continue
			}
			page := filepath.Join(root, "frontend", "app", filepath.FromSlash(item.Path), "page.tsx")
			if _, err := os.Stat(page); err != nil {
				t.Errorf("%s declares menu %q at %s but frontend/app%s/page.tsx does not exist;"+
					" remove the entry or build the screen", name, item.Label, item.Path, item.Path)
			}
		}
	}
}

// A menu label missing a locale does not fail, it falls back to English — which
// is why the gap went unnoticed until a screen showed three languages at once.
// Coverage is therefore asserted rather than left to be noticed.
func TestEveryMenuLabelCoversEverySupportedLocale(t *testing.T) {
	for name, module := range everyModule {
		for _, item := range module.Menus() {
			if item.Label == "" {
				t.Errorf("%s/%s: empty English label", name, item.ID)
			}
			for _, locale := range config.SupportedLocales {
				if locale == "en" {
					continue // en is the fallback and lives in the Label field
				}
				if item.Labels[locale] == "" {
					t.Errorf("%s/%s: no %s translation", name, item.ID, locale)
				}
			}
		}
	}
}

// repoRoot is the directory holding both halves of the repository.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cwd: %v", err)
	}
	for range 8 {
		if _, err := os.Stat(filepath.Join(dir, "frontend", "app")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skip("frontend tree not found next to the backend module; skipping the page-existence check")
	return ""
}
