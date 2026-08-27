/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Package nexus is the contract between this platform and the apps built on it.
 *
 * Everything in `internal/` is closed to other repositories — that is the Go
 * language rule, not a policy — and until this package existed the only way to
 * build a product on Gerege Nexus was to fork it. One fork per product means
 * every fix is applied once per product for ever, which is the failure this
 * package exists to prevent. What is here is what an app module needs in order
 * to be an app module, and nothing else:
 *
 *	Module            what a module is, and what the platform will ask of it
 *	Register          how a module joins the binary it is compiled into
 *	Dependency        what it needs installed first
 *	PermissionDefinition, MenuDefinition   what it declares to the platform
 *
 * The implementations stay in `internal/`. This package is a contract, and a
 * contract that also contained the machinery would drag the machinery into the
 * semver promise with it.
 *
 * # Stability
 *
 * This package is versioned with the module and does not break inside a major
 * version. That is the whole point of it: a distribution repository pins
 * `github.com/gerege-systems/open-gerege-nexus/backend vX.Y.Z` and expects its
 * modules to keep compiling until X changes. Anything that would break a
 * caller goes through deprecation and one major cycle — see CONTRIBUTING.
 *
 * The platform's own thirteen modules are written against this package rather
 * than against `internal/`. That is deliberate: an SDK its author does not use
 * is an SDK nobody has tried.
 */
package nexus

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Dependency describes an app module dependency and semver constraint.
type Dependency struct {
	ID                string `json:"id"`
	VersionConstraint string `json:"version_constraint"`
}

// PermissionDefinition defines an RBAC permission provided by a module.
type PermissionDefinition struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// AdminOnly withholds this permission from the default manager and user
	// roles when the app is installed. The tenant administrator still receives
	// it, and it can still be handed to any role by hand from Access control.
	//
	// It exists because the installer otherwise decides who gets a permission
	// by looking at the end of its code: anything ending `.read` is granted to
	// every member. That is a fine default for reading this organisation's own
	// rows and a bad one for reading somebody's national registry record, which
	// is a `.read` by grammar and an administrative act by consequence.
	AdminOnly bool `json:"admin_only,omitempty"`

	// DefaultRoles names the system roles this permission is granted to when
	// the app is installed. Valid values are "manager" and "user"; the tenant
	// administrator receives every permission regardless.
	//
	// It exists because until now a module could not say. The installer decided
	// by reading the end of the code — `.read` to everybody, `.manage` to
	// managers — and anything that did not fit the grammar was named in the
	// installer itself. Five `gov.*` codes were, and stayed there after
	// gov-services moved to gerege-gov: a permission this binary does not have,
	// granted by a switch this binary still runs. A module outside this
	// repository had no way in at all, so its only option was to name its
	// permissions to suit somebody else's suffix rule.
	//
	// AdminOnly wins, and a definition that sets both is refused rather than
	// reconciled: the two say opposite things, and quietly merging opposite
	// statements about who may do what is how a permission goes missing.
	DefaultRoles []string `json:"default_roles,omitempty"`
}

// The system roles a permission may be granted to by default. Not an exhaustive
// list of roles — a tenant creates as many as it likes in Access control — but
// of the ones that exist before anybody has configured anything.
const (
	DefaultRoleManager = "manager"
	DefaultRoleUser    = "user"
)

// Validate reports whether a permission definition contradicts itself.
//
// Called by the installer before anything is written, and by
// catalog.ValidateManifest so that an external app's declaration is held to the
// same rule as a compiled module's. A manifest arrives from a registry; a
// module is compiled here. Neither is a reason to check one and not the other.
func (p PermissionDefinition) Validate() error {
	if p.AdminOnly && len(p.DefaultRoles) > 0 {
		return fmt.Errorf("permission %s is AdminOnly and also asks for default roles %v; "+
			"the two say opposite things about who may do this", p.Code, p.DefaultRoles)
	}
	for _, role := range p.DefaultRoles {
		if role != DefaultRoleManager && role != DefaultRoleUser {
			return fmt.Errorf("permission %s asks for the default role %q; only %q and %q exist "+
				"before a tenant has configured anything", p.Code, role, DefaultRoleManager, DefaultRoleUser)
		}
	}
	return nil
}

// The two headers an app's screens hang under. A module names one in
// MenuDefinition.Group; anything else, including the empty string, is treated
// as MenuGroupModules.
const (
	MenuGroupModules  = "modules"
	MenuGroupSettings = "settings"
)

// MenuDefinition defines a navigation menu item for an app module.
type MenuDefinition struct {
	ID       string `json:"id"`
	AppID    string `json:"app_id,omitempty"`
	AppName  string `json:"app_name,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
	Label    string `json:"label"`
	Path     string `json:"path,omitempty"`
	// ExternalURL is set instead of Path by an app that lives outside this
	// platform. The two are alternatives, not a pair: Path is a route in this
	// application and ExternalURL is somewhere else entirely, so the shell
	// renders one as a link it owns and the other as a link it is only
	// pointing at.
	ExternalURL string `json:"external_url,omitempty"`
	Icon        string `json:"icon"`
	Order       int    `json:"order"`

	// Group is which of an app's two headers this entry hangs under: "modules"
	// for the screens people work in, "settings" for the ones an administrator
	// configures. Empty means modules, which is what most entries are.
	//
	// The platform decides the parent id — an app cannot point an entry at
	// another app's group — and this is how a module says which of its own two
	// it means. Before it existed, that answer lived in a table inside the
	// platform keyed by app id, which no module outside this repository could
	// add to; internal/workspace/menu/blueprints.go was that table.
	Group string `json:"-"`

	// AppOrder and AppChrome describe the *app* this entry belongs to rather
	// than the entry itself, and the platform fills them in from the app's
	// manifest — a module does not set them, the same as AppID and AppName.
	//
	// They are here because the shell had them hard-coded. Layout.tsx carried a
	// list of nine app ids in source order and a constant naming the one app it
	// draws as part of itself; six of the nine had already left this repository
	// and the list still named them. Where an app sits in the chrome is a
	// property of the app, so it travels with the app.
	AppOrder int `json:"app_order,omitempty"`
	// AppChrome marks an app the shell presents as part of itself rather than
	// as a tile in the app rail — see catalog.Manifest.Chrome.
	AppChrome bool `json:"app_chrome,omitempty"`

	// Labels holds per-locale overrides keyed by ISO 639-1 code. The menu API
	// resolves Label from the caller's locale before responding, so the client
	// never has to translate server-owned content.
	Labels map[string]string `json:"-"`
}

// LocalizedLabel returns the label for the requested locale, falling back to
// the default Label when no translation exists.
func (m MenuDefinition) LocalizedLabel(locale string) string {
	if label, ok := m.Labels[locale]; ok && label != "" {
		return label
	}
	return m.Label
}

// Module defines the contract every compile-time app module must implement.
//
// A module is a Go type with these seven methods and a constructor that calls
// Register. The platform owns HTTP, sessions, the database and the store; a
// module owns its own routes, its own tables and what it declares here.
//
// RegisterRoutes is handed the root router rather than a pre-scoped group, and
// the middleware it is given carries two decisions the platform has already
// made: whether this tenant has the app installed, and — for apps the platform
// knows a permission prefix for — whether the caller may make this kind of
// request. A module that wants a finer split applies its own permission
// middleware on top, which is what most of them do.
type Module interface {
	ID() string
	Name() string
	Version() string
	Dependencies() []Dependency
	Permissions() []PermissionDefinition
	Menus() []MenuDefinition
	RegisterRoutes(r chi.Router, tenantAuthMiddleware func(http.Handler) http.Handler)
}

// AccessPolicy is how a module asks the platform to enforce permissions on its
// behalf. Implementing it is optional; a module that does not is gated only by
// whether the tenant has it installed, and is expected to check permissions
// itself.
//
// It exists because the platform used to hold this knowledge in two switch
// statements keyed by app ID — one deciding whether a menu entry is visible,
// one deciding whether a request is allowed. Both listed every app by name, and
// a module that lives in another repository cannot add itself to a switch in
// this one. The consequence was not a compile error but a silent downgrade: an
// extracted app would keep working, keep appearing in the sidebar, and stop
// being gated. A permission check that disappears quietly during a refactor is
// the worst shape this kind of bug can take, so the knowledge now lives with
// the module that owns it.
//
// Both methods may return the empty string, and empty is a real answer rather
// than an omission — it says the platform should not gate this. Saying it out
// loud is the point: silence used to mean both "no permission needed" and "this
// module checks for itself", and nothing in the code could tell them apart.
type AccessPolicy interface {
	// MenuPermission is the permission a member must hold for this app's
	// entries to appear in the navigation menu. Empty means every member of a
	// tenant that has the app installed sees them.
	//
	// This is about visibility, not access: hiding a menu entry is not a
	// security boundary, and the routes behind it are gated separately.
	MenuPermission() string

	// RoutePermissionPrefix asks the platform to gate every route this module
	// registers: `prefix.read` for GET and HEAD, `prefix.manage` for anything
	// that can change something. Empty means the platform gates nothing beyond
	// the installation check, which is the right answer for a module whose
	// rules are finer than the method can express — an approval step that
	// depends on which unit the applicant belongs to cannot be derived from
	// the verb.
	RoutePermissionPrefix() string
}

// MenuPermissionOf and RoutePermissionPrefixOf read a module's access policy,
// answering empty for a module that declares none. They exist so callers do not
// each repeat the type assertion and, more importantly, so there is one place
// that decides what a module's silence means.
func MenuPermissionOf(m Module) string {
	if p, ok := m.(AccessPolicy); ok {
		return p.MenuPermission()
	}
	return ""
}

func RoutePermissionPrefixOf(m Module) string {
	if p, ok := m.(AccessPolicy); ok {
		return p.RoutePermissionPrefix()
	}
	return ""
}
