/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package appinstall

import (
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// overviewApp is one row of the administrator's view of the store.
type overviewApp struct {
	AppID string `json:"app_id"`
	Slug  string `json:"slug"`
	Name  string `json:"name"`

	// The three versions that can disagree, side by side. BinaryVersion is what
	// this build compiled in and is empty for an app with no compiled module —
	// an external app, or a third party's. CatalogVersion is what the catalogue
	// offers. InstalledVersion is what this organisation is running.
	//
	// Two of them differing is the whole reason this screen exists: a catalogue
	// ahead of the installation means an update is waiting, and a catalogue
	// behind the binary means the instance is serving a stale document — the
	// failure that used to be invisible because a stale catalogue and a current
	// one look identical from every other screen.
	BinaryVersion    string `json:"binary_version,omitempty"`
	CatalogVersion   string `json:"catalog_version"`
	InstalledVersion string `json:"installed_version,omitempty"`

	Installed       bool `json:"installed"`
	Enabled         bool `json:"enabled"`
	UpdateAvailable bool `json:"update_available"`
	AutoUpdate      bool `json:"auto_update"`
	// Held is set when a waiting version asks for more than the installed one
	// and is sitting behind an administrator's decision.
	Held          bool   `json:"held,omitempty"`
	PinnedVersion string `json:"pinned_version,omitempty"`
	// Drifted marks the row where the compiled module and the catalogue
	// disagree. It is computed rather than left to the reader because it is the
	// one condition on this screen that is nobody's decision and always a fault.
	Drifted bool `json:"drifted,omitempty"`

	// ReleaseSummary is the chronicle line for the catalogue's version, so the
	// overview says what is waiting and not merely that something is.
	ReleaseKind    string `json:"release_kind,omitempty"`
	ReleaseSummary string `json:"release_summary,omitempty"`
}

// HandleStoreOverview is the administrator's single view of what this instance
// is carrying and where its catalogue came from.
//
// It answers the question the per-app screens cannot: not "is this app up to
// date" but "is anything here quietly wrong". The sync state at the top and the
// drift flag on the rows are the two halves of that — a registry that has been
// unreachable for a week and a catalogue that predates the binary both look
// perfectly normal from the store.
//
// Deliberately not here: how many tenants have installed an app. The plan asked
// for it, and it is the one field on this screen that cannot be produced
// without stepping outside the row-level policies — app_installations carries a
// tenant_id and this endpoint's caller is a tenant administrator, not a
// platform operator. This platform has no such role: `admin` is granted per
// tenant. Answering it would mean one organisation's administrator learning how
// many others run a given app, obtained by deliberately bypassing the isolation
// dbguard exists to enforce. The installation columns below are this tenant's
// own. If a platform-operator role is ever introduced, the count belongs to it.
func (h *Handlers) HandleStoreOverview(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := nexus.RequireWorkspace(w, r)
	if !ok {
		return
	}

	installed, err := h.installer.GetInstallationsForTenant(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load installed apps")
		return
	}

	held, err := h.heldApps(r, tenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load held apps")
		return
	}

	locale := config.LocaleFromRequest(r)
	available := h.installer.GetCatalog()
	rows := make([]overviewApp, 0, len(available))
	drifted := 0
	for _, app := range available {
		localized := app.Localized(locale)
		row := overviewApp{
			AppID: app.ID, Slug: app.Slug, Name: localized.Name,
			CatalogVersion: app.Version,
		}
		if module, compiled := nexus.Get(app.ID); compiled {
			row.BinaryVersion = module.Version()
			row.Drifted = module.Version() != app.Version
			if row.Drifted {
				drifted++
			}
		}
		if state, yes := installed[app.ID]; yes {
			row.Installed = true
			row.Enabled = state.Enabled
			row.InstalledVersion = state.Version
			row.AutoUpdate = state.AutoUpdate
			row.PinnedVersion = state.PinnedVersion
			row.UpdateAvailable = catalog.IsNewerVersion(app.Version, state.Version)
			row.Held = held[app.ID]
		}
		if notes := app.Manifest.ReleaseNotes; notes != nil {
			row.ReleaseKind = notes.Kind
			row.ReleaseSummary = pickLocale(notes.Summary, locale)
		}
		rows = append(rows, row)
	}

	h.syncMu.RLock()
	at, syncOK, failure := h.lastSyncAt, h.lastSyncOK, h.lastSyncErr
	h.syncMu.RUnlock()

	source := "file"
	if h.catalogue.Remote() {
		source = "registry"
	}
	sync := map[string]any{
		"source":        source,
		"sync_interval": h.catalogue.SyncInterval().String(),
	}
	if !at.IsZero() {
		sync["last_sync_at"] = at
		sync["last_sync_ok"] = syncOK
		if failure != "" {
			sync["last_sync_error"] = failure
		}
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"platform_version": config.PlatformVersion,
		"sync":             sync,
		"apps":             rows,
		"summary": map[string]int{
			"catalog":   len(available),
			"installed": len(installed),
			"updates":   countIf(rows, func(a overviewApp) bool { return a.UpdateAvailable }),
			"held":      countIf(rows, func(a overviewApp) bool { return a.Held }),
			"drifted":   drifted,
		},
	})
}

// heldApps reports which of this tenant's installations are waiting on a
// decision, keyed by app id.
//
// "Held" is not a column: it is the most recent event for an installation being
// a hold. An app that was held and has since been upgraded is not held any
// more, and reading only for the presence of a held event would keep saying it
// was.
func (h *Handlers) heldApps(r *http.Request, tenantID string) (map[string]bool, error) {
	rows, err := h.db.Query(r.Context(),
		`SELECT ai.app_id
		   FROM workspace.app_installations ai
		   JOIN LATERAL (
		        SELECT e.event_type FROM workspace.installation_events e
		         WHERE e.installation_id = ai.id
		         ORDER BY e.created_at DESC, e.id LIMIT 1
		   ) latest ON TRUE
		  WHERE ai.tenant_id = $1 AND latest.event_type = 'held'`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]bool{}
	for rows.Next() {
		var appID string
		if err := rows.Scan(&appID); err != nil {
			return nil, err
		}
		out[appID] = true
	}
	return out, rows.Err()
}

func countIf(rows []overviewApp, match func(overviewApp) bool) int {
	n := 0
	for _, row := range rows {
		if match(row) {
			n++
		}
	}
	return n
}
