/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Who may do what inside one organisation, and how somebody gets back in.
 *
 * The store beside this answers the question on every request; these are the
 * screens that change the answer — roles, the permissions on them, who holds
 * them — and the two links the console hands out when somebody cannot sign in
 * at all.
 */

package access

import (
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/cache"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handlers serve the access-control screens and the recovery links.
type Handlers struct {
	db *pgxpool.Pool
	// bus drops one organisation's cached grants everywhere at once. A role
	// edited on one replica is a role every replica must stop believing in.
	bus *cache.Bus
	// authn issues the session a redeemed link ends in, and answers whether the
	// organisation behind it is still open.
	authn *auth.Handlers
}

// New builds them. It performs no I/O.
func New(db *pgxpool.Pool, bus *cache.Bus, authn *auth.Handlers) *Handlers {
	return &Handlers{db: db, bus: bus, authn: authn}
}

// forgetGrants drops one organisation's cached permissions everywhere.
func (h *Handlers) forgetGrants(tenantID string) {
	h.bus.Invalidate(GrantCacheName, TenantPrefix(tenantID))
}
