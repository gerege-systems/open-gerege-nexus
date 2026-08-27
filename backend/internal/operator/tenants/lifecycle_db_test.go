package tenants

import (
	"context"
	"errors"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
)

// CP-2's promises, asked of a real database: an organisation cannot be deleted
// by one person, cannot be deleted today, and comes back with one button until
// the grace period runs out. Impersonation leaves a trail on both sides. A
// limit that is hard refuses.

func TestSuspendEndsTheSessionsAndResumeRestores(t *testing.T) {
	pool := optest.Pool(t)
	service := &Service{op: operator.New(pool), db: pool}
	account, _ := optest.Account(t, pool, operator.RoleOperator)
	sess := optest.Session(account)
	tenantID, _ := optest.Tenant(t, pool)
	userID, _ := optest.Person(t, pool, tenantID)

	ctx := context.Background()
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenant.sessions (token_hash, user_id, tenant_id, expires_at)
		 VALUES (repeat('a', 64), $1::uuid, $2::uuid, NOW() + INTERVAL '1 hour')`,
		userID, tenantID); err != nil {
		t.Fatalf("give the person a session: %v", err)
	}

	if err := service.Suspend(ctx, sess, tenantID, "unpaid invoices"); err != nil {
		t.Fatalf("suspend: %v", err)
	}

	var suspended bool
	var reason string
	if err := pool.QueryRow(ctx,
		`SELECT suspended_at IS NOT NULL, suspension_reason FROM platform.tenants WHERE id = $1::uuid`,
		tenantID).Scan(&suspended, &reason); err != nil {
		t.Fatalf("read the organisation: %v", err)
	}
	if !suspended || reason != "unpaid invoices" {
		t.Fatalf("the organisation was not suspended with its reason: %v %q", suspended, reason)
	}

	// The people already signed in are stopped now, not when their sessions
	// would have expired.
	var live int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM tenant.sessions
		  WHERE tenant_id = $1::uuid AND revoked_at IS NULL AND expires_at > NOW()`,
		tenantID).Scan(&live); err != nil {
		t.Fatalf("count the sessions: %v", err)
	}
	if live != 0 {
		t.Fatalf("%d sessions survived the suspension", live)
	}

	if err := service.Resume(ctx, sess, tenantID, "paid"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT suspended_at IS NOT NULL FROM platform.tenants WHERE id = $1::uuid`, tenantID).
		Scan(&suspended); err != nil {
		t.Fatalf("read the organisation: %v", err)
	}
	if suspended {
		t.Fatal("the organisation is still suspended after being resumed")
	}
	// Resuming an organisation that is running is a mistake worth naming
	// rather than a silent no-op.
	if err := service.Resume(ctx, sess, tenantID, "again"); !errors.Is(err, ErrNotSuspended) {
		t.Fatalf("resuming a running organisation answered %v", err)
	}
}

// The other half of the sweep: when the day does come, it goes.
func TestTheSweepDeletesWhenTheGracePeriodHasEnded(t *testing.T) {
	pool := optest.Pool(t)
	service := &Service{op: operator.New(pool), db: pool}
	tenantID, _ := optest.Tenant(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`UPDATE platform.tenants SET deletion_scheduled_at = NOW() - INTERVAL '1 minute' WHERE id = $1::uuid`,
		tenantID); err != nil {
		t.Fatalf("backdate the deletion: %v", err)
	}

	service.SweepDeletions(ctx)

	var alive bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM platform.tenants WHERE id = $1::uuid)`, tenantID).
		Scan(&alive); err != nil {
		t.Fatalf("look for the organisation: %v", err)
	}
	if alive {
		t.Fatal("an organisation whose grace period ended is still there")
	}
}

// Impersonation writes to both trails before anybody has gone anywhere, and
// refuses an organisation that is closed.
func TestImpersonationRecordsBothSides(t *testing.T) {
	pool := optest.Pool(t)
	service := &Service{op: operator.New(pool), db: pool}
	account, _ := optest.Account(t, pool, operator.RoleSupport)
	sess := optest.Session(account)
	tenantID, _ := optest.Tenant(t, pool)
	userID, _ := optest.Person(t, pool, tenantID)
	ctx := context.Background()

	if _, err := service.op.BeginImpersonation(ctx, sess, tenantID, userID, ""); !errors.Is(err, operator.ErrReasonRequired) {
		t.Fatalf("impersonating without a reason answered %v", err)
	}

	link, err := service.op.BeginImpersonation(ctx, sess, tenantID, userID, "customer reported a missing invoice")
	if err != nil {
		t.Fatalf("begin the impersonation: %v", err)
	}
	if link == "" {
		t.Fatal("no handover link was produced")
	}

	// The operator's trail.
	if got := optest.AuditCount(t, pool, account.ID, "user.impersonate"); got != 1 {
		t.Fatalf("the operator audit has %d rows, want 1", got)
	}
	// And the organisation's own — which is the half that makes this
	// answerable by them rather than by us.
	var theirs int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM tenant.audit_events
		  WHERE tenant_id = $1::uuid AND action = 'security.impersonation.requested'`,
		tenantID).Scan(&theirs); err != nil {
		t.Fatalf("read the organisation's trail: %v", err)
	}
	if theirs != 1 {
		t.Fatalf("the organisation's trail has %d rows, want 1", theirs)
	}

	// A person who does not work there cannot be borrowed.
	otherTenant, _ := optest.Tenant(t, pool)
	if _, err := service.op.BeginImpersonation(ctx, sess, otherTenant, userID, "fishing"); !errors.Is(err, operator.ErrNotAMember) {
		t.Fatalf("impersonating a non-member answered %v", err)
	}

	// And a suspended organisation is not a way in.
	if err := service.Suspend(ctx, optest.Session(account), tenantID, "suspended"); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if _, err := service.op.BeginImpersonation(ctx, sess, tenantID, userID, "still fishing"); !errors.Is(err, operator.ErrTenantSuspended) {
		t.Fatalf("impersonating into a suspended organisation answered %v", err)
	}
}

func TestQuotaIsStoredAndCounted(t *testing.T) {
	pool := optest.Pool(t)
	service := &Service{op: operator.New(pool), db: pool}
	account, _ := optest.Account(t, pool, operator.RoleOperator)
	tenantID, _ := optest.Tenant(t, pool)
	optest.Person(t, pool, tenantID)
	ctx := context.Background()

	limit := 1
	if err := service.SetQuota(ctx, optest.Session(account), tenantID, Quota{
		MaxUsers: &limit, Enforcement: EnforcementHard,
	}, "trial account"); err != nil {
		t.Fatalf("set the limits: %v", err)
	}

	quota, err := service.GetQuota(ctx, tenantID)
	if err != nil {
		t.Fatalf("read the limits: %v", err)
	}
	if quota.MaxUsers == nil || *quota.MaxUsers != 1 {
		t.Fatalf("the user limit came back as %v", quota.MaxUsers)
	}
	if quota.Users != 1 {
		t.Fatalf("the organisation has %d people, want 1", quota.Users)
	}
	// A limit nobody set stays nil rather than becoming zero — the difference
	// between "no limit" and "nobody may join".
	if quota.MaxStorageMB != nil {
		t.Fatalf("an unset storage limit came back as %v", *quota.MaxStorageMB)
	}
	if quota.Enforcement != EnforcementHard {
		t.Fatalf("enforcement is %q", quota.Enforcement)
	}
	if err := service.SetQuota(ctx, optest.Session(account), tenantID,
		Quota{Enforcement: "whenever"}, "typo"); !errors.Is(err, ErrUnknownEnforcement) {
		t.Fatalf("an unknown enforcement mode answered %v", err)
	}
}
