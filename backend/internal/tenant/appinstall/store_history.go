/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package appinstall

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/go-chi/chi/v5"
)

// The history of an app is two records that have never been read together.
//
// app_versions says what the publisher released and, since the chronicle,
// why. installation_events says what this tenant did about it — installed,
// upgraded, held back, and whether a person or the nightly sweep decided.
// Separately each answers half a question. An administrator looking at an app
// wants the other half: this version appeared on the 7th, we took it on the
// 9th, and the one after it is waiting because it asks for a permission we
// have not agreed to.
//
// So they are merged into one timeline, newest first, and the two kinds of
// entry are distinguished by "type" rather than by which list they came from.

// historyEntry is one line of the merged timeline.
type historyEntry struct {
	At   time.Time `json:"at"`
	Type string    `json:"type"`
	// Version is the release this line is about. Present on every kind: a
	// release announces it, an installation event moved to it.
	Version string `json:"version,omitempty"`
	// From is the version an upgrade left behind.
	From string `json:"from,omitempty"`

	// Release lines carry the chronicle entry, already reduced to the caller's
	// language so the client renders a string rather than picking a key.
	Kind    string   `json:"kind,omitempty"`
	Summary string   `json:"summary,omitempty"`
	Details string   `json:"details,omitempty"`
	Authors []string `json:"authors,omitempty"`
	Refs    []string `json:"refs,omitempty"`

	// Installation lines carry who did it. System is true when the actor was
	// the auto-update sweep rather than a person — the distinction an operator
	// asks about first when a version moved without anybody remembering doing
	// it.
	ActorID   string `json:"actor_id,omitempty"`
	ActorName string `json:"actor_name,omitempty"`
	System    bool   `json:"system,omitempty"`
	// Held lines say what stopped them.
	Reason string `json:"reason,omitempty"`
	Added  string `json:"added,omitempty"`
}

// HandleAppHistory answers with one app's releases and this tenant's dealings
// with it.
//
// It is a member-level read behind the app's own permission rather than an
// administrative one: "what changed in the app I use" is a question the people
// using it ask, and the answer names no one outside their own organisation.
func (h *Handlers) HandleAppHistory(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := nexus.RequireTenant(w, r)
	if !ok {
		return
	}
	slug := chi.URLParam(r, "slug")
	if !security.IsValidSlug(slug) {
		httpx.Error(w, http.StatusBadRequest, "invalid app slug format")
		return
	}
	app, found := h.installer.GetAppBySlug(slug)
	if !found {
		httpx.Error(w, http.StatusNotFound, "app not found")
		return
	}

	locale := config.LocaleFromRequest(r)
	releases, err := h.appReleaseHistory(r, app.ID, locale)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to read the release history")
		return
	}
	events, installedVersion, err := h.appInstallationHistory(r, tenantID, app.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to read the installation history")
		return
	}

	timeline := append(releases, events...)
	// Newest first, and an installation event sorts above the release it is
	// about when the two share a timestamp — which they do on the sweep that
	// syncs a catalogue and upgrades in one pass. Reading "we took 1.1.0"
	// above "1.1.0 was released" is the order the sentence is spoken in.
	sort.SliceStable(timeline, func(i, j int) bool {
		if timeline[i].At.Equal(timeline[j].At) {
			return timeline[i].Type != "release" && timeline[j].Type == "release"
		}
		return timeline[i].At.After(timeline[j].At)
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"app_id":            app.ID,
		"slug":              app.Slug,
		"name":              app.Localized(locale).Name,
		"installed_version": installedVersion,
		"latest_version":    app.Version,
		"timeline":          timeline,
	})
}

// appReleaseHistory reads what the publisher shipped, from the manifests
// SyncCatalog has been recording since long before anything read them back.
func (h *Handlers) appReleaseHistory(r *http.Request, appID, locale string) ([]historyEntry, error) {
	rows, err := h.db.Query(r.Context(),
		`SELECT version, manifest, published_at FROM registry.app_versions
		  WHERE app_id = $1 ORDER BY published_at DESC`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]historyEntry, 0, 8)
	for rows.Next() {
		var version string
		var raw []byte
		var publishedAt time.Time
		if err := rows.Scan(&version, &raw, &publishedAt); err != nil {
			return nil, err
		}
		entry := historyEntry{At: publishedAt, Type: "release", Version: version}
		// A manifest that will not parse is a recorded version whose notes are
		// unreadable, not a broken history: the line still says the version
		// shipped and when. Older rows predate the chronicle and simply carry
		// no notes.
		var manifest catalog.Manifest
		if json.Unmarshal(raw, &manifest) == nil && manifest.ReleaseNotes != nil {
			note := manifest.ReleaseNotes
			entry.Kind = note.Kind
			entry.Summary = pickLocale(note.Summary, locale)
			entry.Details = pickLocale(note.Details, locale)
			entry.Authors, entry.Refs = note.Authors, note.Refs
			if note.ReleasedAt != "" {
				if on, err := time.Parse(time.DateOnly, note.ReleasedAt); err == nil {
					entry.At = on
				}
			}
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

// appInstallationHistory reads what this tenant did, and what it is running.
func (h *Handlers) appInstallationHistory(r *http.Request, tenantID, appID string) ([]historyEntry, string, error) {
	rows, err := h.db.Query(r.Context(),
		`SELECT e.event_type, e.details, e.created_at, ai.installed_version,
		        COALESCE(u.name, '')
		   FROM tenant.installation_events e
		   JOIN tenant.app_installations ai ON ai.id = e.installation_id
		   LEFT JOIN registry.users u
		          ON u.id::text = NULLIF(e.details ->> 'user_id', 'system')
		  WHERE ai.tenant_id = $1 AND ai.app_id = $2
		  ORDER BY e.created_at DESC`, tenantID, appID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	out := make([]historyEntry, 0, 8)
	installedVersion := ""
	for rows.Next() {
		var eventType, actorName string
		var raw []byte
		var at time.Time
		if err := rows.Scan(&eventType, &raw, &at, &installedVersion, &actorName); err != nil {
			return nil, "", err
		}
		details := map[string]string{}
		_ = json.Unmarshal(raw, &details)

		actor := details["user_id"]
		out = append(out, historyEntry{
			At: at, Type: eventType,
			Version:   firstNonEmpty(details["to"], details["version"]),
			From:      details["from"],
			ActorID:   actor,
			ActorName: actorName,
			// The sweep records itself as "system" so a version that moved on
			// its own is distinguishable from one somebody chose to move.
			System: actor == "system",
			Reason: details["reason"],
			Added:  details["added"],
		})
	}
	return out, installedVersion, rows.Err()
}

// pickLocale resolves a translated field, falling back the way the rest of the
// platform does: the asked-for language, then Mongolian as the source, then
// English. An empty result means the note carried none of the three, which
// ValidateChronicle does not permit for a summary.
func pickLocale(text map[string]string, locale string) string {
	for _, key := range [...]string{locale, "mn", "en"} {
		if value := text[key]; value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
