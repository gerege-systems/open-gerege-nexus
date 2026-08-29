package observability

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The front page counts the scheduled reports that never ran and the ones that
// failed. This screen is the "which ones" behind that count, so the two in
// trouble have to come first — an operator looking for what is broken should
// not have to page past the healthy ones.
func TestSchedulesPutTheBrokenOnesFirst(t *testing.T) {
	pool := optest.Pool(t)
	service := New(operator.New(pool), Deps{DB: pool})
	tenantID, _ := optest.Tenant(t, pool)
	ctx := context.Background()

	healthy := schedule(t, pool, tenantID, "healthy", "ok", true)
	failed := schedule(t, pool, tenantID, "failed", "smtp refused", true)
	never := schedule(t, pool, tenantID, "never", "", false)

	schedules, err := service.Schedules(ctx)
	if err != nil {
		t.Fatalf("read the schedules: %v", err)
	}

	position := map[string]int{}
	for index, item := range schedules {
		position[item.ID] = index
	}
	for _, id := range []string{healthy, failed, never} {
		if _, listed := position[id]; !listed {
			t.Fatalf("a schedule just written is not in the list of %d", len(schedules))
		}
	}
	if position[never] > position[healthy] || position[failed] > position[healthy] {
		t.Fatalf("the healthy schedule sorts above the broken ones: %v", position)
	}

	for _, item := range schedules {
		if item.ID != failed {
			continue
		}
		if item.TenantName == "" {
			t.Error("a schedule does not name the organisation it belongs to")
		}
		if item.LastStatus != "smtp refused" || item.LastRunAt == nil {
			t.Errorf("the failed schedule reads %+v", item)
		}
	}
}

// schedule writes one, either run (with a status) or never run.
func schedule(t *testing.T, pool *pgxpool.Pool, tenantID, name, status string, ran bool) string {
	t.Helper()
	var lastRun *time.Time
	if ran {
		when := time.Now().Add(-time.Hour)
		lastRun = &when
	}
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO workspace.report_schedules
		     (tenant_id, report_key, name, cron, recipients, active, last_run_at, last_status)
		 VALUES ($1::uuid, 'test.report', $2, '0 6 * * *', ARRAY['ops@example.test'], TRUE, $3, $4)
		 RETURNING id::text`,
		tenantID, fmt.Sprintf("%s-%d", name, time.Now().UnixNano()), lastRun, status).Scan(&id); err != nil {
		t.Fatalf("write a schedule: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM workspace.report_schedules WHERE id = $1::uuid`, id)
	})
	return id
}
