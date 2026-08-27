/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package tenants

import (
	"context"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
	"github.com/google/uuid"
)

// A person's home is not a customer.
//
// Migration 00085 made every account that belongs to no organisation its own
// workspace, which means registry.tenants now holds two different kinds of
// thing. This screen is for one of them: an operator opens it to find who is
// suspended, who is near a quota, who telephoned. A list that is mostly
// citizens answers none of those.
//
// It is also what keeps tenantPageSize honest. That bound is 200 with a comment
// saying a deployment past it needs a different screen rather than a second
// page — true of customers, and untrue within a week of homes joining them. The
// list would have silently stopped at 200 rows of the wrong sort.
func TestAPersonsHomeIsNotOnTheConsolesList(t *testing.T) {
	pool := optest.Pool(t)
	service := &Service{op: operator.New(pool), db: pool}
	ctx := context.Background()

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name) VALUES ($1,'x',$2) RETURNING id::text`,
		"consolehome-"+suffix+"@example.mn", "Иргэн "+suffix).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id=$1`, userID) })

	var homeID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.tenants (slug, name, kind, owner_user_id)
		 VALUES ($1, $2, 'personal', $3::uuid) RETURNING id::text`,
		"home-"+suffix, "Иргэн "+suffix, userID).Scan(&homeID); err != nil {
		t.Fatal(err)
	}

	// An organisation alongside it, so a list that came back empty for the
	// wrong reason — a broken query, a database with nothing in it — cannot
	// pass as a list that filtered correctly.
	orgID, _ := optest.Tenant(t, pool)

	rows, err := service.ListTenants(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	var sawHome, sawOrg bool
	for _, row := range rows {
		if row.ID == homeID {
			sawHome = true
		}
		if row.ID == orgID {
			sawOrg = true
		}
	}
	if sawHome {
		t.Error("a person's home is listed as an organisation on the console")
	}
	if !sawOrg {
		t.Error("the organisation is missing from the list, so the filter is refusing everything")
	}

	// Search is the other way in, and it takes a different branch of the same
	// WHERE clause: one that reads the name a citizen is called by.
	found, err := service.ListTenants(ctx, "Иргэн "+suffix)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range found {
		if row.ID == homeID {
			t.Error("searching by name reached a person's home")
		}
	}
}
