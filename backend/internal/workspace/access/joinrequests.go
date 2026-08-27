/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The organisation's half of "let me in".
 *
 * Somebody with no membership asks (internal/person), and this is where it is
 * answered. It sits in access rather than beside the asking because that is
 * what it is: a decision about who may act inside this organisation, next to
 * the roles and the memberships it results in.
 *
 * Nothing here is privileged. The queue is this workspace's own rows under the
 * ordinary policy, and accepting writes a membership the same way every other
 * path does. The only thing that reaches outside is the last line — telling the
 * person, whose copy of the request lives in a workspace this session cannot
 * see. That goes through the rail the platform publishes for everybody
 * (nexus.PersonFeed), because a core that declares a capability and never calls
 * it has built the same half-thing pkg/nexus/capability.go was written about.
 */

package access

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// JoinRequest is one person waiting at the door.
type JoinRequest struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Message   string `json:"message"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// ErrNoSuchRequest is a request this organisation has not been asked, or has
// already answered. The two are one error on purpose: an administrator poking
// at ids should not learn which of the two it was.
var ErrNoSuchRequest = errors.New("no open request with that id")

// PendingJoinRequests is the queue, oldest first.
//
// No tenant_id in the WHERE clause: the policy is what limits this to the
// workspace being acted in, and repeating the condition here would be a second
// answer to the same question — one of which somebody could edit.
func (h *Handlers) PendingJoinRequests(ctx context.Context) ([]JoinRequest, error) {
	rows, err := h.db.Query(ctx, `
		SELECT j.id::text, j.user_id::text, u.name, u.email, j.message, j.status, j.created_at
		  FROM workspace.join_requests j
		  JOIN registry.users u ON u.id = j.user_id
		 WHERE j.status = 'PENDING'
		 ORDER BY j.created_at, j.id`)
	if err != nil {
		return nil, fmt.Errorf("list the people asking to join: %w", err)
	}
	defer rows.Close()

	queue := make([]JoinRequest, 0, 8)
	for rows.Next() {
		var one JoinRequest
		var createdAt time.Time
		if err := rows.Scan(&one.ID, &one.UserID, &one.Name, &one.Email,
			&one.Message, &one.Status, &createdAt); err != nil {
			return nil, fmt.Errorf("read a request: %w", err)
		}
		one.CreatedAt = createdAt.Format(time.RFC3339)
		queue = append(queue, one)
	}
	return queue, rows.Err()
}

// Decide answers one request, and tells the person.
//
// Accepting writes the membership here rather than through a trigger on the
// request, so the quota is checked at the moment it is spent and the failure
// arrives as an error the administrator can read instead of a row that was
// silently not created.
func (h *Handlers) Decide(ctx context.Context, requestID, actorUserID string, accept bool) error {
	tx, err := h.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var tenantID, userID string
	// FOR UPDATE, so two administrators pressing accept at the same moment do
	// not both write a membership: the second one finds the row already
	// decided and stops at the status check below.
	err = tx.QueryRow(ctx,
		`SELECT tenant_id::text, user_id::text FROM workspace.join_requests
		  WHERE id = $1::uuid AND status = 'PENDING' FOR UPDATE`, requestID).Scan(&tenantID, &userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoSuchRequest
	}
	if err != nil {
		return fmt.Errorf("read the request: %w", err)
	}

	status := "DECLINED"
	if accept {
		status = "ACCEPTED"
		if err := h.authn.CheckUserQuota(ctx, tenantID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO workspace.memberships (tenant_id, user_id)
			 SELECT $1::uuid, $2::uuid
			  WHERE NOT EXISTS (SELECT 1 FROM workspace.memberships
			                     WHERE tenant_id = $1::uuid AND user_id = $2::uuid)`,
			tenantID, userID); err != nil {
			return fmt.Errorf("add them to the organisation: %w", err)
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE workspace.join_requests
		    SET status = $2, decided_by = $3::uuid, decided_at = NOW()
		  WHERE id = $1::uuid`, requestID, status, actorUserID); err != nil {
		return fmt.Errorf("record the decision: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	// After the commit, not inside it. The person's copy is a projection: if
	// publishing fails the decision still stands, and the next decision on the
	// same request — or a retry — writes it. Inside the transaction a failure
	// here would roll back a membership that was correctly granted.
	feed, err := nexus.Capability[nexus.PersonFeed]()
	if err != nil {
		return nil
	}
	return feed.PublishTo(ctx, userID, nexus.PersonItem{
		SourceApp:           "io.gerege.nexus.core",
		SourceRef:           requestID,
		ProviderWorkspaceID: tenantID,
		Code:                "join_request",
		Status:              status,
	})
}
