/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package people is everybody with an account on this deployment.
//
// The help desk searches: type three characters, act on one person. That is
// the right shape for "somebody rang up" and the wrong one for every question
// about the population — how many accounts are there, how many can actually
// sign in, who is in no organisation at all, who has been linked to a provider
// that is going away. Those are asked of the whole list or not at all.
//
// It is read-only. Everything that changes a person — unlocking them, ending
// their sessions, sending them a way back in — stays in support, where it is
// one action about one person with a reason attached.
package people

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/jackc/pgx/v5"
)

// Person is one row of the roster.
type Person struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	// Verified is whether eID has ever vouched for this account.
	Verified bool `json:"verified"`
	// Providers is how many federated providers it is linked to.
	Providers int `json:"providers"`
	// Organisations counts memberships, personal home included: a person with
	// none has an account and nowhere to use it.
	Organisations int `json:"organisations"`
	Sessions      int `json:"sessions"`
	// LastSeenAt is the newest session this account has, which is as close to
	// "when were they last here" as this schema holds: registry.users records
	// failures and lockouts, not successes.
	LastSeenAt  *time.Time `json:"last_seen_at"`
	LockedUntil *time.Time `json:"locked_until"`
	// Active is the account's own switch; a disabled one cannot sign in
	// however many ways in it has.
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// Roster is the population, and the four numbers about it worth having above
// the list.
type Roster struct {
	People []Person `json:"people"`
	Total  int      `json:"total"`
	Counts struct {
		Verified int `json:"verified"`
		Locked   int `json:"locked"`
		Homeless int `json:"homeless"`
		SignedIn int `json:"signed_in"`
	} `json:"counts"`
}

const rosterPageSize = 100

// List reads the roster.
//
// The counts are of the whole population rather than of the page: they are
// what the screen is for, and a "verified" figure that changed as somebody
// paged would be worse than no figure.
func (s *Service) List(ctx context.Context, search, filter string, offset int) (Roster, error) {
	ctx = operator.Scoped(ctx)
	search = strings.TrimSpace(search)
	var roster Roster
	roster.People = []Person{}

	if err := s.db.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE e.user_id IS NOT NULL),
		       count(*) FILTER (WHERE u.locked_until IS NOT NULL AND u.locked_until > NOW()),
		       count(*) FILTER (WHERE NOT EXISTS (
		           SELECT 1 FROM workspace.memberships m WHERE m.user_id = u.id)),
		       count(*) FILTER (WHERE EXISTS (
		           SELECT 1 FROM workspace.sessions s
		            WHERE s.user_id = u.id AND s.revoked_at IS NULL AND s.expires_at > NOW()))
		  FROM registry.users u
		  LEFT JOIN registry.user_eid_identities e ON e.user_id = u.id`).
		Scan(&roster.Total, &roster.Counts.Verified, &roster.Counts.Locked,
			&roster.Counts.Homeless, &roster.Counts.SignedIn); err != nil {
		return Roster{}, fmt.Errorf("control plane: count the people: %w", err)
	}

	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.Query(ctx, `
		SELECT u.id::text, u.email, u.name,
		       e.user_id IS NOT NULL,
		       (SELECT count(*) FROM registry.user_sso_identities i WHERE i.user_id = u.id),
		       (SELECT count(*) FROM workspace.memberships m WHERE m.user_id = u.id),
		       (SELECT count(*) FROM workspace.sessions s
		         WHERE s.user_id = u.id AND s.revoked_at IS NULL AND s.expires_at > NOW()),
		       (SELECT max(s.last_seen_at) FROM workspace.sessions s WHERE s.user_id = u.id),
		       u.locked_until, u.active, u.created_at
		  FROM registry.users u
		  LEFT JOIN registry.user_eid_identities e ON e.user_id = u.id
		 WHERE ($1 = '' OR u.email ILIKE '%' || $1 || '%' OR u.name ILIKE '%' || $1 || '%')
		   AND ($2 <> 'verified' OR e.user_id IS NOT NULL)
		   AND ($2 <> 'locked'   OR (u.locked_until IS NOT NULL AND u.locked_until > NOW()))
		   AND ($2 <> 'homeless' OR NOT EXISTS (
		           SELECT 1 FROM workspace.memberships m WHERE m.user_id = u.id))
		 ORDER BY u.created_at DESC
		 LIMIT $3 OFFSET $4`, search, filter, rosterPageSize, offset)
	if err != nil {
		return Roster{}, fmt.Errorf("control plane: list the people: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var person Person
		if err := rows.Scan(&person.ID, &person.Email, &person.Name, &person.Verified,
			&person.Providers, &person.Organisations, &person.Sessions,
			&person.LastSeenAt, &person.LockedUntil, &person.Active, &person.CreatedAt); err != nil {
			return Roster{}, fmt.Errorf("control plane: read a person: %w", err)
		}
		roster.People = append(roster.People, person)
	}
	return roster, rows.Err()
}

// Identity is one way an account can be signed into.
type Identity struct {
	Kind string `json:"kind"`
	// Subject is the identifier the provider knows them by — a registration
	// number for eID, an issuer's subject for a federated provider.
	Subject    string     `json:"subject"`
	Detail     string     `json:"detail"`
	LinkedAt   time.Time  `json:"linked_at"`
	LastSeenAt *time.Time `json:"last_seen_at"`
}

// Membership is one organisation this person belongs to, and as what.
type Membership struct {
	TenantID   string    `json:"tenant_id"`
	TenantName string    `json:"tenant_name"`
	Slug       string    `json:"slug"`
	Roles      []string  `json:"roles"`
	JoinedAt   time.Time `json:"joined_at"`
}

// Session is a way in that is open right now.
type Session struct {
	ID         string     `json:"id"`
	TenantID   *string    `json:"tenant_id"`
	CreatedAt  time.Time  `json:"created_at"`
	LastSeenAt *time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
}

// Visit is an operator having looked at the platform as this person.
type Visit struct {
	OperatorEmail string    `json:"operator_email"`
	Reason        string    `json:"reason"`
	CreatedAt     time.Time `json:"created_at"`
}

// Detail is everything this console holds about one person.
type Detail struct {
	Person
	Identities  []Identity   `json:"identities"`
	Memberships []Membership `json:"memberships"`
	// OpenSessions is the list; Person.Sessions above is how many there are.
	// Two names because they are two things, and one name would shadow the
	// count the roster row carries.
	OpenSessions []Session `json:"open_sessions"`
	Visits       []Visit   `json:"impersonations"`
}

// Read assembles it.
func (s *Service) Read(ctx context.Context, id string) (Detail, error) {
	ctx = operator.Scoped(ctx)
	detail := Detail{
		Identities: []Identity{}, Memberships: []Membership{},
		OpenSessions: []Session{}, Visits: []Visit{},
	}

	err := s.db.QueryRow(ctx, `
		SELECT u.id::text, u.email, u.name,
		       e.user_id IS NOT NULL,
		       (SELECT count(*) FROM registry.user_sso_identities i WHERE i.user_id = u.id),
		       (SELECT count(*) FROM workspace.memberships m WHERE m.user_id = u.id),
		       (SELECT count(*) FROM workspace.sessions s
		         WHERE s.user_id = u.id AND s.revoked_at IS NULL AND s.expires_at > NOW()),
		       (SELECT max(s.last_seen_at) FROM workspace.sessions s WHERE s.user_id = u.id),
		       u.locked_until, u.active, u.created_at
		  FROM registry.users u
		  LEFT JOIN registry.user_eid_identities e ON e.user_id = u.id
		 WHERE u.id = $1::uuid`, id).
		Scan(&detail.ID, &detail.Email, &detail.Name, &detail.Verified, &detail.Providers,
			&detail.Organisations, &detail.Person.Sessions, &detail.LastSeenAt,
			&detail.LockedUntil, &detail.Active, &detail.CreatedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Detail{}, errors.New("no such person")
	case err != nil:
		if operator.IsInvalidUUID(err) {
			return Detail{}, errors.New("no such person")
		}
		return Detail{}, fmt.Errorf("control plane: read the person: %w", err)
	}

	// Every way in, in one list. eID first: it is the one that says a person
	// proved who they are rather than that a provider vouched for an address.
	var eid Identity
	err = s.db.QueryRow(ctx, `
		SELECT COALESCE(reg_number, civil_id, person_etsi),
		       TRIM(COALESCE(surname, '') || ' ' || COALESCE(given_name, '')),
		       linked_at, last_seen_at
		  FROM registry.user_eid_identities WHERE user_id = $1::uuid`, id).
		Scan(&eid.Subject, &eid.Detail, &eid.LinkedAt, &eid.LastSeenAt)
	if err == nil {
		eid.Kind = "eid"
		detail.Identities = append(detail.Identities, eid)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, fmt.Errorf("control plane: read the eID identity: %w", err)
	}

	rows, err := s.db.Query(ctx, `
		SELECT issuer, subject, COALESCE(email, ''), linked_at, last_seen_at
		  FROM registry.user_sso_identities WHERE user_id = $1::uuid ORDER BY linked_at`, id)
	if err != nil {
		return Detail{}, fmt.Errorf("control plane: read the federated identities: %w", err)
	}
	for rows.Next() {
		var identity Identity
		var issuer string
		if err := rows.Scan(&issuer, &identity.Subject, &identity.Detail,
			&identity.LinkedAt, &identity.LastSeenAt); err != nil {
			rows.Close()
			return Detail{}, fmt.Errorf("control plane: read a federated identity: %w", err)
		}
		identity.Kind = issuer
		detail.Identities = append(detail.Identities, identity)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Detail{}, err
	}

	rows, err = s.db.Query(ctx, `
		SELECT t.id::text, t.name, t.slug, m.created_at,
		       COALESCE(array_agg(r.code ORDER BY r.code) FILTER (WHERE r.code IS NOT NULL), '{}')
		  FROM workspace.memberships m
		  JOIN registry.tenants t ON t.id = m.tenant_id
		  LEFT JOIN workspace.membership_roles mr ON mr.membership_id = m.id
		  LEFT JOIN workspace.roles r ON r.id = mr.role_id
		 WHERE m.user_id = $1::uuid
		 GROUP BY t.id, t.name, t.slug, m.created_at
		 ORDER BY t.name`, id)
	if err != nil {
		return Detail{}, fmt.Errorf("control plane: read the memberships: %w", err)
	}
	for rows.Next() {
		var membership Membership
		if err := rows.Scan(&membership.TenantID, &membership.TenantName, &membership.Slug,
			&membership.JoinedAt, &membership.Roles); err != nil {
			rows.Close()
			return Detail{}, fmt.Errorf("control plane: read a membership: %w", err)
		}
		detail.Memberships = append(detail.Memberships, membership)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Detail{}, err
	}

	rows, err = s.db.Query(ctx, `
		SELECT id::text, tenant_id::text, created_at, last_seen_at, expires_at
		  FROM workspace.sessions
		 WHERE user_id = $1::uuid AND revoked_at IS NULL AND expires_at > NOW()
		 ORDER BY last_seen_at DESC NULLS LAST LIMIT 25`, id)
	if err != nil {
		return Detail{}, fmt.Errorf("control plane: read the sessions: %w", err)
	}
	for rows.Next() {
		var session Session
		if err := rows.Scan(&session.ID, &session.TenantID, &session.CreatedAt,
			&session.LastSeenAt, &session.ExpiresAt); err != nil {
			rows.Close()
			return Detail{}, fmt.Errorf("control plane: read a session: %w", err)
		}
		detail.OpenSessions = append(detail.OpenSessions, session)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Detail{}, err
	}

	// Who has looked at the platform as them. A person is entitled to that
	// answer, and the operator reading this screen is the one who has to give
	// it.
	rows, err = s.db.Query(ctx, `
		SELECT operator_email, reason, created_at
		  FROM registry.operator_impersonations
		 WHERE user_id = $1::uuid ORDER BY created_at DESC LIMIT 25`, id)
	if err != nil {
		return Detail{}, fmt.Errorf("control plane: read the impersonations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var visit Visit
		if err := rows.Scan(&visit.OperatorEmail, &visit.Reason, &visit.CreatedAt); err != nil {
			return Detail{}, fmt.Errorf("control plane: read an impersonation: %w", err)
		}
		detail.Visits = append(detail.Visits, visit)
	}
	return detail, rows.Err()
}
