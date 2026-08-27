/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package approvals is the second person: a deletion is asked for by one
// operator and agreed to by another, and never by the same one twice.
//
// It stands on internal/operator/operator, which decides who is asking and
// records what they did; nothing in this plane stands on this package.
package approvals

import (
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/go-chi/chi/v5"
)

func (s *Service) handleListApprovals(w http.ResponseWriter, r *http.Request) {
	approvals, err := s.ListApprovals(r.Context())
	if err != nil {
		fail(w, err, "could not list the open requests")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"approvals": approvals})
}

func (s *Service) handleApprove(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body operator.Reasoned
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.Approve(r.Context(), sess, chi.URLParam(r, "id"), body.Reason); err != nil {
		fail(w, err, "could not approve the request")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (s *Service) handleReject(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body operator.Reasoned
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.Reject(r.Context(), sess, chi.URLParam(r, "id"), body.Reason); err != nil {
		fail(w, err, "could not reject the request")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

// Routes are this screen's, mounted on the console's signed-in group. The
// capability each one asks for is the decision about who may see or do it.
func (s *Service) handleRequestDeletion(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body operator.Reasoned
	if !operator.Decode(w, r, &body) {
		return
	}
	approvalID, err := s.RequestDeletion(r.Context(), sess, chi.URLParam(r, "id"), body.Reason)
	if err != nil {
		fail(w, err, "could not ask for the organisation to be deleted")
		return
	}
	// Deliberately explicit about what has and has not happened: the operator
	// pressed "delete" and nothing has been deleted.
	httpx.JSON(w, http.StatusAccepted, map[string]any{
		"status":      "awaiting a second superadmin",
		"approval_id": approvalID,
		"grace_days":  int(DeletionGrace.Hours() / 24),
	})
}

// Routes are this screen's: asking for a deletion, and the second person
// agreeing to it.
func (s *Service) Routes(r chi.Router) {
	r.With(s.op.RequireCapability(operator.CapTenantDelete), s.op.RequireStepUp).
		Post("/tenants/{id}/deletion", s.handleRequestDeletion)

	r.With(s.op.RequireCapability(operator.CapApprove)).Get("/approvals", s.handleListApprovals)
	r.With(s.op.RequireCapability(operator.CapApprove), s.op.RequireStepUp).
		Post("/approvals/{id}/approve", s.handleApprove)
	r.With(s.op.RequireCapability(operator.CapApprove)).
		Post("/approvals/{id}/reject", s.handleReject)
}
