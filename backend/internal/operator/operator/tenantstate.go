/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * What an organisation's row says about its life.
 *
 * Every screen that changes one reads this first, and hands it to the audit
 * trail as the "before" — so it is the console's shared vocabulary rather than
 * any one screen's, and it lives with the middleware that records the change.
 */

package operator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// TenantState is the lifecycle half of an organisation's row, used as the
// "before" of every change so the audit trail records what was undone.
type TenantState struct {
	ID                  string     `json:"id"`
	Slug                string     `json:"slug"`
	Name                string     `json:"name"`
	SuspendedAt         *time.Time `json:"suspended_at"`
	SuspensionReason    string     `json:"suspension_reason"`
	DeletionScheduledAt *time.Time `json:"deletion_scheduled_at"`
}

func (c *Console) StateOf(ctx context.Context, tenantID string) (TenantState, error) {
	var state TenantState
	err := c.db.QueryRow(Scoped(ctx),
		`SELECT id::text, slug, name, suspended_at, suspension_reason, deletion_scheduled_at
		   FROM registry.tenants WHERE id = $1::uuid`, tenantID).
		Scan(&state.ID, &state.Slug, &state.Name, &state.SuspendedAt,
			&state.SuspensionReason, &state.DeletionScheduledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TenantState{}, ErrTenantNotFound
	}
	if err != nil {
		if IsInvalidUUID(err) {
			return TenantState{}, ErrTenantNotFound
		}
		return TenantState{}, fmt.Errorf("control plane: read the organisation's state: %w", err)
	}
	return state, nil
}

// ErrTenantNotFound is what a detail page gets for an id that is not there.
var ErrTenantNotFound = errors.New("no such organisation")
