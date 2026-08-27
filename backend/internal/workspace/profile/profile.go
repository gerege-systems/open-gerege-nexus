/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package profile is what a person and an organisation say about themselves.
//
// Two screens and two tables: the person's own preferences on their account,
// and the organisation's legal profile — the registration number, the address,
// the contact somebody at a registry would use. Both are edited by whoever they
// belong to, which is what makes them this plane's rather than the console's.
package profile

import (
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handlers serve both screens.
type Handlers struct {
	db       *pgxpool.Pool
	sessions *auth.SessionStore
	// identity answers the "ways in" half of a person's profile: which
	// providers this account signs in through. The list is identity's to keep
	// and this screen's to show.
	identity *identity.Handlers
}

// New builds them. It performs no I/O.
func New(db *pgxpool.Pool, sessions *auth.SessionStore, ids *identity.Handlers) *Handlers {
	return &Handlers{db: db, sessions: sessions, identity: ids}
}
