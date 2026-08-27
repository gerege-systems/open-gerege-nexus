/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package settings

import (
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/credentials"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/flags"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps are what this screen needs of the deployment. The console core is
// separate: it decides who is asking and records what they did, and every
// screen holds the same one.
type Deps struct {
	// Warnings are the platform's own complaints about how it is configured,
	// shown above the fields they are about.
	Warnings func() []string
	DB       *pgxpool.Pool
	Settings *settings.Store
	// Flags is read, not written, by this screen: a flag that has outlived the
	// date somebody gave it is one of the warnings.
	Flags *flags.Store
	// Credentials are the keys this deployment reaches other systems with.
	//
	// The same screen as the settings, because an operator asking "why is the
	// copilot off" should not have to know whether the answer is a setting or a
	// key. Not the same store, and deliberately not: a credential is sealed,
	// write-only and lives in its own table, so that the settings registry's
	// refusal to hold a secret stays a refusal rather than becoming a comment.
	Credentials *credentials.Store
}

// Service is this screen.
type Service struct {
	op           *operator.Console
	warningsFrom func() []string
	db           *pgxpool.Pool
	settings     *settings.Store
	flags        *flags.Store
	credentials  *credentials.Store
}

// New builds it. It performs no I/O.
func New(op *operator.Console, deps Deps) *Service {
	return &Service{op: op, warningsFrom: deps.Warnings, db: deps.DB,
		settings: deps.Settings, flags: deps.Flags, credentials: deps.Credentials}
}
