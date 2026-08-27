/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import (
	"context"
	"sync"
	"time"
)

// The contract a report implements.
//
// A report is not a screen and not a query — it is a declaration. It says what
// it is called in every language the platform speaks, what parameters it will
// accept, what columns it produces, and how to produce them. Everything else —
// the list screen, the parameter form, the table, the chart, the Excel export,
// the schedule that mails it out, the audit entry recording that somebody ran
// it — is written once in the platform and applies to every report any module
// ever adds. That is the Odoo shape: a module writes an implementation and
// registers it, and writes no handler, no CSV writer and no form.
//
// This lives in the SDK rather than in the engine because a distribution's
// modules have reports too, and a contract they cannot import is a contract
// they cannot implement. The engine — parameter binding, totals, exports,
// schedules, cross-organisation grants — stays inside the platform, where it
// can change without moving the ecosystem's floor.
//
// The isolation rules are unchanged and are not enforced here. A report's Run
// receives a Querier bound to the caller's tenant, under the same row-level
// policies as every handler; a report that forgets its `WHERE tenant_id = $1`
// returns nothing rather than another organisation's numbers.

// Report is what a module implements to add a report to the platform.
type Report interface {
	// Key identifies the report everywhere: in the API, in a schedule row, in
	// a grant. Dotted, stable, and never reused for something else —
	// "billing.revenue_by_month".
	Key() string

	// App is the module id this report belongs to, e.g.
	// "io.gerege.nexus.billing". A tenant that has not installed that app does
	// not see the report at all.
	App() string

	// Titles is the report's name per locale. "mn" is required; the resolver
	// falls back to "en" and then to the key.
	Titles() map[string]string

	// Params declares what the caller may pass. Anything not declared is
	// rejected before Run is reached.
	Params() []ParamSpec

	// Columns describes the result, for the table, the chart and the export
	// header. Run must return rows matching it.
	Columns() []ColumnSpec

	// Run executes the report. It must use only the Querier it is handed, and
	// only the parameters in p.
	Run(ctx context.Context, q Querier, p Params) (Result, error)
}

// Querier is the read surface a report gets. Deliberately narrower than
// pgxpool.Pool: a report reads. It cannot Exec, cannot open a transaction and
// cannot reach for a connection of its own — which would be a connection
// outside the tenant binding.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
}

// Rows is the cursor a report iterates. It mirrors the part of pgx.Rows a
// report needs, so that reports do not depend on the driver.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// ParamKind is how a parameter is rendered and validated.
type ParamKind string

const (
	// ParamDateRange is a from/to pair. It arrives as two keys — `<key>_from`
	// and `<key>_to` — and is the parameter almost every report has.
	ParamDateRange ParamKind = "date_range"
	// ParamUUID is a reference to a row the tenant owns: a warehouse, a
	// contact. Options are filled by OptionsQuery.
	ParamUUID ParamKind = "uuid"
	// ParamSelect is a closed list declared in code.
	ParamSelect ParamKind = "select"
	// ParamText is free text. Always passed as a query parameter, never
	// interpolated — see params.go.
	ParamText ParamKind = "text"
	// ParamBool is a checkbox.
	ParamBool ParamKind = "bool"
)

// ParamSpec declares one parameter.
type ParamSpec struct {
	Key      string            `json:"key"`
	Kind     ParamKind         `json:"kind"`
	Titles   map[string]string `json:"titles"`
	Required bool              `json:"required"`
	// Options is the closed list for ParamSelect.
	Options []ParamOption `json:"options,omitempty"`
	// OptionsQuery is the SQL that fills a ParamUUID's dropdown. It must select
	// exactly two columns, id and label, and it runs under the caller's tenant
	// binding like everything else.
	OptionsQuery string `json:"-"`
	// Default is used when the caller omits the parameter. For a date range it
	// is a duration back from today, e.g. 30 * 24 * time.Hour.
	Default       any           `json:"default,omitempty"`
	DefaultWindow time.Duration `json:"-"`
}

// ParamOption is one entry of a ParamSelect.
type ParamOption struct {
	Value  string            `json:"value"`
	Titles map[string]string `json:"titles"`
}

// ColumnKind decides formatting in the table, in the chart and in the export.
type ColumnKind string

const (
	ColumnText    ColumnKind = "text"
	ColumnNumber  ColumnKind = "number"
	ColumnMoney   ColumnKind = "money"
	ColumnDate    ColumnKind = "date"
	ColumnMonth   ColumnKind = "month"
	ColumnPercent ColumnKind = "percent"
)

// ChartRole is the hint the frontend uses to draw a report without being told
// about it. A report with exactly one Category column and at least one Value
// column gets a chart; anything else gets a table only.
type ChartRole string

const (
	ChartNone     ChartRole = ""
	ChartCategory ChartRole = "category"
	ChartValue    ChartRole = "value"
)

// ColumnSpec describes one output column.
type ColumnSpec struct {
	Key    string            `json:"key"`
	Titles map[string]string `json:"titles"`
	Kind   ColumnKind        `json:"kind"`
	Chart  ChartRole         `json:"chart,omitempty"`
	// Total asks the engine to sum this column across the rows and put the
	// figure in Result.Totals. Only meaningful for number and money.
	Total bool `json:"total,omitempty"`
}

// Result is what a report returns.
type Result struct {
	Columns []ColumnSpec     `json:"columns"`
	Rows    []map[string]any `json:"rows"`
	// Totals holds the sums for columns declared with Total. Computed by the
	// engine rather than by each report, so a report cannot return a total that
	// disagrees with its own rows.
	Totals map[string]float64 `json:"totals,omitempty"`
	// Notes carries anything the reader has to know to read the numbers
	// correctly — most importantly, in a consolidated run, which organisations
	// failed and are therefore missing from the totals.
	Notes []Note `json:"notes,omitempty"`
}

// Note is a per-result remark, in the caller's locale where the engine writes
// it and in Mongolian where a report does.
type Note struct {
	Level   string `json:"level"` // "info" | "warning"
	Message string `json:"message"`
}

// LocalizedTitle resolves a title map against a locale, falling back to
// Mongolian, then English, then the fallback string.
//
// Mongolian first because it is this platform's source language: a report added
// with only `mn` filled in is readable to the people it was written for, and a
// missing translation should not make a report anonymous.
func LocalizedTitle(titles map[string]string, locale, fallback string) string {
	if title, ok := titles[locale]; ok && title != "" {
		return title
	}
	if title, ok := titles["mn"]; ok && title != "" {
		return title
	}
	if title, ok := titles["en"]; ok && title != "" {
		return title
	}
	return fallback
}

// Params is a validated parameter set.
//
// It is built by Bind, which is the only constructor: a report is handed
// Params it cannot have been given raw request data through. Every accessor
// returns a typed value, and every value went through the validation for the
// kind its spec declared — so a report's SQL takes p.UUID("warehouse_id") as a
// query argument and there is no path by which that is a string a caller chose.
type Params struct {
	values map[string]any
	locale string
}

// Locale is the caller's language, for a report that formats something itself.
func (p Params) Locale() string {
	if p.locale == "" {
		return "mn"
	}
	return p.locale
}

// Time returns a date parameter. A date range declared as `period` is read as
// Time("period_from") and Time("period_to").
func (p Params) Time(key string) time.Time {
	value, _ := p.values[key].(time.Time)
	return value
}

// UUID returns a reference parameter, or the empty string when it was not
// given. A report treats empty as "all", which is what an unset dropdown means.
func (p Params) UUID(key string) string {
	value, _ := p.values[key].(string)
	return value
}

// String returns a select or text parameter.
func (p Params) String(key string) string {
	value, _ := p.values[key].(string)
	return value
}

// Bool returns a checkbox parameter.
func (p Params) Bool(key string) bool {
	value, _ := p.values[key].(bool)
	return value
}

// Raw is the validated set, for the engine and for a schedule row. Not for
// reports — a report that reaches for this is reaching around its own
// declaration.
func (p Params) Raw() map[string]any {
	copied := make(map[string]any, len(p.values))
	for key, value := range p.values {
		copied[key] = value
	}
	return copied
}

// reportSink is where RegisterReport delivers, and pending is what it holds
// until somebody is listening.
//
// The buffer is the difference between this and UseAuditSink, which drops when
// unsunk. A dropped audit line is a gap in a log; a dropped report is a feature
// that silently does not exist — no error, no empty list, just a report nobody
// can find and no way to tell that from "not written yet". Buffering makes the
// order in which a distribution's main() does things stop mattering.
// ReportSink is what a Report is delivered to. Named, where it used to be a
// bare func(Report), because a capability is looked up by its type and an
// unnamed one is the same type as every other func(Report) in the ecosystem.
type ReportSink func(Report)

var (
	reportMu      sync.Mutex
	pendingReport []Report
)

// UseReportSink installs the engine's registry.
//
// Deprecated: use Provide[ReportSink] instead. This is a wrapper over it and
// behaves identically, including the delivery of anything registered before it.
// It stays for one major version so a distribution pinned to v1 keeps
// compiling, and goes in v2 — see docs/RELEASING.md.
func UseReportSink(sink func(Report)) { Provide[ReportSink](sink) }

// deliverBufferedReports hands the sink everything registered before it arrived.
//
// Called by Provide through the hook table in capability.go rather than from
// inside Provide itself, which must not know one capability from another. The
// buffer and the thing that drains it stay in this file.
func deliverBufferedReports(sink ReportSink) {
	reportMu.Lock()
	queued := pendingReport
	pendingReport = nil
	reportMu.Unlock()

	for _, report := range queued {
		sink(report)
	}
}

// RegisterReport adds a report to the platform. A module calls this from its
// constructor, the way it calls Register for itself.
func RegisterReport(report Report) {
	// The lock is held across the lookup so that a sink arriving between the
	// two cannot leave this report both unbuffered and undelivered.
	reportMu.Lock()
	sink, err := Capability[ReportSink]()
	if err != nil {
		pendingReport = append(pendingReport, report)
	}
	reportMu.Unlock()

	if err == nil {
		sink(report)
	}
}

// Collect drains a report's rows through the scan function it supplies.
//
// The scan closure returns one row as a map because a report's columns are
// declared rather than typed — the alternative is every report writing the same
// six lines of Next/Scan/Err/Close and one of them forgetting Err, which turns
// a database error into a short report that looks like a true answer.
func Collect(rows Rows, scan func() (map[string]any, error)) ([]map[string]any, error) {
	defer rows.Close()
	collected := make([]map[string]any, 0, 64)
	for rows.Next() {
		row, err := scan()
		if err != nil {
			return nil, err
		}
		collected = append(collected, row)
	}
	return collected, rows.Err()
}

// ReportCounterpartyKey carries the organisation a consolidated run is
// currently reading, for a report that declares it supports that scope.
const ReportCounterpartyKey = "__counterparty"

// Counterparty is the organisation whose rows this run is reading, empty on an
// ordinary run.
//
// A report that declares support for the counterparty scope must apply this in
// its own SQL. Nothing can check that it did — the filter lives inside the
// report's query — which is why the scope is opt-in and asserted per report
// rather than assumed.
func (p Params) Counterparty() string {
	value, _ := p.values[ReportCounterpartyKey].(string)
	return value
}

// NewParams builds a parameter set. The platform's binder is the caller that
// matters: it validates raw request data against each ParamSpec before getting
// here, which is why a report can hand p.UUID("warehouse_id") straight to SQL.
//
// It is exported because Params now lives here and the binder does not. That is
// weaker than an unexported constructor and worth being honest about — a module
// could build its own. It buys nothing: a report already writes its own SQL,
// and what keeps it inside its organisation is the tenant-bound Querier, not
// the provenance of the parameters.
func NewParams(values map[string]any, locale string) Params {
	return Params{values: values, locale: locale}
}

// WorkspaceOf is WorkspaceID for a report, with the error dropped.
//
// A report reads; if the context carries no organisation the query it builds
// filters on the empty string and returns nothing, which is the right answer
// and the one the platform's own version has always given. The two-value form
// belongs in a handler, which has somewhere to send a 400.
func WorkspaceOf(ctx context.Context) string {
	workspaceID, _ := WorkspaceID(ctx)
	return workspaceID
}

// The grants a report may opt into, named here so a module can say which of
// them it supports.
//
// ReportScopeFull was declared and its twin was not, which left half the
// vocabulary inside internal/workspace/reporting: a module could say "full" and
// could not say "counterparty", and could not say "not counterparty" either.
// Declaring an interface in internal types makes it an internal interface
// however carefully it was inverted (see meetings.go); declaring half its
// vocabulary there does the same to the half that is missing.
const (
	// ReportScopeFull is the hierarchical case: a parent organisation
	// consolidating its subordinates' rows.
	ReportScopeFull = "full"
	// ReportScopeCounterparty is the contracted-parties case: one organisation
	// seeing only the rows that name it. A report with nothing in it that
	// identifies a counterparty must not offer this — a grant asking for it
	// would quietly become a view of everything.
	ReportScopeCounterparty = "counterparty"
)
