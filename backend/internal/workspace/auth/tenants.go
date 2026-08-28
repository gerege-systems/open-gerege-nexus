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
// they belong to no organisation.
//
// It no longer stops a sign-in. Since migration 00085 everybody has somewhere
// to sign in to — their own home workspace — so this says "not a member of any
// organisation", which is a thing a screen may want to know and not a refusal.
// It is returned by FirstOrganisationFor and by nothing on the sign-in path.
var ErrNoOrganisation = errors.New("this account does not belong to any organisation")

// FirstTenantFor is the workspace a session for this person opens in.
//
// Their oldest organisation, or none at all.
//
// "None at all" is a real answer since 00094 and not a failure: a session may
// carry no workspace, and somebody who belongs to no organisation signs in to
// the platform as themselves. What they can reach is their own record — the
// rows keyed on them by 00093 — and nothing that belongs to a workspace, which
// is the correct set and is enforced by the database rather than by a screen.
//
// This has now been three things in three days, and the history is the argument
// for where it landed. It was the oldest organisation, and people with no
// organisation could not sign in at all. Then it was a personal workspace
// always, which fixed that and made a second, worse problem: a row in
// registry.tenants for every human being who ever authenticated, on a table
// that is the customer list and is the parent of thirty-nine others. Measured
// at a million people that was 3.9 GB, most of it access-control rows for
// workspaces with one member who owned them.
//
// The mistake both times was answering "which workspace" when the honest answer
// is that a person is not a workspace. A citizen reading what a ministry told
// them is not acting for an organisation, and inventing one to hold them was a
// way of avoiding saying so in the schema. Now the schema says it.
func (h *Handlers) FirstTenantFor(ctx context.Context, userID string) (string, error) {
	tenantID, err := h.FirstOrganisationFor(ctx, userID)
	if errors.Is(err, ErrNoOrganisation) {
		return "", nil
	}
	return tenantID, err
}

// FirstOrganisationFor is the oldest organisation this person belongs to.
//
// Homes are excluded by kind rather than by owner: a home is not an
// organisation whichever way it was made, and a query that filtered on
// owner_user_id would start returning them the day anything else sets it.
func (h *Handlers) FirstOrganisationFor(ctx context.Context, userID string) (string, error) {
	var tenantID string
	err := h.db.QueryRow(ctx,
		`SELECT m.tenant_id::text
		   FROM workspace.memberships m
		   JOIN registry.tenants t ON t.id = m.tenant_id
		  WHERE m.user_id = $1 AND t.kind = 'organisation'
		  ORDER BY m.created_at, m.tenant_id LIMIT 1`, userID).Scan(&tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNoOrganisation
	}
	return tenantID, err
}
