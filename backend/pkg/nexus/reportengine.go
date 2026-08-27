/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import "context"

// Running a report, as the app that shows them sees it.
//
// The engine stays the platform's: it binds parameters, opens a read-only
// transaction bound to the caller's organisation, runs the SQL a report
// declared, sums the columns marked total and renders a spreadsheet. None of
// that is a decision an app makes, and CORE_BOUNDARY_PLAN §4.2 says so —
// the engine is core, the screens are an app.
//
// What the app was doing instead was calling fifteen functions of the engine's
// package directly: Get, Bind, Run, Export, Filename, ParseFormat, ParseCron,
// ForApps, LocalizedTitle and the rest. Every one of them is under internal/,
// so the screens could not be built anywhere else — and publishing them as they
// stand would commit the engine's whole shape to semver so that one screen
// could move.
//
// Six methods instead, named for what the app does rather than for how the
// engine does it. Get-then-Bind-then-Run is one call here, because a caller
// that could do the three separately could do them in the wrong order.
type ReportEngine interface {
	// Available is what this tenant may run, given the apps it has installed.
	// A report belonging to an app nobody installed is not listed and cannot
	// be run: the gate is the installation, not the permission.
	Available(installed map[string]bool) []ReportDescription

	// Describe is one report, or false. Titles and scopes, no rows.
	Describe(key string) (ReportDescription, bool)

	// Form is what a screen needs to ask for a report: the parameters it takes,
	// with any dropdown already filled from this organisation's own rows, and
	// the columns it will answer with.
	//
	// Filled here rather than by the caller because a dropdown is SQL a report
	// declares — OptionsQuery — and running a report's SQL is the engine's to
	// do. What comes back never carries it: it is a statement this platform
	// runs, and a browser has no use for it.
	Form(ctx context.Context, workspaceID, key, locale string) (*ReportForm, error)

	// Run binds the parameters and produces the rows, in one call.
	//
	// The three steps are not offered separately on purpose: binding is what
	// rejects a parameter a report did not declare, and a caller that could
	// run without binding would be running a report with whatever the browser
	// sent.
	Run(ctx context.Context, workspaceID, key string, params map[string]string, locale string) (*ReportRun, error)

	// Export runs a report and renders it to a file.
	//
	// It returns the bytes with the two things a caller has to send beside
	// them — a filename and a content type — because a caller that had to
	// derive those would be deriving them from a format string the engine
	// already parsed.
	Export(ctx context.Context, workspaceID, key string, params map[string]string, locale, format string) (*ReportExport, error)

	// ValidateSchedule refuses a schedule the engine could not later run:
	// unknown report, parameters it did not declare, a cron it cannot parse,
	// a format it cannot render. It runs nothing.
	//
	// One call rather than four validators, because a schedule accepted with
	// three of the four checked is a schedule that fails at three in the
	// morning to nobody.
	ValidateSchedule(key string, params map[string]string, locale, cron, format string) error

	// RunConsolidated runs a report over every organisation that has agreed to
	// share it with this one, and its own rows beside them.
	//
	// A different act from Run and a separate method for that reason: it names
	// no counterparty because the agreements decide which rows are in reach,
	// it is audited on both sides, and a report that was not written to be
	// shared refuses it. Folding it into Run would make the safe operation and
	// the dangerous one the same call.
	RunConsolidated(ctx context.Context, workspaceID, key string, params map[string]string, locale, actorUserID string) (*ReportRun, error)

	// ValidateCron and NormalizeFormat are the two halves of a schedule that
	// are not about any particular report: whether the expression parses, and
	// what this engine will store for a format somebody typed in whatever case
	// they typed it.
	//
	// Separate from ValidateSchedule because a screen validates a field as it
	// is filled in, and asking about a whole schedule to check one field would
	// mean inventing the rest of it.
	ValidateCron(expression string) error
	NormalizeFormat(raw string) (string, error)

	// Deliverable reports whether a scheduled report can actually be sent.
	// False on a deployment with no mail configured, which a screen should say
	// before somebody schedules something that will never arrive.
	Deliverable() bool
}

// ReportDescription is a report as a catalogue entry: what it is called and
// whether it can be shared. Not how it is run.
type ReportDescription struct {
	Key    string            `json:"key"`
	App    string            `json:"app"`
	Titles map[string]string `json:"titles"`
	// Scopes are the sharing scopes this report can honour. Empty means it was
	// not written to be shared, and no grant may name it — see
	// ReportScopeFull and ReportScopeCounterparty.
	Scopes []string `json:"scopes,omitempty"`
}

// ReportForm is a report's parameters and columns, ready to render.
type ReportForm struct {
	Key     string            `json:"key"`
	App     string            `json:"app"`
	Titles  map[string]string `json:"titles"`
	Params  []ParamSpec       `json:"params"`
	Columns []ColumnSpec      `json:"columns"`
}

// ReportRun is a report's answer, with the title already resolved for the
// language it was asked in.
type ReportRun struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Result Result `json:"result"`
}

// ReportExport is a rendered report, ready to send.
type ReportExport struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Bytes       []byte `json:"-"`
	// Rows is what was exported, for the audit line the caller writes. An
	// export is a copy of an organisation's data leaving the platform, and the
	// count is the part of it worth recording.
	Rows int `json:"rows"`
}

// Reports returns the engine this deployment provides.
func Reports() (ReportEngine, error) { return Capability[ReportEngine]() }

// What this contract carries now, and what it deliberately does not.
//
// Three things were named here as missing on 2026-08-23, each with the reason
// it had not been written. Two are above: RunConsolidated is its own method,
// for the reason it always should have been, and the records the screens list
// — schedules and grants — are two contracts of their own in
// pkg/nexus/reportrecords.go rather than bolted onto this one.
//
// The third is not a contract at all. The sweep that mails a report at three in
// the morning ran because the app happened to start it, which made a screen
// responsible for a deployment's housekeeping; the platform starts it now, and
// a deployment with no reports app installed still delivers the schedules it
// has. That was the right change and it needed no SDK surface.
