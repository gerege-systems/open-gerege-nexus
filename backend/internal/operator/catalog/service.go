/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package catalog

import (
	"context"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/observability"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
)

// Deps are what this screen needs of the deployment. The console core is
// separate: it decides who is asking and records what they did, and every
// screen holds the same one.
type Deps struct {
	// Observability holds what the last fetch did, which is what these three
	// routes answer with.
	Observability *observability.Service
	SyncCatalog   func(ctx context.Context) (bool, error)
}

// Service is this screen.
type Service struct {
	op *operator.Console
	// observability holds what the last fetch did, which is what two of these
	// three routes answer with.
	observability *observability.Service
	syncCatalogFn func(ctx context.Context) (bool, error)
}

// New builds it. It performs no I/O.
func New(op *operator.Console, deps Deps) *Service {
	return &Service{op: op, observability: deps.Observability, syncCatalogFn: deps.SyncCatalog}
}
