/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package audit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
)

// The audit screen is useful only if its three filters describe the same row
// the write path recorded. Exercise the database query rather than duplicating
// it in a handler mock: ordering, JSON decoding and the operator database role
// all live below the HTTP layer.
func TestAuditTrailCanBeFilteredToOneActionAndTarget(t *testing.T) {
	pool := optest.Pool(t)
	op := operator.New(pool)
	service := New(op, Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleAuditor)
	targetID := fmt.Sprintf("audit-probe-%d", time.Now().UnixNano())
	ctx := context.Background()

	change := operator.Change{
		Action:     "audit.probe",
		TargetType: "test-target",
		TargetID:   targetID,
		Reason:     "prove that the audit screen reads what the writer recorded",
		Before:     map[string]any{"state": "before"},
		After:      map[string]any{"state": "after"},
	}
	if err := op.Do(ctx, optest.Session(account), change, nil); err != nil {
		t.Fatalf("record the audit probe: %v", err)
	}

	entries, err := service.ListAudit(ctx, change.Action, change.TargetType, change.TargetID)
	if err != nil {
		t.Fatalf("read the audit trail: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("filtered audit trail has %d rows, want 1", len(entries))
	}
	entry := entries[0]
	if entry.OperatorID != account.ID || entry.OperatorEmail != account.Email ||
		entry.Action != change.Action || entry.TargetType != change.TargetType ||
		entry.TargetID != change.TargetID || entry.Reason != change.Reason {
		t.Fatalf("audit row does not match the change: %+v", entry)
	}
	if string(entry.Before) != `{"state": "before"}` && string(entry.Before) != `{"state":"before"}` {
		t.Errorf("before = %s", entry.Before)
	}
	if string(entry.After) != `{"state": "after"}` && string(entry.After) != `{"state":"after"}` {
		t.Errorf("after = %s", entry.After)
	}

	none, err := service.ListAudit(ctx, "audit.some-other-action", change.TargetType, change.TargetID)
	if err != nil {
		t.Fatalf("read a non-matching filter: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("a non-matching action returned %d rows", len(none))
	}
}

// The roster deliberately exposes lifecycle state and deliberately omits all
// credential material. This test pins the lifecycle half, which is easy to
// lose when the SELECT list and OperatorSummary are edited separately.
func TestOperatorRosterIncludesDisabledLifecycleState(t *testing.T) {
	pool := optest.Pool(t)
	service := New(operator.New(pool), Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleSupport)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`UPDATE operator.operator_accounts
		    SET disabled_at = NOW(), last_login_at = NOW()
		  WHERE id = $1::uuid`, account.ID); err != nil {
		t.Fatalf("set lifecycle state: %v", err)
	}

	roster, err := service.ListOperators(ctx)
	if err != nil {
		t.Fatalf("list operators: %v", err)
	}
	for _, row := range roster {
		if row.ID != account.ID {
			continue
		}
		if row.Email != account.Email || row.Role != operator.RoleSupport {
			t.Fatalf("roster returned the wrong account: %+v", row)
		}
		if row.DisabledAt == nil || row.LastLoginAt == nil || row.CreatedAt.IsZero() {
			t.Fatalf("roster lost lifecycle state: %+v", row)
		}
		return
	}
	t.Fatalf("operator %s is absent from the roster", account.Email)
}
