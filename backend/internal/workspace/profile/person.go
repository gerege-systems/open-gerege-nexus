/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * A person's own record of themselves.
 *
 * Everything here answers for the caller and only the caller. There is no id
 * parameter anywhere in this file on purpose: the session decides whose record
 * is read, so there is no version of these queries that can be pointed at
 * somebody else. An administrator looking at another person belongs behind the
 * access-control screens, which is a different question with a different
 * answer.
 *
 * It is a platform screen rather than an installed app. Apps are installed per
 * organisation and an administrator can remove one; a person's own record of
 * which identities are linked to their account is not something their employer
 * should be able to take away. And somebody who belongs to several
 * organisations has one profile, not one per membership.
 */

package profile

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// handleProfile answers with the caller's own record.
func (h *Handlers) HandleProfile(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	ctx := r.Context()

	var name, email string
	var createdAt time.Time
	if err := h.db.QueryRow(ctx,
		`SELECT name, email, created_at FROM registry.users WHERE id = $1`, claims.UserID).
		Scan(&name, &email, &createdAt); err != nil {
		slog.Error("could not read a profile", "error", err, "user_id", claims.UserID)
		httpx.Error(w, http.StatusInternalServerError, "could not load the profile")
		return
	}

	// The organisations this person belongs to. Crosses tenants by definition,
	// so it runs on the platform path — under the caller's own policies a
	// membership elsewhere is not visible, and the list would be one long.
	memberships, err := h.sessions.TenantsForUser(nexus.WithoutWorkspace(ctx), claims.UserID)
	if err != nil {
		slog.Warn("could not list a person's organisations", "error", err)
		memberships = nil
	}

	identities := h.identity.LinkedIdentities(ctx, claims.UserID)

	// How many other places this account is signed in. Not the tokens — those
	// are never readable — only that they exist, which is what somebody needs
	// in order to decide whether to end them.
	var activeSessions int
	_ = h.db.QueryRow(ctx,
		`SELECT count(*) FROM workspace.sessions
		  WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > NOW()`,
		claims.UserID).Scan(&activeSessions)

	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":              claims.UserID,
		"name":            name,
		"email":           email,
		"created_at":      createdAt,
		"is_admin":        claims.IsAdmin,
		"current_tenant":  claims.WorkspaceID,
		"organisations":   memberships,
		"identities":      identities,
		"active_sessions": activeSessions,
	})
}
