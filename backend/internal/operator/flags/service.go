/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package flags

import (
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/flags"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
)

// Deps are what this screen needs of the deployment. The console core is
// separate: it decides who is asking and records what they did, and every
// screen holds the same one.
type Deps struct {
	Flags *flags.Store
}

// Service is this screen.
type Service struct {
	op    *operator.Console
	flags *flags.Store
}

// New builds it. It performs no I/O.
func New(op *operator.Console, deps Deps) *Service {
	return &Service{op: op, flags: deps.Flags}
}
