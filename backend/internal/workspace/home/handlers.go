/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package home

import (
	"net/http"
	"strconv"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
)

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
