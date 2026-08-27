/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The tenant-facing half of two things the control plane starts: choosing a
 * password from an invitation or a reset, and stepping into an organisation as
 * somebody who works there.
 */

package access

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/jackc/pgx/v5"
)

// Both flows here start in the console, on another hostname, and finish here.
// They have to: a cookie belongs to one host, and the person setting a password
// is not an operator — they are a citizen with a link, reaching the tenant
// plane on the address every tenant uses.
//
// Both are therefore unauthenticated endpoints that accept a token — which is
// the shape that has to be got right. In both cases the token is
//
//	256 bits of crypto/rand, stored only as a SHA-256 digest,
//	single-use (claimed with a conditional UPDATE, not read-then-write),
//	short-lived, and useless for anything but the one act it names.
//
// The conditional UPDATE is the part worth being deliberate about: two
// requests arriving together must not both find an unredeemed row.

// minChosenPasswordLength is what somebody setting their own password is held
// to. Long rather than complicated: a length rule is the one requirement that
// reliably adds entropy without teaching people to write it on a note.
const minChosenPasswordLength = 10

// maxRecoveryBody bounds these bodies. A token and a password.
const maxRecoveryBody = 4 << 10

func hashRecoveryToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// HandleCredentialCheck says whether a link is still worth filling in a form
// for.
//
// It answers the same shape for an unknown token as for an expired one, so the
// endpoint cannot be used to tell which tokens exist. What it does reveal, for
// a valid token, is the address it belongs to — which is the address of the
// person holding the link, in their own mail.
func (h *Handlers) HandleCredentialCheck(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	var email, purpose string
	err := h.db.QueryRow(r.Context(),
		`SELECT u.email, g.purpose
		   FROM registry.credential_grants g
		   JOIN registry.users u ON u.id = g.user_id
		  WHERE g.token_hash = $1 AND g.redeemed_at IS NULL AND g.expires_at > NOW()`,
		hashRecoveryToken(token)).Scan(&email, &purpose)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("could not check a credential link", "error", err)
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"valid": false})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"valid": true, "email": email, "purpose": purpose,
	})
}

// HandleCredentialRedeem sets a password and spends the link.
func (h *Handlers) HandleCredentialRedeem(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRecoveryBody)
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		httpx.Error(w, http.StatusBadRequest, "a link and a password are required")
		return
	}
	if len(req.Password) < minChosenPasswordLength {
		httpx.Error(w, http.StatusBadRequest,
			"choose a password of at least 10 characters")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.Error("could not hash a chosen password", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "that could not be saved")
		return
	}

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "that could not be saved")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	// Claimed and read in one statement. A SELECT followed by an UPDATE would
	// let the same link be spent twice by two requests a millisecond apart.
	var userID, purpose string
	err = tx.QueryRow(r.Context(),
		`UPDATE registry.credential_grants SET redeemed_at = NOW()
		  WHERE token_hash = $1 AND redeemed_at IS NULL AND expires_at > NOW()
		RETURNING user_id::text, purpose`,
		hashRecoveryToken(req.Token)).Scan(&userID, &purpose)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusGone, "that link has already been used or has expired")
		return
	}
	if err != nil {
		slog.Error("could not claim a credential link", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "that could not be saved")
		return
	}

	if _, err := tx.Exec(r.Context(),
		`UPDATE registry.users SET password_hash = $2, failed_login_attempts = 0, locked_until = NULL
		  WHERE id = $1::uuid`, userID, hash); err != nil {
		slog.Error("could not set a chosen password", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "that could not be saved")
		return
	}
	// Every session the account had, ended. Somebody who has just been given a
	// new password is often somebody whose old one was known to a person it
	// should not have been.
	if _, err := tx.Exec(r.Context(),
		`UPDATE workspace.sessions SET revoked_at = NOW()
		  WHERE user_id = $1::uuid AND revoked_at IS NULL AND expires_at > NOW()`,
		userID); err != nil {
		slog.Error("could not end the sessions after a password change", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "that could not be saved")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "that could not be saved")
		return
	}

	audit.Record(r.Context(), "", userID, "auth.password_set", "user",
		map[string]any{"purpose": purpose})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// HandleImpersonationRedeem exchanges the console's handover for a session.
//
// This is where an operator becomes, for thirty minutes, somebody who works at
// the organisation. Everything about the session says so: it carries
// impersonated_by, which /me reports, which the shell turns into a banner
// nobody can dismiss, and which every audit row written from it is marked
// with.
func (h *Handlers) HandleImpersonationRedeem(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRecoveryBody)
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		httpx.Error(w, http.StatusBadRequest, "a handover token is required")
		return
	}

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "that could not be started")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	var impersonationID, tenantID, userID, operatorID, operatorEmail, reason string
	var endsAt time.Time
	err = tx.QueryRow(r.Context(),
		`UPDATE registry.operator_impersonations SET redeemed_at = NOW()
		  WHERE handover_hash = $1 AND redeemed_at IS NULL AND handover_expires_at > NOW()
		RETURNING id::text, tenant_id::text, user_id::text, operator_id::text,
		          operator_email, reason, ends_at`,
		hashRecoveryToken(req.Token)).
		Scan(&impersonationID, &tenantID, &userID, &operatorID, &operatorEmail, &reason, &endsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusGone, "that handover has already been used or has expired")
		return
	}
	if err != nil {
		slog.Error("could not claim an impersonation handover", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "that could not be started")
		return
	}

	// Checked here as well as in the console: the organisation may have been
	// suspended in the minute between the operator asking and following the
	// link, and a suspension that impersonation could step around would not be
	// a suspension.
	if suspended, _ := h.authn.TenantSuspended(r.Context(), tenantID); suspended {
		httpx.Error(w, http.StatusForbidden, auth.ErrTenantSuspended.Error())
		return
	}

	token, err := newImpersonationSession(r, tx, userID, tenantID, operatorID, endsAt)
	if err != nil {
		slog.Error("could not create an impersonation session", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "that could not be started")
		return
	}

	// The organisation's own trail again, now that it has actually happened.
	// The console wrote "requested" when the link was minted; this is the row
	// that says somebody walked through the door.
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO workspace.audit_events (tenant_id, user_id, action, resource, details)
		 VALUES ($1::uuid, $2, 'security.impersonation.started', $3, $4)`,
		tenantID, "operator:"+operatorID, userID,
		map[string]any{
			"operator_email": operatorEmail,
			"reason":         reason,
			"ends_at":        endsAt,
		}); err != nil {
		slog.Error("could not record the start of an impersonation", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "that could not be started")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "that could not be started")
		return
	}

	slog.Warn("an operator is acting inside an organisation",
		"operator_email", operatorEmail, "tenant_id", tenantID, "user_id", userID,
		"ends_at", endsAt, "reason", reason)

	auth.SetSessionCookie(w, token, endsAt)
	httpx.JSON(w, http.StatusOK, map[string]any{
		"status": "started", "ends_at": endsAt, "operator": operatorEmail,
	})
}

// newImpersonationSession writes the borrowed session.
//
// It does not go through auth.SessionStore.Create, and the difference is the
// whole point: this session expires when the impersonation does rather than
// after the usual twelve hours, and it carries the operator's id so that
// everything downstream can tell what it is.
func newImpersonationSession(r *http.Request, tx pgx.Tx, userID, tenantID, operatorID string, endsAt time.Time) (string, error) {
	token, err := auth.NewSessionToken()
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO workspace.sessions (token_hash, user_id, tenant_id, auth_method, user_agent, ip_address,
		                       expires_at, impersonated_by)
		 VALUES ($1, $2::uuid, $3::uuid, 'impersonation', $4, $5, $6, $7::uuid)`,
		auth.HashSessionToken(token), userID, tenantID, r.UserAgent(),
		security.ClientIP(r), endsAt, operatorID); err != nil {
		return "", err
	}
	return token, nil
}

// EndImpersonations closes the impersonation rows whose sessions have run out,
// so that the console and the organisation both show a visit as finished
// rather than open for ever.
func (h *Handlers) EndImpersonations(ctx context.Context) {
	sweepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if _, err := h.db.Exec(sweepCtx,
		`UPDATE registry.operator_impersonations SET ended_at = NOW()
		  WHERE ended_at IS NULL AND ends_at <= NOW()`); err != nil {
		slog.Warn("could not close the finished impersonations", "error", err)
	}
}
