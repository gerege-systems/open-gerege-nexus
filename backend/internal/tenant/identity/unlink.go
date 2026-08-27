/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Taking a way in away again.
 *
 * It sat beside the profile screen that lists them, because that is where
 * somebody presses the button. What it edits is this package's two tables, and
 * the rule it enforces — never remove the last one — is about identities rather
 * than about profiles.
 */

package identity

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/auth"
)

// unlinkRequest names one identity to detach. Deliberately not an id: the
// caller says which provider and which account at that provider, and the query
// is scoped to their own user row, so there is no way to phrase this request
// that reaches somebody else's identity.
type unlinkRequest struct {
	Kind    string `json:"kind"`
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
}

// handleUnlinkIdentity detaches one linked identity from the caller's account.
//
// Deleting the row is the whole of it. There is no "disabled" state to fall
// back to, because a way in that is remembered but refused is still a record
// of somebody's national identity or their account elsewhere, kept after they
// asked for it to be forgotten.
//
// Signing in with that provider afterwards behaves exactly as it did the first
// time: unrecognised, so Google is parked and eID is asked for again. That is
// not a special case — it falls out of the row being gone.
func (h *Handlers) HandleUnlinkIdentity(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req unlinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "could not read the request")
		return
	}

	ctx := r.Context()

	// Counted here rather than trusted from the screen. The list the browser
	// holds may be minutes old, and the one identity it does not know about is
	// exactly the one that makes this safe or unsafe.
	current := h.LinkedIdentities(ctx, claims.UserID)
	if len(current) <= 1 {
		httpx.Error(w, http.StatusConflict, "the last way of signing in cannot be removed")
		return
	}

	var tag int64
	switch req.Kind {
	case "eid":
		ct, err := h.db.Exec(ctx,
			`DELETE FROM registry.user_eid_identities WHERE user_id = $1 AND person_etsi = $2`,
			claims.UserID, req.Subject)
		if err != nil {
			slog.Error("could not unlink an eID identity", "error", err, "user_id", claims.UserID)
			httpx.Error(w, http.StatusInternalServerError, "could not unlink the identity")
			return
		}
		tag = ct.RowsAffected()
	case "sso":
		ct, err := h.db.Exec(ctx,
			`DELETE FROM registry.user_sso_identities WHERE user_id = $1 AND issuer = $2 AND subject = $3`,
			claims.UserID, req.Issuer, req.Subject)
		if err != nil {
			slog.Error("could not unlink a provider identity", "error", err, "user_id", claims.UserID)
			httpx.Error(w, http.StatusInternalServerError, "could not unlink the identity")
			return
		}
		tag = ct.RowsAffected()
	default:
		httpx.Error(w, http.StatusBadRequest, "unknown identity kind")
		return
	}

	if tag == 0 {
		// Either it was never linked or somebody unlinked it already. Both are
		// the state the caller asked for, but saying so distinguishes a stale
		// screen from a successful removal.
		httpx.Error(w, http.StatusNotFound, "no such linked identity")
		return
	}

	slog.Info("a person unlinked one of their identities",
		"user_id", claims.UserID, "kind", req.Kind, "issuer", req.Issuer)
	httpx.JSON(w, http.StatusOK, map[string]any{"identities": h.LinkedIdentities(ctx, claims.UserID)})
}
