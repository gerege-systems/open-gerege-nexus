/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package tenants is an organisation as the deployment sees it: created,
// suspended, resumed, exported, deleted after a grace period, and held to a quota.
//
// It stands on internal/operator/operator, which decides who is asking and
// records what they did; nothing in this plane stands on this package.
package tenants

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// SetTenantMaintenance opens or closes one organisation for writing.
func (s *Service) SetTenantMaintenance(ctx context.Context, sess operator.Session, tenantID string, on bool, message, reason string) error {
	before, err := s.op.StateOf(ctx, tenantID)
	if err != nil {
		return err
	}
	defer s.changed(tenantID)
	return s.op.Do(ctx, sess, operator.Change{
		Action:     "tenant.maintenance",
		TargetType: "tenant",
		TargetID:   tenantID,
		Reason:     reason,
		Before:     before,
		After:      map[string]any{"maintenance": on, "message": message},
	}, func(ctx context.Context, tx pgx.Tx) error {
		if !on {
			_, err := tx.Exec(ctx,
				`UPDATE registry.tenants SET maintenance_at = NULL, maintenance_message = '' WHERE id = $1::uuid`,
				tenantID)
			return err
		}
		_, err := tx.Exec(ctx,
			`UPDATE registry.tenants SET maintenance_at = NOW(), maintenance_message = $2 WHERE id = $1::uuid`,
			tenantID, message)
		return err
	})
}

func (s *Service) handleListTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := s.ListTenants(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		slog.Error("control plane: could not list the organisations", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "could not read the organisations")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"tenants": tenants})
}

func (s *Service) handleGetTenant(w http.ResponseWriter, r *http.Request) {
	detail, err := s.GetTenant(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, operator.ErrTenantNotFound) {
		httpx.Error(w, http.StatusNotFound, "no such organisation")
		return
	}
	if err != nil {
		// An id that is not a UUID reaches here as a database error rather than
		// as operator.ErrTenantNotFound, and answering 500 for a typed-in URL would put
		// a red line in the error tracker for somebody's slip.
		if operator.IsInvalidUUID(err) {
			httpx.Error(w, http.StatusNotFound, "no such organisation")
			return
		}
		slog.Error("control plane: could not read the organisation", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "could not read the organisation")
		return
	}
	httpx.JSON(w, http.StatusOK, detail)
}

func (s *Service) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var params NewTenant
	if !operator.Decode(w, r, &params) {
		return
	}
	created, err := s.CreateTenant(r.Context(), sess, params)
	if err != nil {
		fail(w, err, "could not create the organisation")
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

func (s *Service) handleSuspendTenant(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body operator.Reasoned
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.Suspend(r.Context(), sess, chi.URLParam(r, "id"), body.Reason); err != nil {
		fail(w, err, "could not suspend the organisation")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "suspended"})
}

func (s *Service) handleResumeTenant(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body operator.Reasoned
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.Resume(r.Context(), sess, chi.URLParam(r, "id"), body.Reason); err != nil {
		fail(w, err, "could not resume the organisation")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "active"})
}

func (s *Service) handleCancelDeletion(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body operator.Reasoned
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.CancelDeletion(r.Context(), sess, chi.URLParam(r, "id"), body.Reason); err != nil {
		fail(w, err, "could not cancel the deletion")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deletion cancelled"})
}

func (s *Service) handleExportTenant(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	bundle, err := s.ExportTenant(r.Context(), sess, chi.URLParam(r, "id"))
	if err != nil {
		fail(w, err, "could not export the organisation")
		return
	}
	w.Header().Set("Content-Disposition",
		`attachment; filename="`+bundle.Tenant.Slug+`-export.json"`)
	httpx.JSON(w, http.StatusOK, bundle)
}

func (s *Service) handleSetQuota(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		Quota
		Reason string `json:"reason"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.SetQuota(r.Context(), sess, chi.URLParam(r, "id"), body.Quota, body.Reason); err != nil {
		fail(w, err, "could not set the limits")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (s *Service) handleListDeletions(w http.ResponseWriter, r *http.Request) {
	pending, err := s.TenantsAwaitingDeletion(r.Context())
	if err != nil {
		fail(w, err, "could not list the organisations awaiting deletion")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"tenants": pending})
}

func (s *Service) handleImpersonate(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		operator.Reasoned
		UserID string `json:"user_id"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	link, err := s.op.BeginImpersonation(r.Context(), sess, chi.URLParam(r, "id"), body.UserID, body.Reason)
	if err != nil {
		fail(w, err, "could not start the session")
		return
	}
	// The link is returned rather than redirected to: the console is on
	// another hostname, and the operator's browser has to make the journey
	// itself for the cookie to land where it belongs.
	httpx.JSON(w, http.StatusOK, map[string]any{
		"url":     link,
		"minutes": int(operator.ImpersonationWindow.Minutes()),
	})
}

func (s *Service) handleTenantMaintenance(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		operator.Reasoned
		On      bool   `json:"on"`
		Message string `json:"message"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.SetTenantMaintenance(r.Context(), sess, chi.URLParam(r, "id"),
		body.On, body.Message, body.Reason); err != nil {
		fail(w, err, "could not change the maintenance state")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// Routes are this screen's, mounted on the console's signed-in group. The
// capability each one asks for is the decision about who may see or do it.
func (s *Service) Routes(r chi.Router) {

	r.With(s.op.RequireCapability(operator.CapTenantRead)).Get("/tenants", s.handleListTenants)
	r.With(s.op.RequireCapability(operator.CapTenantRead)).Get("/tenants/{id}", s.handleGetTenant)

	// The organisation's life. Suspension is reversible and needs one
	// operator; deletion is not and needs two, which is why it goes
	// through the approvals below rather than having a route of its own
	// that does the deed.
	r.With(s.op.RequireCapability(operator.CapTenantCreate)).
		Post("/tenants", s.handleCreateTenant)
	r.With(s.op.RequireCapability(operator.CapTenantSuspend), s.op.RequireStepUp).
		Post("/tenants/{id}/suspend", s.handleSuspendTenant)
	r.With(s.op.RequireCapability(operator.CapTenantSuspend), s.op.RequireStepUp).
		Post("/tenants/{id}/resume", s.handleResumeTenant)
	// Cancelling a deletion needs neither a second person nor a second
	// factor. It is the safe direction: the asymmetry is the point of a
	// grace period, and a recovery that is harder than the mistake is a
	// recovery nobody manages in time.
	r.With(s.op.RequireCapability(operator.CapTenantSuspend)).
		Delete("/tenants/{id}/deletion", s.handleCancelDeletion)
	// The export reads the organisation's actual data, so it is gated like
	// the deletion it usually precedes rather than like a read: the same
	// capability, and a second factor. See export.go for why this one
	// action is allowed to leave the console's usual boundary.
	r.With(s.op.RequireCapability(operator.CapTenantDelete), s.op.RequireStepUp).
		Get("/tenants/{id}/export", s.handleExportTenant)
	r.With(s.op.RequireCapability(operator.CapQuotaWrite), s.op.RequireStepUp).
		Put("/tenants/{id}/quota", s.handleSetQuota)

	// What is counting down. On its own route rather than inside the
	// organisation list, because it is the one screen an operator should
	// look at without being asked to.
	r.With(s.op.RequireCapability(operator.CapTenantRead)).Get("/deletions", s.handleListDeletions)

	r.With(s.op.RequireCapability(operator.CapImpersonate), s.op.RequireStepUp).
		Post("/tenants/{id}/impersonate", s.handleImpersonate)

	r.With(s.op.RequireCapability(operator.CapSettingsWrite)).
		Post("/tenants/{id}/maintenance", s.handleTenantMaintenance)
}
