/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package tenants

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
)

// The three properties bulk.go promises, asserted together because they only
// mean anything together: a batch with a bad row in the middle must open the
// organisations around it, report each one, and settle the row that already
// exists as settled rather than as an error.
func TestABatchOpensWhatItCanAndReportsEachRow(t *testing.T) {
	pool := optest.Pool(t)
	op := operator.New(pool)
	service := New(op, Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	sess := optest.Session(account)
	ctx := context.Background()

	// A chosen administrator rather than an invited one, for the reason
	// TestAChosenAdministratorIsNotInvited exists: an e-mail address sends the
	// creation down the invitation path, and this test's Deps carry no mail
	// service. What is under test is the batch, not the invitation.
	home, _ := optest.Tenant(t, pool)
	admin := verifiedUser(t, pool, home)

	stamp := time.Now().UnixNano()
	first := fmt.Sprintf("bulk-a-%d", stamp)
	second := fmt.Sprintf("bulk-b-%d", stamp)

	// One organisation is opened the ordinary way first, so the batch below
	// contains a row that is already in place.
	existing, err := service.CreateTenant(ctx, sess, NewTenant{
		Name: "Already Here", Slug: first, Reason: "seed the duplicate",
		AdminUserID: admin})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE slug = ANY($1)`,
			[]string{first, second})
	})

	outcomes, err := service.CreateTenants(ctx, sess, []NewTenant{
		{Name: "Already Here", Slug: first, Reason: "re-run the file", AdminUserID: admin},
		{Name: "Bad Slug", Slug: "NOT a slug", Reason: "the row that cannot open", AdminUserID: admin},
		{Name: "Genuinely New", Slug: second, Reason: "the row after the bad one", AdminUserID: admin},
	})
	if err != nil {
		t.Fatalf("the batch itself was refused: %v", err)
	}
	if len(outcomes) != 3 {
		t.Fatalf("got %d outcomes for 3 rows", len(outcomes))
	}

	if outcomes[0].Status != BulkExists {
		t.Errorf("a slug already in place reads %q, want %q — a re-run must not look like a failure",
			outcomes[0].Status, BulkExists)
	}
	if outcomes[1].Status != BulkFailed || outcomes[1].Error == "" {
		t.Errorf("the bad row reads %+v; it should fail and say why", outcomes[1])
	}
	// The point of the whole file: the row after the failure still opened.
	if outcomes[2].Status != BulkCreated {
		t.Fatalf("the row after the bad one reads %q — a failure stopped the batch",
			outcomes[2].Status)
	}
	if outcomes[2].Created == nil || outcomes[2].Created.ID == "" {
		t.Error("a created row carries no organisation; the per-row report is what the screen shows")
	}

	var live int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM registry.tenants WHERE slug = ANY($1)`,
		[]string{first, second}).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != 2 {
		t.Errorf("%d organisations exist, want 2 (the seeded one and the new one)", live)
	}
	_ = existing
}

// A batch larger than the cap is refused whole, before anything is written.
// The cap is a blast radius, not a performance limit — see bulk.go.
func TestATooLargeBatchOpensNothing(t *testing.T) {
	pool := optest.Pool(t)
	op := operator.New(pool)
	service := New(op, Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleSuperadmin)

	list := make([]NewTenant, MaxBulkTenants+1)
	for i := range list {
		list[i] = NewTenant{Name: "Too Many",
			Slug: fmt.Sprintf("toomany-%d-%d", time.Now().UnixNano(), i)}
	}

	outcomes, err := service.CreateTenants(context.Background(), optest.Session(account), list)
	if err == nil {
		t.Fatal("a batch over the cap was accepted")
	}
	if outcomes != nil {
		t.Error("a refused batch returned outcomes; nothing should have been attempted")
	}
}
