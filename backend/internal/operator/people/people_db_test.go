/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package people

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The roster is the whole population, and its numbers are of the population
// rather than of the page: a "verified" figure that changed as somebody paged
// would be worse than no figure at all.
func TestTheRosterCountsThePopulationNotThePage(t *testing.T) {
	pool := optest.Pool(t)
	service := New(operator.New(pool), Deps{DB: pool})
	tenantID, _ := optest.Tenant(t, pool)
	verified := withEID(t, pool, tenantID)
	ctx := context.Background()

	roster, err := service.List(ctx, "", "", 0)
	if err != nil {
		t.Fatalf("read the roster: %v", err)
	}
	if roster.Total < 1 || roster.Counts.Verified < 1 {
		t.Fatalf("the counts read %+v", roster)
	}
	if len(roster.People) > roster.Total {
		t.Fatalf("a page of %d in a population of %d", len(roster.People), roster.Total)
	}

	// The filter narrows the list without touching the counts.
	filtered, err := service.List(ctx, "", "verified", 0)
	if err != nil {
		t.Fatalf("filter the roster: %v", err)
	}
	if filtered.Total != roster.Total {
		t.Errorf("the population changed with the filter: %d then %d", roster.Total, filtered.Total)
	}
	for _, person := range filtered.People {
		if !person.Verified {
			t.Fatalf("%s is on the verified list without an eID identity", person.Email)
		}
	}

	var found bool
	for _, person := range filtered.People {
		if person.ID == verified {
			found = true
			if person.Organisations < 1 {
				t.Error("a person in an organisation is counted as being in none")
			}
		}
	}
	if !found {
		t.Error("the person just verified is not on the verified list")
	}
}

// One person, and every way into their account. This is the screen's whole
// point: "why can this person get in" is answered by the list of identities,
// not by the fact that they exist.
func TestOnePersonCarriesEveryWayIn(t *testing.T) {
	pool := optest.Pool(t)
	service := New(operator.New(pool), Deps{DB: pool})
	tenantID, _ := optest.Tenant(t, pool)
	userID := withEID(t, pool, tenantID)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `
		INSERT INTO registry.user_sso_identities (user_id, issuer, subject, email, name, claims)
		VALUES ($1::uuid, 'https://accounts.google.com', $2, 'someone@example.test', 'Someone', '{}'::jsonb)`,
		userID, fmt.Sprintf("sub-%d", time.Now().UnixNano())); err != nil {
		t.Fatalf("link a federated identity: %v", err)
	}

	detail, err := service.Read(ctx, userID)
	if err != nil {
		t.Fatalf("read the person: %v", err)
	}
	kinds := map[string]bool{}
	for _, identity := range detail.Identities {
		kinds[identity.Kind] = true
	}
	if !kinds["eid"] || !kinds["https://accounts.google.com"] {
		t.Fatalf("the person's ways in read %+v", detail.Identities)
	}
	if len(detail.Memberships) < 1 || detail.Memberships[0].TenantName == "" {
		t.Fatalf("the memberships read %+v", detail.Memberships)
	}
	if len(detail.Memberships[0].Roles) == 0 {
		t.Error("a membership carries no role, so the screen cannot say what they may do")
	}
}

// Somebody who does not exist is said so plainly, and an id that is not an id
// is the same answer rather than a 500.
func TestAnUnknownPersonIsSaidPlainly(t *testing.T) {
	pool := optest.Pool(t)
	service := New(operator.New(pool), Deps{DB: pool})
	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "not-an-id"} {
		if _, err := service.Read(context.Background(), id); err == nil {
			t.Errorf("%q was read as a person", id)
		}
	}
}

// withEID makes a person this platform has watched sign in with eID.
func withEID(t *testing.T, pool *pgxpool.Pool, tenantID string) string {
	t.Helper()
	userID, _ := optest.Person(t, pool, tenantID)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO registry.user_eid_identities (user_id, civil_id, reg_number, person_etsi, given_name, surname)
		VALUES ($1::uuid, $2, $2, $2, 'Test', 'Person')`,
		userID, fmt.Sprintf("EID%d", time.Now().UnixNano())); err != nil {
		t.Fatalf("link an eID identity: %v", err)
	}
	return userID
}
