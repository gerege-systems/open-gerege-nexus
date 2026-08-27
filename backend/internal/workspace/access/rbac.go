package access

import (
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// PermissionStore answers what a person may do in an organisation.
//
// An alias, not a second interface: the store the platform builds is handed to
// nexus.RequirePermission, and two structurally identical named interfaces
// would satisfy each other only by accident of Go's rules rather than by
// anybody having decided they are the same thing.
type PermissionStore = nexus.PermissionStore

// ErrForbidden is the refusal a permission check produces.
var ErrForbidden = nexus.ErrForbidden

// RequirePermission returns an HTTP middleware that enforces server-side permission authorization.
//
// The check moved to `backend/pkg/nexus` with the PermissionStore interface it
// reads through: a module in another repository refuses its own requests, and
// it cannot import a package under internal/ to do it. This forwards.
func RequirePermission(store PermissionStore, permissionCode string) func(http.Handler) http.Handler {
	return nexus.RequirePermission(store, permissionCode)
}
