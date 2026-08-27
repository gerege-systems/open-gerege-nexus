/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The contract a report implements, and the shapes it declares itself in.
 */

// Package reporting is the platform's reporting engine.
//
// A report is not a screen and not a query — it is a declaration. It says what
// it is called in every language the platform speaks, what parameters it will
// accept, what columns it produces, and how to produce them. Everything else —
// the list screen, the parameter form, the table, the chart, the Excel export,
// the schedule that mails it out, the audit entry recording that somebody ran
// it — is written once here and applies to every report any module ever adds.
//
// That is the Odoo shape, and it is the reason this is a platform package and
// not an app. A module that wants a report writes an implementation of Report
// and registers it in its init; it writes no handler, no CSV writer, no form.
//
// The isolation rules are unchanged. A report's Run receives a Querier bound to
// the caller's tenant, which is the same pool every handler uses, under the same
// row-level policies from migration 00029. A report that forgets its
// `WHERE tenant_id = $1` returns nothing rather than another organisation's
// numbers. Crossing that line at all is §3.5's business — see grants.go — and
// it happens by running the same query, unchanged, inside the *other* tenant's
// context.
package reporting

import (
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// The report contract moved to pkg/nexus so that a distribution's modules can
// implement it — see the ecosystem strategy §2.1. These aliases keep the engine
// and every existing report compiling against `reporting.X`, and mean there is
// one set of types rather than two that have to be kept in step.

type (
	Report      = nexus.Report
	Querier     = nexus.Querier
	Rows        = nexus.Rows
	ParamKind   = nexus.ParamKind
	ParamSpec   = nexus.ParamSpec
	ParamOption = nexus.ParamOption
	Params      = nexus.Params
	ColumnKind  = nexus.ColumnKind
	ChartRole   = nexus.ChartRole
	ColumnSpec  = nexus.ColumnSpec
	Result      = nexus.Result
	Note        = nexus.Note
)

const (
	ParamDateRange = nexus.ParamDateRange
	ParamUUID      = nexus.ParamUUID
	ParamSelect    = nexus.ParamSelect
	ParamText      = nexus.ParamText
	ParamBool      = nexus.ParamBool

	ColumnText    = nexus.ColumnText
	ColumnNumber  = nexus.ColumnNumber
	ColumnMoney   = nexus.ColumnMoney
	ColumnDate    = nexus.ColumnDate
	ColumnMonth   = nexus.ColumnMonth
	ColumnPercent = nexus.ColumnPercent

	ChartNone     = nexus.ChartNone
	ChartCategory = nexus.ChartCategory
	ChartValue    = nexus.ChartValue
)

// Collect and LocalizedTitle are the two helpers a report itself calls.
var (
	Collect        = nexus.Collect
	LocalizedTitle = nexus.LocalizedTitle
)
