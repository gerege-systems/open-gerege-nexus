/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package operator is who the console lets in, and what every screen behind it
// has to pass through.
//
// The accounts, their roles and capabilities, the sessions and the second
// factor are here; so are the four middlewares every other screen is mounted
// under, and the transaction wrapper that makes a change and its audit row one
// commit. Everything else in this plane stands on this package, which is why
// nothing here may stand on any of them.
package operator

import (
	"net/netip"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Console is the shared half of the operator console: who is signed in, what
// they may do, and how a change is recorded.
type Console struct {
	db       *pgxpool.Pool
	sessions *SessionStore
	// host is the only hostname the console answers on, from
	// CONTROL_PLANE_HOST. Empty has a meaning that depends on the environment —
	// see HostGate.
	host string
	// allowedCIDRs are the addresses the console answers to while the platform
	// is private. Empty means no address restriction. See address.go.
	allowedCIDRs []netip.Prefix
}

// New builds it. It performs no I/O: a deployment without the migrations still
// constructs one, and its routes refuse at the door.
func New(db *pgxpool.Pool) *Console {
	return &Console{
		db: db, sessions: NewSessionStore(db),
		host:         normaliseHost(os.Getenv("CONTROL_PLANE_HOST")),
		allowedCIDRs: allowedCIDRsFromEnv(),
	}
}

// ConfiguredHost is the hostname the console answers on, or empty on a
// deployment that has none.
//
// The same value New reads, through the same normalisation, so that the screen
// that tells somebody where their console will be cannot disagree with the gate
// that will refuse them. Exported for the first-run wizard: a deployment with
// no console address is not offered a console to enable.
func ConfiguredHost() string { return normaliseHost(os.Getenv("CONTROL_PLANE_HOST")) }

// Sessions is the store the console's own sign-in path issues through.
func (c *Console) Sessions() *SessionStore { return c.sessions }
