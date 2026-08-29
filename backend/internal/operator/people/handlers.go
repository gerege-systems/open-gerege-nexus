/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package people

import (
	"net/http"
	"strconv"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/go-chi/chi/v5"
)

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	roster, err := s.List(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("filter"), offset)
	if err != nil {
		fail(w, err, "could not read the people")
		return
	}
	httpx.JSON(w, http.StatusOK, roster)
}

func (s *Service) handleRead(w http.ResponseWriter, r *http.Request) {
	detail, err := s.Read(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		fail(w, err, "could not read the person")
		return
	}
	httpx.JSON(w, http.StatusOK, detail)
}

// Routes are this screen's. Both are reads of personal data — names,
// addresses, which organisations somebody belongs to — so both ask for the
// capability that already draws that line: support may see people, and the
// operator role deliberately may not (§2.2).
func (s *Service) Routes(r chi.Router) {
	r.With(s.op.RequireCapability(operator.CapSupport)).Get("/people/roster", s.handleList)
	r.With(s.op.RequireCapability(operator.CapSupport)).Get("/people/{id}", s.handleRead)
}
