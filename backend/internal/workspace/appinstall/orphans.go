/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package appinstall

import (
	"context"
	"log/slog"
	"sort"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// ReportUncarriedApps says which installed apps this binary cannot serve.
//
// An installation row outlives the module. When an app moves to another
// repository — six did on 2026-08-23 — every tenant that had it keeps the row,
// and this binary stops carrying the code: the routes answer 404, the menus API
// returns nothing for it, and the app simply disappears from the sidebar. On a
// phone, where the app switcher is the only way between apps, that reads as the
// menu having broken.
//
// Nothing was wrong with the deployment and nothing said anything. That is the
// failure this reports: not that the state is invalid — an operator upgrading a
// platform past an app's departure is in it legitimately — but that it was
// silent. The answer is a line at startup naming the apps, so "where did our
// documents go" is answered by the log rather than by a bisect.
//
// External apps are excluded: they run somewhere else and this binary was never
// meant to carry them.
func (ai *AppInstaller) ReportUncarriedApps(ctx context.Context) {
	rows, err := ai.db.Query(ctx,
		`SELECT DISTINCT app_id FROM workspace.app_installations WHERE status = 'installed' AND enabled`)
	if err != nil {
		slog.Warn("catalog: could not check which installed apps this binary carries", "error", err)
		return
	}
	defer rows.Close()

	var uncarried []string
	for rows.Next() {
		var appID string
		if err := rows.Scan(&appID); err != nil {
			slog.Warn("catalog: could not read an installation row", "error", err)
			return
		}
		if app, known := ai.GetAppByID(appID); known && app.Manifest.IsExternal() {
			continue
		}
		if nexus.VerifyModuleExists(appID) != nil {
			uncarried = append(uncarried, appID)
		}
	}
	if err := rows.Err(); err != nil {
		slog.Warn("catalog: could not read the installation rows", "error", err)
		return
	}
	if len(uncarried) == 0 {
		return
	}
	sort.Strings(uncarried)
	// Error rather than warning: every one of these is an app somebody installed
	// and can no longer reach, and the people who notice first are the ones
	// using it.
	slog.Error("catalog: organisations have apps installed that this binary does not carry;"+
		" their routes answer 404 and they appear in no sidebar."+
		" Either deploy a distribution that compiles them, or uninstall them",
		"apps", uncarried)
}
