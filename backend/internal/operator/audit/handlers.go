/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package audit is the trail every console write leaves. Reading it is a
// screen; writing it is not optional and not here — see internal/operator/operator.
//
// It stands on internal/operator/operator, which decides who is asking and
// records what they did; nothing in this plane stands on this package.
package audit

import (
	"log/slog"
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/go-chi/chi/v5"
)

func (s *Service) handleListAudit(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	entries, err := s.ListAudit(r.Context(),
		query.Get("action"), query.Get("target_type"), query.Get("target_id"))
	if err != nil {
		slog.Error("control plane: could not read the audit trail", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "could not read the audit trail")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Service) handleListOperators(w http.ResponseWriter, r *http.Request) {
	operators, err := s.ListOperators(r.Context())
	if err != nil {
		slog.Error("control plane: could not list the operators", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "could not read the operators")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"operators": operators})
}

// Routes are this screen's, mounted on the console's signed-in group. The
// capability each one asks for is the decision about who may see or do it.
func (s *Service) Routes(r chi.Router) {
	r.With(s.op.RequireCapability(operator.CapAuditRead)).Get("/audit", s.handleListAudit)
	r.With(s.op.RequireCapability(operator.CapOperatorRead)).Get("/operators", s.handleListOperators)
}
