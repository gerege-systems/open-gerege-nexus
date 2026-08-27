/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package approvals

import (
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps are what this screen needs of the deployment. The console core is
// separate: it decides who is asking and records what they did, and every
// screen holds the same one.
type Deps struct {
	TenantChanged func(tenantID string)
	DB            *pgxpool.Pool
}

// Service is this screen.
type Service struct {
	op            *operator.Console
	tenantChanged func(tenantID string)
	db            *pgxpool.Pool
}

// New builds it. It performs no I/O.
func New(op *operator.Console, deps Deps) *Service {
	return &Service{op: op, tenantChanged: deps.TenantChanged, db: deps.DB}
}

// changed tells the platform that an organisation's lifecycle moved, so it can
// drop what it has cached about it — on every replica, through the invalidation
// bus, rather than after each of them has waited out its own copy.
func (s *Service) changed(tenantID string) {
	if s.tenantChanged != nil {
		s.tenantChanged(tenantID)
	}
}
