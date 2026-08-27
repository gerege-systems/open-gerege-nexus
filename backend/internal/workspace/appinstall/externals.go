/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 *
 * The app gate, continued into applications that run somewhere else.
 */

package appinstall

import (
	"context"
	"errors"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/memo"
	"github.com/jackc/pgx/v5"
)

// AppInstalled reports whether a tenant has an app installed and enabled.
//
// It is asked on the way into every request a module app serves, and again at
// the authorization endpoint for every external one, so it is cached: the row
// changes when an administrator presses a button, a few times in the life of a
// deployment. The negative answer is cached too — a client polling an app the
// tenant does not have should not cost a query each time — but only a definite
// one is. A database that is down would otherwise pin "not installed" onto the
// tenant for the length of the entry, and the app would stay missing after it
// came back.
func (h *Handlers) AppInstalled(ctx context.Context, tenantID, appID string) (bool, error) {
	cacheKey := memo.Key(tenantID, appID)
	if enabled, cached := h.appGate.Get(cacheKey); cached {
		return enabled, nil
	}

	var enabled bool
	err := h.db.QueryRow(ctx,
		`SELECT enabled FROM workspace.app_installations WHERE tenant_id = $1 AND app_id = $2`,
		tenantID, appID).Scan(&enabled)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		h.appGate.Put(cacheKey, false)
		return false, nil
	case err != nil:
		return false, err
	}
	h.appGate.Put(cacheKey, enabled)
	return enabled, nil
}

// InstalledAppSet answers "which apps does this tenant have" in one query.
//
// The per-app gate above is the cached one, asked once per request for one
// known app. The reports module needs the whole set, and asking the cache
// twelve times would be twelve round trips on a cold cache to build a list.
// Uncached deliberately: it is one query per report listing, and a stale answer
// here would show a report for an app that had just been uninstalled.
func (h *Handlers) InstalledAppSet(ctx context.Context, tenantID string) (map[string]bool, error) {
	rows, err := h.db.Query(ctx,
		`SELECT app_id FROM workspace.app_installations WHERE tenant_id = $1 AND enabled`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	installed := make(map[string]bool, 12)
	for rows.Next() {
		var appID string
		if err := rows.Scan(&appID); err != nil {
			return nil, err
		}
		installed[appID] = true
	}
	return installed, rows.Err()
}

// externalAppGate answers the SSO provider's question: may this tenant's user
// sign in to this client?
//
// A module app is kept from an uninstalled tenant by GateMiddleware, which
// stands in front of the routes the module serves. An external app serves no
// routes here — the only thing this platform does for it is say who somebody is
// — so the authorization endpoint is where the same rule has to be applied, and
// this is what applies it.
//
// It takes its two dependencies as functions rather than reaching into the
// server, so the decision can be tested without a database behind it.
type externalAppGate struct {
	catalog   func() []catalog.CatalogApp
	installed func(ctx context.Context, tenantID, appID string) (bool, error)
}

func (h *Handlers) NewExternalAppGate() externalAppGate {
	return externalAppGate{catalog: h.installer.GetCatalog, installed: h.AppInstalled}
}

// AllowClient reports whether the client is reachable for the tenant.
//
// A client that belongs to no external app is allowed: first-party clients, the
// developer portal's own, anything a tenant registered for itself. The mapping
// is rebuilt from the catalogue on each call rather than kept, because a
// registry sync can replace the catalogue at any moment and this list is a
// handful of entries long.
func (g externalAppGate) AllowClient(ctx context.Context, tenantID, clientID string) (bool, error) {
	appID := ""
	for _, app := range g.catalog() {
		external := app.Manifest.External
		if app.Manifest.IsExternal() && external != nil && external.SSOClientID == clientID {
			appID = app.ID
			break
		}
	}
	if appID == "" {
		return true, nil
	}
	return g.installed(ctx, tenantID, appID)
}
