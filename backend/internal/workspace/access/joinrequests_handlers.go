/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package access

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
)

// HandleJoinRequests is the queue. Administrator only, like everything else
// under /admin/access: it is a list of people's names and addresses, and the
// decision it leads to is who may act here.
func (h *Handlers) HandleJoinRequests(w http.ResponseWriter, r *http.Request) {
	queue, err := h.PendingJoinRequests(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to list join requests")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"requests": queue})
}

// HandleDecideJoinRequest accepts or declines one.
//
// The verb is in the body rather than in two routes, because it is one decision
// with two answers and an administrator who can give either can give both — two
// routes would suggest a permission boundary that is not there.
func (h *Handlers) HandleDecideJoinRequest(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Accept bool `json:"accept"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "malformed request body")
		return
	}

	switch err := h.Decide(r.Context(), chi.URLParam(r, "id"), claims.UserID, body.Accept); {
	case err == nil:
		httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
	case errors.Is(err, ErrNoSuchRequest):
		httpx.Error(w, http.StatusNotFound, "no open request with that id")
	default:
		// A quota refusal reaches the administrator in its own words: it is the
		// one failure here they can do something about, and "internal error"
		// would send them looking in the wrong place.
		var refusal auth.SignInError
		if errors.As(err, &refusal) {
			httpx.Error(w, http.StatusConflict, refusal.Error())
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "failed to decide the request")
	}
}
