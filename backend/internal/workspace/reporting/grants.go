/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Cross-tenant reporting: the grant, and running a report inside somebody
 * else's organisation with their permission.
 */

package reporting

import (
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"

	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/jackc/pgx/v5"
)

// Grant scopes.
const (
	// ScopeCounterparty is the contracted-parties case: a mine seeing the
	// transport company's rows *that relate to the mine*, and nothing else.
	// The report is handed the counterparty reference and filters on it.
	ScopeCounterparty = nexus.ReportScopeCounterparty
	// ScopeFull is the hierarchical case: a parent organisation consolidating
	// a subsidiary it owns. The whole report, unfiltered.
	ScopeFull = nexus.ReportScopeFull
)

// Shareable is the opt-in a report makes before it can be granted at all.
//
// A marker rather than a flag on Report, because sharing is a property a report
// has to be written for: a report becomes cross-tenant readable, and one that
// was written assuming a single organisation may aggregate in a way that leaks
// more than its rows do — a count of one is a fact about one row.
//
// Default deny in the type system as well as in the database: a report that
// does not implement this cannot be named in a grant, and the grant endpoint
// refuses it.
type Shareable interface {
	Report

	// Scopes lists what this report can honour. A report whose data has no
	// notion of a counterparty returns only ScopeFull — and a grant asking for
	// counterparty scope on it is refused, rather than silently becoming a full
	// one.
	Scopes() []string
}

// SupportsScope reports whether a report may be granted with the given scope.
func SupportsScope(report Report, scope string) bool {
	shareable, ok := report.(Shareable)
	if !ok {
		return false
	}
	for _, supported := range shareable.Scopes() {
		if supported == scope {
			return true
		}
	}
	return false
}

// Counterparty is the reference the report must filter on, empty in an ordinary
// run.
//
// A report that declares ScopeCounterparty must apply it. There is no way for
// the engine to check that it did — the filter lives inside the report's own
// SQL — so it is stated here, tested per report, and is the reason the scope is
// opt-in rather than assumed.
const counterpartyKey = nexus.ReportCounterpartyKey

// withCounterparty returns a copy of the parameters carrying the reference.
func withCounterparty(p Params, ref string) Params {
	values := p.Raw()
	values[counterpartyKey] = ref
	return nexus.NewParams(values, p.Locale())
}

// Grant is one live permission, as the engine reads it.
type Grant struct {
	ID              string
	GrantorTenantID string
	GrantorName     string
	Scope           string
	CounterpartyRef string
}

// ActiveGrants finds who has agreed to show this report to this organisation.
//
// The filter is the whole security model in one WHERE clause, so each line is
// deliberate: not revoked, accepted by the grantor (a request nobody answered
// is not a permission), within its validity window, and for this exact report
// key. Anything missing from that list is a way for a lapsed agreement to keep
// returning data.
//
// It runs on the platform path — outside the row-level policies — because it
// deliberately reads rows belonging to other organisations. That is safe here
// and nowhere else: the query is fixed, the grantee is not taken from a
// request, and what comes back is only the grants naming them.
func ActiveGrants(ctx context.Context, db Queryer, granteeTenantID, reportKey string) ([]Grant, error) {
	rows, err := db.Query(nexus.WithoutTenant(ctx), `
		SELECT g.id, g.grantor_tenant_id, t.name, g.scope, g.counterparty_ref
		  FROM workspace.report_grants g
		  JOIN registry.tenants t ON t.id = g.grantor_tenant_id
		 WHERE g.grantee_tenant_id = $1
		   AND g.report_key = $2
		   AND g.revoked_at IS NULL
		   AND g.accepted_at IS NOT NULL
		   AND g.valid_from <= NOW()
		   AND (g.valid_until IS NULL OR g.valid_until > NOW())
		 ORDER BY t.name`, granteeTenantID, reportKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	grants := make([]Grant, 0, 8)
	for rows.Next() {
		var grant Grant
		if err := rows.Scan(&grant.ID, &grant.GrantorTenantID, &grant.GrantorName,
			&grant.Scope, &grant.CounterpartyRef); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}

// Queryer is the narrow read surface the grant functions need.
type Queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// OrganisationColumn is prepended to a consolidated result. It is not declared
// by any report: a report knows nothing about being consolidated, and the
// column belongs to the consolidation rather than to it.
func OrganisationColumn() ColumnSpec {
	return ColumnSpec{
		Key:   "__organisation",
		Kind:  ColumnText,
		Chart: ChartCategory,
		Titles: map[string]string{
			"mn": "Байгууллага", "en": "Organisation", "ru": "Организация",
			"zh": "组织", "fr": "Organisation", "es": "Organización", "ar": "المؤسسة",
		},
	}
}

// RunConsolidated runs one report across every organisation that has granted it
// to this one, and merges the answers.
//
// The mechanism is the one thing about §3.5 that has to be got right: each
// grantor's rows are produced by running the ordinary report **inside that
// grantor's own tenant context**. No policy is relaxed, no clause is rewritten,
// no query reads across organisations, and the report cannot tell it is being
// consolidated except by the counterparty reference it was handed.
//
// One grantor failing does not fail the run. A hundred transport companies is a
// hundred chances for one slow query, and a consolidated report that refuses to
// produce anything because the ninety-seventh organisation timed out is a
// report nobody can use. The failure is named in the result instead — a total
// that is quietly missing a company is worse than one that says so.
func (e *Engine) RunConsolidated(ctx context.Context, granteeTenantID string,
	report Report, params Params, actorUserID string) (Result, error) {

	if !SupportsScope(report, ScopeCounterparty) && !SupportsScope(report, ScopeFull) {
		return Result{}, fmt.Errorf("report %q cannot be shared across organisations", report.Key())
	}

	grants, err := ActiveGrants(ctx, e.db, granteeTenantID, report.Key())
	if err != nil {
		return Result{}, fmt.Errorf("read the grants: %w", err)
	}

	merged := Result{
		Columns: append([]ColumnSpec{OrganisationColumn()}, report.Columns()...),
		Rows:    make([]map[string]any, 0, 64),
	}

	if len(grants) == 0 {
		// Not an error. Nobody has agreed to share this report, which is the
		// default state and the correct answer — an empty table with a note,
		// rather than a failure somebody reads as a bug.
		merged.Notes = append(merged.Notes, Note{
			Level:   "info",
			Message: "Танд энэ тайланг хуваалцсан байгууллага алга.",
		})
		merged.Totals = computeTotals(merged)
		return merged, nil
	}

	for _, grant := range grants {
		rows, err := e.runOneGrant(ctx, grant, report, params)
		if err != nil {
			// Named, not swallowed. Whoever reads the total has to know which
			// organisation is missing from it.
			merged.Notes = append(merged.Notes, Note{
				Level:   "warning",
				Message: grant.GrantorName + ": тайлан ажиллуулж чадсангүй (алдаатай)",
			})
			slog.Error("reports: a grantor failed inside a consolidated run",
				"report", report.Key(), "grantor", grant.GrantorTenantID, "error", err)
			// Both sides are told, including about the failure: an access
			// attempt is an access attempt whether or not it returned rows.
			e.auditBothSides(ctx, grant, granteeTenantID, report.Key(), actorUserID, 0, err)
			continue
		}

		for _, row := range rows {
			row["__organisation"] = grant.GrantorName
			merged.Rows = append(merged.Rows, row)
		}
		e.auditBothSides(ctx, grant, granteeTenantID, report.Key(), actorUserID, len(rows), nil)
	}

	merged.Totals = computeTotals(merged)
	return merged, nil
}

// runOneGrant produces one organisation's rows.
func (e *Engine) runOneGrant(ctx context.Context, grant Grant, report Report, params Params) ([]map[string]any, error) {
	scoped := params
	if grant.Scope == ScopeCounterparty {
		if !SupportsScope(report, ScopeCounterparty) {
			// A grant stored before the report stopped supporting the scope.
			// Refused rather than downgraded to a full read, which would hand
			// over every row the agreement never covered.
			return nil, errors.New("this report no longer supports counterparty scope")
		}
		if grant.CounterpartyRef == "" {
			return nil, errors.New("the grant has no counterparty reference")
		}
		scoped = withCounterparty(params, grant.CounterpartyRef)
	} else if !SupportsScope(report, ScopeFull) {
		return nil, errors.New("this report does not support full scope")
	}

	// The tenant here is the grantor's. Everything below this line — the
	// connection binding, the row-level policy, the report's own WHERE clause —
	// is the grantor's own, unchanged.
	result, err := e.Run(ctx, grant.GrantorTenantID, report, scoped)
	if err != nil {
		return nil, err
	}
	return result.Rows, nil
}

// auditBothSides records one grantor's part of a consolidated run, twice.
//
// §3.5's fifth principle, and the reason the data owner will agree to this at
// all: the transport company can open "who read my data" and see the mine, the
// report, the moment and how many rows. One record on the reader's side alone
// would be a log the person being read cannot see.
func (e *Engine) auditBothSides(ctx context.Context, grant Grant, granteeTenantID,
	reportKey, actorUserID string, rows int, failure error) {

	outcome := "ok"
	if failure != nil {
		outcome = "failed"
	}
	details := map[string]any{
		"grant_id": grant.ID,
		"scope":    grant.Scope,
		"rows":     rows,
		"outcome":  outcome,
	}

	// The reader's side: "we read this organisation's data".
	granteeDetails := cloneDetails(details)
	granteeDetails["grantor_tenant_id"] = grant.GrantorTenantID
	granteeDetails["grantor_name"] = grant.GrantorName
	audit.Record(nexus.WithTenantID(ctx, granteeTenantID), granteeTenantID, actorUserID,
		"reports.consolidated_read", reportKey, granteeDetails)

	// The owner's side: "this organisation read ours". The actor is not named
	// as a user id here — they are not a person this organisation has, and a
	// user id from another tenant on their audit screen names somebody they
	// cannot look up.
	grantorDetails := cloneDetails(details)
	grantorDetails["grantee_tenant_id"] = granteeTenantID
	audit.Record(nexus.WithTenantID(ctx, grant.GrantorTenantID), grant.GrantorTenantID, "",
		"reports.data_shared", reportKey, grantorDetails)
}

func cloneDetails(details map[string]any) map[string]any {
	copied := make(map[string]any, len(details)+2)
	for key, value := range details {
		copied[key] = value
	}
	return copied
}

// GrantRow is a grant as the API returns it, from either side.
type GrantRow struct {
	ID              string     `json:"id"`
	ReportKey       string     `json:"report_key"`
	GrantorTenantID string     `json:"grantor_tenant_id"`
	GrantorName     string     `json:"grantor_name"`
	GranteeTenantID string     `json:"grantee_tenant_id"`
	GranteeName     string     `json:"grantee_name"`
	Scope           string     `json:"scope"`
	CounterpartyRef string     `json:"counterparty_ref,omitempty"`
	ValidFrom       time.Time  `json:"valid_from"`
	ValidUntil      *time.Time `json:"valid_until,omitempty"`
	RevokedAt       *time.Time `json:"revoked_at,omitempty"`
	AcceptedAt      *time.Time `json:"accepted_at,omitempty"`
	Note            string     `json:"note,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	// Direction says which side of this grant the caller is on, so one screen
	// can show "requests we received" and "access we were given" from one list.
	Direction string            `json:"direction"`
	Titles    map[string]string `json:"titles,omitempty"`
}

// ListGrants returns every grant this organisation is a party to, in either
// direction.
//
// The two-sided row-level policy on report_grants is what makes this one query
// rather than two: a tenant sees the rows where it is the grantor and the rows
// where it is the grantee, and nothing else. The `WHERE` below repeats it, for
// the same reason every query in this codebase repeats its tenant clause.
func ListGrants(ctx context.Context, db Queryer, tenantID string) ([]GrantRow, error) {
	rows, err := db.Query(nexus.WithTenantID(ctx, tenantID), `
		SELECT g.id, g.report_key,
		       g.grantor_tenant_id, grantor.name,
		       g.grantee_tenant_id, grantee.name,
		       g.scope, g.counterparty_ref,
		       g.valid_from, g.valid_until, g.revoked_at, g.accepted_at,
		       g.note, g.created_at
		  FROM workspace.report_grants g
		  JOIN registry.tenants grantor ON grantor.id = g.grantor_tenant_id
		  JOIN registry.tenants grantee ON grantee.id = g.grantee_tenant_id
		 WHERE g.grantor_tenant_id = $1 OR g.grantee_tenant_id = $1
		 ORDER BY g.created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	grants := make([]GrantRow, 0, 16)
	for rows.Next() {
		var row GrantRow
		if err := rows.Scan(&row.ID, &row.ReportKey,
			&row.GrantorTenantID, &row.GrantorName,
			&row.GranteeTenantID, &row.GranteeName,
			&row.Scope, &row.CounterpartyRef,
			&row.ValidFrom, &row.ValidUntil, &row.RevokedAt, &row.AcceptedAt,
			&row.Note, &row.CreatedAt); err != nil {
			return nil, err
		}
		row.Direction = "received"
		if row.GrantorTenantID == tenantID {
			row.Direction = "given"
		}
		if report, ok := Get(row.ReportKey); ok {
			row.Titles = report.Titles()
		}
		grants = append(grants, row)
	}
	return grants, rows.Err()
}
