package reporting_test

import (
	"context"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/reporting"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The four tests §3.5 asks for, against a real database:
//
//  1. with no grant, nothing is visible;
//  2. a revoked grant closes immediately;
//  3. counterparty scope does not include another mine's rows;
//  4. the audit is written on both sides.
//
// They are integration tests because every one of them is a claim about what
// PostgreSQL does — the row-level policies, the partial unique index, the
// tenant binding — and a mock would only test the test.

// tripFixture stands in for the transport report the proposal describes: rows
// that belong to a counterparty, filterable by that counterparty's registration
// number. No shipped module has such a table yet (see the note in
// billing/reports.go), so the schema is created by the test.
type tripFixture struct{}

func (tripFixture) Key() string { return "test.trips" }
func (tripFixture) App() string { return "io.gerege.nexus.billing" }
func (tripFixture) Titles() map[string]string {
	return map[string]string{"mn": "Рейс", "en": "Trips"}
}
func (tripFixture) Params() []reporting.ParamSpec { return nil }
func (tripFixture) Scopes() []string {
	return []string{reporting.ScopeCounterparty, reporting.ScopeFull}
}

func (tripFixture) Columns() []reporting.ColumnSpec {
	return []reporting.ColumnSpec{
		{Key: "route", Kind: reporting.ColumnText, Titles: map[string]string{"mn": "Чиглэл"}},
		{Key: "tonnes", Kind: reporting.ColumnNumber, Total: true, Titles: map[string]string{"mn": "Тонн"}},
	}
}

// Run honours the counterparty reference. This is the contract a report signs
// by declaring ScopeCounterparty, and the thing the third test below checks.
func (tripFixture) Run(ctx context.Context, q reporting.Querier, p reporting.Params) (reporting.Result, error) {
	sql := `SELECT route, tonnes FROM reporting_test_trips WHERE tenant_id = $1`
	args := []any{reporting.TenantOf(ctx)}
	if ref := p.Counterparty(); ref != "" {
		sql += ` AND counterparty_ref = $2`
		args = append(args, ref)
	}
	sql += ` ORDER BY route`

	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return reporting.Result{}, err
	}
	collected, err := reporting.Collect(rows, func() (map[string]any, error) {
		var route string
		var tonnes float64
		if err := rows.Scan(&route, &tonnes); err != nil {
			return nil, err
		}
		return map[string]any{"route": route, "tonnes": tonnes}, nil
	})
	if err != nil {
		return reporting.Result{}, err
	}
	return reporting.Result{Rows: collected}, nil
}

// sharingFixture builds two mines and one transport company with trips for
// each, and returns their ids.
type sharingFixture struct {
	pool      *pgxpool.Pool
	transport string
	mineA     string
	mineB     string
	regA      string
	regB      string
}

func newSharingFixture(t *testing.T) sharingFixture {
	t.Helper()
	pool := openPool(t)
	ctx := context.Background()

	// The trips table. Created and dropped by the test: it stands for a module
	// that does not exist in this repository yet.
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS reporting_test_trips (
		    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		    tenant_id        UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		    counterparty_ref TEXT NOT NULL,
		    route            TEXT NOT NULL,
		    tonnes           NUMERIC(12,2) NOT NULL
		)`)
	if err != nil {
		t.Fatalf("create the trips table: %v", err)
	}
	// The same policy every tenant table carries, so the fixture is not a
	// weaker environment than production.
	for _, statement := range []string{
		`ALTER TABLE reporting_test_trips ENABLE ROW LEVEL SECURITY`,
		`ALTER TABLE reporting_test_trips FORCE ROW LEVEL SECURITY`,
		`GRANT SELECT, INSERT ON reporting_test_trips TO gerege_nexus_tenant`,
		`DROP POLICY IF EXISTS tenant_isolation ON reporting_test_trips`,
		`CREATE POLICY tenant_isolation ON reporting_test_trips TO gerege_nexus_tenant
		     USING (tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
		     WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)`,
	} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("prepare the trips table: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DROP TABLE IF EXISTS reporting_test_trips`)
	})

	suffix := uuid.NewString()[:8]
	fixture := sharingFixture{
		pool:      pool,
		transport: seedTenant(t, pool, "share-transport-"+suffix, 0),
		mineA:     seedTenant(t, pool, "share-mine-a-"+suffix, 0),
		mineB:     seedTenant(t, pool, "share-mine-b-"+suffix, 0),
		regA:      "REG-A-" + suffix,
		regB:      "REG-B-" + suffix,
	}

	// Each mine has a registration number, which is what a grant is keyed on.
	for tenantID, registration := range map[string]string{
		fixture.mineA: fixture.regA,
		fixture.mineB: fixture.regB,
	} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO workspace.tenant_profiles (tenant_id, registration_number) VALUES ($1, $2)
			 ON CONFLICT (tenant_id) DO UPDATE SET registration_number = EXCLUDED.registration_number`,
			tenantID, registration); err != nil {
			t.Fatalf("set a registration number: %v", err)
		}
	}

	// The transport company's own rows: two trips for mine A, one for mine B.
	for _, trip := range []struct {
		ref    string
		route  string
		tonnes float64
	}{
		{fixture.regA, "Багануур → Улаанбаатар", 40},
		{fixture.regA, "Багануур → Дархан", 60},
		{fixture.regB, "Таван толгой → Гашуунсухайт", 100},
	} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO reporting_test_trips (tenant_id, counterparty_ref, route, tonnes)
			 VALUES ($1, $2, $3, $4)`,
			fixture.transport, trip.ref, trip.route, trip.tonnes); err != nil {
			t.Fatalf("seed a trip: %v", err)
		}
	}

	return fixture
}

// grant inserts an accepted agreement and returns its id.
func (f sharingFixture) grant(t *testing.T, granteeTenantID, scope, counterpartyRef string) string {
	t.Helper()
	var id string
	err := f.pool.QueryRow(context.Background(), `
		INSERT INTO workspace.report_grants
		    (grantor_tenant_id, grantee_tenant_id, report_key, scope, counterparty_ref, accepted_at)
		VALUES ($1, $2, 'test.trips', $3, $4, NOW())
		RETURNING id`,
		f.transport, granteeTenantID, scope, counterpartyRef).Scan(&id)
	if err != nil {
		t.Fatalf("create a grant: %v", err)
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM workspace.report_grants WHERE id = $1`, id)
	})
	return id
}

//  1. Default deny. Without a grant, a consolidated run returns nothing at all —
//     not an error, not a partial answer, nothing.
func TestConsolidatedShowsNothingWithoutAGrant(t *testing.T) {
	fixture := newSharingFixture(t)
	engine := reporting.NewEngine(fixture.pool)

	report := tripFixture{}
	params, _ := reporting.Bind(report, map[string]string{}, "mn")

	result, err := engine.RunConsolidated(context.Background(), fixture.mineA, report, params, "")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(result.Rows) != 0 {
		t.Fatalf("a mine with no agreement saw %d rows", len(result.Rows))
	}
	if len(result.Notes) == 0 {
		t.Error("an empty consolidated result should say why it is empty")
	}
}

// 2. Revoking closes it immediately — the next run, not the next day.
func TestRevokingAGrantClosesItAtOnce(t *testing.T) {
	fixture := newSharingFixture(t)
	engine := reporting.NewEngine(fixture.pool)

	id := fixture.grant(t, fixture.mineA, reporting.ScopeCounterparty, fixture.regA)

	report := tripFixture{}
	params, _ := reporting.Bind(report, map[string]string{}, "mn")

	before, err := engine.RunConsolidated(context.Background(), fixture.mineA, report, params, "")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(before.Rows) != 2 {
		t.Fatalf("expected the two trips for this mine, got %d", len(before.Rows))
	}

	if _, err := fixture.pool.Exec(context.Background(),
		`UPDATE workspace.report_grants SET revoked_at = NOW() WHERE id = $1`, id); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	after, err := engine.RunConsolidated(context.Background(), fixture.mineA, report, params, "")
	if err != nil {
		t.Fatalf("run after revoking: %v", err)
	}
	if len(after.Rows) != 0 {
		t.Fatalf("a revoked agreement still returned %d rows", len(after.Rows))
	}
}

//  3. Counterparty scope means this mine's rows and no other mine's. It is the
//     principle the whole feature rests on: the transport company agreed to show
//     the work it did *for this mine*, not its business.
func TestCounterpartyScopeExcludesAnotherMinesRows(t *testing.T) {
	fixture := newSharingFixture(t)
	engine := reporting.NewEngine(fixture.pool)

	fixture.grant(t, fixture.mineA, reporting.ScopeCounterparty, fixture.regA)

	report := tripFixture{}
	params, _ := reporting.Bind(report, map[string]string{}, "mn")

	result, err := engine.RunConsolidated(context.Background(), fixture.mineA, report, params, "")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("expected two rows, got %d", len(result.Rows))
	}
	for _, row := range result.Rows {
		if row["route"] == "Таван толгой → Гашуунсухайт" {
			t.Fatal("counterparty scope returned another mine's trip")
		}
	}
	// 40 + 60, and not the 100 that belongs to the other mine.
	if result.Totals["tonnes"] != 100 {
		t.Fatalf("the total is %v; the other mine's 100 tonnes should be absent", result.Totals["tonnes"])
	}
}

//  4. Both sides are told. The reader's organisation records that it read; the
//     owner's records that it was read — which is what lets the transport
//     company answer "who has seen our data".
func TestConsolidatedRunIsAuditedOnBothSides(t *testing.T) {
	fixture := newSharingFixture(t)
	engine := reporting.NewEngine(fixture.pool)

	audit.UseDatabase(fixture.pool)
	t.Cleanup(func() { audit.UseDatabase(nil) })

	fixture.grant(t, fixture.mineA, reporting.ScopeCounterparty, fixture.regA)

	report := tripFixture{}
	params, _ := reporting.Bind(report, map[string]string{}, "mn")

	if _, err := engine.RunConsolidated(context.Background(), fixture.mineA, report, params, ""); err != nil {
		t.Fatalf("run: %v", err)
	}

	// The write is best effort and asynchronous only in the sense that it is
	// bounded; it is done by the time RunConsolidated returns. Read it back
	// directly rather than sleeping.
	assertAudited(t, fixture.pool, fixture.mineA, "reports.consolidated_read")
	assertAudited(t, fixture.pool, fixture.transport, "reports.data_shared")
}

func assertAudited(t *testing.T, pool *pgxpool.Pool, tenantID, action string) {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM workspace.audit_events
		  WHERE tenant_id = $1 AND action = $2 AND created_at > NOW() - interval '1 minute'`,
		tenantID, action).Scan(&count)
	if err != nil {
		t.Fatalf("read the audit trail: %v", err)
	}
	if count == 0 {
		t.Fatalf("%s was not recorded for %s", action, tenantID)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM workspace.audit_events WHERE tenant_id = $1 AND action = $2`, tenantID, action)
	})
}

// A report that has not opted in cannot be consolidated at all, whatever rows
// a grant may name.
func TestAReportThatIsNotShareableCannotBeConsolidated(t *testing.T) {
	fixture := newSharingFixture(t)
	engine := reporting.NewEngine(fixture.pool)

	report := revenueFixture{} // no Scopes method
	params, _ := reporting.Bind(report, map[string]string{}, "mn")

	if _, err := engine.RunConsolidated(context.Background(), fixture.mineA, report, params, ""); err == nil {
		t.Fatal("a report that never opted in to sharing was consolidated")
	}
}

// An expired agreement stops working on its own, without anybody revoking it.
func TestAnExpiredGrantStopsWorking(t *testing.T) {
	fixture := newSharingFixture(t)
	engine := reporting.NewEngine(fixture.pool)

	id := fixture.grant(t, fixture.mineA, reporting.ScopeCounterparty, fixture.regA)
	if _, err := fixture.pool.Exec(context.Background(),
		`UPDATE workspace.report_grants SET valid_until = $2 WHERE id = $1`,
		id, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("expire the grant: %v", err)
	}

	report := tripFixture{}
	params, _ := reporting.Bind(report, map[string]string{}, "mn")

	result, err := engine.RunConsolidated(context.Background(), fixture.mineA, report, params, "")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(result.Rows) != 0 {
		t.Fatalf("an expired agreement returned %d rows", len(result.Rows))
	}
}

// A request nobody answered is not a permission.
func TestAnUnacceptedRequestGrantsNothing(t *testing.T) {
	fixture := newSharingFixture(t)
	engine := reporting.NewEngine(fixture.pool)

	var id string
	err := fixture.pool.QueryRow(context.Background(), `
		INSERT INTO workspace.report_grants
		    (grantor_tenant_id, grantee_tenant_id, report_key, scope, counterparty_ref)
		VALUES ($1, $2, 'test.trips', 'counterparty', $3)
		RETURNING id`, fixture.transport, fixture.mineA, fixture.regA).Scan(&id)
	if err != nil {
		t.Fatalf("create the request: %v", err)
	}
	t.Cleanup(func() {
		_, _ = fixture.pool.Exec(context.Background(), `DELETE FROM workspace.report_grants WHERE id = $1`, id)
	})

	report := tripFixture{}
	params, _ := reporting.Bind(report, map[string]string{}, "mn")

	result, err := engine.RunConsolidated(context.Background(), fixture.mineA, report, params, "")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(result.Rows) != 0 {
		t.Fatalf("an unanswered request returned %d rows", len(result.Rows))
	}
}
