package menu

import (
	"context"
	"sort"
	"strings"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// defaultMenuOrder is where an entry sits when its module expresses no
// preference: after anything that asked to come first, before the blueprint
// entries, which start at 20.
const defaultMenuOrder = 10

type InstalledAppStore interface {
	GetEnabledAppIDsForTenant(context.Context, string) ([]string, error)
	// GetCatalog is what external apps are read from. They have no compiled
	// module to ask for menus, so their manifest is the only place their
	// navigation exists.
	GetCatalog() []catalog.CatalogApp
}

func GetTenantMenus(ctx context.Context, store InstalledAppStore, tenantID, locale string) ([]nexus.MenuDefinition, error) {
	enabledIDs, err := store.GetEnabledAppIDsForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	enabled := map[string]bool{}
	for _, id := range enabledIDs {
		enabled[id] = true
	}
	// The manifests, by app id. What the shell needs from them is where an app
	// sits in the rail and whether it is drawn as part of the shell at all —
	// both properties of the app, both of which used to be lists in
	// frontend/components/Layout.tsx.
	manifests := map[string]catalog.Manifest{}
	for _, app := range store.GetCatalog() {
		manifests[app.ID] = app.Manifest
	}

	menus := make([]nexus.MenuDefinition, 0)
	for _, mod := range nexus.List() {
		if !enabled[mod.ID()] {
			continue
		}
		manifest := manifests[mod.ID()]
		// A module with no blueprint still has screens: the ones it registers
		// itself. Skipping it outright is how an app ships with three working
		// pages and nothing in the sidebar pointing at them, which is exactly
		// what happened to the organisation app — a blueprint lists the entries
		// still to be built, so having none of those is an ordinary state, not
		// a reason to go unlisted.
		slug := routeSlug(mod.ID())
		modulesID, settingsID := slug+"_modules", slug+"_settings"
		before := len(menus)
		menus = append(menus,
			localized(nexus.MenuDefinition{ID: modulesID, AppID: mod.ID(), AppName: mod.Name(), Label: "Modules", Icon: "boxes", Order: 10, Labels: groupModules}, locale),
			localized(nexus.MenuDefinition{ID: settingsID, AppID: mod.ID(), AppName: mod.Name(), Label: "Settings", Icon: "settings", Order: 20, Labels: groupSettings}, locale),
		)
		for _, item := range mod.Menus() {
			// The parent is the platform's to decide and the group is the
			// module's: an app has two headers and only it knows which of them
			// a screen belongs under. Anything else — the order, the label —
			// stays as the module declared it. The order used to be overwritten
			// with 10 for every entry, which left the organisation app's
			// screens sorting equal and coming out in whatever order the sort
			// happened to leave them, changing between builds.
			parent := modulesID
			if item.Group == nexus.MenuGroupSettings {
				parent = settingsID
			}
			item.AppID, item.AppName, item.ParentID = mod.ID(), mod.Name(), parent
			if item.Order == 0 {
				item.Order = defaultMenuOrder
			}
			menus = append(menus, localized(item, locale))
		}
		// Stamped afterwards rather than threaded through four construction
		// sites: it is the same answer for every row this module contributed.
		for i := before; i < len(menus); i++ {
			menus[i].AppOrder, menus[i].AppChrome = manifest.Order, manifest.Chrome
		}
	}
	// External apps: a third-party service the tenant has installed. There is no
	// Go module behind them, so what they contribute is exactly what their
	// manifest declares — usually one entry pointing out of this platform
	// altogether.
	for _, app := range store.GetCatalog() {
		if !app.Manifest.IsExternal() || !enabled[app.ID] {
			continue
		}
		modulesID := app.Slug + "_modules"
		// No Chrome: ValidateManifest refuses it on an external app, which runs
		// somewhere else and cannot be part of this shell.
		menus = append(menus,
			localized(nexus.MenuDefinition{ID: modulesID, AppID: app.ID, AppName: app.Name, Label: "Modules", Icon: "boxes", Order: 10, AppOrder: app.Manifest.Order, Labels: groupModules}, locale))
		for _, item := range app.Manifest.Menus {
			item.AppID, item.AppName, item.ParentID = app.ID, app.Name, modulesID
			item.AppOrder = app.Manifest.Order
			if item.Order == 0 {
				item.Order = defaultMenuOrder
			}
			menus = append(menus, localized(item, locale))
		}
	}

	// Stable, so entries that share an order keep the order their module
	// declared them in rather than one the sort invented.
	sort.SliceStable(menus, func(i, j int) bool {
		if menus[i].AppID != menus[j].AppID {
			return menus[i].AppID < menus[j].AppID
		}
		if menus[i].ParentID != menus[j].ParentID {
			return menus[i].ParentID < menus[j].ParentID
		}
		return menus[i].Order < menus[j].Order
	})
	return menus, nil
}

// routeSlug is the last segment of an app id — io.gerege.nexus.organisation -> organisation — which
// is the convention every blueprint slug already follows.
func routeSlug(appID string) string {
	slug := appID
	if idx := strings.LastIndex(appID, "."); idx >= 0 {
		slug = appID[idx+1:]
	}
	return strings.ReplaceAll(slug, "_", "-")
}

func localized(item nexus.MenuDefinition, locale string) nexus.MenuDefinition {
	item.Label = item.LocalizedLabel(locale)
	return item
}

// The two group headers every app's menu hangs under. They are the platform's
// words rather than an app's: every app has the same two, and an app that could
// name them would be an app that could call its settings something else.
var (
	groupModules = map[string]string{
		"mn": "Модуль", "ar": "الوحدات", "zh": "模块", "fr": "Modules", "ru": "Модули", "es": "Módulos"}
	groupSettings = map[string]string{
		"mn": "Тохиргоо", "ar": "الإعدادات", "zh": "设置", "fr": "Paramètres", "ru": "Настройки", "es": "Configuración"}
)

// futureDefinition turned a blueprint entry into a menu row until 2026-08-23.
// A module declares its own entries now, in whichever of its two groups it
// means — see nexus.MenuDefinition.Group — and internal/workspace/menu no longer
// holds a table of screens keyed by app id that only this repository's apps
// could appear in.
