/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package appinstall

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/menu"
	"github.com/go-chi/chi/v5"
)

func (h *Handlers) HandleMenus(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := nexus.RequireTenant(w, r)
	if !ok {
		return
	}

	menus, err := menu.GetTenantMenus(r.Context(), h.installer, tenantID, config.LocaleFromRequest(r))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to fetch menus")
		return
	}
	claims, _ := auth.UserFromContext(r.Context())
	if !claims.IsAdmin {
		permissions, permissionErr := h.permissions.GetUserPermissions(r.Context(), tenantID, claims.UserID)
		if permissionErr != nil {
			httpx.Error(w, http.StatusInternalServerError, "failed to resolve menu access")
			return
		}
		visible := menus[:0]
		for _, item := range menus {
			permission := h.appReadPermission(item.AppID)
			if permission == "" || permissions[permission] {
				visible = append(visible, item)
			}
		}
		menus = visible
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(menus)
}

// appReadPermission decides which permission a menu entry is hidden behind.
//
// An external app has no Go module to ask — the whole point is that it arrives
// from a registry rather than from a compiler — so its manifest answers for it.
// A tenant that installs somebody else's HRMS is not thereby putting a link to
// it in front of every member of the organisation.
func (h *Handlers) appReadPermission(appID string) string {
	if app, found := h.installer.GetAppByID(appID); found && app.Manifest.IsExternal() {
		for _, permission := range app.Manifest.Permissions {
			if strings.HasSuffix(permission.Code, ".read") {
				return permission.Code
			}
		}
		// A manifest that asks for nothing is visible to the tenant that
		// installed it — the same answer a module gives when it declares no
		// menu permission.
		return ""
	}
	return appReadPermission(appID)
}

// lookupModule is nexus.Get behind a seam.
//
// The seam is for tests. Both functions below resolve an app ID to a module,
// and a test that wants to assert on a made-up module would otherwise have to
// put it in the global registry — where the next test to build a Server finds
// it and tries to mount its routes. That is not hypothetical: it is how this
// variable came to exist.
var lookupModule = nexus.Get

// appReadPermission asks the module. It used to be a switch listing every app
// by name, which meant a module in another repository could not answer for
// itself — and losing the entry was silent, so an extracted app would keep
// appearing in everyone's sidebar. See nexus.AccessPolicy.
func appReadPermission(appID string) string {
	mod, found := lookupModule(appID)
	if !found {
		return ""
	}
	return nexus.MenuPermissionOf(mod)
}

// runnableHere reports whether this binary could actually run the app.
//
// The catalogue is not this repository's list. It arrives signed from a store
// that serves every deployment in the field, so it advertises apps built from
// other repositories — and after a distribution split it goes on advertising
// the app that left for as long as the store's document says so, which is a
// decision somebody makes deliberately rather than a side effect of a merge.
//
// Offering one of those is offering a button that cannot work: the installer
// checks for a compiled module and refuses, correctly, with an error about
// binary registries that means nothing to the administrator who pressed
// Install. Not listing it is the honest answer — this deployment does not have
// that app, and the fix is to run a distribution that does.
//
// External apps are the deliberate exception. They have no Go module by
// definition, because they are somebody else's running service reached over
// OIDC; requiring one would hide the whole category.
func runnableHere(app catalog.CatalogApp) bool {
	if app.Manifest.IsExternal() {
		return true
	}
	_, compiled := lookupModule(app.ID)
	return compiled
}

func (h *Handlers) HandleListStoreApps(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := nexus.TenantID(r.Context())
	available := h.installer.GetCatalog()

	// "installed" and "enabled" are distinct states: an app can be installed
	// and then disabled. Deriving both from the enabled-only query reported
	// disabled apps as never installed, so the UI offered "Install" again.
	installedStates, err := h.installer.GetInstallationsForTenant(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load installed apps")
		return
	}

	type StoreAppResponse struct {
		catalog.CatalogApp
		Installed bool `json:"installed"`
		Enabled   bool `json:"enabled"`
		// What this tenant is running, and what the catalogue carries. They
		// were the same number for the life of the platform because an
		// installation's version never moved; now that it does, the store is
		// where the difference has to show.
		InstalledVersion string `json:"installed_version,omitempty"`
		LatestVersion    string `json:"latest_version"`
		UpdateAvailable  bool   `json:"update_available"`
	}

	locale := config.LocaleFromRequest(r)
	res := make([]StoreAppResponse, 0, len(available))
	for _, app := range available {
		if !runnableHere(app) {
			continue
		}
		held, installed := installedStates[app.ID]
		res = append(res, StoreAppResponse{
			CatalogApp:       app.Localized(locale),
			Installed:        installed,
			Enabled:          held.Enabled,
			InstalledVersion: held.Version,
			LatestVersion:    app.Version,
			UpdateAvailable:  installed && catalog.IsNewerVersion(app.Version, held.Version),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (h *Handlers) HandleGetStoreApp(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !security.IsValidSlug(slug) {
		httpx.Error(w, http.StatusBadRequest, "invalid app slug format")
		return
	}

	// Same rule as the list, for the same reason: an app this binary cannot run
	// is not an app this deployment has. Answering 404 rather than a detail
	// page keeps a direct link from offering an Install button that the
	// installer will refuse.
	app, ok := h.installer.GetAppBySlug(slug)
	if !ok || !runnableHere(app) {
		httpx.Error(w, http.StatusNotFound, "app not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(app.Localized(config.LocaleFromRequest(r)))
}

// presentableInstallation reports whether an installation row names an app this
// deployment can actually run.
//
// It exists because the installed-apps screen was the one place the split was
// still visible as a lie. The store stopped offering apps this binary cannot run
// (see runnableHere); the list of what a tenant *has* went on showing four of
// them — State Services, Products, Inventory, Billing — as installed, active,
// with a button to disable them. They mount no routes and appear in no menu, so
// nothing about them works; the row is all that is left, and a row that says
// "Идэвхтэй" about an app with no code is worse than no row at all.
//
// Two questions, in this order. If the catalogue knows the app, runnableHere is
// the same answer the store gives — one rule, so the two screens cannot
// disagree. If the catalogue has never heard of it, a compiled module is enough:
// a distribution's own module is real and working from the moment the binary
// starts, and may reach the catalogue minutes later or never.
//
// The rows themselves are deliberately left in the database. Migration 00058's
// note explains the reasoning where an app was absorbed; here the apps went to
// distributions that this deployment may yet run, and deleting an installation
// because the screen cannot render it is the wrong way round.
func (h *Handlers) presentableInstallation(appID string) bool {
	if app, ok := h.installer.GetAppByID(appID); ok {
		return runnableHere(app)
	}
	_, compiled := lookupModule(appID)
	return compiled
}

func (h *Handlers) HandleListInstalledApps(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := nexus.RequireTenant(w, r)
	if !ok {
		return
	}

	rows, err := h.db.Query(r.Context(),
		`SELECT ai.id, ai.app_id, a.slug, a.name, ai.installed_version, ai.status, ai.enabled,
		        ai.installed_at, ai.auto_update, COALESCE(ai.pinned_version, ''),
		        COALESCE((SELECT e.details ->> 'added' FROM workspace.installation_events e
		                   WHERE e.installation_id = ai.id AND e.event_type = 'held'
		                   ORDER BY e.created_at DESC LIMIT 1), ''),
		        COALESCE((SELECT e.details ->> 'reason' FROM workspace.installation_events e
		                   WHERE e.installation_id = ai.id AND e.event_type = 'held'
		                   ORDER BY e.created_at DESC LIMIT 1), '')
		 FROM workspace.app_installations ai
		 JOIN registry.apps a ON a.id = ai.app_id
		 WHERE ai.tenant_id = $1`, tenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type InstalledApp struct {
		ID               string    `json:"id"`
		AppID            string    `json:"app_id"`
		Slug             string    `json:"slug"`
		Name             string    `json:"name"`
		InstalledVersion string    `json:"installed_version"`
		Status           string    `json:"status"`
		Enabled          bool      `json:"enabled"`
		InstalledAt      time.Time `json:"installed_at"`
		// What this app does about new versions, and what is waiting.
		AutoUpdate      bool   `json:"auto_update"`
		PinnedVersion   string `json:"pinned_version,omitempty"`
		LatestVersion   string `json:"latest_version,omitempty"`
		UpdateAvailable bool   `json:"update_available"`
		// HeldFor lists what a waiting version asks for that the installed one
		// did not, and HeldReason says why it is waiting at all. Either being
		// set means an administrator has a decision to make rather than a
		// button to press — see AutoUpdate.
		HeldFor    []string `json:"held_for,omitempty"`
		HeldReason string   `json:"held_reason,omitempty"`
	}

	locale := config.LocaleFromRequest(r)
	list := make([]InstalledApp, 0)
	for rows.Next() {
		var item InstalledApp
		// Skipping unreadable rows reported a tenant's app as not installed,
		// and the store then offered to install it again over the top.
		var heldFor, heldReason string
		if err := rows.Scan(&item.ID, &item.AppID, &item.Slug, &item.Name, &item.InstalledVersion,
			&item.Status, &item.Enabled, &item.InstalledAt, &item.AutoUpdate,
			&item.PinnedVersion, &heldFor, &heldReason); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "failed to read installed apps")
			return
		}
		// An app that left this binary is not something this tenant has, whatever
		// the row says. See presentableInstallation.
		if !h.presentableInstallation(item.AppID) {
			continue
		}
		// apps.name is the manifest's English name, and this was the one catalogue
		// surface that answered with it: the store and the sidebar both resolve
		// through the caller's locale. So an installed app was called "Digital
		// Documents & Signatures" here and "Баримт ба цахим гарын үсэг" in the menu
		// beside it — the same app under two names, which reads as an app that is
		// installed and yet missing from the menu.
		if catalogApp, ok := h.installer.GetAppBySlug(item.Slug); ok {
			if localized := catalogApp.Localized(locale).Name; localized != "" {
				item.Name = localized
			}
			item.LatestVersion = catalogApp.Version
			item.UpdateAvailable = catalog.IsNewerVersion(catalogApp.Version, item.InstalledVersion)
			// Only report a hold that is still true: once the pin is at the
			// version being offered, or the offer is gone, the recorded reason
			// is history rather than a decision.
			if item.UpdateAvailable && item.PinnedVersion == item.InstalledVersion {
				item.HeldFor = parseHeldFor(heldFor)
				item.HeldReason = heldReason
			}
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to read installed apps")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (h *Handlers) HandleInstallApp(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !security.IsValidSlug(slug) {
		httpx.Error(w, http.StatusBadRequest, "invalid app slug format")
		return
	}

	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.installer.InstallApp(r.Context(), claims.TenantID, slug, claims.UserID); err != nil {
		// The failure used to be handed to the browser verbatim. That answered
		// a database outage with "bad request" and described the inside of the
		// server — constraint names, the module registry, the dependency graph
		// — to anyone who could press Install. Only the caller's own mistake is
		// reported as such; the rest goes to the log, where an operator can act
		// on it.
		if errors.Is(err, ErrAppNotFound) {
			httpx.Error(w, http.StatusNotFound, "app not found")
			return
		}
		slog.Error("app installation failed", "error", err, "app_slug", slug, "tenant_id", claims.TenantID)
		httpx.Error(w, http.StatusInternalServerError,
			"could not install this app; the failure has been logged for your administrator")
		return
	}
	// The app gate reads a cached copy of this row, so the screen that just
	// pressed the button has to stop being told the old answer.
	h.ForgetGate(claims.TenantID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "installed", "app": slug})
}

// HandleUpgradeApp moves this tenant to the version the catalogue carries.
//
// Separate from install rather than folded into it: pressing "Install" on an
// app you already have is a mistake worth ignoring, while pressing "Update"
// when there is nothing to update is a question worth answering — and the
// answer, 409, is what stops a store screen offering the button for ever.
func (h *Handlers) HandleUpgradeApp(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !security.IsValidSlug(slug) {
		httpx.Error(w, http.StatusBadRequest, "invalid app slug format")
		return
	}

	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	from, to, err := h.installer.UpgradeApp(r.Context(), claims.TenantID, slug, claims.UserID)
	switch {
	case errors.Is(err, ErrAppNotFound):
		httpx.Error(w, http.StatusNotFound, "app not found")
		return
	case errors.Is(err, ErrNotInstalled):
		httpx.Error(w, http.StatusNotFound, "this app is not installed for your organisation")
		return
	case errors.Is(err, ErrAlreadyCurrent):
		httpx.JSON(w, http.StatusConflict, map[string]string{
			"error":             "this app is already on the latest version",
			"installed_version": from,
			"latest_version":    to,
		})
		return
	case err != nil:
		slog.Error("app upgrade failed", "error", err, "app_slug", slug, "tenant_id", claims.TenantID)
		httpx.Error(w, http.StatusInternalServerError,
			"could not update this app; the failure has been logged for your administrator")
		return
	}

	// The app gate reads a cached copy of this row, so the screen that just
	// pressed the button has to stop being told the old answer.
	h.ForgetGate(claims.TenantID)

	httpx.JSON(w, http.StatusOK, map[string]string{
		"status": "upgraded", "app": slug, "from": from, "to": to,
	})
}

// HandleSyncCatalog is the "check for updates" button.
//
// The background sync runs on its own clock, which is the right cadence for a
// catalogue and the wrong one for an administrator who has just been told a new
// version exists. It answers what happened rather than always "ok": an
// administrator who presses it needs to know whether anything moved.
func (h *Handlers) HandleSyncCatalog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Deprecation", "true")
	w.Header().Set("Link", `</api/platform/v1/catalog/sync>; rel="successor-version"`)
	w.Header().Set("Sunset", "Sat, 01 Mar 2026 00:00:00 GMT")

	if !h.catalogue.Remote() {
		httpx.Error(w, http.StatusNotImplemented,
			"this deployment reads its app catalog from a file; there is no registry to sync with")
		return
	}

	changed, err := h.SyncCatalog(r.Context())
	if err != nil {
		slog.Error("catalog: manual registry sync failed", "error", err)
		httpx.Error(w, http.StatusBadGateway, "could not reach the app registry; the current catalog is unchanged")
		return
	}

	status := "unchanged"
	if changed {
		status = "updated"
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"status": status,
		"apps":   len(h.installer.GetCatalog()),
	})
}

// parseHeldFor reads back what holdForApproval recorded.
//
// It was written with fmt.Sprint over a slice — "[a b c]" — because the details
// column is a flat string map. Read here rather than at the point of storage so
// the event keeps the shape everything else in that table has.
func parseHeldFor(recorded string) []string {
	trimmed := strings.Trim(recorded, "[]")
	if trimmed == "" {
		return nil
	}
	return strings.Fields(trimmed)
}

// HandleSetAutoUpdate records whether an app should follow the catalogue.
//
// Turning it on also clears any pin: an administrator saying "keep this current"
// and one saying "hold this version" are the same decision from either end, and
// leaving the pin would make the switch look broken.
func (h *Handlers) HandleSetAutoUpdate(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !security.IsValidSlug(slug) {
		httpx.Error(w, http.StatusBadRequest, "invalid app slug format")
		return
	}

	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "malformed request body")
		return
	}

	switch err := h.installer.SetAutoUpdate(r.Context(), claims.TenantID, slug, body.Enabled); {
	case errors.Is(err, ErrAppNotFound):
		httpx.Error(w, http.StatusNotFound, "app not found")
		return
	case errors.Is(err, ErrNotInstalled):
		httpx.Error(w, http.StatusNotFound, "this app is not installed for your organisation")
		return
	case err != nil:
		slog.Error("could not set auto-update", "error", err, "app_slug", slug, "tenant_id", claims.TenantID)
		httpx.Error(w, http.StatusInternalServerError, "could not save that preference")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"app": slug, "auto_update": body.Enabled})
}

// HandleCatalogStatus reports where the catalogue comes from and how the last
// attempt to refresh it went.
//
// The manual sync button answers for itself; the hourly one leaves a log line
// on a server nobody is watching, so a registry that has been unreachable for a
// week looks exactly like one that has published nothing. This is the screen
// that tells them apart — and it is also where an app that is held back stops
// being a mystery, because the reason is on the installed-apps list beside it.
func (h *Handlers) HandleCatalogStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Deprecation", "true")
	w.Header().Set("Link", `</api/platform/v1/catalog/status>; rel="successor-version"`)
	w.Header().Set("Sunset", "Sat, 01 Feb 2027 00:00:00 GMT")

	h.syncMu.RLock()
	at, ok, failure := h.lastSyncAt, h.lastSyncOK, h.lastSyncErr
	h.syncMu.RUnlock()

	status := map[string]any{
		"source":        "file",
		"apps":          len(h.installer.GetCatalog()),
		"sync_interval": h.catalogue.SyncInterval().String(),
	}
	if h.catalogue.Remote() {
		status["source"] = "registry"
	}
	if !at.IsZero() {
		status["last_sync_at"] = at
		status["last_sync_ok"] = ok
		// The registry's own words, not a redaction: this is a tenant
		// administrator being told why their store is not moving, and "an error
		// occurred" is not something anybody can act on. It says nothing about
		// this deployment that the catalogue URL does not.
		if failure != "" {
			status["last_sync_error"] = failure
		}
	}
	httpx.JSON(w, http.StatusOK, status)
}

func (h *Handlers) HandleDisableApp(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !security.IsValidSlug(slug) {
		httpx.Error(w, http.StatusBadRequest, "invalid app slug format")
		return
	}

	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.installer.DisableApp(r.Context(), claims.TenantID, slug, claims.UserID); err != nil {
		if errors.Is(err, ErrAppNotFound) {
			httpx.Error(w, http.StatusNotFound, "app not found")
			return
		}
		slog.Error("could not disable an app", "error", err, "app_slug", slug, "tenant_id", claims.TenantID)
		httpx.Error(w, http.StatusInternalServerError, "could not disable this app")
		return
	}
	// The app gate reads a cached copy of this row, so the screen that just
	// pressed the button has to stop being told the old answer.
	h.ForgetGate(claims.TenantID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "disabled", "app": slug})
}

func (h *Handlers) HandleEnableApp(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !security.IsValidSlug(slug) {
		httpx.Error(w, http.StatusBadRequest, "invalid app slug format")
		return
	}

	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.installer.EnableApp(r.Context(), claims.TenantID, slug, claims.UserID); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	// The app gate reads a cached copy of this row, so the screen that just
	// pressed the button has to stop being told the old answer.
	h.ForgetGate(claims.TenantID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "enabled", "app": slug})
}
