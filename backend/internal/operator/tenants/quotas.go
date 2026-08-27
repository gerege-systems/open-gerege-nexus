package tenants

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"

	"github.com/jackc/pgx/v5"
)

// Limits per organisation (§3.A).
//
// Two things about the shape are deliberate and easy to get wrong the other
// way round:
//
//   - NULL is "no limit", and 0 is a limit of zero. A scheme where 0 meant
//     "unlimited" cannot express "this organisation may add nobody", which is
//     what a limit means during a billing dispute.
//   - `soft` warns and `hard` refuses, and soft is the default. A platform
//     that starts refusing on the day limits are introduced refuses work
//     somebody was in the middle of; measuring first and enforcing second is
//     the order that does not break anybody's afternoon.
//
// Only the user count is enforced in CP-2, because it is the only one this
// platform can currently count. Storage and AI calls are stored, shown, and
// marked as not-yet-enforced; CP-5's usage_events is what gives them numbers,
// and the plan says so — the check reads from there rather than growing a
// second counting mechanism here.

// Quota is one organisation's limits and where it stands against them.
type Quota struct {
	TenantID string `json:"tenant_id"`
	// Nil means no limit. Kept as pointers rather than -1 sentinels so that
	// the difference between "unset" and "zero" survives the JSON.
	MaxUsers          *int      `json:"max_users"`
	MaxStorageMB      *int      `json:"max_storage_mb"`
	MaxAICallsMonthly *int      `json:"max_ai_calls_monthly"`
	Enforcement       string    `json:"enforcement"`
	UpdatedAt         time.Time `json:"updated_at"`
	// Users is what they actually have, so the console can show 18/20 without
	// a second request.
	Users int `json:"users"`
	// Enforced lists the limits this build actually applies. The console shows
	// the rest as "recorded, not yet enforced" — a limit that looks live and
	// is not is worse than no limit at all.
	Enforced []string `json:"enforced"`
}

// EnforcementSoft warns; EnforcementHard refuses.
const (
	EnforcementSoft = "soft"
	EnforcementHard = "hard"
)

// ErrUnknownEnforcement is a mode that is neither.
var ErrUnknownEnforcement = errors.New(`enforcement must be "soft" or "hard"`)

// GetQuota reads an organisation's limits, defaulting to none.
func (s *Service) GetQuota(ctx context.Context, tenantID string) (Quota, error) {
	quota := Quota{TenantID: tenantID, Enforcement: EnforcementSoft, Enforced: []string{"users"}}

	err := s.db.QueryRow(operator.Scoped(ctx),
		`SELECT COALESCE(q.max_users, -1), COALESCE(q.max_storage_mb, -1),
		        COALESCE(q.max_ai_calls_monthly, -1),
		        COALESCE(q.enforcement, 'soft'), COALESCE(q.updated_at, NOW()),
		        (SELECT count(*) FROM tenant.memberships m WHERE m.tenant_id = t.id)
		   FROM platform.tenants t
		   LEFT JOIN platform.tenant_quotas q ON q.tenant_id = t.id
		  WHERE t.id = $1::uuid`, tenantID).
		Scan(orNil(&quota.MaxUsers), orNil(&quota.MaxStorageMB), orNil(&quota.MaxAICallsMonthly),
			&quota.Enforcement, &quota.UpdatedAt, &quota.Users)
	if errors.Is(err, pgx.ErrNoRows) {
		return Quota{}, operator.ErrTenantNotFound
	}
	if err != nil {
		if operator.IsInvalidUUID(err) {
			return Quota{}, operator.ErrTenantNotFound
		}
		return Quota{}, fmt.Errorf("control plane: read the limits: %w", err)
	}
	return quota, nil
}

// SetQuota writes an organisation's limits.
func (s *Service) SetQuota(ctx context.Context, sess operator.Session, tenantID string, wanted Quota, reason string) error {
	if wanted.Enforcement != EnforcementSoft && wanted.Enforcement != EnforcementHard {
		return ErrUnknownEnforcement
	}
	before, err := s.GetQuota(ctx, tenantID)
	if err != nil {
		return err
	}

	return s.op.Do(ctx, sess, operator.Change{
		Action:     "tenant.quota.set",
		TargetType: "tenant",
		TargetID:   tenantID,
		Reason:     reason,
		Before:     before,
		After:      wanted,
	}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO platform.tenant_quotas
			     (tenant_id, max_users, max_storage_mb, max_ai_calls_monthly, enforcement, updated_by, updated_at)
			 VALUES ($1::uuid, $2, $3, $4, $5, $6::uuid, NOW())
			 ON CONFLICT (tenant_id) DO UPDATE
			    SET max_users = EXCLUDED.max_users,
			        max_storage_mb = EXCLUDED.max_storage_mb,
			        max_ai_calls_monthly = EXCLUDED.max_ai_calls_monthly,
			        enforcement = EXCLUDED.enforcement,
			        updated_by = EXCLUDED.updated_by,
			        updated_at = NOW()`,
			tenantID, wanted.MaxUsers, wanted.MaxStorageMB, wanted.MaxAICallsMonthly,
			wanted.Enforcement, sess.ID)
		return err
	})
}

// orNil turns the -1 the query uses for SQL NULL back into a nil pointer.
//
// COALESCE to a sentinel and convert here, rather than scanning into **int:
// pgx can do the latter, and it makes every one of these six scan targets a
// double pointer that the next person to read this has to hold in their head.
func orNil(target **int) any { return &nullableInt{target: target} }

type nullableInt struct{ target **int }

func (n *nullableInt) Scan(value any) error {
	number, ok := value.(int64)
	if !ok {
		return fmt.Errorf("expected an integer limit, got %T", value)
	}
	if number < 0 {
		*n.target = nil
		return nil
	}
	limit := int(number)
	*n.target = &limit
	return nil
}
