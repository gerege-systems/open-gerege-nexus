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
// An organisation if they are in one, and their own home if they are not.
// Nobody is turned away: the schema has held people at the deployment's level
// since users became global, and this is the sign-in path finally agreeing
// with it. What used to happen instead was EID_JIT_TENANT_SLUG — a citizen
// signing in with eID was made a *member* of whichever organisation an
// environment variable named, counted against its quota and listed in its
// directory, because somebody had to be somewhere.
//
// Organisations come first when there are both. Somebody who works at a
// company and also has a home opens at work; the home is a row in the switcher
// rather than a door they have to walk back out of every morning.
//
// Separate from the identity lookup on purpose: which workspace somebody works
// in has no bearing on whether the provider account is theirs, and letting it
// decide is what made a linked identity vanish.
func (h *Handlers) FirstTenantFor(ctx context.Context, userID string) (string, error) {
	tenantID, err := h.FirstOrganisationFor(ctx, userID)
	if err == nil {
		return tenantID, nil
	}
	if !errors.Is(err, ErrNoOrganisation) {
		return "", err
	}
	return h.HomeFor(ctx, userID)
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
		return tenantID, nil
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

	// The membership is what the rest of the platform reads; the owner column
	// is only how the home is found. Both, or a person owns a workspace they
	// are not a member of and every screen refuses them.
	if _, err = tx.Exec(ctx,
		`INSERT INTO workspace.memberships (tenant_id, user_id)
		 SELECT $1::uuid, $2::uuid
		  WHERE NOT EXISTS (SELECT 1 FROM workspace.memberships
		                     WHERE tenant_id = $1::uuid AND user_id = $2::uuid)`,
		tenantID, userID); err != nil {
		return "", fmt.Errorf("make this person a member of their own home: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return tenantID, nil
}
