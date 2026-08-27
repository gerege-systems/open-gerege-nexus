/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Keeping the catalogue, the apps table and the installations in step.
 *
 * The provider decides where a catalogue comes from; this decides when to ask
 * again, what to do with the answer, and what to tell an administrator who
 * presses "check for updates". The console asks the same two questions through
 * the callbacks in its Deps, which is why the status is held rather than
 * recomputed.
 */

package appinstall

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/async"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// startCatalogSync keeps this instance's catalogue in step with the registry.
//
// It is a poll rather than a push because an instance may be behind anything —
// a firewall, a home connection, an air gap that was opened for an hour — and
// the registry knowing how to reach every one of them is a coupling this
// architecture spent its effort avoiding. A failed round is a warning: the
// catalogue in hand keeps serving, and installed apps do not depend on this at
// all.
// StartCatalogSync keeps the catalogue in step with the registry.
func (h *Handlers) StartCatalogSync(ctx context.Context) {
	if !h.catalogue.Remote() {
		return
	}
	async.Go("catalog-sync", func() {
		// The interval is read on every round rather than captured, because it
		// is a platform setting now: an operator who slows the polling down
		// during a registry incident should not have to restart the platform
		// for it to take effect. A change is felt after the current wait, which
		// is the most anybody could reasonably expect of a poll.
		ticker := time.NewTicker(h.catalogInterval())
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ticker.Reset(h.catalogInterval())
				syncCtx, cancel := context.WithTimeout(ctx, CatalogLoadTimeout)
				changed, err := h.SyncCatalog(syncCtx)
				cancel()
				switch {
				case err != nil:
					slog.Warn("catalog: registry sync failed; keeping the catalogue in hand", "error", err)
				case changed:
					slog.Info("catalog: updated from the registry", "apps", len(h.installer.GetCatalog()))
				}
			}
		}
	})
}

// catalogInterval is how long to wait before asking the registry again.
//
// The setting, then whatever the catalogue source worked out from the
// environment at startup. A minute is the floor for the reason source.go gives:
// below that this stops being a poll and becomes a load generator pointed at
// somebody else's registry.
func (h *Handlers) catalogInterval() time.Duration {
	interval := settings.Duration(settings.CatalogSyncInterval)
	if interval <= 0 {
		interval = h.catalogue.SyncInterval()
	}
	if interval < time.Minute {
		interval = time.Minute
	}
	return interval
}

// catalogSyncStatus is what the console shows about the catalogue: when it was
// last fetched, whether that worked, and why not.
func (h *Handlers) CatalogStatus() (time.Time, bool, string) {
	h.syncMu.RLock()
	defer h.syncMu.RUnlock()
	// A deployment in file mode has never synced and never will, which is not
	// a failure — it is what "no registry configured" looks like.
	if h.lastSyncAt.IsZero() && !h.catalogue.Remote() {
		return time.Time{}, true, "the catalogue comes from the bundled file"
	}
	return h.lastSyncAt, h.lastSyncOK, h.lastSyncErr
}

// recordSync remembers how the last attempt went.
func (h *Handlers) recordSync(err error) {
	h.syncMu.Lock()
	h.lastSyncAt, h.lastSyncOK = time.Now(), err == nil
	h.lastSyncErr = ""
	if err != nil {
		h.lastSyncErr = err.Error()
	}
	h.syncMu.Unlock()
}

// syncCatalogFromRegistry fetches, accepts and publishes a new catalogue.
//
// The order matters: the apps table has to carry a row before an installation
// can reference it, and every replica's app gate has to stop answering from a
// catalogue that no longer exists. The gate is dropped for every tenant rather
// than one, because a catalogue change is not a tenant's act.
func (h *Handlers) SyncCatalog(ctx context.Context) (changed bool, err error) {
	defer func() { h.recordSync(err) }()

	catalog, changed, err := h.catalogue.Refresh(ctx)
	if err != nil {
		return false, err
	}

	if changed {
		h.installer.SetCatalog(catalog)
		if err := h.installer.SyncCatalog(ctx); err != nil {
			return true, fmt.Errorf("sync the new catalogue into the database: %w", err)
		}
		h.bus.Invalidate(GateCacheName, "")
	}

	// After every sync, not only after a change.
	//
	// A catalogue that has not moved can still be ahead of an installation —
	// one made before the version was published, or one whose upgrade failed
	// the first time — and an instance that only ever swept on change would
	// leave those behind for ever, with an update the store offers and nothing
	// takes. The sweep is a no-op where there is nothing to do.
	h.ApplyCatalogToInstallations(ctx)
	return changed, nil
}

// ApplyCatalogToInstallations carries tenants forward to the catalogue in hand.
//
// Its failures are logged rather than returned: this runs on a timer and at
// startup, where there is nobody to hand an error to, and an installation left
// where it is is the safe outcome — the store still offers the update.
func (h *Handlers) ApplyCatalogToInstallations(ctx context.Context) {
	// The modules' own schemas first, and on every sweep rather than only at
	// the install that first needed them: a module can gain a schema — or have
	// one moved into it out of db/migrations — long after its app was
	// installed, and the install path is not reached again for an app that is
	// already there. See MigrateModules for what that cost the first time.
	//
	// Logged and not fatal, like the catalogue sync above it. A database that
	// is not up yet must not stop the process from booting; this runs again on
	// the next sweep, and the error says what is broken until it does.
	// Which installed apps this binary cannot serve. Not a fault to correct
	// here — an operator upgrading past an app's departure is in that state
	// legitimately — but a fault to say out loud, because nothing else does.
	h.installer.ReportUncarriedApps(ctx)

	if err := h.installer.MigrateModules(ctx); err != nil {
		slog.Error("catalog: a module's own schema could not be applied — its routes will fail until it is",
			"error", err)
	}

	// The distribution's default apps next: a tenant that never got one has no
	// way to install it either, because on a deployment where the store itself
	// sits behind an app the missing app is what would have carried it.
	// A no-op where the list is empty, which is every deployment that has not
	// set platform.Options.DefaultApps — this repository's own included.
	if err := h.installer.EnsureDefaultApps(ctx); err != nil {
		slog.Error("catalog: could not install the default apps", "error", err)
	}

	swept, err := h.installer.AutoUpdate(ctx)
	if err != nil {
		slog.Error("catalog: could not apply the catalogue to installations", "error", err)
		return
	}
	if len(swept.Upgraded) == 0 && len(swept.Held) == 0 {
		return
	}
	slog.Info("catalog: installations followed the catalogue",
		"upgraded", len(swept.Upgraded), "held_for_approval", len(swept.Held))
	// Their menus and gates were decided by the version that just moved.
	h.bus.Invalidate(GateCacheName, "")
}

// CatalogLoadTimeout bounds the catalogue fetch that boot waits on. A registry
// that is merely slow must not hold a deployment open; the fallbacks are there
// precisely so this can give up.
const CatalogLoadTimeout = 20 * time.Second

// appinstall.VerifyCatalogVersions holds a catalogue against the modules compiled into this
// binary.
//
// The two drifted apart unnoticed — esign shipped 2.0.0 as a module and 1.0.0 in
// the catalogue, and the developer portal did the same — because nothing ever
// compared them. Once a registry outside this repository publishes versions, a
// number the store advertises but the binary does not have is an upgrade that
// silently does nothing.
//
// It runs against every candidate catalogue, so what it means depends on where
// the catalogue came from: the bundled file failing it is a startup error, the
// same way its manifests failing validation is, while a registry answer failing
// it is discarded in favour of the cache or the file. The catalogue/manifest
// half of the comparison lives in catalog.ValidateCatalog, which every source
// goes through.
//
// An app with no compiled module is not an error. External apps have none by
// definition.
func VerifyCatalogVersions(catalog []catalog.CatalogApp) error {
	present := make(map[string]bool, len(catalog))
	for _, app := range catalog {
		present[app.ID] = true
		mod, ok := nexus.Get(app.ID)
		if !ok {
			continue
		}
		if mod.Version() != app.Version {
			return fmt.Errorf("module %s is compiled at version %q but the catalog declares %q",
				app.ID, mod.Version(), app.Version)
		}
	}

	// A catalogue without the platform's own default apps is not a catalogue
	// this build should run on — it is one that predates it. The version check
	// above cannot see that: it skips ids with no compiled module, and after a
	// rename every renamed app looks exactly like a third party's, so an entire
	// stale catalogue passes without a word.
	//
	// That is not hypothetical. A cache written before the ids were renamed was
	// accepted whole, the organisation app was absent from it, no tenant got
	// the screens, and every app in the store offered an install that failed on
	// a foreign key. The bundled file always matches the binary, so refusing
	// here is what reaches it.
	//
	// This is a staleness check, not a claim that the app is mandatory: a
	// tenant may remove it, and that removal is a row in app_installations
	// rather than an absence from the catalogue.
	for _, appID := range DefaultApps {
		if !present[appID] {
			return fmt.Errorf("catalog does not carry the platform's own app %s", appID)
		}
	}
	return nil
}
