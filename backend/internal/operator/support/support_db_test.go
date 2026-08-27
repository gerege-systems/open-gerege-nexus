/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package support

import (
	"context"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
)

// Unlocking is the one thing the console may write to somebody's account, and
// the database is what says so: the same handler cannot touch a password.
func TestUnlockIsAllTheConsoleMayWriteToAnAccount(t *testing.T) {
	pool := optest.Pool(t)
	service := New(operator.New(pool), Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleSupport)
	tenantID, _ := optest.Tenant(t, pool)
	userID, _ := optest.Person(t, pool, tenantID)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`UPDATE registry.users SET failed_login_attempts = 5, locked_until = NOW() + INTERVAL '15 minutes'
		  WHERE id = $1::uuid`, userID); err != nil {
		t.Fatalf("lock the account: %v", err)
	}

	if err := service.Unlock(ctx, optest.Session(account), userID, "they telephoned"); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	var locked bool
	if err := pool.QueryRow(ctx,
		`SELECT locked_until IS NOT NULL FROM registry.users WHERE id = $1::uuid`, userID).Scan(&locked); err != nil {
		t.Fatalf("read the account: %v", err)
	}
	if locked {
		t.Fatal("the account is still locked")
	}

	// The column grant, exercised directly. If this ever succeeds, the console
	// can set somebody's password, and every claim made in support.go about
	// what it cannot do is false.
	if _, err := pool.Exec(operator.Scoped(ctx),
		`UPDATE registry.users SET password_hash = 'x' WHERE id = $1::uuid`, userID); err == nil {
		t.Fatal("the operator role changed a password")
	}
	if _, err := pool.Exec(operator.Scoped(ctx),
		`UPDATE registry.users SET email = 'taken@example.test' WHERE id = $1::uuid`, userID); err == nil {
		t.Fatal("the operator role changed an address")
	}
	// And it cannot delete an organisation, which is what makes the grace
	// period a guarantee rather than a habit.
	if _, err := pool.Exec(operator.Scoped(ctx), `DELETE FROM registry.tenants WHERE id = $1::uuid`, tenantID); err == nil {
		t.Fatal("the operator role deleted an organisation")
	}
}
