/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Durable state for the shared signing usecase.
 */

package eidmongolia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/async"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// stateTTL is how long a ceremony's state is kept.
//
// It bounds abandonment, not the citizen: the row holds the PDF being signed,
// so rows nobody returns to are the only unbounded growth here. eID's own
// EXPIRED state is what ends a real wait — an earlier version of the
// authentication flow invented a two-minute deadline and cut people off who
// were still reaching for their phone.
const stateTTL = 30 * time.Minute

// stateStore satisfies the cache interface the signing package
// (internal/kernel/eidsign) expects. That interface was written for Redis when
// it lived in a shared library, but the contract is only
// Set/Get of a string, and Postgres both satisfies it and survives a restart
// mid-ceremony — which matters, because losing the state loses the PDF the
// citizen has already approved on their phone.
type stateStore struct{ db *pgxpool.Pool }

// Set stores a value. The usecase hands over a struct, so anything that is not
// already a string is marshalled — matching what a JSON round-trip through
// Redis would have produced.
func (s *stateStore) Set(ctx context.Context, key string, value any) error {
	encoded, err := encodeState(value)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx,
		`INSERT INTO registry.eid_sign_state (key, value, expires_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (key) DO UPDATE
		    SET value = EXCLUDED.value, updated_at = NOW(), expires_at = EXCLUDED.expires_at`,
		key, encoded, time.Now().Add(stateTTL))
	if err != nil {
		return fmt.Errorf("eidmongolia: storing signing state: %w", err)
	}
	return nil
}

// Get reads a value back. A missing key and an expired key are the same
// answer: the ceremony is gone.
func (s *stateStore) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRow(ctx,
		`SELECT value FROM registry.eid_sign_state WHERE key = $1 AND expires_at > NOW()`, key).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("eidmongolia: signing state %q not found", key)
	}
	if err != nil {
		return "", fmt.Errorf("eidmongolia: reading signing state: %w", err)
	}
	return value, nil
}

func encodeState(value any) (string, error) {
	if s, ok := value.(string); ok {
		return s, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("eidmongolia: encoding signing state: %w", err)
	}
	return string(raw), nil
}

// sweepState deletes abandoned ceremonies.
func (s *Service) sweepState(ctx context.Context) (int64, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM registry.eid_sign_state WHERE expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// StartHousekeeping removes abandoned ceremony state until ctx is cancelled.
func (s *Service) StartHousekeeping(ctx context.Context) {
	async.Go("eid-state-housekeeping", func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Bounded so a slow database cannot pile sweeps on top of one
				// another once the interval comes round again.
				sweepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				removed, err := s.sweepState(sweepCtx)
				cancel()
				if err != nil {
					slog.Warn("eidmongolia: could not sweep abandoned signing state", "error", err)
					continue
				}
				if removed > 0 {
					slog.Info("eidmongolia: swept abandoned signing state", "count", removed)
				}
			}
		}
	})
}
