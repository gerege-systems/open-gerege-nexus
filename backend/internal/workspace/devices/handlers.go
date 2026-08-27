/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package devices is the terminals an organisation enrols: tills, kiosks and
// the phones its people carry.
//
// A device is not a person. It is enrolled into exactly one organisation and
// reads as that organisation — there is no "this person also belongs to" for a
// till — which is why the five device tables take the narrow form of the
// tenant-isolation policy and say so in db/migrations/policy_shape_test.go.
package devices

import (
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/devices/staffpin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handlers serve the enrolment, the telemetry and the shared-till sign-in.
type Handlers struct {
	db       *pgxpool.Pool
	staffPIN *staffpin.Service
	// authn mints the session a till gets when somebody types a PIN into it.
	// The sign-in is the platform's and the credential is not — see
	// HandleDeviceStaffPIN.
	authn *auth.Handlers
}

// New builds them. It performs no I/O.
func New(db *pgxpool.Pool, pin *staffpin.Service, authn *auth.Handlers) *Handlers {
	return &Handlers{db: db, staffPIN: pin, authn: authn}
}
