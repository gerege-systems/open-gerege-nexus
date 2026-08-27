/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Running a report: the timeout, the tenant, the totals.
 */

package reporting

import (
	"context"
	"fmt"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/jackc/pgx/v5"
)

// statementTimeout is the ceiling on one report's query.
//
// Reporting queries aggregate across the tables the platform is also serving
// requests from, and the failure mode is not a slow report — it is a report
// holding a pool connection for four minutes while every other request waits
// for one. Thirty seconds is long enough for a year of invoices on a large
// tenant and short enough that a report nobody could have read anyway gives the
// connection back.
const statementTimeout = 30 * time.Second

// statementTimeoutSQL is the same figure, as PostgreSQL spells it. A literal
// rather than a parameter because SET LOCAL takes no parameters; it is a
// constant in this file and never reaches a request.
const statementTimeoutSQL = "'30s'"

// timeoutGrace is how much longer the context waits than the database. Without
// it the two race, and the context usually wins — which turns "your report
// exceeded the thirty-second limit", an answer somebody can act on, into a
// bare context deadline with nothing to say why.
const timeoutGrace = 5 * time.Second

// Engine runs reports against a pool.
type Engine struct {
	db nexus.DB
}

// NewEngine builds the engine. One per process, held by the reports module.
func NewEngine(db nexus.DB) *Engine { return &Engine{db: db} }

// DB exposes the pool for the parts of the module that are not report
// execution — schedules, grants, the option lists behind a parameter form.
func (e *Engine) DB() nexus.DB { return e.db }

// Run executes one report in one tenant's context.
//
// The tenant is put into the context rather than passed to the report, because
// that is what dbguard reads when the pool hands out a connection: the report's
// own SQL still carries its `WHERE tenant_id = $1`, and the row-level policy is
// the layer underneath that says so even if it does not.
//
// This is also the seam §3.5 turns on. A consolidated run calls Run once per
// grantor with that grantor's id, and every query inside is bound to them —
// no policy is relaxed, no clause is rewritten, and the report cannot tell the
// difference.
func (e *Engine) Run(ctx context.Context, tenantID string, report Report, params Params) (Result, error) {
	// Two deadlines, and both are wanted. The context one stops this process
	// waiting; the SET LOCAL below stops PostgreSQL *working*. Cancelling a
	// context sends a cancel request the server may take its time acting on,
	// and a report that has already been abandoned should not still be burning
	// a CPU on the machine serving everybody else.
	runCtx, cancel := context.WithTimeout(nexus.WithTenantID(ctx, tenantID),
		statementTimeout+timeoutGrace)
	defer cancel()

	// A read-only transaction. The timeout has to be SET LOCAL, which requires
	// one — and read-only is the honest mode besides: the Querier a report gets
	// cannot Exec, and this is the layer that says so to the database rather
	// than to the compiler.
	tx, err := e.db.BeginTx(runCtx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return Result{}, fmt.Errorf("open the reporting transaction: %w", err)
	}
	// Always rolled back. Nothing here writes, so there is nothing to commit,
	// and a rollback returns the connection without waiting on a flush.
	defer func() { _ = tx.Rollback(context.WithoutCancel(runCtx)) }()

	if _, err := tx.Exec(runCtx, "SET LOCAL statement_timeout = "+statementTimeoutSQL); err != nil {
		return Result{}, fmt.Errorf("bound the reporting query: %w", err)
	}

	result, err := report.Run(runCtx, &txQuerier{tx: tx}, params)
	if err != nil {
		return Result{}, err
	}

	result.Columns = report.Columns()
	result.Totals = computeTotals(result)
	return result, nil
}

// TenantOf is the organisation the report is running for.
//
// Every report's SQL passes it as `$1` against its own `WHERE tenant_id = $1`.
// That looks redundant beside the row-level policy that would filter the rows
// anyway, and it is not: the application filter has always been the primary
// control here and the policy is the layer underneath it (see
// internal/kernel/dbguard). Taking it from the context rather than from a
// parameter is what makes a consolidated run possible without a report knowing
// it is in one.
func TenantOf(ctx context.Context) string { return nexus.TenantOf(ctx) }

// computeTotals sums the columns that asked for it.
//
// Done here rather than in each report's SQL, so a report cannot return a total
// that disagrees with the rows above it — which is the single most common way a
// report loses the reader's trust, and the hardest to notice in review.
func computeTotals(result Result) map[string]float64 {
	var wanted []string
	for _, column := range result.Columns {
		if column.Total {
			wanted = append(wanted, column.Key)
		}
	}
	if len(wanted) == 0 {
		return nil
	}

	totals := make(map[string]float64, len(wanted))
	for _, key := range wanted {
		totals[key] = 0
	}
	for _, row := range result.Rows {
		for _, key := range wanted {
			totals[key] += asFloat(row[key])
		}
	}
	return totals
}

func asFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

// txQuerier is the Querier a report is handed. It carries no tenant of its own:
// the binding is in the context, which is where dbguard reads it when the
// transaction's connection is acquired.
type txQuerier struct {
	tx pgx.Tx
}

func (q *txQuerier) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	rows, err := q.tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgxRows{rows}, nil
}

type pgxRows struct{ pgx.Rows }

func (r pgxRows) Close() { r.Rows.Close() }
