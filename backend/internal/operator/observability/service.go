/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package observability

import (
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/backup"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps are what this screen needs of the deployment. The console core is
// separate: it decides who is asking and records what they did, and every
// screen holds the same one.
type Deps struct {
	// Warnings are the deployment's own complaints about its configuration.
	Warnings func() []string
	// Backup answers what was last kept and whether restoring it was tried.
	Backup          *backup.Service
	DB              *pgxpool.Pool
	CatalogStatus   func() (time.Time, bool, string)
	PlatformVersion string
}

// Service is this screen.
type Service struct {
	op *operator.Console
	// backup answers what was last kept; the front page shows it beside
	// everything else about how the deployment is holding up.
	backup            *backup.Service
	warningsFrom      func() []string
	db                *pgxpool.Pool
	catalogStatusFrom func() (time.Time, bool, string)
	platformVersion   string
}

// New builds it. It performs no I/O.
func New(op *operator.Console, deps Deps) *Service {
	return &Service{op: op, warningsFrom: deps.Warnings, backup: deps.Backup, db: deps.DB, catalogStatusFrom: deps.CatalogStatus, platformVersion: deps.PlatformVersion}
}
