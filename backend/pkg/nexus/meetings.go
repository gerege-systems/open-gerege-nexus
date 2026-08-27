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

// Booking a conferencing link, as a module sees it.
//
// The platform holds connectors to Zoom, Teams and the rest, with credentials,
// delivery logs and an OAuth dance behind them. A module that books an
// appointment needs none of that. It needs to know that a tenant has a
// connector, and it needs a URL to put in front of a citizen.
//
// This contract exists because the shape of the previous one made a module
// unmovable. The government-services module declared its own MeetingBooker
// interface — dependency inversion, correctly applied — but the interface
// spoke in `*integration.Connector` and `*integration.Meeting`, which live
// under internal/. Go's internal rule then decided something nobody had
// decided on purpose: that module could never be compiled outside this
// repository. And the connector type it could not do without is a
// fifteen-field storage record of which the module reads exactly one field,
// `ID`, to hand straight back to the next call.
//
// So the lesson is narrower than "expose more from the SDK". It is that a
// dependency's *type* travels as far as the dependency does. An interface
// written in internal types is an internal interface however carefully it was
// inverted.
//
// There is a second lesson, learned from this very file. Declaring the
// interface is half the work: the adapter satisfying it was written the same
// day in internal/workspace/integration, and for the six days after that
// AsMeetingBooker had no callers — not because nobody wanted a booking, but
// because there was no way to ask for one. A contract with no way to reach an
// implementation is a contract nobody can use. Meetings, below, is that way.
type MeetingBooker interface {
	// FirstMeetingConnector returns a connector this tenant has that can host
	// a meeting, or an error when there is none. "First" rather than "the":
	// choosing between several is a product question nobody has asked yet, and
	// a module should not be the place it gets answered by accident.
	FirstMeetingConnector(ctx context.Context, tenantID string) (*MeetingConnector, error)

	// CreateMeeting books a slot and returns somewhere to join it. reference is
	// the caller's own identifier for the thing being booked — an appointment
	// row, usually — so the two can be reconciled later without the platform
	// having to know what a module's records look like.
	CreateMeeting(ctx context.Context, tenantID, integrationID, title string,
		startsAt time.Time, duration time.Duration, reference string) (*Meeting, error)
}

// MeetingConnector is a conferencing integration a tenant has connected.
//
// Three fields, where the platform's own record has fifteen. Name and Provider
// are here because a module that books on a tenant's behalf may reasonably say
// which service it booked on; everything else — credentials, status, ping
// times, the audit trail — is the platform's business and stays there.
type MeetingConnector struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

// Meeting is a booked conferencing slot.
type Meeting struct {
	IntegrationID   string `json:"integration_id"`
	IntegrationName string `json:"integration_name"`
	Provider        string `json:"provider"`
	JoinURL         string `json:"join_url"`
	ExternalID      string `json:"external_id,omitempty"`
}

// Meetings returns the booking capability this deployment provides.
//
// Asked for per use rather than captured at construction, the same as Ring and
// Documents: a module that books an appointment may be built before the
// platform has finished wiring its integrations.
//
// No sentinel of its own, unlike ErrNoLink and ErrNoDocumentFiler. Those two
// predate the capability registry and are kept because callers may already
// compare against them; a contract added afterwards has no such callers, and
// Capability's error already names the type that is missing.
func Meetings() (MeetingBooker, error) { return Capability[MeetingBooker]() }
