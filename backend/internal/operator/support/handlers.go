/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package support is the help desk — finding a person, unlocking them, ending
// their sessions, and handing them a link back in.
//
// It stands on internal/operator/operator, which decides who is asking and
// records what they did; nothing in this plane stands on this package.
package support

import (
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/go-chi/chi/v5"
)

func (s *Service) handleFindPeople(w http.ResponseWriter, r *http.Request) {
	people, err := s.FindPeople(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		fail(w, err, "could not search for people")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"people": people})
}

func (s *Service) handleUnlock(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body operator.Reasoned
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.Unlock(r.Context(), sess, chi.URLParam(r, "id"), body.Reason); err != nil {
		fail(w, err, "could not unlock the account")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "unlocked"})
}

func (s *Service) handleRevokeSessions(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body operator.Reasoned
	if !operator.Decode(w, r, &body) {
		return
	}
	ended, err := s.RevokeSessions(r.Context(), sess, chi.URLParam(r, "id"), body.Reason)
	if err != nil {
		fail(w, err, "could not end the sessions")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "revoked", "sessions": ended})
}

func (s *Service) handleCredentialLink(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		operator.Reasoned
		// TenantID is which organisation the mail is sent on behalf of. The
		// verification service counts its quota per organisation, so the
		// answer cannot be "none" — the console sends the one the operator was
		// looking at.
		TenantID string `json:"tenant_id"`
		Purpose  string `json:"purpose"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	if body.Purpose == "" {
		body.Purpose = "reset"
	}
	if err := s.SendCredentialLink(r.Context(), sess, chi.URLParam(r, "id"),
		body.TenantID, body.Purpose, body.Reason); err != nil {
		fail(w, err, "could not send the link")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// Routes are this screen's, mounted on the console's signed-in group. The
// capability each one asks for is the decision about who may see or do it.
func (s *Service) Routes(r chi.Router) {

	// The help desk.
	r.With(s.op.RequireCapability(operator.CapSupport)).Get("/people", s.handleFindPeople)
	r.With(s.op.RequireCapability(operator.CapSupport), s.op.RequireStepUp).
		Post("/people/{id}/unlock", s.handleUnlock)
	r.With(s.op.RequireCapability(operator.CapSupport), s.op.RequireStepUp).
		Post("/people/{id}/sessions/revoke", s.handleRevokeSessions)
	r.With(s.op.RequireCapability(operator.CapSupport), s.op.RequireStepUp).
		Post("/people/{id}/credential-link", s.handleCredentialLink)
}
