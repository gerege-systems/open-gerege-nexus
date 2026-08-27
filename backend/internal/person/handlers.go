/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package person

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
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
