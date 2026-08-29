/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package tenants

import (
	"os"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/credentials"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/geregecore"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/support"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps are what this screen needs of the deployment. The console core is
// separate: it decides who is asking and records what they did, and every
// screen holds the same one.
type Deps struct {
	// Audit reads the trail; the organisation's detail page shows its last few
	// rows beside everything else about it.
	Audit         *audit.Service
	TenantChanged func(tenantID string)
	Support       *support.Service
	DB            *pgxpool.Pool
	Installer     Installer
}

// Service is this screen.
type Service struct {
	op *operator.Console
	// core is the Gerege Core directory: a registration number in, an
	// organisation out. Built here rather than passed in so the console has
	// one client with one token source, and read through credentials so a
	// rotation from the settings screen takes effect without a restart.
	core *geregecore.Client
	// support is where an invitation comes from: creating an organisation
	// gives its first administrator the same link the help desk would.
	support       *support.Service
	audit         *audit.Service
	tenantChanged func(tenantID string)
	db            *pgxpool.Pool
	installer     Installer
}

// New builds it. It performs no I/O.
func New(op *operator.Console, deps Deps) *Service {
	return &Service{
		op: op, audit: deps.Audit, tenantChanged: deps.TenantChanged,
		support: deps.Support, db: deps.DB, installer: deps.Installer,
		core: geregecore.New(os.Getenv("GEREGE_CORE_URL"),
			func() string { return credentials.Get(credentials.CoreAPIToken) }),
	}
}

// changed tells the platform that an organisation's lifecycle moved, so it can
// drop what it has cached about it — on every replica, through the invalidation
// bus, rather than after each of them has waited out its own copy.
func (s *Service) changed(tenantID string) {
	if s.tenantChanged != nil {
		s.tenantChanged(tenantID)
	}
}
