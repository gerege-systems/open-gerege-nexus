/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package appinstall

import (
	"log/slog"
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/flags"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/access"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

func (h *Handlers) GateMiddleware(appID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return h.authn.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, ok := nexus.RequireWorkspace(w, r)
			if !ok {
				return
			}

			// Whether a tenant has this app — see AppInstalled, which the
			// authorization endpoint asks the same question of on behalf of
			// apps that run outside this binary. A database that cannot answer
			// refuses here rather than admitting: this is the check that keeps
			// one tenant out of another tenant's application.
			// A module's kill switch, before the installation check: an app
			// being switched off platform-wide is an operator's decision
			// during an incident, and it should not depend on what any
			// organisation has installed.
			if flags.Enabled(r.Context(), flags.ModuleKillSwitch(appID)) {
				httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{
					"error": "энэ модулийг түр хугацаанд унтраасан байна",
				})
				return
			}

			enabled, err := h.AppInstalled(r.Context(), tenantID, appID)
			if err != nil {
				slog.Error("could not check the app installation", "error", err,
					"app_id", appID, "tenant_id", tenantID)
			}

			if !enabled {
				httpx.Error(w, http.StatusForbidden, "forbidden: app module "+appID+" is not installed or enabled for this tenant")
				return
			}

			// Model-level access rights are additive across all assigned roles,
			// matching Odoo's ir.model.access behaviour. Government workflow has
			// its own action- and unit-aware permission checks.
			if permission := appRequestPermission(appID, r.Method, r.URL.Path); permission != "" {
				access.RequirePermission(h.permissions, permission)(next).ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}

// appRequestPermission asks the module which permission this request needs.
//
// The mapping used to be a literal in this function listing five apps by ID.
// That worked while every module was compiled from this repository and broke
// quietly the moment one was not: an extracted module simply fell out of the
// map, and falling out of the map means no permission is required. See
// nexus.AccessPolicy for why the answer now travels with the module.
func appRequestPermission(appID, method, path string) string {
	mod, found := lookupModule(appID)
	if !found {
		return ""
	}
	prefix := nexus.RoutePermissionPrefixOf(mod)
	if prefix == "" {
		return ""
	}
	if method == http.MethodGet || method == http.MethodHead {
		return prefix + ".read"
	}
	return prefix + ".manage"
}
