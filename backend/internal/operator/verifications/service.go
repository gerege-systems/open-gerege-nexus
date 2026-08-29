/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package verifications

import (
	"context"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Health is what the deployment says about the address-verification service it
// shares: whether a key is configured at all, whether the service answered,
// and where it is administered.
//
// The console cannot ask the service itself — the client belongs to the other
// plane — so the answer arrives as a callback, the way the configuration
// warnings and the catalogue's status already do.
type Health struct {
	Configured  bool   `json:"configured"`
	Reachable   bool   `json:"reachable"`
	Detail      string `json:"health,omitempty"`
	ProviderURL string `json:"provider_url"`
	AdminURL    string `json:"admin_url"`
}

// Probe answers that question. Nil is a deployment that cannot be asked, which
// reads on the screen as "not configured" rather than as an error.
type Probe func(ctx context.Context) Health

// Deps are what this screen needs of the deployment.
type Deps struct {
	DB    *pgxpool.Pool
	Probe Probe
}

// Service is this screen.
type Service struct {
	op    *operator.Console
	db    *pgxpool.Pool
	probe Probe
}

// New builds it. It performs no I/O.
func New(op *operator.Console, deps Deps) *Service {
	return &Service{op: op, db: deps.DB, probe: deps.Probe}
}
