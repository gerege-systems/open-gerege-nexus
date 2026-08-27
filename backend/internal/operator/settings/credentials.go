/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package settings

import (
	"context"
	"errors"
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/credentials"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// The keys this deployment reaches other systems with.
//
// Three things are true of every handler below and none of them is true of the
// settings handlers beside it:
//
//   - **Nothing here reads a value out.** There is no GET that returns one and
//     no field on the wire that could carry one. What an operator gets back is
//     a name, whether it is set, where it came from and four characters.
//   - **Every write re-confirms the second factor.** Setting a credential is
//     pointing this platform at a system it will then trust, which is the
//     shape of an attack somebody performs with a stolen console session.
//   - **The audit row carries no value.** It records that the key changed and
//     who changed it. A trail that quoted the old value would be a list of
//     this deployment's retired credentials, kept for ever, readable by every
//     auditor.

// ErrNoCredentialStore is a deployment whose console was built without one.
var ErrNoCredentialStore = errors.New("this deployment holds no credential store")

// ListCredentials returns every registered credential and where its value comes
// from. Never a value.
func (s *Service) ListCredentials(ctx context.Context) ([]credentials.Status, error) {
	if s.credentials == nil {
		return nil, ErrNoCredentialStore
	}
	return s.credentials.List(operator.Scoped(ctx))
}

// SetCredential seals a new value and records that it changed.
func (s *Service) SetCredential(ctx context.Context, sess operator.Session, name, value, reason string) error {
	if s.credentials == nil {
		return ErrNoCredentialStore
	}
	if _, known := credentials.Lookup(name); !known {
		return credentials.ErrUnknownCredential
	}
	if !security.EncryptionConfigured() {
		return security.ErrNoEncryptionKey
	}
	if err := s.op.Do(ctx, sess, operator.Change{
		Action:     "credential.set",
		TargetType: "credential",
		TargetID:   name,
		Reason:     reason,
		// No Before and no value in After. What changed is the fact, not the
		// secret, and an audit table is the last place a secret should land.
		After: map[string]any{"source": "database"},
	}, func(ctx context.Context, tx pgx.Tx) error {
		return s.credentials.Set(ctx, tx, name, value, sess.ID)
	}); err != nil {
		return err
	} else {
		s.credentials.Changed(ctx)
		return nil
	}
}

// ClearCredential removes the stored value, so the deployment falls back to its
// environment variable — or to having none, which for every credential here is
// an ordinary state with a feature switched off.
func (s *Service) ClearCredential(ctx context.Context, sess operator.Session, name, reason string) error {
	if s.credentials == nil {
		return ErrNoCredentialStore
	}
	if _, known := credentials.Lookup(name); !known {
		return credentials.ErrUnknownCredential
	}
	if err := s.op.Do(ctx, sess, operator.Change{
		Action:     "credential.clear",
		TargetType: "credential",
		TargetID:   name,
		Reason:     reason,
		After:      map[string]any{"source": "environment or unset"},
	}, func(ctx context.Context, tx pgx.Tx) error {
		return s.credentials.Clear(ctx, tx, name, sess.ID)
	}); err != nil {
		return err
	} else {
		s.credentials.Changed(ctx)
		return nil
	}
}

func (s *Service) handleListCredentials(w http.ResponseWriter, r *http.Request) {
	list, err := s.ListCredentials(r.Context())
	if err != nil {
		fail(w, err, "could not read the credentials")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"credentials": list,
		// Whether a value could be stored at all. Without the deployment's
		// encryption key the screen shows the fields as read-only and says
		// why, rather than refusing at the moment somebody presses save.
		"sealing_configured": security.EncryptionConfigured(),
	})
}

func (s *Service) handleSetCredential(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		operator.Reasoned
		Value string `json:"value"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.SetCredential(r.Context(), sess, chi.URLParam(r, "name"), body.Value, body.Reason); err != nil {
		fail(w, err, "could not set the credential")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (s *Service) handleClearCredential(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body operator.Reasoned
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.ClearCredential(r.Context(), sess, chi.URLParam(r, "name"), body.Reason); err != nil {
		fail(w, err, "could not clear the credential")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}
