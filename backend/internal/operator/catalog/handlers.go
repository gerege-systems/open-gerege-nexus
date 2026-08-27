/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package catalog is the console's view of the app catalogue: where it came
// from, when it was last fetched, and asking for it again.
//
// It stands on internal/operator/operator, which decides who is asking and
// records what they did; nothing in this plane stands on this package.
package catalog

import (
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/go-chi/chi/v5"
)

func (s *Service) handleCatalogStatusRoute(w http.ResponseWriter, r *http.Request) {
	status := s.observability.CatalogStatus(r.Context())
	httpx.JSON(w, http.StatusOK, status)
}

func (s *Service) handleCatalogOverviewRoute(w http.ResponseWriter, r *http.Request) {
	status := s.observability.CatalogStatus(r.Context())
	httpx.JSON(w, http.StatusOK, map[string]any{
		"catalog":  status,
		"platform": s.observability.Version(r.Context()),
	})
}

func (s *Service) handleCatalogSyncRoute(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body operator.Reasoned
	if !operator.Decode(w, r, &body) {
		return
	}
	if len(body.Reason) > 500 {
		httpx.Error(w, http.StatusBadRequest, "reason must be 500 characters or less")
		return
	}
	if s.syncCatalogFn == nil {
		httpx.Error(w, http.StatusNotImplemented, "this deployment reads its app catalog from a file; there is no registry to sync with")
		return
	}
	changed, err := s.syncCatalogFn(r.Context())
	if err != nil {
		fail(w, err, "could not sync the catalog")
		return
	}
	// Record the audit trail outside the sync transaction: the catalog sync
	// writes to the platform database, while the audit record belongs in the
	// operator database. They are separate concerns and must not share a tx.
	if err := s.op.Do(r.Context(), sess, operator.Change{
		Action:     "catalog.sync",
		TargetType: "platform",
		TargetID:   "catalog",
		Reason:     body.Reason,
	}, nil); err != nil {
		fail(w, err, "could not record the audit trail")
		return
	}
	status := "unchanged"
	if changed {
		status = "updated"
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": status, "changed": changed})
}

// Routes are this screen's, mounted on the console's signed-in group. The
// capability each one asks for is the decision about who may see or do it.
func (s *Service) Routes(r chi.Router) {
	r.With(s.op.RequireCapability(operator.CapTenantRead)).Get("/catalog/status", s.handleCatalogStatusRoute)
	r.With(s.op.RequireCapability(operator.CapTenantRead)).Get("/catalog/overview", s.handleCatalogOverviewRoute)
	r.With(s.op.RequireCapability(operator.CapSettingsWrite), s.op.RequireStepUp).
		Post("/catalog/sync", s.handleCatalogSyncRoute)
}
