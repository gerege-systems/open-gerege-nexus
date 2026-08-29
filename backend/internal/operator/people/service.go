/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package people

import (
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps are what this screen needs of the deployment.
type Deps struct {
	DB *pgxpool.Pool
}

// Service is this screen.
type Service struct {
	op *operator.Console
	db *pgxpool.Pool
}

// New builds it. It performs no I/O.
func New(op *operator.Console, deps Deps) *Service {
	return &Service{op: op, db: deps.DB}
}

// fail answers a refusal. This screen has no sentinels of its own, so every
// answer is the console's shared one.
func fail(w http.ResponseWriter, err error, doing string) { operator.Fail(w, err, doing) }
