/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package metering is what each organisation used, as the console charts it and
// as a spreadsheet.
//
// It stands on internal/operator/operator, which decides who is asking and
// records what they did; nothing in this plane stands on this package.
package metering

import (
	"log/slog"
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/go-chi/chi/v5"
)

func (s *Service) handleUsage(w http.ResponseWriter, r *http.Request) {
	usage, err := s.UsageFor(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		fail(w, err, "could not read the usage")
		return
	}
	httpx.JSON(w, http.StatusOK, usage)
}

func (s *Service) handleUsageCSV(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "id")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="usage-`+tenantID+`.csv"`)
	if err := s.WriteUsageCSV(r.Context(), w, tenantID); err != nil {
		// The header is already on the wire by the time this can fail, so
		// there is no status left to send: the log is where this goes, and the
		// operator sees a short file.
		slog.Error("control plane: could not write the usage export", "error", err)
	}
}

// Routes are this screen's, mounted on the console's signed-in group. The
// capability each one asks for is the decision about who may see or do it.
func (s *Service) Routes(r chi.Router) {

	// What each organisation used, and the same thing as a spreadsheet.
	r.With(s.op.RequireCapability(operator.CapTenantRead)).
		Get("/tenants/{id}/usage", s.handleUsage)
	r.With(s.op.RequireCapability(operator.CapTenantRead)).
		Get("/tenants/{id}/usage.csv", s.handleUsageCSV)
}
