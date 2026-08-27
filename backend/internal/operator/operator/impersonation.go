package operator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/mailrail"
	"github.com/jackc/pgx/v5"
)

// Looking at the platform as somebody else (§3.B).
//
// This is the one thing the console does that reaches a customer's data, so it
// is the one thing built to be impossible to do quietly. Five conditions, none
// of them optional:
//
//	a reason, typed          — stored, shown to the organisation, not a checkbox
//	a second factor, again   — RequireStepUp on the route
//	thirty minutes           — the session expires; it is not renewed
//	a banner                 — the tenant UI reads it from the session itself
//	two audit trails         — the operator's, and the organisation's own
//
// The last one matters most. An impersonation that only the operators could
// see would be surveillance with paperwork; the row lands in the
// organisation's own audit_events and in operator_impersonations, which their
// administrators can read (migration 00050's policy), so "who looked at our
// data" is a question they can answer without asking us.
//
// The handover exists because a cookie cannot cross hostnames. The console
// runs on cp.nexus.gerege.mn and the session has to be set on
// nexus.gerege.mn, so the console mints a single-use token, hands the operator
// a link, and the platform side exchanges it for the session — once, within a
// minute.

// ImpersonationWindow is how long the borrowed session lasts. Thirty minutes,
// from §3.B, and it is not extended by use: the operator is doing one thing.
const ImpersonationWindow = 30 * time.Minute

// handoverWindow is how long the link is worth following. It is a redirect the
// operator's own browser makes immediately, so a minute is generous.
const handoverWindow = time.Minute

var (
	// ErrNotAMember is impersonating somebody in an organisation they do not
	// belong to. The pair has to exist: an operator naming a user and a tenant
	// that have nothing to do with each other is either a mistake or an
	// attempt to manufacture access.
	ErrNotAMember = errors.New("that person is not a member of that organisation")
	// ErrTenantSuspended is impersonating into an organisation that is closed.
	ErrTenantSuspended = errors.New("that organisation is suspended")
)

// Impersonation is a live or past visit, as both sides see it.
type Impersonation struct {
	ID            string     `json:"id"`
	OperatorEmail string     `json:"operator_email"`
	TenantID      string     `json:"tenant_id"`
	UserID        string     `json:"user_id"`
	UserEmail     string     `json:"user_email"`
	Reason        string     `json:"reason"`
	RedeemedAt    *time.Time `json:"redeemed_at"`
	EndsAt        time.Time  `json:"ends_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

// BeginImpersonation records the visit and returns the link that starts it.
//
// Nothing is signed in yet when this returns: what exists is a permission to
// sign in as somebody, once, in the next minute. The session itself is created
// on the platform's side, by RedeemImpersonation, which is where the cookie
// can be set.
func (c *Console) BeginImpersonation(ctx context.Context, sess Session, tenantID, userID, reason string) (string, error) {
	if reason == "" {
		return "", ErrReasonRequired
	}

	state, err := c.StateOf(ctx, tenantID)
	if err != nil {
		return "", err
	}
	if state.SuspendedAt != nil {
		// Refused rather than allowed as an exception. A suspended
		// organisation is one nobody may act in, and an operator borrowing an
		// account to work around that would be doing exactly what the
		// suspension exists to prevent.
		return "", ErrTenantSuspended
	}

	var email string
	err = c.db.QueryRow(Scoped(ctx),
		`SELECT u.email FROM registry.users u
		   JOIN tenant.memberships m ON m.user_id = u.id AND m.tenant_id = $2::uuid
		  WHERE u.id = $1::uuid`, userID, tenantID).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotAMember
	}
	if err != nil {
		if IsInvalidUUID(err) {
			return "", ErrNotAMember
		}
		return "", fmt.Errorf("control plane: check the membership: %w", err)
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate the handover: %w", err)
	}
	handover := hex.EncodeToString(buf)

	err = c.Do(ctx, sess, Change{
		Action:     "user.impersonate",
		TargetType: "tenant",
		TargetID:   tenantID,
		Reason:     reason,
		After: map[string]any{
			"user_id": userID, "user_email": email,
			"minutes": int(ImpersonationWindow.Minutes()),
		},
	}, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO registry.operator_impersonations
			     (operator_id, operator_email, tenant_id, user_id, reason,
			      handover_hash, handover_expires_at, ends_at)
			 VALUES ($1::uuid, $2, $3::uuid, $4::uuid, $5, $6, NOW() + $7::interval, NOW() + $8::interval)`,
			sess.ID, sess.Email, tenantID, userID, reason, HashToken(handover),
			handoverWindow.String(), ImpersonationWindow.String()); err != nil {
			return fmt.Errorf("record the impersonation: %w", err)
		}

		// The organisation's own trail, in the same transaction. This is the
		// notice §3.B asks for: it appears on the screen their administrators
		// already read, beside their own people's actions, without anybody
		// having to be told to look somewhere new.
		if _, err := tx.Exec(ctx,
			`INSERT INTO tenant.audit_events (tenant_id, user_id, action, resource, details)
			 VALUES ($1::uuid, $2, 'security.impersonation.requested', $3, $4)`,
			tenantID, "operator:"+sess.ID, email,
			map[string]any{
				"operator_email":  sess.Email,
				"reason":          reason,
				"ends_in_minutes": int(ImpersonationWindow.Minutes()),
			}); err != nil {
			return fmt.Errorf("notify the organisation: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	return mailrail.PublicOrigin() + "/impersonate?token=" + handover, nil
}

// ListImpersonations shows an organisation who has been inside it.
func (c *Console) ListImpersonations(ctx context.Context, tenantID string) ([]Impersonation, error) {
	rows, err := c.db.Query(Scoped(ctx),
		`SELECT i.id::text, i.operator_email, i.tenant_id::text, i.user_id::text,
		        COALESCE(u.email, ''), i.reason, i.redeemed_at, i.ends_at, i.created_at
		   FROM registry.operator_impersonations i
		   LEFT JOIN registry.users u ON u.id = i.user_id
		  WHERE i.tenant_id = $1::uuid
		  ORDER BY i.created_at DESC
		  LIMIT 50`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("control plane: list the impersonations: %w", err)
	}
	defer rows.Close()

	visits := make([]Impersonation, 0, 8)
	for rows.Next() {
		var visit Impersonation
		if err := rows.Scan(&visit.ID, &visit.OperatorEmail, &visit.TenantID, &visit.UserID,
			&visit.UserEmail, &visit.Reason, &visit.RedeemedAt, &visit.EndsAt, &visit.CreatedAt); err != nil {
			return nil, fmt.Errorf("control plane: read an impersonation: %w", err)
		}
		visits = append(visits, visit)
	}
	return visits, rows.Err()
}
