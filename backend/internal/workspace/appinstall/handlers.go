/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * What an organisation has installed, and the gate that follows from it.
 *
 * The installer beside this writes app_installations; these are the screens an
 * administrator installs through and the middleware every module's routes sit
 * behind. They are one package because they are one fact read two ways: the
 * store shows what is installed, and the gate refuses everything that is not.
 */

package appinstall

import (
	"sync"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/appcatalog"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/cache"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/memo"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/access"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps are what the store screens and the gate need.
type Deps struct {
	DB        *pgxpool.Pool
	Installer *AppInstaller
	// Catalogue is where the catalogue came from and where a refresh goes.
	Catalogue   *appcatalog.Provider
	Permissions *access.SQLPermissionStore
	// Gate is the cached "has this organisation installed that app" answer, and
	// Bus is how an install on one replica reaches the others.
	Gate *memo.Cache[bool]
	Bus  *cache.Bus
	// Authn is the middleware the gate layers itself on top of: a request that
	// is not signed in never reaches the question.
	Authn *auth.Handlers
}

// Handlers serve the store screens and gate the modules' routes.
type Handlers struct {
	db          *pgxpool.Pool
	installer   *AppInstaller
	catalogue   *appcatalog.Provider
	permissions *access.SQLPermissionStore
	appGate     *memo.Cache[bool]
	bus         *cache.Bus
	authn       *auth.Handlers

	// The last thing the catalogue sync did. An administrator pressing "check
	// for updates" gets an answer; the hourly one leaves only a log line, and a
	// registry that has been failing for a week is exactly the thing nobody
	// notices.
	syncMu      sync.RWMutex
	lastSyncAt  time.Time
	lastSyncOK  bool
	lastSyncErr string
}

// New builds them. It performs no I/O.
func New(deps Deps) *Handlers {
	return &Handlers{
		db: deps.DB, installer: deps.Installer, catalogue: deps.Catalogue,
		permissions: deps.Permissions, appGate: deps.Gate, bus: deps.Bus,
		authn: deps.Authn,
	}
}

// ForgetGate drops one organisation's cached installation answers, here and on
// every other replica.
func (h *Handlers) ForgetGate(tenantID string) {
	h.bus.Invalidate(GateCacheName, memo.Key(tenantID, ""))
}

// GateCacheName is what the invalidation bus knows the gate cache as.
const GateCacheName = "appgate"
