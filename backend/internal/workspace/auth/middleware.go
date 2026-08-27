/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package auth

import (
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

func (h *Handlers) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := TokenFromRequest(r)
		if token == "" {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized: missing session token")
			return
		}

		claims, err := h.sessions.Resolve(r.Context(), token)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized: invalid or expired session")
			return
		}

		// A suspended organisation is one nobody may act in, including the
		// people already signed in to it. Suspending revokes their sessions in
		// the same transaction, so this is the belt to that braces: a client
		// holding a token issued a moment before, or a replica whose cache is
		// a few seconds behind, is refused here.
		if h.RefuseIfSuspended(w, r, claims.WorkspaceID) {
			return
		}

		// Maintenance is checked after suspension and before anything else,
		// and only for writes: the point of a Maintenance window is that
		// people can still see what they need.
		if h.RefuseIfReadOnly(w, r, claims.WorkspaceID) {
			return
		}

		ctx := WithUserContext(r.Context(), claims)
		if claims.Impersonated {
			// Everything this request records is marked as ours. It is done
			// here, once, rather than in the handlers that write audit rows:
			// there are dozens of them, in every module, and a mark that each
			// of them has to remember is a mark that is missing from whichever
			// one somebody writes next.
			ctx = audit.MarkImpersonated(ctx, claims.ImpersonatedBy)
		}
		ctx = nexus.WithWorkspaceID(ctx, claims.WorkspaceID)
		// The organisations this session reads across, straight from the
		// session row. dbguard turns it into the policy's array; almost every
		// session carries none and behaves exactly as it always has.
		ctx = nexus.WithAllowedWorkspaces(ctx, claims.AllowedWorkspaceIDs)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin gates tenant-administrative endpoints. It must be layered after
// Middleware.
func (h *Handlers) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := UserFromContext(r.Context())
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if !claims.IsAdmin {
			httpx.Error(w, http.StatusForbidden, "forbidden: tenant administrator role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
