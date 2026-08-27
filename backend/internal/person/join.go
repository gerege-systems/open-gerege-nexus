/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Asking an organisation to let you in.
 *
 * Until this existed the only way into an organisation was an invitation:
 * somebody there, or an operator, chose you and sent a link. Migration 00085
 * made that gap visible — a person with no membership can now sign in and land
 * in a workspace of their own, and from there had nowhere to go but wait.
 *
 * The request is the person's, so it starts here. The row it makes lives in the
 * organisation being asked, because that is where it is answered; the person
 * sees it through the same projection everything else on this screen comes
 * through. See db/migrations/00089_join_requests.sql.
 */

package person

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// CoreApp is what the platform publishes under.
//
// A module id shaped like every other one, because person_items does not know
// the difference and should not: the day an app publishes beside this, the two
// rows are the same kind of thing and sort together.
const CoreApp = "io.gerege.nexus.core"

// ErrNotAsked is a slug nobody here answers to. Kept apart from the rest so a
// handler can say "no such organisation" without leaking whether one exists
// under a different name.
var ErrNotAsked = errors.New("no organisation answers to that name")

// Ask records that this person would like to join an organisation.
//
// The write crosses a workspace boundary — the asker is bound to their own and
// the row belongs to the organisation's — so it goes through
// registry.request_to_join, which is the only thing allowed to make that
// crossing and is deliberately narrow about it. Everything this function adds
// is the part that is not a database rule: turning the outcome into something
// the person can read.
func (s *Store) Ask(ctx context.Context, userID, slug, message string) error {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return ErrNotAsked
	}

	var requestID, tenantID, tenantName string
	err := s.db.QueryRow(ctx,
		`SELECT request_id::text, workspace_id::text, workspace_name
		   FROM registry.request_to_join($1::uuid, $2::text, $3::text)`,
		userID, slug, message).Scan(&requestID, &tenantID, &tenantName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotAsked
		}
		return fmt.Errorf("ask %q to let this person in: %w", slug, err)
	}

	// The person's own copy. Published rather than read back, because the row
	// itself is in a workspace this session cannot see — which is the whole
	// reason the projection exists.
	return s.PublishTo(ctx, userID, nexus.PersonItem{
		SourceApp:           CoreApp,
		SourceRef:           requestID,
		ProviderWorkspaceID: tenantID,
		Code:                JoinRequestCode,
		Status:              StatusPending,
		Answer:              "",
	})
}

// The three states a join request is in, named once.
//
// They are the platform's own words and travel to the screen unchanged —
// person_items does not interpret a status, it stores one. Written as constants
// because the same three strings are set here, checked in the decision below
// and asserted in the tests, and a typo in any of the three is a request that
// silently never leaves the queue.
const (
	JoinRequestCode = "join_request"
	StatusPending   = "PENDING"
	StatusAccepted  = "ACCEPTED"
	StatusDeclined  = "DECLINED"
)
