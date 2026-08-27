/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * What `nexus.Platform` means on this deployment.
 *
 * The SDK says what a module may ask the platform for; this is the platform
 * answering. It is four lines because that is the whole of it — the interface
 * was kept to what the modules measurably use, and everything larger they touch
 * (the report engine, the catalogue, the state rails) is passed to the one or
 * two modules that need it rather than offered to all of them.
 */

package appinstall

import (
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/access"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/jackc/pgx/v5/pgxpool"
)

// modulePlatform is handed to every module at construction.
//
// The pool is the same one the platform serves its own handlers from, and that
// matters: every connection it hands out is bound to the caller's organisation
// by dbguard, so a module's query is inside the row-level policies without the
// module doing anything to be there — including a module written in another
// repository by somebody who has never read this file.
type modulePlatform struct {
	db    *pgxpool.Pool
	perms nexus.PermissionStore
}

func NewModulePlatform(db *pgxpool.Pool) modulePlatform {
	return modulePlatform{db: db, perms: access.NewSQLPermissionStore(db)}
}

func (p modulePlatform) DB() nexus.DB { return p.db }

// Permissions is one shared store rather than one per module.
//
// Each module used to build its own with access.NewSQLPermissionStore, which was
// both a leak — the constructor is internal, so no external module could do the
// same — and a waste: the store caches grants per tenant and invalidates across
// replicas, and fourteen of them meant fourteen caches of the same rows and
// fourteen things to invalidate.
func (p modulePlatform) Permissions() nexus.PermissionStore { return p.perms }
