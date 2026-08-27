/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Signing in, and the two checks every signed-in request passes.
 *
 * This is the bottom of the tenant plane: nothing else in it can be reached
 * without coming through here first, and so nothing here may reach back. The
 * permission store and the federated provider are named as a shape rather than
 * imported for exactly that reason — access and ssoclient both stand on this
 * package, and an import in this direction would be a cycle.
 */

package auth

import (
	"context"
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/cache"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/memo"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PermissionReader is what HandleMe needs of the permission store: the rights
// one person holds in one organisation. Declared here rather than imported so
// that internal/workspace/access can go on standing on this package.
type PermissionReader interface {
	GetUserPermissions(ctx context.Context, tenantID, userID string) (map[string]bool, error)
}

// EndSessionURLFunc ends the federated half of a sign-out: it answers with the
// provider's logout address and clears the hint this deployment was holding. It
// is nil on a deployment that authenticates its own people, and an empty answer
// means the provider offers no RP-initiated logout.
//
// A callback rather than the client itself, for the same reason as above and in
// the same shape the console's Deps already use: the seam holds both halves and
// is the one place that may name them together.
type EndSessionURLFunc func(w http.ResponseWriter, r *http.Request) string

// Deps are what signing in needs from the rest of the deployment.
type Deps struct {
	DB       *pgxpool.Pool
	Sessions *SessionStore
	// Suspended is the cached "is this organisation closed" answer, and Bus is
	// how the console's decision reaches every replica at once. Both are held
	// here because the check runs on the request path, on every request.
	Suspended   *memo.Cache[bool]
	Bus         *cache.Bus
	Permissions PermissionReader
	EndSession  EndSessionURLFunc
}

// Handlers serve the sign-in half of the tenant plane.
type Handlers struct {
	db          *pgxpool.Pool
	sessions    *SessionStore
	suspended   *memo.Cache[bool]
	bus         *cache.Bus
	permissions PermissionReader
	endSession  EndSessionURLFunc
}

// New builds them. It performs no I/O.
func New(deps Deps) *Handlers {
	return &Handlers{
		db:          deps.DB,
		sessions:    deps.Sessions,
		suspended:   deps.Suspended,
		bus:         deps.Bus,
		permissions: deps.Permissions,
		endSession:  deps.EndSession,
	}
}

// Sessions is the store the rest of the plane issues and resolves sessions
// through. One store, so that a session created by one path is seen by all of
// them.
func (h *Handlers) Sessions() *SessionStore { return h.sessions }
