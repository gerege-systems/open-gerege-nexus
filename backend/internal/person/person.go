/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * A person's own workspace, and the requests they made to other people's.
 *
 * Not a plane. The first design for this was one — internal/person, its own
 * database role, its own GUC, its own policy on the supplier's tables — because
 * the question looked like "how does a citizen read across a hundred
 * organisations". Migration 00085 changed the question by giving every person a
 * workspace of their own, and a citizen acting inside one is an ordinary member
 * of an ordinary workspace: app.current_tenant is set, tenant_isolation
 * applies, gerege_nexus_tenant is the role. There is nothing to read across.
 *
 * So this is a subpackage of the workspace plane, and the whole of the read
 * side is the one statement below. See docs/WORKSPACE_NAMING_PROPOSAL.md §4.9.
 */

package person

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// Store answers for one person's home.
type Store struct{ db *pgxpool.Pool }

// New builds it. It performs no I/O.
func New(db *pgxpool.Pool) *Store { return &Store{db: db} }

// Item is one row as a screen wants it. Separate from nexus.PersonItem, which
// is what a module hands in: this one carries what the platform added — when it
// arrived, when it last moved, and the name of the organisation holding it.
type Item struct {
	ID        string `json:"id"`
	SourceApp string `json:"source_app"`
	SourceRef string `json:"source_ref"`
	Provider  string `json:"provider"`
	Code      string `json:"code"`
	Status    string `json:"status"`
	Answer    string `json:"answer"`
	OpenedAt  string `json:"opened_at"`
	UpdatedAt string `json:"updated_at"`
}

// Items is everything published into the workspace this request acts for.
//
// No tenant_id in the WHERE clause, and that is not an oversight: the row-level
// policy on workspace.person_items is what limits this to the caller's own
// workspace, and writing the condition again here would mean two answers to the
// same question — one of which somebody could edit.
//
// Named columns rather than a star, the way every statement in this repository
// is written: a column added to the table later should not arrive on a screen
// because a SELECT was lazy.
func (s *Store) Items(ctx context.Context, limit int) ([]Item, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `
		SELECT i.id::text, i.source_app, i.source_ref,
		       COALESCE(t.name, ''), i.code, i.status, i.answer,
		       i.opened_at, i.updated_at
		  FROM workspace.person_items i
		  LEFT JOIN registry.tenants t ON t.id = i.provider_tenant_id
		 ORDER BY i.updated_at DESC, i.id
		 LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list this person's requests: %w", err)
	}
	defer rows.Close()

	items := make([]Item, 0, 16)
	for rows.Next() {
		var item Item
		var opened, updated time.Time
		if err := rows.Scan(&item.ID, &item.SourceApp, &item.SourceRef,
			&item.Provider, &item.Code, &item.Status, &item.Answer,
			&opened, &updated); err != nil {
			return nil, fmt.Errorf("read a request: %w", err)
		}
		// RFC 3339 rather than the driver's own rendering: this is a wire
		// format, and a screen that parses two shapes on two deployments is a
		// bug waiting for a timezone.
		item.OpenedAt = opened.Format(time.RFC3339)
		item.UpdatedAt = updated.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

// Publish is nexus.PersonFeed: a module telling a citizen where their request
// has got to.
//
// Everything that makes this safe is in the database. registry.publish_person_item
// is SECURITY DEFINER — it has to be, because the row goes into a workspace the
// caller is not bound to and the policy would refuse it — so the function is
// deliberately narrow: it finds the home by the Gerege number, refuses anything
// that is not a personal workspace, writes one table's named columns, and holds
// EXECUTE for one role. Migration 00086 sets those four rules out and the tests
// beside it hold them.
//
// This wrapper adds nothing to them, which is the point. It takes the caller's
// context, so the write joins the module's own transaction and its own audit
// row rather than opening a second one that can commit without them.
func (s *Store) Publish(ctx context.Context, geID int64, item nexus.PersonItem) error {
	var provider any
	if item.ProviderWorkspaceID != "" {
		provider = item.ProviderWorkspaceID
	}
	if _, err := s.db.Exec(ctx,
		// Every parameter cast, because the function is resolved by its argument
		// types and pgx sends them untyped: without the casts PostgreSQL cannot
		// choose the overload and refuses the call before it runs.
		`SELECT registry.publish_person_item($1::bigint, $2::uuid, $3::text, $4::text, $5::text, $6::text, $7::text)`,
		geID, provider, item.SourceApp, item.SourceRef, item.Code, item.Status, item.Answer); err != nil {
		return fmt.Errorf("publish a request into %d's home: %w", geID, err)
	}
	return nil
}

// AsPersonFeed is the adapter the plane provides, kept beside the thing it
// adapts the way every other rail in this tree is.
func AsPersonFeed(s *Store) nexus.PersonFeed { return s }
