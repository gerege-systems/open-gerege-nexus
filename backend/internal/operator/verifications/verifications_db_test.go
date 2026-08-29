/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package verifications

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The point of moving this screen is that it counts every organisation at
// once: two tenants, three rows, one ledger. It is also the test that fails if
// migration 00095's policy is dropped — without it the console's own database
// role reads no rows at all and every number is a confident zero.
func TestTheLedgerCountsEveryOrganisation(t *testing.T) {
	pool := optest.Pool(t)
	service := New(operator.New(pool), Deps{DB: pool})
	first, _ := optest.Tenant(t, pool)
	second, _ := optest.Tenant(t, pool)
	ctx := context.Background()

	write(t, pool, first, "one@example.test", "VERIFIED", time.Now().Add(time.Hour))
	write(t, pool, second, "two@example.test", "PENDING", time.Now().Add(time.Hour))
	// Pending, but its deadline has gone: the screen must call this expired
	// rather than tell an operator somebody can still act on it.
	write(t, pool, second, "three@example.test", "PENDING", time.Now().Add(-time.Hour))

	overview, err := service.Read(ctx, 25)
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}

	mine := map[string]Verification{}
	for _, row := range overview.Recent {
		if row.TenantID == first || row.TenantID == second {
			mine[row.Email] = row
		}
	}
	if len(mine) != 3 {
		t.Fatalf("the ledger lists %d of the three rows just written", len(mine))
	}
	if mine["one@example.test"].TenantID != first || mine["two@example.test"].TenantID != second {
		t.Fatalf("rows are attributed to the wrong organisations: %+v", mine)
	}
	if mine["one@example.test"].TenantName == "" {
		t.Fatal("the ledger does not name the organisation a row belongs to")
	}
	if overview.Stats.Total < 3 || overview.Stats.Verified < 1 || overview.Stats.Pending < 1 || overview.Stats.Expired < 1 {
		t.Fatalf("counts do not cover the rows just written: %+v", overview.Stats)
	}
	if overview.Stats.Tenants < 2 {
		t.Fatalf("the ledger counted %d organisations, want at least the two written to", overview.Stats.Tenants)
	}
}

// The service's own health is not something this plane can ask, so a
// deployment that cannot be asked has to read as "not configured" rather than
// as an error on an otherwise working screen.
func TestAMissingProbeIsNotAnError(t *testing.T) {
	pool := optest.Pool(t)
	service := New(operator.New(pool), Deps{DB: pool})

	overview, err := service.Read(context.Background(), 5)
	if err != nil {
		t.Fatalf("read the ledger without a probe: %v", err)
	}
	if overview.Health.Configured || overview.Health.Reachable {
		t.Fatalf("with no probe the screen claims %+v", overview.Health)
	}
}

// The probe's answer is passed through as it stands: this screen reports the
// other plane's verdict, it does not form one.
func TestTheProbesAnswerReachesTheScreen(t *testing.T) {
	pool := optest.Pool(t)
	service := New(operator.New(pool), Deps{DB: pool, Probe: func(context.Context) Health {
		return Health{Configured: true, Reachable: false, Detail: "health check failed", ProviderURL: "https://verify.test"}
	}})

	overview, err := service.Read(context.Background(), 5)
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	if !overview.Health.Configured || overview.Health.Reachable ||
		overview.Health.Detail != "health check failed" || overview.Health.ProviderURL != "https://verify.test" {
		t.Fatalf("the screen reports %+v, not what the probe said", overview.Health)
	}
}

func write(t *testing.T, pool *pgxpool.Pool, tenantID, email, status string, expires time.Time) {
	t.Helper()
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		t.Fatalf("make a token: %v", err)
	}
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO workspace.email_verifications
		     (tenant_id, source, purpose, email, token_hash, status, expires_at)
		 VALUES ($1::uuid, 'test', 'ledger', $2, $3, $4, $5) RETURNING id::text`,
		tenantID, email, hex.EncodeToString(token), status, expires).Scan(&id); err != nil {
		t.Fatalf("write a verification: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM workspace.email_verifications WHERE id = $1::uuid`, id)
	})
}
