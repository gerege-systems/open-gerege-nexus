/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package metering

import (
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/tenants"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps are what this screen needs of the deployment. The console core is
// separate: it decides who is asking and records what they did, and every
// screen holds the same one.
type Deps struct {
	// Tenants answers what the organisation was sold, which is what the
	// numbers on this screen are checked against.
	Tenants *tenants.Service
	DB      *pgxpool.Pool
}

// Service is this screen.
type Service struct {
	op      *operator.Console
	tenants *tenants.Service
	db      *pgxpool.Pool
}

// NewScreen builds the usage screen. It performs no I/O.
func NewScreen(op *operator.Console, deps Deps) *Service {
	return &Service{op: op, tenants: deps.Tenants, db: deps.DB}
}
