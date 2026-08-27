/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package directory answers who belongs to an organisation, for modules.
//
// The queries were in domain/organisation/postgres, which is to say a module
// was reading `users`, `memberships`, `roles`, `membership_roles` and `tenants`
// with its own SQL. Those are the platform's most careful tables — a query that
// forgets a tenant clause on `memberships` hands one organisation the names of
// another's staff — and a module reading them is a dependency on the platform's
// schema that no compiler sees.
//
// So the SQL lives here, beside the tables, and modules ask nexus.Directory.
package directory

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// New builds the directory over the platform's pool.
//
// Every query is bound to the caller's organisation by dbguard before it runs,
// so the tenant arguments here narrow a set the row-level policy has already
// closed — belt and braces, in the order that leaves the braces load-bearing.
func New(db *pgxpool.Pool) nexus.Directory { return store{db} }

type store struct{ db *pgxpool.Pool }

func (s store) People(ctx context.Context, tenantIDs []string) ([]nexus.DirectoryPerson, error) {
	rows, err := s.db.Query(ctx,
		`SELECT ms.id::text, u.id::text, u.name, u.email, u.phone,
		        ms.active, u.is_admin, ms.created_at::text,
		        COALESCE(ARRAY_AGG(r.code) FILTER (WHERE r.code IS NOT NULL), '{}'),
		        ms.tenant_id::text, tn.name
		   FROM workspace.memberships ms
		   JOIN registry.users u ON u.id = ms.user_id
		   JOIN registry.tenants tn ON tn.id = ms.tenant_id
		   LEFT JOIN workspace.membership_roles mr ON mr.membership_id = ms.id
		   LEFT JOIN workspace.roles r ON r.id = mr.role_id
		  WHERE ms.tenant_id = ANY($1::uuid[])
		  GROUP BY ms.id, u.id, tn.name
		  ORDER BY tn.name, ms.active DESC, u.name`, tenantIDs)
	if err != nil {
		return nil, fmt.Errorf("read the directory: %w", err)
	}
	defer rows.Close()

	people := make([]nexus.DirectoryPerson, 0)
	for rows.Next() {
		var person nexus.DirectoryPerson
		if err := rows.Scan(&person.MembershipID, &person.UserID, &person.Name,
			&person.Email, &person.Phone, &person.Active, &person.IsAdmin,
			&person.JoinedAt, &person.Roles, &person.TenantID, &person.TenantName); err != nil {
			return nil, fmt.Errorf("read the directory: %w", err)
		}
		people = append(people, person)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the directory: %w", err)
	}
	return people, nil
}

func (s store) Membership(ctx context.Context, tenantID, membershipID string) (nexus.DirectoryMembership, error) {
	var found nexus.DirectoryMembership
	err := s.db.QueryRow(ctx,
		`SELECT ms.user_id::text, u.is_admin
		   FROM workspace.memberships ms JOIN registry.users u ON u.id = ms.user_id
		  WHERE ms.id = $1 AND ms.tenant_id = $2`,
		membershipID, tenantID).Scan(&found.UserID, &found.IsAdmin)
	if err != nil {
		return nexus.DirectoryMembership{}, fmt.Errorf("read the membership: %w", err)
	}
	return found, nil
}

func (s store) CountAdmins(ctx context.Context, tenantID, exceptMembershipID string) (int, error) {
	var count int
	err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM workspace.memberships ms
		   JOIN registry.users u ON u.id = ms.user_id
		  WHERE ms.tenant_id = $1 AND ms.active AND u.is_admin AND ms.id <> $2`,
		tenantID, exceptMembershipID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count the administrators: %w", err)
	}
	return count, nil
}

func (s store) SetActive(ctx context.Context, tenantID, membershipID string, active bool) (bool, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE workspace.memberships SET active = $3,
		        deactivated_at = CASE WHEN $3 THEN NULL ELSE NOW() END
		  WHERE id = $1 AND tenant_id = $2`, membershipID, tenantID, active)
	if err != nil {
		return false, fmt.Errorf("change the membership: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ErrNoSuchMembership is what a caller gets when the organisation has no such
// row. Kept as pgx's own sentinel rather than wrapped into a new one: a caller
// that wants to tell "not found" from "the database is down" already compares
// against it, and inventing a second name for the same condition is how two
// halves of a codebase come to disagree about which one means what.
var ErrNoSuchMembership = pgx.ErrNoRows
