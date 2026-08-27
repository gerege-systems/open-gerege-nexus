/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Which organisation an account signs into when it does not say.
 *
 * Asked by three paths — a password sign-in, an eID one and a federated one —
 * which is why it is here rather than beside any of them.
 */

package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrNoOrganisation means the account is real and the identity is theirs, but
// they belong to no organisation, so there is nothing to sign in to.
//
// Deliberately not a SignInError. That type is what the Google callback reads
// as "nobody here recognises this account", and it answers by parking the
// identity and asking for eID. Somebody whose provider account is already
// linked must never be sent down that road again — it is the loop this whole
// change exists to close, and it would be indistinguishable from the bug.
var ErrNoOrganisation = errors.New("this account does not belong to any organisation")

// FirstTenantFor is the organisation a session for this person opens in.
//
// Separate from the identity lookup on purpose: which organisation somebody
// works in has no bearing on whether the provider account is theirs, and
// letting it decide is what made a linked identity vanish.
func (h *Handlers) FirstTenantFor(ctx context.Context, userID string) (string, error) {
	var tenantID string
	err := h.db.QueryRow(ctx,
		`SELECT tenant_id::text FROM workspace.memberships WHERE user_id = $1
		  ORDER BY created_at, tenant_id LIMIT 1`, userID).Scan(&tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNoOrganisation
	}
	return tenantID, err
}
