/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package person

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// Routes mounts what a person asks for on their own behalf.
//
// The gate is handed in rather than imported, and that is the whole reason this
// package can sit beside the workspace plane instead of inside it. Resolving a
// session is the workspace plane's work — it owns the table, the cookie and the
// rules about suspension — and importing it here would bring every query in
// that package with it, each one written for somebody acting inside an
// organisation. So the host, which is the one file allowed to name more than
// one of these trees, passes the middleware in. See pkg/host/server.go.
func (s *Store) Routes(r chi.Router, gate func(http.Handler) http.Handler) {
	r.Route("/api/v1/me", func(mr chi.Router) {
		mr.Use(gate)
		mr.Get("/items", s.HandleItems)
		mr.Post("/join-requests", s.HandleAsk)
	})
}

// HandleItems answers "what did I ask for, and where has it got to".
//
// Behind the ordinary authenticated group, with no permission of its own. A
// permission would be the wrong shape: this endpoint answers for the workspace
// the request already acts in, and in a home that is one person — there is
// nobody to withhold it from. The row-level policy is what makes that true, and
// it is true whether the workspace is a home or a company: an organisation that
// nothing has published into gets an empty list, not somebody else's.
func (s *Store) HandleItems(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, _ = strconv.Atoi(raw)
	}
	items, err := s.Items(r.Context(), limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to list requests")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// HandleAsk is somebody asking an organisation to let them in.
//
// By slug, because that is the name a person is given when they are told where
// to apply — it is in the address of every screen that organisation serves. The
// alternative is a picker over every organisation on the deployment, which is a
// directory, and a directory is a decision about what a citizen may enumerate
// rather than a detail of this endpoint.
func (s *Store) HandleAsk(w http.ResponseWriter, r *http.Request) {
	claims, err := nexus.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Slug    string `json:"slug"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "malformed request body")
		return
	}

	switch err := s.Ask(r.Context(), claims.UserID, body.Slug, body.Message); {
	case err == nil:
		httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
	case errors.Is(err, ErrNotAsked):
		httpx.Error(w, http.StatusNotFound, "no organisation answers to that name")
	default:
		// Everything the function refuses is something the person can act on —
		// already a member, the organisation is closed — so its own words go
		// back rather than a number. They are database messages, which is not
		// ideal prose; the screen has the room to say it better and the
		// endpoint should not decide that for it.
		httpx.Error(w, http.StatusConflict, err.Error())
	}
}
