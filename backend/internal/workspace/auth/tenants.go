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
	"fmt"
	"strings"

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
// Everybody gets a space of their own, and somebody who works somewhere opens
// at work.
//
// Those are two decisions and they were run together four times, which is why
// this took four attempts. "Where does the door open" and "does this person
// have a personal space" are not the same question, and answering the first
// kept silently answering the second:
//
//   - the organisation, or refuse — people with no organisation could not sign
//     in at all;
//   - a personal space, made only for people with no organisation — so an
//     employee had none, and the switcher only appears with more than one
//     workspace, which put the whole personal side out of reach of anybody
//     with a job;
//   - the personal space always, for everybody — which fixed that and cost a
//     click every morning to whoever signed in to do their work;
//   - nowhere at all for people with no organisation — which fixed the cost and
//     took the personal space away from everybody.
//
// Answering them separately makes all four complaints go away at once. The
// space is made whether or not it is where the session opens; where the session
// opens prefers work. Anybody who has both sees a switcher, because they have
// two workspaces, and that is what makes the personal side reachable from an
// account with a job.
//
// The cost is one row in registry.tenants per person, which is what migration
// 00092 was for: a personal space is seeded with one role rather than an
// organisation's three, and at a million people that is half the database it
// would otherwise be.
func (h *Handlers) FirstTenantFor(ctx context.Context, userID string) (string, error) {
	// Their own space first, and its error is not fatal to the sign-in. It is
	// where they end up only if they work nowhere; somebody with an
	// organisation to open should not be kept out of it because the workspace
	// they were not going to has failed to be made.
	home, homeErr := h.HomeFor(ctx, userID)

	tenantID, err := h.FirstOrganisationFor(ctx, userID)
	if err == nil {
		return tenantID, nil
	}
	if !errors.Is(err, ErrNoOrganisation) {
		return "", err
	}
	return home, homeErr
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

// HomeFor is this person's own workspace, made on first use.
//
// Lazy rather than created with the account, because most accounts are made by
// an administrator adding somebody to an organisation and a home nobody ever
// opens is a row, a profile, a set of roles and a quota bucket that exist to be
// counted. The first sign-in that needs one is the first evidence anybody does.
//
// The race is real and is settled by the database: two tabs signing in at once
// both find no home and both insert. The partial unique index makes the second
// insert fail, and the loser reads the winner's row — which is why the conflict
// is handled by re-reading rather than by a lock somebody has to remember.
func (h *Handlers) HomeFor(ctx context.Context, userID string) (string, error) {
	var tenantID string
	err := h.db.QueryRow(ctx,
		`SELECT id::text FROM registry.tenants WHERE owner_user_id = $1::uuid AND kind = 'personal'`,
		userID).Scan(&tenantID)
	if err == nil {
		// Found, and the membership is checked anyway.
		//
		// Owning a workspace and being a member of it are two rows, and
		// everything downstream reads the second: a home whose membership went
		// missing — removed by an administrator's sweep, or by a test — locks
		// its owner out of their own space with no way back, because nothing
		// else in the platform will ever make it again. Cheap to assert, and
		// the failure it prevents has no other repair.
		return tenantID, h.ensureHomeMembership(ctx, tenantID, userID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// The person's own name, so the switcher reads as a place rather than an
	// identifier. Empty is fine and stays empty: a name is not a precondition
	// for having somewhere to stand.
	var name string
	_ = tx.QueryRow(ctx, `SELECT name FROM registry.users WHERE id = $1::uuid`, userID).Scan(&name)
	if strings.TrimSpace(name) == "" {
		name = "Миний гэр"
	}

	// The slug is derived from the user id rather than from the name: names
	// collide, are edited, and are somebody's actual name in a URL.
	slug := "home-" + strings.ReplaceAll(userID, "-", "")
	err = tx.QueryRow(ctx,
		`INSERT INTO registry.tenants (slug, name, kind, owner_user_id)
		 VALUES ($1, $2, 'personal', $3::uuid)
		 ON CONFLICT DO NOTHING
		 RETURNING id::text`, slug, name, userID).Scan(&tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Somebody else got there first. Their row is as good as ours.
		if err = tx.QueryRow(ctx,
			`SELECT id::text FROM registry.tenants WHERE owner_user_id = $1::uuid AND kind = 'personal'`,
			userID).Scan(&tenantID); err != nil {
			return "", fmt.Errorf("read the home another sign-in just made: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("make this person a home: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return tenantID, h.ensureHomeMembership(ctx, tenantID, userID)
}

// ensureHomeMembership is the second row a home needs.
//
// The membership is what the rest of the platform reads; the owner column is
// only how the home is found. Both, or a person owns a workspace they are not
// a member of and every screen refuses them — including the sign-in that was
// trying to open it.
func (h *Handlers) ensureHomeMembership(ctx context.Context, tenantID, userID string) error {
	if _, err := h.db.Exec(ctx,
		`INSERT INTO workspace.memberships (tenant_id, user_id)
		 SELECT $1::uuid, $2::uuid
		  WHERE NOT EXISTS (SELECT 1 FROM workspace.memberships
		                     WHERE tenant_id = $1::uuid AND user_id = $2::uuid)`,
		tenantID, userID); err != nil {
		return fmt.Errorf("make this person a member of their own home: %w", err)
	}
	return nil
}
