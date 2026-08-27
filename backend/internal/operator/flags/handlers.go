/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package flags is a feature turned on for everybody, for one organisation, or
// for nobody.
//
// It stands on internal/operator/operator, which decides who is asking and
// records what they did; nothing in this plane stands on this package.
package flags

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/flags"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// ListFlags returns every feature flag.
func (s *Service) ListFlags(ctx context.Context) ([]flags.Flag, error) {
	if s.flags == nil {
		return nil, ErrNoFlagStore
	}
	return s.flags.List(operator.Scoped(ctx))
}

// FlagInput is a flag as the console writes it.
type FlagInput struct {
	Key         string     `json:"key"`
	Description string     `json:"description"`
	Owner       string     `json:"owner"`
	Kind        string     `json:"kind"`
	Enabled     bool       `json:"enabled"`
	Rollout     int        `json:"rollout"`
	ExpiresAt   *time.Time `json:"expires_at"`
	Reason      string     `json:"reason"`
}

// SaveFlag creates or updates a flag.
func (s *Service) SaveFlag(ctx context.Context, sess operator.Session, input FlagInput) error {
	if s.flags == nil {
		return ErrNoFlagStore
	}
	if input.Key == "" {
		return errors.New("a flag needs a key")
	}
	switch input.Kind {
	case flags.KindRelease, flags.KindKillSwitch, flags.KindExperiment:
	case "":
		input.Kind = flags.KindRelease
	default:
		return fmt.Errorf("%q is not a kind of flag", input.Kind)
	}
	if input.Rollout < 0 || input.Rollout > 100 {
		return errors.New("a rollout is 0 to 100")
	}

	err := s.op.Do(ctx, sess, operator.Change{
		Action:     "flag.save",
		TargetType: "flag",
		TargetID:   input.Key,
		Reason:     input.Reason,
		After: map[string]any{
			"enabled": input.Enabled, "rollout": input.Rollout, "kind": input.Kind,
		},
	}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO platform.feature_flags (key, description, owner, kind, enabled, rollout, expires_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
			 ON CONFLICT (key) DO UPDATE
			    SET description = EXCLUDED.description, owner = EXCLUDED.owner,
			        kind = EXCLUDED.kind, enabled = EXCLUDED.enabled,
			        rollout = EXCLUDED.rollout, expires_at = EXCLUDED.expires_at,
			        updated_at = NOW()`,
			input.Key, input.Description, input.Owner, input.Kind,
			input.Enabled, input.Rollout, input.ExpiresAt)
		return err
	})
	if err != nil {
		return err
	}
	s.flags.Changed(ctx)
	return nil
}

// DeleteFlag removes a flag.
//
// The console can do this, unlike almost everything else, because a flag that
// cannot be removed is flag debt by construction: the expiry warning would
// name flags nobody could act on.
func (s *Service) DeleteFlag(ctx context.Context, sess operator.Session, key, reason string) error {
	if s.flags == nil {
		return ErrNoFlagStore
	}
	err := s.op.Do(ctx, sess, operator.Change{
		Action:     "flag.delete",
		TargetType: "flag",
		TargetID:   key,
		Reason:     reason,
	}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM platform.feature_flags WHERE key = $1`, key)
		return err
	})
	if err != nil {
		return err
	}
	s.flags.Changed(ctx)
	return nil
}

// SetFlagOverride decides a flag for one organisation, or removes the decision.
func (s *Service) SetFlagOverride(ctx context.Context, sess operator.Session, key, tenantID string, enabled *bool, reason string) error {
	if s.flags == nil {
		return ErrNoFlagStore
	}
	err := s.op.Do(ctx, sess, operator.Change{
		Action:     "flag.override",
		TargetType: "flag",
		TargetID:   key,
		Reason:     reason,
		After:      map[string]any{"tenant_id": tenantID, "enabled": enabled},
	}, func(ctx context.Context, tx pgx.Tx) error {
		if enabled == nil {
			_, err := tx.Exec(ctx,
				`DELETE FROM platform.feature_flag_overrides WHERE flag_key = $1 AND tenant_id = $2::uuid`,
				key, tenantID)
			return err
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO platform.feature_flag_overrides (flag_key, tenant_id, enabled, updated_at)
			 VALUES ($1, $2::uuid, $3, NOW())
			 ON CONFLICT (flag_key, tenant_id) DO UPDATE
			    SET enabled = EXCLUDED.enabled, updated_at = NOW()`,
			key, tenantID, *enabled)
		return err
	})
	if err != nil {
		return err
	}
	s.flags.Changed(ctx)
	return nil
}

var (
	// ErrNoFlagStore is the same for flags.
	ErrNoFlagStore = errors.New("this console has no feature flag store")
)

func (s *Service) handleListFlags(w http.ResponseWriter, r *http.Request) {
	list, err := s.ListFlags(r.Context())
	if err != nil {
		fail(w, err, "could not read the flags")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"flags": list})
}

func (s *Service) handleSaveFlag(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var input FlagInput
	if !operator.Decode(w, r, &input) {
		return
	}
	if err := s.SaveFlag(r.Context(), sess, input); err != nil {
		fail(w, err, "could not save the flag")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (s *Service) handleDeleteFlag(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body operator.Reasoned
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.DeleteFlag(r.Context(), sess, chi.URLParam(r, "key"), body.Reason); err != nil {
		fail(w, err, "could not delete the flag")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Service) handleFlagOverride(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		operator.Reasoned
		TenantID string `json:"tenant_id"`
		// A pointer: null means "remove the override and go back to the
		// rollout", which is a different instruction from "off".
		Enabled *bool `json:"enabled"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.SetFlagOverride(r.Context(), sess, chi.URLParam(r, "key"),
		body.TenantID, body.Enabled, body.Reason); err != nil {
		fail(w, err, "could not set the override")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// Routes are this screen's, mounted on the console's signed-in group. The
// capability each one asks for is the decision about who may see or do it.
func (s *Service) Routes(r chi.Router) {

	r.With(s.op.RequireCapability(operator.CapTenantRead)).Get("/flags", s.handleListFlags)
	r.With(s.op.RequireCapability(operator.CapFlagsWrite)).Post("/flags", s.handleSaveFlag)
	r.With(s.op.RequireCapability(operator.CapFlagsWrite)).Delete("/flags/{key}", s.handleDeleteFlag)
	r.With(s.op.RequireCapability(operator.CapFlagsWrite)).
		Put("/flags/{key}/override", s.handleFlagOverride)
}
