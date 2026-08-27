/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import (
	"context"
	"time"
)

// The state's registers, as a module sees them.
//
// The platform holds the ХУР client: an endpoint, a client id and secret, a
// mock mode, a token it refreshes. A module that looks a citizen up needs none
// of that. It needs the record.
//
// This exists for the same reason MeetingBooker does, and the comment there
// (meetings.go:32-34) is the one to read: a dependency's *type* travels as far
// as the dependency does. internal/apps/egov — the app-facing surface of these
// integrations, and therefore the module most obviously belonging outside this
// repository — was pinned here by `*gerege.GeregeService` in one struct field.
type CitizenRecord struct {
	RegNumber      string `json:"reg_number"`
	CivilID        string `json:"civil_id"`
	LastName       string `json:"last_name"`
	FirstName      string `json:"first_name"`
	Gender         string `json:"gender"`
	Address        string `json:"address"`
	PassportStatus string `json:"passport_status"`
	Verified       bool   `json:"verified"`
}

// CompanyRecord is a legal entity as the state's register holds it.
type CompanyRecord struct {
	CompanyReg   string `json:"company_reg"`
	Name         string `json:"name"`
	Executive    string `json:"executive"`
	Address      string `json:"address"`
	VatPayer     bool   `json:"vat_payer"`
	Status       string `json:"status"`
	FoundingDate string `json:"founding_date"`
}

// StateRegistry answers a lookup against the state's registers.
//
// Both methods may answer (nil, nil): a register that has no such person is not
// an error, and a module that treats "not found" as a failure tells a citizen
// their query broke when it worked.
type StateRegistry interface {
	Citizen(ctx context.Context, regNumber string) (*CitizenRecord, error)
	Company(ctx context.Context, companyReg string) (*CompanyRecord, error)
}

// StateRegistryOf returns what this deployment provides, or an error naming the
// contract. A deployment with no ХУР credentials still provides one — the
// client answers in mock mode — so an error here means the platform did not
// wire it, not that the state is unreachable.
func StateRegistryOf() (StateRegistry, error) { return Capability[StateRegistry]() }

// ----------------------------------------------------------- audit, reading

// AuditEntry is one line of the trail Audit writes.
type AuditEntry struct {
	Action  string         `json:"action"`
	UserID  string         `json:"user_id"`
	Details map[string]any `json:"details,omitempty"`
	At      time.Time      `json:"at"`
}

// AuditReader reads back what this organisation did.
//
// Audit has been writable from a module since the SDK existed and readable from
// one never, which left an app that shows "what have we looked up" reaching
// into workspace.audit_events with its own SQL — a cross-repository dependency on a
// platform table, invisible to any compiler. This is the read half.
//
// Scoped by action prefix rather than by app id: an app that was renamed still
// has to find the acts it recorded under its old name, and the tenant's own
// rows are the only ones any caller can see regardless.
type AuditReader interface {
	RecentByPrefix(ctx context.Context, workspaceID string, prefixes []string, limit int) ([]AuditEntry, error)
}

// AuditHistory returns the audit reader, or an error naming the contract.
func AuditHistory() (AuditReader, error) { return Capability[AuditReader]() }
