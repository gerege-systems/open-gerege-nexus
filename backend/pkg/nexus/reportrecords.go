/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import (
	"context"
	"errors"
	"time"
)

// ------------------------------------------------- the engine's own records
//
// Two things about reports are kept by the platform rather than by the app that
// shows them, and both are kept because something other than a screen acts on
// them.
//
// A schedule is mailed at three in the morning by the platform's scheduler,
// with nobody present and no request to hang the work on. A grant is what lets
// one organisation's report read another's rows, and it is checked by the
// engine on every consolidated run — a permission across a tenant boundary,
// which is the thing this platform is most careful about.
//
// So the rows are the platform's and the screens are an app's, and these two
// contracts are the seam. They were the last reason internal/apps/reports could
// not leave this repository: pkg/nexus/reportengine.go named them, said they
// belonged in a contract of their own rather than bolted onto the engine, and
// left them for the day somebody wrote it. This is that day.
//
// Both records are wide — thirteen fields and sixteen — and that is honest
// rather than accidental: a screen that lists schedules shows when each last
// ran and why it failed, and a screen that lists agreements shows both parties,
// the scope, the dates and which side the reader is on. A narrower record would
// mean a second call per row for the rest.

// ReportSchedules is the platform's record of what it mails and when.
type ReportSchedules interface {
	// List is one organisation's schedules, newest first.
	List(ctx context.Context, workspaceID string) ([]ReportSchedule, error)

	// Create records a new one and returns its id.
	//
	// It does not validate: ReportEngine.ValidateSchedule is what refuses a
	// schedule the engine could not later run, and a caller that skipped it
	// would be storing something that fails at three in the morning to nobody.
	Create(ctx context.Context, workspaceID string, schedule ReportSchedule) (string, error)

	// Update replaces one, reporting whether it was this organisation's to
	// replace. False is a schedule belonging to somebody else, which is the
	// same answer as one that does not exist — deliberately.
	Update(ctx context.Context, workspaceID, id string, schedule ReportSchedule) (bool, error)

	// Delete removes one and answers with the report it named, for the audit
	// entry the caller writes, or ErrReportScheduleNotFound.
	Delete(ctx context.Context, workspaceID, id string) (reportKey string, err error)
}

// ReportSchedule is one standing instruction to mail a report.
type ReportSchedule struct {
	ID        string            `json:"id"`
	ReportKey string            `json:"report_key"`
	Name      string            `json:"name"`
	Params    map[string]string `json:"params"`
	// Cron is a five-field expression in the deployment's timezone.
	Cron       string   `json:"cron"`
	Format     string   `json:"format"`
	Recipients []string `json:"recipients"`
	Active     bool     `json:"active"`
	// The last attempt, which is what a screen shows instead of "scheduled":
	// an organisation asking why nothing arrived is asking about this.
	LastRunAt  *time.Time `json:"last_run_at,omitempty"`
	LastStatus string     `json:"last_status,omitempty"`
	LastError  string     `json:"last_error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	CreatedBy  string     `json:"created_by,omitempty"`
	// Titles is the named report's own, filled on read so that a list of
	// schedules does not need a second call per row to be readable.
	Titles map[string]string `json:"titles,omitempty"`
}

// ReportGrants is one organisation's agreements to share a report with another.
//
// Every method takes the organisation acting, and none of them takes both
// parties: which side may accept and which may revoke is the contract's
// decision rather than the caller's, because a caller that could choose would
// be a caller that could accept its own request.
type ReportGrants interface {
	// List is every agreement this organisation is on either side of.
	List(ctx context.Context, workspaceID string) ([]ReportGrant, error)

	// History is who has read this organisation's data under those agreements.
	//
	// A read of somebody else's rows is the act this whole mechanism exists to
	// govern, and an organisation is entitled to know when it happened.
	History(ctx context.Context, workspaceID string) ([]ReportGrantUse, error)

	// Request asks another organisation for a report and returns the id.
	//
	// ErrReportGrantExists when one live agreement already covers this pair
	// and report — a partial unique index, not a check, so a second request
	// cannot slip through between two callers.
	Request(ctx context.Context, grant ReportGrant) (string, error)

	// Accept is the *grantor* agreeing. It answers with the report key, or
	// ErrReportGrantNotPending — which is also the answer to a grantee trying
	// to accept their own request, because the statement is scoped to the
	// grantor.
	Accept(ctx context.Context, grantorWorkspaceID, id, actorUserID string) (reportKey string, err error)

	// Revoke may be either party. It answers with the report key and which side
	// ended it — "given" or "received" — or ErrReportGrantNotFound.
	Revoke(ctx context.Context, workspaceID, id string) (reportKey, side string, err error)

	// OrganisationByRegistration finds the organisation a request names.
	//
	// Here rather than on a directory contract because this is the only reason
	// an app is allowed to look outside its own organisation at all: asking for
	// a report names the other party by the number on their registration
	// certificate, which is public. ErrOrganisationNotFound when no
	// organisation on this deployment has it.
	OrganisationByRegistration(ctx context.Context, registration string) (workspaceID string, err error)

	// RegistrationOf is this organisation's own number, or empty when it has
	// not set one. Empty rather than an error: not having filled in a legal
	// profile is a state, not a fault.
	RegistrationOf(ctx context.Context, workspaceID string) (string, error)
}

// ReportGrant is one agreement, as both parties see it.
type ReportGrant struct {
	ID                 string `json:"id"`
	ReportKey          string `json:"report_key"`
	GrantorWorkspaceID string `json:"grantor_tenant_id"`
	GrantorName        string `json:"grantor_name,omitempty"`
	GranteeWorkspaceID string `json:"grantee_tenant_id"`
	GranteeName        string `json:"grantee_name,omitempty"`
	// Scope is ReportScopeFull or ReportScopeCounterparty. A report that was
	// not written to be shared at that scope cannot be named here.
	Scope string `json:"scope"`
	// CounterpartyRef narrows a counterparty-scoped grant to the rows that
	// mention the asking organisation, and is empty for a full-scope one.
	CounterpartyRef string     `json:"counterparty_ref,omitempty"`
	ValidFrom       time.Time  `json:"valid_from"`
	ValidUntil      *time.Time `json:"valid_until,omitempty"`
	RevokedAt       *time.Time `json:"revoked_at,omitempty"`
	AcceptedAt      *time.Time `json:"accepted_at,omitempty"`
	Note            string     `json:"note,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	CreatedBy       string     `json:"created_by,omitempty"`
	// Direction says which side of this grant the reader is on, so one screen
	// can show "requests we received" and "access we were given" from one list.
	Direction string            `json:"direction,omitempty"`
	Titles    map[string]string `json:"titles,omitempty"`
}

// ReportGrantUse is one occasion on which somebody read this organisation's
// rows under an agreement.
type ReportGrantUse struct {
	At         time.Time `json:"at"`
	ReportKey  string    `json:"report_key"`
	ReaderName string    `json:"reader_name"`
	Scope      string    `json:"scope"`
	Rows       int       `json:"rows"`
}

// What the two contracts refuse, and why each is a sentinel rather than a
// message: the app turns each into a different answer to the browser, and a
// caller that could only see "it failed" would answer 500 to somebody's typo.
var (
	// ErrReportScheduleNotFound is a schedule that is not this organisation's,
	// which reads the same as one that never existed.
	ErrReportScheduleNotFound = errors.New("nexus: no such report schedule")
	// ErrReportGrantExists is one live agreement already covering this pair and
	// report.
	ErrReportGrantExists = errors.New("nexus: an agreement for this report already exists between these organisations")
	// ErrReportGrantNotPending is an agreement that cannot be accepted: already
	// accepted, revoked, or not this organisation's to accept.
	ErrReportGrantNotPending = errors.New("nexus: no such pending report request")
	// ErrReportGrantNotFound is an agreement neither party has, or one already
	// ended.
	ErrReportGrantNotFound = errors.New("nexus: no such report agreement")
	// ErrOrganisationNotFound is a registration number no organisation on this
	// deployment carries.
	ErrOrganisationNotFound = errors.New("nexus: no organisation with that registration number")
)
