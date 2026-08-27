/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Persistence for the email verification service.
 */

package emailverify

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type store struct {
	db *pgxpool.Pool
}

const verificationColumns = `
	id::text, tenant_id::text, source, purpose, email, redirect_url, status,
	expires_at, verified_at, created_at`

func scanVerification(row pgx.Row) (*Verification, error) {
	var v Verification
	if err := row.Scan(&v.ID, &v.TenantID, &v.Source, &v.Purpose, &v.Email,
		&v.RedirectURL, &v.Status, &v.ExpiresAt, &v.VerifiedAt, &v.CreatedAt); err != nil {
		return nil, err
	}
	return &v, nil
}

type newVerification struct {
	TenantID    string
	Source      string
	Purpose     string
	Email       string
	RefHash     string
	RedirectURL string
	RequestedIP string
	ExpiresAt   time.Time
}

func (s *store) insertVerification(ctx context.Context, in newVerification) (*Verification, error) {
	row := s.db.QueryRow(ctx, `
		INSERT INTO workspace.email_verifications
			(tenant_id, source, purpose, email, token_hash, redirect_url,
			 requested_ip, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+verificationColumns,
		in.TenantID, in.Source, in.Purpose, in.Email, in.RefHash,
		in.RedirectURL, in.RequestedIP, in.ExpiresAt)
	return scanVerification(row)
}

func (s *store) deleteVerification(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM workspace.email_verifications WHERE id = $1`, id)
	return err
}

// setExpiry replaces the local placeholder deadline with the provider's, which
// is the one that actually governs the link.
func (s *store) setExpiry(ctx context.Context, id string, expiresAt time.Time) (*Verification, error) {
	row := s.db.QueryRow(ctx, `
		UPDATE workspace.email_verifications SET expires_at = $2 WHERE id = $1
		RETURNING `+verificationColumns, id, expiresAt)
	return scanVerification(row)
}

// claimVerification marks a request confirmed, and answers whether it was this
// call that confirmed it.
//
// The condition lives in the UPDATE rather than in a preceding SELECT so that
// two returns arriving at once cannot both succeed — a browser reloading the
// landing page would otherwise race itself, and a reference that travelled
// through a mailbox must not be replayable.
func (s *store) claimVerification(ctx context.Context, refHash string) (*Verification, error) {
	row := s.db.QueryRow(ctx, `
		UPDATE workspace.email_verifications
		SET status = 'VERIFIED', verified_at = NOW()
		WHERE token_hash = $1 AND status = 'PENDING' AND expires_at > NOW()
		RETURNING `+verificationColumns, refHash)
	verification, err := scanVerification(row)
	if errors.Is(err, pgx.ErrNoRows) {
		// Honoured, expired, or never issued — one answer for all three.
		return nil, ErrLinkSpent
	}
	return verification, err
}

// countTenantSends returns how many links a tenant asked for inside the window
// and when the oldest of them was, which is what a Retry-After is computed from.
func (s *store) countTenantSends(ctx context.Context, tenantID string, since time.Time) (int, *time.Time, error) {
	var count int
	var oldest *time.Time
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*), MIN(created_at)
		FROM workspace.email_verifications
		WHERE tenant_id = $1 AND created_at >= $2`, tenantID, since).Scan(&count, &oldest)
	return count, oldest, err
}

func (s *store) lastSendTo(ctx context.Context, tenantID, email string) (*time.Time, error) {
	var last *time.Time
	err := s.db.QueryRow(ctx, `
		SELECT MAX(created_at)
		FROM workspace.email_verifications
		WHERE tenant_id = $1 AND email = $2`, tenantID, email).Scan(&last)
	return last, err
}

func (s *store) recent(ctx context.Context, tenantID string, limit int) ([]Verification, error) {
	rows, err := s.db.Query(ctx, `
		SELECT `+verificationColumns+`
		FROM workspace.email_verifications
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]Verification, 0, limit)
	for rows.Next() {
		verification, scanErr := scanVerification(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		list = append(list, *verification)
	}
	return list, rows.Err()
}

// stats counts a tenant's verifications in one pass. A pending row whose
// deadline has passed is reported as expired even before the sweep rewrites it,
// so the screen never claims somebody is still able to act on a dead link.
func (s *store) stats(ctx context.Context, tenantID string) (*Stats, error) {
	var st Stats
	err := s.db.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'VERIFIED'),
			COUNT(*) FILTER (WHERE status = 'PENDING' AND expires_at > NOW()),
			COUNT(*) FILTER (WHERE status = 'EXPIRED' OR (status = 'PENDING' AND expires_at <= NOW())),
			COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '24 hours')
		FROM workspace.email_verifications
		WHERE tenant_id = $1`, tenantID).
		Scan(&st.Total, &st.Verified, &st.Pending, &st.Expired, &st.Last24h)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *store) expirePending(ctx context.Context) (int64, error) {
	tag, err := s.db.Exec(ctx, `
		UPDATE workspace.email_verifications
		SET status = 'EXPIRED'
		WHERE status = 'PENDING' AND expires_at <= NOW()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *store) purgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM workspace.email_verifications WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
