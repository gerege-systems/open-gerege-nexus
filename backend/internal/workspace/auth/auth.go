package auth

import (
	"context"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// ErrUnauthorized is returned when a context carries no caller.
var ErrUnauthorized = nexus.ErrUnauthenticated

// UserClaims is who the platform decided the caller is.
//
// The type and its context key moved to `backend/pkg/nexus`: a module in
// another repository names the caller on nearly every handler, and it cannot
// import a package under internal/ to do it. This is an alias rather than a
// second struct, so the value the session middleware writes is the value a
// module reads — two identically shaped types would not have been.
type UserClaims = nexus.UserClaims

// HashPassword and CheckPasswordHash are kernel/security's: the console hashes
// an operator's password with the same cost, and one of those two answers being
// different from the other is not a thing to discover later.
var (
	HashPassword      = security.HashPassword
	CheckPasswordHash = security.CheckPasswordHash
	NeedsRehash       = security.NeedsRehash
)

// WithUserContext injects the caller's claims into a context.
func WithUserContext(ctx context.Context, claims UserClaims) context.Context {
	return nexus.WithUser(ctx, claims)
}

// UserFromContext returns who the platform decided the caller is.
func UserFromContext(ctx context.Context) (UserClaims, error) {
	return nexus.UserFromContext(ctx)
}
