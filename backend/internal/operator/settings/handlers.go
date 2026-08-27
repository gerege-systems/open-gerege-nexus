/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package settings is what the console can change about the platform without a
// deployment, and the history of every change.
//
// It stands on internal/operator/operator, which decides who is asking and
// records what they did; nothing in this plane stands on this package.
package settings

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// ListSettings returns every registered setting with its current value.
func (s *Service) ListSettings(ctx context.Context) ([]settings.Value, error) {
	if s.settings == nil {
		return nil, ErrNoSettingsStore
	}
	return s.settings.List(operator.Scoped(ctx))
}

// SetSetting writes a value.
//
// The audit row carries both values, so the trail answers "what was it before"
// without anybody having to open the history — the two questions arrive
// together in an incident.
func (s *Service) SetSetting(ctx context.Context, sess operator.Session, key, value, reason string) error {
	if s.settings == nil {
		return ErrNoSettingsStore
	}
	spec, known := settings.Lookup(key)
	if !known {
		return fmt.Errorf("%w: %s", settings.ErrUnknownSetting, key)
	}

	before := settings.Get(key)
	err := s.op.Do(ctx, sess, operator.Change{
		Action:     "settings.set",
		TargetType: "setting",
		TargetID:   key,
		Reason:     reason,
		Before:     map[string]any{"value": before},
		After:      map[string]any{"value": value, "kind": string(spec.Kind)},
	}, func(ctx context.Context, tx pgx.Tx) error {
		return s.settings.Set(ctx, tx, key, value, sess.ID, reason)
	})
	if err != nil {
		return err
	}
	// After the commit: the caches — this replica's and every other one's —
	// must not be told about a value that then failed to land.
	s.settings.Changed(ctx)
	return nil
}

// SettingHistory returns what a setting has been. An empty key returns every
// setting's changes, which is the screen an operator wants after an incident:
// "what did we change this afternoon".
func (s *Service) SettingHistory(ctx context.Context, key string) ([]settings.Change, error) {
	if s.settings == nil {
		return nil, ErrNoSettingsStore
	}
	return s.settings.History(operator.Scoped(ctx), key)
}

// RollbackSetting puts a setting back to what a named change moved it from.
//
// A rollback is itself a change: it writes a new history row rather than
// removing the one it undoes. A history that could be rewound would be a
// history somebody could edit, and the value of this table is that it cannot.
func (s *Service) RollbackSetting(ctx context.Context, sess operator.Session, changeID, reason string) error {
	if s.settings == nil {
		return ErrNoSettingsStore
	}

	var key string
	var previous *string
	err := s.db.QueryRow(operator.Scoped(ctx),
		`SELECT key, previous_value FROM operator.platform_settings_history WHERE id = $1::uuid`,
		changeID).Scan(&key, &previous)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrHistoryNotFound
	}
	if err != nil {
		if operator.IsInvalidUUID(err) {
			return ErrHistoryNotFound
		}
		return fmt.Errorf("control plane: read the change: %w", err)
	}

	// A change whose previous value was NULL is the first time the setting was
	// written at all, so undoing it means going back to the environment or the
	// default — which is what the spec's own default is.
	target := ""
	if previous != nil {
		target = *previous
	} else if spec, known := settings.Lookup(key); known {
		target = spec.Default
	}

	return s.SetSetting(ctx, sess, key, target,
		reason+" (буцаалт: "+changeID+")")
}

var (
	// ErrNoSettingsStore is a deployment whose console was built without one.
	// It cannot happen in the server, and it is checked so a test that builds a
	// bare Service gets a sentence rather than a nil dereference.
	ErrNoSettingsStore = errors.New("this console has no settings store")
)

var (
	// ErrHistoryNotFound is a rollback naming a change that is not there.
	ErrHistoryNotFound = errors.New("no such change")
)

func (s *Service) handleListSettings(w http.ResponseWriter, r *http.Request) {
	values, err := s.ListSettings(r.Context())
	if err != nil {
		fail(w, err, "could not read the settings")
		return
	}
	// The warnings ride along with the values, because they are about the
	// values: a configuration that contradicts itself belongs on the screen
	// where somebody can fix it.
	httpx.JSON(w, http.StatusOK, map[string]any{
		"settings": values,
		"warnings": s.warnings(),
	})
}

func (s *Service) handleSettingHistory(w http.ResponseWriter, r *http.Request) {
	changes, err := s.SettingHistory(r.Context(), r.URL.Query().Get("key"))
	if err != nil {
		fail(w, err, "could not read the history")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"changes": changes})
}

func (s *Service) handleSetSetting(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		operator.Reasoned
		Value string `json:"value"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.SetSetting(r.Context(), sess, chi.URLParam(r, "key"), body.Value, body.Reason); err != nil {
		fail(w, err, "could not change the setting")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (s *Service) handleRollbackSetting(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body operator.Reasoned
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.RollbackSetting(r.Context(), sess, chi.URLParam(r, "id"), body.Reason); err != nil {
		fail(w, err, "could not roll the setting back")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "rolled back"})
}

// Routes are this screen's, mounted on the console's signed-in group. The
// capability each one asks for is the decision about who may see or do it.
func (s *Service) Routes(r chi.Router) {

	// How the platform behaves. Reading is part of the tenant-read
	// capability because "what is this deployment configured to do" is
	// context for every other screen; writing is its own.
	r.With(s.op.RequireCapability(operator.CapTenantRead)).Get("/settings", s.handleListSettings)
	r.With(s.op.RequireCapability(operator.CapTenantRead)).Get("/settings/history", s.handleSettingHistory)
	// Step-up on the write: the access mode is here, and switching a
	// platform to public is the single most consequential field in the
	// console.
	r.With(s.op.RequireCapability(operator.CapSettingsWrite), s.op.RequireStepUp).
		Put("/settings/{key}", s.handleSetSetting)
	r.With(s.op.RequireCapability(operator.CapSettingsWrite), s.op.RequireStepUp).
		Post("/settings/rollback/{id}", s.handleRollbackSetting)

	// The keys, beside the settings they are not. Reading is the same
	// capability as reading a setting because nothing readable here is secret;
	// writing asks for the second factor again, because setting a credential
	// is pointing this deployment at a system it will then trust.
	r.With(s.op.RequireCapability(operator.CapTenantRead)).Get("/credentials", s.handleListCredentials)
	r.With(s.op.RequireCapability(operator.CapSettingsWrite), s.op.RequireStepUp).
		Put("/credentials/{name}", s.handleSetCredential)
	r.With(s.op.RequireCapability(operator.CapSettingsWrite), s.op.RequireStepUp).
		Delete("/credentials/{name}", s.handleClearCredential)
}

// warnings is what the configuration screen shows above the fields: the
// platform's own complaints, plus the feature flags that have outlived the date
// somebody gave them.
//
// Flag debt is only ever paid when somebody is reminded, and the moment they
// are looking at the configuration is the moment to remind them.
func (s *Service) warnings() []string {
	warnings := make([]string, 0, 2)
	if s.warningsFrom != nil {
		warnings = append(warnings, s.warningsFrom()...)
	}
	if s.flags != nil {
		if _, expired := s.flags.Snapshot(time.Now()); len(expired) > 0 {
			warnings = append(warnings,
				"Хугацаа нь дууссан feature flag: "+strings.Join(expired, ", ")+
					". Кодоос нь салгаад flag-ийг устгах цаг болсон.")
		}
	}
	return warnings
}
