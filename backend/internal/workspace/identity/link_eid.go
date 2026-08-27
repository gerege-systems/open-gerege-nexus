/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Linking an eID to an account somebody is already signed in to.
 *
 * The three eID flows this platform had all ended in a session: sign in with
 * eID, or prove who you are so a parked provider identity can be attached.
 * Neither is what a person needs from their profile screen, where they are
 * already signed in and want the platform to know their national identity.
 *
 * For a citizen that is not a convenience. The Gerege number arrives with the
 * eID and nowhere else, and it is what a supplier's module names them by when
 * it publishes the state of a request into their home — see pkg/nexus.PersonFeed
 * and migration 00086. An account opened with a password has no Gerege number
 * at all, so until this existed a citizen could sign in, ask an organisation
 * for something, and still be unreachable by the thing that answers.
 */

package identity

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
)

// HandleLinkEID finishes an eID session against the signed-in account.
//
// The session is started by the ordinary /auth/eid/start, which is public and
// unchanged: starting one proves nothing and needs no account. What needed an
// authenticated endpoint is the finish, because the answer is written to
// whoever is asking — and "whoever is asking" is the session, never the body.
func (h *Handlers) HandleLinkEID(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req struct {
		SessionID string `json:"session_id"`
	}
	if httpx.DecodeLimited(r, &req, 8<<10) != nil || strings.TrimSpace(req.SessionID) == "" {
		httpx.Error(w, http.StatusBadRequest, "session_id is required")
		return
	}

	result, err := h.eidSvc.Poll(r.Context(), req.SessionID)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "eID Mongolia session check failed")
		return
	}
	// Still waiting on the phone. Handed back as it came, so the screen can
	// keep polling with the same code it uses for signing in.
	if result.State != "COMPLETE" {
		httpx.JSON(w, http.StatusOK, result)
		return
	}
	if result.Identity == nil || !result.Identity.VerifiedStatus {
		httpx.Error(w, http.StatusUnauthorized, "eID identity verification failed")
		return
	}

	// Strict, unlike the sign-in path's best-effort call: this write is the
	// whole of what was asked for, so a failure has to reach the person who
	// asked rather than a log line they will never read.
	if err := h.authn.LinkEIDIdentityStrict(r.Context(), claims.UserID, result.Identity); err != nil {
		if errors.Is(err, auth.ErrEIDBelongsToSomebodyElse) {
			// 409 rather than 403: nothing is forbidden, the identity is
			// simply spoken for. The distinction matters on screen, where the
			// useful next step is "sign in as that account", not "ask an
			// administrator".
			httpx.Error(w, http.StatusConflict, err.Error())
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "could not link the eID identity")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"state":      "COMPLETE",
		"identities": h.LinkedIdentities(r.Context(), claims.UserID),
	})
}
