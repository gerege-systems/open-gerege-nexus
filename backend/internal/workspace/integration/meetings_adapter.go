/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package integration

import (
	"context"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// AsMeetingBooker presents the manager as the SDK's nexus.MeetingBooker.
//
// The conversion is the point of the adapter, not an inconvenience of it. A
// module gets three fields of a connector and cannot reach the credentials,
// the delivery log or the OAuth state that sit beside them in the same struct
// — not because it is trusted less, but because a contract that hands over a
// storage record makes every field of that record part of the contract.
//
// It lives here rather than in the module that consumes it because the types
// being converted from are this package's, and this is where a change to them
// has to be noticed.
func AsMeetingBooker(m *Manager) nexus.MeetingBooker {
	if m == nil {
		// A nil manager is a deployment with no conferencing configured, and
		// the module already handles a nil booker by recording why an
		// appointment has no link. Wrapping nil into a non-nil interface would
		// turn that into a panic at the first booking.
		return nil
	}
	return meetingBooker{m}
}

type meetingBooker struct{ m *Manager }

func (b meetingBooker) FirstMeetingConnector(ctx context.Context, tenantID string) (*nexus.MeetingConnector, error) {
	conn, err := b.m.FirstMeetingConnector(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return &nexus.MeetingConnector{ID: conn.ID, Name: conn.Name, Provider: string(conn.Provider)}, nil
}

func (b meetingBooker) CreateMeeting(ctx context.Context, tenantID, integrationID, title string,
	startsAt time.Time, duration time.Duration, reference string) (*nexus.Meeting, error) {
	meeting, err := b.m.CreateMeeting(ctx, tenantID, integrationID, title, startsAt, duration, reference)
	if err != nil {
		return nil, err
	}
	return &nexus.Meeting{
		IntegrationID:   meeting.IntegrationID,
		IntegrationName: meeting.IntegrationName,
		Provider:        meeting.Provider,
		JoinURL:         meeting.JoinURL,
		ExternalID:      meeting.ExternalID,
	}, nil
}
