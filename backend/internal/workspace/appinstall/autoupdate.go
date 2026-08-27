/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package appinstall

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"
)

// SystemActor is what an upgrade nobody pressed a button for is recorded as.
// It reads as a user id in installation_events, so it is a value no user can
// have rather than an empty string that would read as "unknown".
const SystemActor = "system"

// autoUpdateLockKey names the advisory lock the sweep runs under. Any constant
// would do; this one is a word so a `pg_locks` reading is legible.
const autoUpdateLockKey = 0x6175746F757064 // "autoupd"

// AutoUpdateResult is what one sweep did.
type AutoUpdateResult struct {
	Upgraded []AutoUpdateEntry
	Held     []AutoUpdateEntry
}

// AutoUpdateEntry is one tenant's one app.
type AutoUpdateEntry struct {
	TenantID string
	AppID    string
	Slug     string
	From     string
	To       string
	// Added lists what a held version asks for that the installed one did not.
	Added []string
	// Reason is why it was held, in words an administrator can act on. Empty
	// means it was not held.
	Reason string
}

// AutoUpdate moves installations forward to the catalogue, and refuses to move
// the ones that would quietly widen what an app may do.
//
// It runs after the catalogue changes, which with a registry configured is
// whenever a publisher publishes. Without it the column added in migration
// 00033 is a preference nothing reads: every tenant would wait for an
// administrator to press Update, on an instance whose administrator may not
// know a version exists.
//
// Three reasons an installation is left alone:
//
//   - auto_update is off, which is an administrator saying so;
//   - it is pinned, which is the same thing about one version;
//   - the new version asks for more than the installed one — see §4.4 of the
//     separation plan. That case is not a skip but a hold: the installation is
//     pinned where it is and the reason is recorded, so the administrator is
//     shown a decision rather than left with an app that silently stopped
//     updating.
func (ai *AppInstaller) AutoUpdate(ctx context.Context) (AutoUpdateResult, error) {
	var result AutoUpdateResult

	// One replica at a time. Every replica syncs on its own timer, so without
	// this they would all sweep the same installations at the same moment: the
	// upgrades are idempotent, but the trail would carry three 'upgraded'
	// events for one move and the database would do the work three times.
	//
	// try, not wait: a sweep somebody else is already doing is a sweep that
	// does not need doing again, and holding the timer open to find that out
	// is worse than skipping this tick.
	conn, err := ai.db.Acquire(ctx)
	if err != nil {
		return result, err
	}
	defer conn.Release()

	var acquired bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, autoUpdateLockKey).Scan(&acquired); err != nil {
		return result, err
	}
	if !acquired {
		slog.Info("auto-update: another replica is sweeping; skipping this round")
		return result, nil
	}
	defer func() {
		if _, err := conn.Exec(context.WithoutCancel(ctx),
			`SELECT pg_advisory_unlock($1)`, autoUpdateLockKey); err != nil {
			slog.Warn("auto-update: could not release the sweep lock", "error", err)
		}
	}()

	rows, err := ai.db.Query(ctx,
		`SELECT tenant_id::text, app_id, installed_version, auto_update, COALESCE(pinned_version, '')
		   FROM workspace.app_installations`)
	if err != nil {
		return result, err
	}

	type candidate struct {
		tenantID, appID, installed string
		auto                       bool
		pinned                     string
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.tenantID, &c.appID, &c.installed, &c.auto, &c.pinned); err != nil {
			rows.Close()
			return result, err
		}
		candidates = append(candidates, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return result, err
	}

	for _, c := range candidates {
		app, known := ai.GetAppByID(c.appID)
		if !known || !catalog.IsNewerVersion(app.Version, c.installed) {
			continue
		}
		if !c.auto || c.pinned != "" {
			continue
		}

		entry := AutoUpdateEntry{
			TenantID: c.tenantID, AppID: app.ID, Slug: app.Slug,
			From: c.installed, To: app.Version,
		}

		added, err := ai.widenedGrant(ctx, app, c.installed)
		switch {
		case err != nil:
			// Not knowing what the installed version asked for is not a licence
			// to assume it asked for nothing more. It is held rather than
			// skipped, because a skip is invisible: the administrator would see
			// an update on offer, auto-update on, and nothing happening, with
			// the reason only in a log they are not reading.
			slog.Warn("auto-update: could not compare versions; holding for approval",
				"error", err, "app_id", c.appID, "tenant_id", c.tenantID)
			entry.Reason = "the installed version's manifest is not on record, so what the new one adds cannot be established"
		case len(added) > 0:
			entry.Added = added
			entry.Reason = "the new version asks for more than the installed one"
		}

		if entry.Reason != "" {
			if err := ai.holdForApproval(ctx, entry); err != nil {
				slog.Error("auto-update: could not hold an installation for approval",
					"error", err, "app_id", c.appID, "tenant_id", c.tenantID)
				continue
			}
			result.Held = append(result.Held, entry)
			continue
		}

		if err := ai.installOrUpgrade(ctx, c.tenantID, app.Slug, SystemActor); err != nil {
			// One tenant's failure is not the sweep's. A missing module or a
			// dependency that no longer resolves stops that installation, and
			// the rest still move.
			slog.Error("auto-update: could not upgrade an installation",
				"error", err, "app_id", c.appID, "tenant_id", c.tenantID)
			continue
		}
		result.Upgraded = append(result.Upgraded, entry)
	}

	return result, nil
}

// widenedGrant reports what the catalogue version asks for that the installed
// version did not.
//
// Only external apps are compared. A module app's permissions come from the
// code compiled into this binary, not from its manifest: the tenant is already
// running that code, so there is nothing for them to approve that they have not
// already been given by the platform's own release.
//
// The installed version's manifest comes from registry.app_versions, which SyncCatalog
// has been filling since the version history was added. An installation older
// than that has no row, and this returns an error rather than a guess.
func (ai *AppInstaller) widenedGrant(ctx context.Context, app catalog.CatalogApp, installedVersion string) ([]string, error) {
	if !app.Manifest.IsExternal() {
		return nil, nil
	}

	var encoded []byte
	err := ai.db.QueryRow(ctx,
		`SELECT manifest FROM registry.app_versions WHERE app_id = $1 AND version = $2`,
		app.ID, installedVersion).Scan(&encoded)
	if err != nil {
		return nil, fmt.Errorf("read the manifest of %s %s: %w", app.ID, installedVersion, err)
	}

	var installed catalog.Manifest
	if err := json.Unmarshal(encoded, &installed); err != nil {
		return nil, fmt.Errorf("decode the manifest of %s %s: %w", app.ID, installedVersion, err)
	}

	added := make([]string, 0)
	held := make([]string, 0, len(installed.Permissions))
	for _, permission := range installed.Permissions {
		held = append(held, permission.Code)
	}
	for _, permission := range app.Manifest.Permissions {
		if !slices.Contains(held, permission.Code) {
			added = append(added, permission.Code)
		}
	}

	// An OAuth scope is a permission over somebody else's data, granted to a
	// service running outside this platform. It widens a grant every bit as
	// much as a permission code does.
	if app.Manifest.External != nil {
		heldScopes := []string{}
		if installed.External != nil {
			heldScopes = installed.External.Scopes
		}
		for _, scope := range app.Manifest.External.Scopes {
			if !slices.Contains(heldScopes, scope) {
				added = append(added, "scope:"+scope)
			}
		}
		// Somewhere else entirely is not the same application. The host is
		// compared rather than the whole URL, because a path change is the
		// publisher moving their own entry point.
		if installed.External != nil && hostOf(installed.External.LaunchURL) != hostOf(app.Manifest.External.LaunchURL) {
			added = append(added, "launch_url:"+hostOf(app.Manifest.External.LaunchURL))
		}
	}

	if len(added) == 0 {
		return nil, nil
	}
	return added, nil
}

// holdForApproval pins an installation where it is and records why.
func (ai *AppInstaller) holdForApproval(ctx context.Context, entry AutoUpdateEntry) error {
	tx, err := ai.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var installID string
	if err := tx.QueryRow(ctx,
		`UPDATE workspace.app_installations SET pinned_version = installed_version, updated_at = $1
		  WHERE tenant_id = $2 AND app_id = $3 AND pinned_version IS NULL
		  RETURNING id::text`,
		time.Now(), entry.TenantID, entry.AppID).Scan(&installID); err != nil {
		return err
	}

	details := map[string]string{
		"from": entry.From, "to": entry.To, "user_id": SystemActor,
		"reason": entry.Reason,
		"added":  fmt.Sprint(entry.Added),
	}
	if err := recordInstallationEvent(ctx, tx, installID, "held", details, time.Now()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetAutoUpdate records whether a tenant wants an app to follow the catalogue.
//
// Turning it back on clears a pin: the two are the same decision seen from
// either end, and leaving a pin behind would make the toggle look broken.
func (ai *AppInstaller) SetAutoUpdate(ctx context.Context, tenantID, appSlug string, enabled bool) error {
	app, ok := ai.GetAppBySlug(appSlug)
	if !ok {
		return fmt.Errorf("%w: %s", ErrAppNotFound, appSlug)
	}

	query := `UPDATE workspace.app_installations SET auto_update = $1, updated_at = $2
	           WHERE tenant_id = $3 AND app_id = $4`
	if enabled {
		query = `UPDATE workspace.app_installations SET auto_update = $1, pinned_version = NULL, updated_at = $2
		          WHERE tenant_id = $3 AND app_id = $4`
	}

	tag, err := ai.db.Exec(ctx, query, enabled, time.Now(), tenantID, app.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", ErrNotInstalled, appSlug)
	}
	return nil
}

// hostOf is the host of a URL, or the whole string when it cannot be parsed —
// an unparseable launch URL never matches a parseable one, which is the
// conservative direction for a comparison that decides whether to hold.
func hostOf(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return parsed.Host
}
