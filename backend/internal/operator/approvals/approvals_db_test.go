/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package approvals

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/tenants"
)

// The whole of the deletion rule in one test: one person cannot do it, a
// second person can only schedule it, and it is reversible until the day it is
// not.
func TestDeletionNeedsTwoPeopleAndThirtyDays(t *testing.T) {
	pool := optest.Pool(t)
	op := operator.New(pool)
	service := New(op, Deps{DB: pool})
	// Cancelling and sweeping are the organisation screen's: this one only
	// records that two people agreed.
	organisations := tenants.New(op, tenants.Deps{DB: pool})
	asker, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	approver, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	tenantID, _ := optest.Tenant(t, pool)
	ctx := context.Background()

	approvalID, err := service.RequestDeletion(ctx, optest.Session(asker), tenantID, "customer asked us to")
	if err != nil {
		t.Fatalf("request the deletion: %v", err)
	}

	// Nothing has happened to the organisation yet.
	var scheduled *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT deletion_scheduled_at FROM registry.tenants WHERE id = $1::uuid`, tenantID).
		Scan(&scheduled); err != nil {
		t.Fatalf("read the organisation: %v", err)
	}
	if scheduled != nil {
		t.Fatal("asking for a deletion scheduled it")
	}

	// The person who asked cannot be the person who agrees.
	if err := service.Approve(ctx, optest.Session(asker), approvalID, "me again"); !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("a self-approval answered %v", err)
	}

	if err := service.Approve(ctx, optest.Session(approver), approvalID, "confirmed with the customer"); err != nil {
		t.Fatalf("approve: %v", err)
	}

	var suspended bool
	if err := pool.QueryRow(ctx,
		`SELECT deletion_scheduled_at, suspended_at IS NOT NULL FROM registry.tenants WHERE id = $1::uuid`,
		tenantID).Scan(&scheduled, &suspended); err != nil {
		t.Fatalf("read the organisation: %v", err)
	}
	if scheduled == nil {
		t.Fatal("approving did not schedule the deletion")
	}
	if !suspended {
		t.Fatal("an organisation on its way out is still open for business")
	}
	// Thirty days, not now.
	if until := time.Until(*scheduled); until < DeletionGrace-time.Hour {
		t.Fatalf("the grace period is %v, want about %v", until, DeletionGrace)
	}

	// The sweep leaves it alone until the day comes. It is the organisation
	// screen's, because deleting is: this screen only records that two people
	// agreed.
	organisations.SweepDeletions(ctx)
	var alive bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM registry.tenants WHERE id = $1::uuid)`, tenantID).
		Scan(&alive); err != nil {
		t.Fatalf("look for the organisation: %v", err)
	}
	if !alive {
		t.Fatal("the sweep deleted an organisation whose grace period had not ended")
	}

	// One button puts it back.
	if err := organisations.CancelDeletion(ctx, optest.Session(approver), tenantID, "customer changed their mind"); err != nil {
		t.Fatalf("cancel the deletion: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT deletion_scheduled_at FROM registry.tenants WHERE id = $1::uuid`, tenantID).Scan(&scheduled); err != nil {
		t.Fatalf("read the organisation: %v", err)
	}
	if scheduled != nil {
		t.Fatal("cancelling did not take the organisation off the list")
	}
}
