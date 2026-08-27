package approvals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DeletionGrace is how long an organisation stays recoverable after somebody
// asks for it to be deleted.
//
// Thirty days is what the plan asks for and what the industry has settled on,
// for a reason worth restating: the person who notices that an organisation is
// gone is usually not the person who asked for it, and they may be on holiday.
const DeletionGrace = 30 * 24 * time.Hour

// ApprovalWindow is how long an unanswered deletion request stays answerable.
// A request nobody acted on becomes a button somebody presses months later
// without remembering what it was for.
const ApprovalWindow = 7 * 24 * time.Hour

// The two-person rule (§2.3).
//
// One operator asks, a different superadmin agrees, and only then does
// anything happen. It is the control that survives an account being taken:
// whoever holds one operator's credentials still cannot delete an
// organisation, because the second signature has to come from somebody else.
//
// Three things enforce "somebody else", and none of them is the handler:
//
//   - the capability table — only superadmin holds operator.CapApprove;
//   - this file — the requester is compared to the approver;
//   - the database — `pending_approvals_two_people` is a CHECK constraint, so
//     even a query written later, by somebody who has not read this file,
//     cannot record a self-approval.
//
// The deletion itself is still not immediate. Approval sets a date thirty days
// out; the sweep in lifecycle.go is what eventually removes anything.

// ApprovalAction is what a request is for. A closed set: an approval whose
// action nothing recognises can never be executed, so a row written by
// something that did not know this list is inert rather than dangerous.
type ApprovalAction string

// ActionTenantDelete is the only two-person action CP-2 has. CP-3's kill
// switch and CP-4's deployment button join it here rather than growing their
// own tables.
const ActionTenantDelete ApprovalAction = "tenant.delete"

var (
	// ErrApprovalNotFound covers unknown, expired and already-answered
	// requests alike — from the caller's side they are the same thing: there
	// is nothing here to answer.
	ErrApprovalNotFound = errors.New("no such open request")
	// ErrSelfApproval is one operator trying to be both people.
	ErrSelfApproval = errors.New("the operator who asked cannot be the one who agrees")
	// ErrAlreadyScheduled is asking to delete an organisation that is already
	// on its way out.
	ErrAlreadyScheduled = errors.New("that organisation is already scheduled for deletion")
)

// Approval is one open request, as the console lists it.
type Approval struct {
	ID              string          `json:"id"`
	Action          ApprovalAction  `json:"action"`
	TargetType      string          `json:"target_type"`
	TargetID        string          `json:"target_id"`
	TargetName      string          `json:"target_name"`
	Payload         json.RawMessage `json:"payload"`
	RequestedBy     string          `json:"requested_by"`
	RequestedByName string          `json:"requested_by_name"`
	RequestedReason string          `json:"requested_reason"`
	RequestedAt     time.Time       `json:"requested_at"`
	ExpiresAt       time.Time       `json:"expires_at"`
}

// RequestDeletion asks for an organisation to be deleted.
//
// Nothing about the organisation changes here. What is created is a request,
// and it is visible to every operator until somebody answers it or it expires
// — which is itself a control: a request to delete a customer's organisation
// should be seen by people who did not make it.
func (s *Service) RequestDeletion(ctx context.Context, sess operator.Session, tenantID, reason string) (string, error) {
	state, err := s.op.StateOf(ctx, tenantID)
	if err != nil {
		return "", err
	}
	if state.DeletionScheduledAt != nil {
		return "", ErrAlreadyScheduled
	}

	var approvalID string
	err = s.op.Do(ctx, sess, operator.Change{
		Action:     "tenant.deletion.request",
		TargetType: "tenant",
		TargetID:   tenantID,
		Reason:     reason,
		Before:     state,
		After:      map[string]any{"grace_days": int(DeletionGrace.Hours() / 24)},
	}, func(ctx context.Context, tx pgx.Tx) error {
		// One open request per organisation. Two would let two approvals
		// schedule the same deletion twice, and the second would look to
		// whoever answered it like they had made the decision.
		var open bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (
			    SELECT 1 FROM operator.pending_approvals
			     WHERE action = $1 AND target_id = $2
			       AND approved_at IS NULL AND rejected_at IS NULL AND expires_at > NOW())`,
			string(ActionTenantDelete), tenantID).Scan(&open); err != nil {
			return fmt.Errorf("check for an open request: %w", err)
		}
		if open {
			return errors.New("somebody has already asked for this organisation to be deleted")
		}

		return tx.QueryRow(ctx,
			`INSERT INTO operator.pending_approvals
			     (action, target_type, target_id, payload, requested_by, requested_reason, expires_at)
			 VALUES ($1, 'tenant', $2, $3, $4::uuid, $5, NOW() + $6::interval)
			 RETURNING id::text`,
			string(ActionTenantDelete), tenantID,
			map[string]any{"slug": state.Slug, "name": state.Name},
			sess.ID, reason, ApprovalWindow.String()).Scan(&approvalID)
	})
	if err != nil {
		return "", err
	}
	return approvalID, nil
}

// ListApprovals returns the requests still waiting for an answer.
func (s *Service) ListApprovals(ctx context.Context) ([]Approval, error) {
	rows, err := s.db.Query(operator.Scoped(ctx),
		`SELECT a.id::text, a.action, a.target_type, a.target_id,
		        COALESCE(t.name, ''), a.payload,
		        a.requested_by::text, COALESCE(o.name, o.email, ''),
		        a.requested_reason, a.requested_at, a.expires_at
		   FROM operator.pending_approvals a
		   LEFT JOIN registry.tenants t ON a.target_type = 'tenant' AND t.id = a.target_id::uuid
		   LEFT JOIN operator.operator_accounts o ON o.id = a.requested_by
		  WHERE a.approved_at IS NULL AND a.rejected_at IS NULL AND a.expires_at > NOW()
		  ORDER BY a.requested_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("control plane: list the open requests: %w", err)
	}
	defer rows.Close()

	approvals := make([]Approval, 0, 4)
	for rows.Next() {
		var approval Approval
		if err := rows.Scan(&approval.ID, &approval.Action, &approval.TargetType, &approval.TargetID,
			&approval.TargetName, &approval.Payload, &approval.RequestedBy, &approval.RequestedByName,
			&approval.RequestedReason, &approval.RequestedAt, &approval.ExpiresAt); err != nil {
			return nil, fmt.Errorf("control plane: read an open request: %w", err)
		}
		approvals = append(approvals, approval)
	}
	return approvals, rows.Err()
}

// Approve agrees to a request and carries it out, in one transaction with the
// audit row.
//
// The request is claimed with a conditional UPDATE rather than read and then
// written: two superadmins pressing the button at the same moment would
// otherwise both see an open request, and the organisation would be scheduled
// twice by two people who each think they were the second signature.
func (s *Service) Approve(ctx context.Context, sess operator.Session, approvalID, reason string) error {
	return s.op.Do(ctx, sess, operator.Change{
		Action:     "approval.approve",
		TargetType: "approval",
		TargetID:   approvalID,
		Reason:     reason,
	}, func(ctx context.Context, tx pgx.Tx) error {
		var action ApprovalAction
		var targetID, requestedBy string
		err := tx.QueryRow(ctx,
			`UPDATE operator.pending_approvals
			    SET approved_by = $2::uuid, approved_at = NOW(), executed_at = NOW()
			  WHERE id = $1::uuid
			    AND approved_at IS NULL AND rejected_at IS NULL AND expires_at > NOW()
			RETURNING action, target_id, requested_by::text`,
			approvalID, sess.ID).Scan(&action, &targetID, &requestedBy)
		if errors.Is(err, pgx.ErrNoRows) {
			// Either there is no such open request, or the CHECK constraint
			// refused a self-approval — the UPDATE matches no row in both
			// cases. The self-approval is checked below so the caller is told
			// which, but a constraint violation would arrive here as an error
			// rather than as no rows, so this ordering is safe either way.
			var requester string
			if err := tx.QueryRow(ctx,
				`SELECT requested_by::text FROM operator.pending_approvals WHERE id = $1::uuid`,
				approvalID).Scan(&requester); err == nil && requester == sess.ID {
				return ErrSelfApproval
			}
			return ErrApprovalNotFound
		}
		if isSelfApproval(err) {
			// The database refused it first — the CHECK constraint is the
			// backstop the Go comparison below can never be relied on to
			// replace — and its message names a constraint rather than a
			// person, so it is translated here.
			return ErrSelfApproval
		}
		if err != nil {
			return fmt.Errorf("claim the request: %w", err)
		}
		if requestedBy == sess.ID {
			return ErrSelfApproval
		}

		if err := execute(ctx, tx, action, targetID); err != nil {
			return err
		}
		// Approving a deletion suspends the organisation, so the platform's
		// view of it is now stale on every replica.
		s.changed(targetID)
		return nil
	})
}

// Reject refuses a request. The organisation is untouched and the request
// cannot be reopened — asking again is a new request, with a new reason.
func (s *Service) Reject(ctx context.Context, sess operator.Session, approvalID, reason string) error {
	if reason == "" {
		return operator.ErrReasonRequired
	}
	return s.op.Do(ctx, sess, operator.Change{
		Action:     "approval.reject",
		TargetType: "approval",
		TargetID:   approvalID,
		Reason:     reason,
	}, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE operator.pending_approvals
			    SET rejected_by = $2::uuid, rejected_at = NOW(), rejected_reason = $3
			  WHERE id = $1::uuid
			    AND approved_at IS NULL AND rejected_at IS NULL AND expires_at > NOW()`,
			approvalID, sess.ID, reason)
		if err != nil {
			return fmt.Errorf("reject the request: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrApprovalNotFound
		}
		return nil
	})
}

// isSelfApproval reports whether PostgreSQL refused the two-person rule.
//
// 23514 is a check-constraint violation, and the constraint's name is what
// distinguishes this one from every other check in the schema.
func isSelfApproval(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514" &&
		pgErr.ConstraintName == "pending_approvals_two_people"
}

// execute carries out an approved request.
//
// A switch with no default that acts: an action this build does not recognise
// leaves the approval marked executed and does nothing, which is the safe way
// round. The alternative — a default that guesses — is how an approval for one
// thing becomes an execution of another.
func execute(ctx context.Context, tx pgx.Tx, action ApprovalAction, targetID string) error {
	switch action {
	case ActionTenantDelete:
		// Not a deletion: a date. Everything the organisation has stays
		// exactly where it is for the next thirty days, the console shows it
		// counting down, and one button cancels it.
		if _, err := tx.Exec(ctx,
			`UPDATE registry.tenants
			    SET deletion_scheduled_at = NOW() + $2::interval,
			        suspended_at = COALESCE(suspended_at, NOW()),
			        suspension_reason = CASE WHEN suspended_at IS NULL
			                                 THEN 'scheduled for deletion'
			                                 ELSE suspension_reason END
			  WHERE id = $1::uuid`, targetID, DeletionGrace.String()); err != nil {
			return fmt.Errorf("schedule the deletion: %w", err)
		}
		// Suspended as well as scheduled, and that is deliberate: an
		// organisation on its way out should stop accumulating work somebody
		// will lose. It is the same suspension, so cancelling the deletion
		// leaves an operator one more button to press, which is the right way
		// round — resuming is a decision, not a side effect.
		if _, err := tx.Exec(ctx,
			`UPDATE workspace.sessions SET revoked_at = NOW()
			  WHERE tenant_id = $1::uuid AND revoked_at IS NULL AND expires_at > NOW()`,
			targetID); err != nil {
			return fmt.Errorf("end the organisation's sessions: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("this build does not know how to carry out %q", action)
	}
}
