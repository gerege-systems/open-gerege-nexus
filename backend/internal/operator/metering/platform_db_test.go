package metering

import (
	"context"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
	"github.com/jackc/pgx/v5/pgxpool"

	usagemetric "github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/usage"
)

// The platform report exists to answer a question the per-organisation screen
// cannot: who is using this deployment. It has to fold two organisations into
// two lines, and it has to aggregate each metric the way that metric means —
// summing daily active users would count one person once per day.
func TestThePlatformReportRollsUpEveryOrganisation(t *testing.T) {
	pool := optest.Pool(t)
	service := usageService(t, pool)
	first, _ := optest.Tenant(t, pool)
	second, _ := optest.Tenant(t, pool)
	ctx := context.Background()

	// The first two days of the month the report covers.
	//
	// It used to be today and yesterday, which are the same month on 30 or 31
	// days out of every 31 and two different months on the other one. On the
	// first of a month "yesterday" belongs to the previous month's report, so
	// half the rows below fell outside the window being asserted on and the
	// test failed — not because the rollup was wrong, but because the calendar
	// had turned over. It failed for the first time on 2026-09-01 and would
	// have failed again on the first of every month after that.
	//
	// What the test is actually about is two *different days inside the
	// reported month*; which two is immaterial, and the query selects on month
	// membership alone. Naming them outright removes the wall clock from the
	// assertion — every month has a first and a second.
	now := time.Now()
	firstDay := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	secondDay := firstDay.AddDate(0, 0, 1)
	// Two days of the same three metrics, so the difference between summing
	// and taking the peak or the latest is visible in the answer.
	count(t, pool, first, usagemetric.Actions, firstDay, 4)
	count(t, pool, first, usagemetric.Actions, secondDay, 6)
	count(t, pool, first, usagemetric.ActiveUsers, firstDay, 9)
	count(t, pool, first, usagemetric.ActiveUsers, secondDay, 3)
	count(t, pool, first, usagemetric.StorageMB, firstDay, 100)
	count(t, pool, first, usagemetric.StorageMB, secondDay, 120)
	count(t, pool, second, usagemetric.Actions, secondDay, 5)

	report, err := service.PlatformUsageReport(ctx)
	if err != nil {
		t.Fatalf("read the platform usage: %v", err)
	}
	if report.Month != time.Now().Format("2006-01") {
		t.Fatalf("the report is for %q", report.Month)
	}

	lines := map[string]TenantUsage{}
	for _, line := range report.Tenants {
		lines[line.TenantID] = line
	}
	one, two := lines[first], lines[second]
	if one.TenantID == "" || two.TenantID == "" {
		t.Fatalf("the report lists %d organisations and not both of the ones written", len(report.Tenants))
	}
	if one.Metrics[usagemetric.Actions] != 10 {
		t.Errorf("actions rolled up to %d, want the two days summed (10)", one.Metrics[usagemetric.Actions])
	}
	if one.Metrics[usagemetric.ActiveUsers] != 9 {
		t.Errorf("active users rolled up to %d, want the month's peak (9)", one.Metrics[usagemetric.ActiveUsers])
	}
	if one.Metrics[usagemetric.StorageMB] != 120 {
		t.Errorf("storage rolled up to %d, want the latest reading (120)", one.Metrics[usagemetric.StorageMB])
	}
	if one.Collected == nil {
		t.Error("an organisation with counted rows reports nothing collected")
	}
	if two.Metrics[usagemetric.Actions] != 5 {
		t.Errorf("the second organisation's actions came out as %d", two.Metrics[usagemetric.Actions])
	}
	if report.Totals[usagemetric.Actions] < 15 {
		t.Errorf("the platform total for actions is %d, less than the two organisations written",
			report.Totals[usagemetric.Actions])
	}
}

// An organisation nobody has counted yet is a line of nothing, not a missing
// line: "never counted" and "used nothing" are different answers and the
// screen shows them differently.
func TestAnOrganisationWithNoUsageStillHasALine(t *testing.T) {
	pool := optest.Pool(t)
	service := usageService(t, pool)
	tenantID, _ := optest.Tenant(t, pool)

	report, err := service.PlatformUsageReport(context.Background())
	if err != nil {
		t.Fatalf("read the platform usage: %v", err)
	}
	for _, line := range report.Tenants {
		if line.TenantID != tenantID {
			continue
		}
		if len(line.Metrics) != 0 || line.Collected != nil {
			t.Fatalf("an uncounted organisation reports %+v", line)
		}
		return
	}
	t.Fatal("an organisation with no usage is missing from the report")
}

// count writes one metering row, the way the collector does.
func count(t *testing.T, pool *pgxpool.Pool, tenantID, metric string, day time.Time, value int64) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO registry.usage_events (tenant_id, day, metric, value)
		 VALUES ($1::uuid, $2::date, $3, $4)
		 ON CONFLICT (tenant_id, day, metric) DO UPDATE SET value = EXCLUDED.value`,
		tenantID, day.Format("2006-01-02"), metric, value); err != nil {
		t.Fatalf("write a usage row: %v", err)
	}
}
