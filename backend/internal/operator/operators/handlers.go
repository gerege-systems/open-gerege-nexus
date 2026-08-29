/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package operators is who may reach this console.
//
// Until now the only way to a second operator was cmd/operator-bootstrap, run
// on the production host as the database owner. That is the right control for
// the first account and the wrong one for the fourth: it puts a shell on the
// server between a platform and every person who joins it, and shells given
// out for that reason are rarely taken back.
//
// So the console mints them now, and it does it through its own database role
// — migration 00097's grant is the control, one line to revoke and visible in
// `\dp`. Three more sit above it: the capability table (operator.write,
// superadmin only), a second factor on the request, and an audit row written
// in the same transaction as the account.
//
// Two things are deliberately not here. Nothing deletes an operator: one who
// should not be here is disabled, and the audit trail keeps pointing at a row
// that still exists. And nothing sets somebody else's password — the console
// generates one, shows it once, and the person changes it themselves.
package operators

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Created is what the screen shows once and never again: the account, the
// secret its authenticator needs, and the password the person signs in with
// the first time.
type Created struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	// Secret and URI enrol the authenticator. The URI is what a QR code is
	// drawn from; the secret is for typing it in by hand.
	Secret string `json:"secret"`
	URI    string `json:"uri"`
	// Password is generated here rather than chosen by the person creating the
	// account: a password somebody else picks is a password they know, and the
	// first thing this one should be used for is changing it.
	Password string `json:"password"`
}

// NewOperator is the request.
type NewOperator struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// Create writes the account, its unconfirmed second factor, and the audit row,
// in one transaction.
//
// The account cannot be used until somebody proves the authenticator works —
// see Confirm. An enrolment that is started and never finished leaves an
// account nobody can sign in to, which is the safe half of that failure.
func (s *Service) Create(ctx context.Context, sess operator.Session, params NewOperator, reason string) (Created, error) {
	email := strings.ToLower(strings.TrimSpace(params.Email))
	name := strings.TrimSpace(params.Name)
	role := operator.Role(strings.TrimSpace(params.Role))
	switch {
	case email == "" || !strings.Contains(email, "@"):
		return Created{}, errors.New("an e-mail address is required")
	case name == "":
		return Created{}, errors.New("a name is required")
	case !role.Valid():
		return Created{}, fmt.Errorf("%q is not one of the four operator roles", params.Role)
	}

	password, err := generatedPassword()
	if err != nil {
		return Created{}, err
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return Created{}, fmt.Errorf("hash the password: %w", err)
	}
	secret, uri, err := operator.NewTOTPSecret(email)
	if err != nil {
		return Created{}, err
	}

	created := Created{Email: email, Name: name, Role: string(role), Secret: secret, URI: uri, Password: password}
	err = s.op.Do(ctx, sess, operator.Change{
		Action:     "operator.create",
		TargetType: "operator",
		TargetID:   email,
		Reason:     reason,
		After:      map[string]any{"email": email, "name": name, "role": string(role)},
	}, func(ctx context.Context, tx pgx.Tx) error {
		var taken bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM operator.operator_accounts WHERE lower(email) = $1)`, email).
			Scan(&taken); err != nil {
			return err
		}
		if taken {
			return operator.ErrOperatorExists
		}
		return tx.QueryRow(ctx,
			`INSERT INTO operator.operator_accounts (email, name, role, password_hash, totp_secret)
			 VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
			email, name, string(role), hash, secret).Scan(&created.ID)
	})
	if err != nil {
		return Created{}, err
	}
	return created, nil
}

// Confirm completes an enrolment with a code the new operator's authenticator
// has just produced.
//
// Asking for the code rather than trusting that the QR was scanned is the
// whole value of the step: it proves the secret arrived intact, on a device
// somebody is holding, before the account can be used. A mistyped enrolment is
// found here rather than the first time that person needs to sign in urgently.
func (s *Service) Confirm(ctx context.Context, sess operator.Session, id, code, reason string) error {
	return s.op.Do(ctx, sess, operator.Change{
		Action:     "operator.enrolment.confirm",
		TargetType: "operator",
		TargetID:   id,
		Reason:     reason,
	}, func(ctx context.Context, tx pgx.Tx) error {
		var secret string
		var confirmed bool
		err := tx.QueryRow(ctx,
			`SELECT totp_secret, totp_confirmed_at IS NOT NULL
			   FROM operator.operator_accounts WHERE id = $1::uuid`, id).Scan(&secret, &confirmed)
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("no such operator")
		}
		if err != nil {
			return err
		}
		if confirmed {
			return errors.New("this operator's authenticator is already confirmed")
		}
		step, ok := operator.VerifyTOTP(secret, code)
		if !ok {
			return errors.New("that code was not right")
		}
		_, err = tx.Exec(ctx,
			`UPDATE operator.operator_accounts
			    SET totp_confirmed_at = NOW(), totp_last_step = $2, updated_at = NOW()
			  WHERE id = $1::uuid`, id, step)
		return err
	})
}

// SetEnabled disables an operator, or lets one back in.
//
// Disabling is how somebody leaves: the row stays, so every audit entry it is
// named in still resolves to an account.
func (s *Service) SetEnabled(ctx context.Context, sess operator.Session, id string, enabled bool, reason string) error {
	if !enabled && id == sess.ID {
		// Not paternalism: the session doing this would keep working until it
		// expired, so the operator would appear to have succeeded and would
		// find out at the worst moment.
		return errors.New("an operator cannot disable their own account")
	}
	action := "operator.disable"
	if enabled {
		action = "operator.enable"
	}
	return s.op.Do(ctx, sess, operator.Change{
		Action:     action,
		TargetType: "operator",
		TargetID:   id,
		Reason:     reason,
		After:      map[string]any{"enabled": enabled},
	}, func(ctx context.Context, tx pgx.Tx) error {
		if !enabled {
			if err := lastSuperadminGuard(ctx, tx, id); err != nil {
				return err
			}
		}
		tag, err := tx.Exec(ctx,
			`UPDATE operator.operator_accounts
			    SET disabled_at = CASE WHEN $2 THEN NULL ELSE COALESCE(disabled_at, NOW()) END,
			        updated_at = NOW()
			  WHERE id = $1::uuid`, id, enabled)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errors.New("no such operator")
		}
		return nil
	})
}

// SetRole changes what an operator may do.
func (s *Service) SetRole(ctx context.Context, sess operator.Session, id string, role operator.Role, reason string) error {
	if !role.Valid() {
		return fmt.Errorf("%q is not one of the four operator roles", role)
	}
	if id == sess.ID {
		// The same reasoning as disabling: a superadmin who demotes themselves
		// keeps the session they did it with and loses the console at its
		// expiry, with nobody left holding the capability to undo it.
		return errors.New("an operator cannot change their own role")
	}
	return s.op.Do(ctx, sess, operator.Change{
		Action:     "operator.role",
		TargetType: "operator",
		TargetID:   id,
		Reason:     reason,
		After:      map[string]any{"role": string(role)},
	}, func(ctx context.Context, tx pgx.Tx) error {
		if role != operator.RoleSuperadmin {
			if err := lastSuperadminGuard(ctx, tx, id); err != nil {
				return err
			}
		}
		tag, err := tx.Exec(ctx,
			`UPDATE operator.operator_accounts SET role = $2, updated_at = NOW()
			  WHERE id = $1::uuid`, id, string(role))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errors.New("no such operator")
		}
		return nil
	})
}

// ChangePassword changes the caller's own, and only the caller's own.
//
// The current password is asked for because a console left open is otherwise a
// console somebody else can lock the owner out of. Anybody else's password is
// not changeable here at all: an operator who has lost theirs gets a new
// account or a bootstrap from the server, both of which leave a record.
func (s *Service) ChangePassword(ctx context.Context, sess operator.Session, current, next string) error {
	if len(next) < operator.MinPasswordLength {
		return fmt.Errorf("the password must be at least %d characters", operator.MinPasswordLength)
	}
	if current == next {
		return errors.New("the new password is the old one")
	}

	scoped := operator.Scoped(ctx)
	var hash string
	if err := s.db.QueryRow(scoped,
		`SELECT password_hash FROM operator.operator_accounts WHERE id = $1::uuid`, sess.ID).Scan(&hash); err != nil {
		return fmt.Errorf("read the account: %w", err)
	}
	if !security.CheckPasswordHash(current, hash) {
		return errors.New("that is not the current password")
	}

	fresh, err := security.HashPassword(next)
	if err != nil {
		return fmt.Errorf("hash the password: %w", err)
	}
	return s.op.Do(ctx, sess, operator.Change{
		Action:     "operator.password",
		TargetType: "operator",
		TargetID:   sess.ID,
		Reason:     "the operator changed their own password",
	}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`UPDATE operator.operator_accounts SET password_hash = $2, updated_at = NOW()
			  WHERE id = $1::uuid`, sess.ID, fresh)
		return err
	})
}

// lastSuperadminGuard refuses a change that would leave the platform with no
// superadmin who can actually sign in.
//
// Counted as "enabled and enrolled", because an account whose authenticator
// was never confirmed cannot sign in either — a deployment whose only other
// superadmin is a half-finished enrolment is locked out just as thoroughly,
// and that is exactly the state a console that mints accounts creates.
func lastSuperadminGuard(ctx context.Context, tx pgx.Tx, id string) error {
	var remaining int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM operator.operator_accounts
		  WHERE role = 'superadmin' AND disabled_at IS NULL
		    AND totp_confirmed_at IS NOT NULL AND id <> $1::uuid`, id).Scan(&remaining); err != nil {
		return err
	}
	if remaining == 0 {
		return errors.New("this is the last superadmin who can sign in; add another one first")
	}
	return nil
}

// generatedPassword is what the new operator signs in with once.
func generatedPassword() (string, error) {
	raw := make([]byte, 18)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate a password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		NewOperator
		Reason string `json:"reason"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	created, err := s.Create(r.Context(), sess, body.NewOperator, body.Reason)
	if err != nil {
		fail(w, err, "could not add the operator")
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

func (s *Service) handleConfirm(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.Confirm(r.Context(), sess, chi.URLParam(r, "id"), body.Code, body.Reason); err != nil {
		fail(w, err, "could not confirm the authenticator")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "confirmed"})
}

func (s *Service) handleSetEnabled(enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, _ := operator.SessionFrom(r.Context())
		var body operator.Reasoned
		if !operator.Decode(w, r, &body) {
			return
		}
		if err := s.SetEnabled(r.Context(), sess, chi.URLParam(r, "id"), enabled, body.Reason); err != nil {
			fail(w, err, "could not change the operator")
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "saved"})
	}
}

func (s *Service) handleSetRole(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		Role   string `json:"role"`
		Reason string `json:"reason"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.SetRole(r.Context(), sess, chi.URLParam(r, "id"), operator.Role(body.Role), body.Reason); err != nil {
		fail(w, err, "could not change the operator's role")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (s *Service) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		Current string `json:"current"`
		Next    string `json:"next"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.ChangePassword(r.Context(), sess, body.Current, body.Next); err != nil {
		fail(w, err, "could not change the password")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "changed"})
}

// Routes are this screen's. Every write asks for operator.write — superadmin
// only — and the second factor, because this is the one action that widens who
// may perform every other action. Changing one's own password is the exception
// to both: it is the account's own, and asking for the current one is the
// check that matters.
func (s *Service) Routes(r chi.Router) {
	r.With(s.op.RequireCapability(operator.CapOperatorWrite), s.op.RequireStepUp).
		Post("/operators", s.handleCreate)
	r.With(s.op.RequireCapability(operator.CapOperatorWrite), s.op.RequireStepUp).
		Post("/operators/{id}/enrolment", s.handleConfirm)
	r.With(s.op.RequireCapability(operator.CapOperatorWrite), s.op.RequireStepUp).
		Post("/operators/{id}/disable", s.handleSetEnabled(false))
	r.With(s.op.RequireCapability(operator.CapOperatorWrite), s.op.RequireStepUp).
		Post("/operators/{id}/enable", s.handleSetEnabled(true))
	r.With(s.op.RequireCapability(operator.CapOperatorWrite), s.op.RequireStepUp).
		Post("/operators/{id}/role", s.handleSetRole)
	r.Post("/me/password", s.handleChangePassword)
}
