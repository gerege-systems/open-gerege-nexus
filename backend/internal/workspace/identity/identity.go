/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package identity is who somebody is, as told by somebody else.
//
// Four rails arrive here — the national eID, ДАН, Google, and whichever
// provider a deployment federates with — and every one of them ends in the same
// two questions: which local account is this, and may an account be made if
// there is none. The answers are auth's; what is here is the protocol in front
// of them and the binding that ties a provider's subject to a person.
//
// The sso identity and the eID binding are one package rather than two because
// the seam between them is a flow, not a boundary: a first Google sign-in is
// sent through eID before it is allowed to become an account. Splitting them
// would put an import cycle exactly where that flow is.
//
// internal/workspace/ssoclient underneath is the OIDC client itself, and knows
// nothing about this platform's accounts.
package identity

import (
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/dan"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/eid"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/ssoclient"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps are the rails this deployment has been given. Any of the three clients
// may be nil, which is the state a deployment is in before somebody configures
// one, and each path says so rather than failing.
type Deps struct {
	DB       *pgxpool.Pool
	Sessions *auth.SessionStore
	EID      *eid.EIDService
	DAN      *dan.DANService
	// Google is a button on this platform's own sign-in screen; SSO is a
	// provider that replaces it. See google.go for why the two are separate
	// despite sharing every line of protocol.
	Google *ssoclient.Client
	SSO    *ssoclient.Client
	Authn  *auth.Handlers
}

// Handlers serve the identity rails.
type Handlers struct {
	db          *pgxpool.Pool
	sessions    *auth.SessionStore
	eidSvc      *eid.EIDService
	danSvc      *dan.DANService
	googleLogin *ssoclient.Client
	ssoClient   *ssoclient.Client
	authn       *auth.Handlers
}

// New builds them. It performs no I/O.
func New(deps Deps) *Handlers {
	return &Handlers{
		db: deps.DB, sessions: deps.Sessions,
		eidSvc: deps.EID, danSvc: deps.DAN,
		googleLogin: deps.Google, ssoClient: deps.SSO,
		authn: deps.Authn,
	}
}
