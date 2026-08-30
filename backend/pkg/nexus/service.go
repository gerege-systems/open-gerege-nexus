/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * What the platform lends a module, and how a module answers.
 *
 * module.go says what a module *is*; this file says what it may *use*. The two
 * halves are the whole contract: a module that can be registered but has no way
 * to read a row, name its caller or refuse a request is a module that can only
 * be declared, not written.
 *
 * The surface here is small on purpose, and it is small because it was measured
 * rather than designed. Across the platform's own fourteen modules the entire
 * demand on the platform was: write a JSON response (420 call sites), find out
 * which organisation and which person the request belongs to (94), refuse it if
 * a permission is missing (24), record that something happened (41), and query
 * the database. Everything else those modules import — the report engine, the
 * catalogue, the state-registry clients, the SSO provider — is either a
 * subsystem in its own right or a specialised rail, and neither belongs in the
 * first version of a contract that cannot be narrowed later.
 *
 * The implementations stay in internal/. What is here is the interface, the two
 * functions with no state to hide, and the sinks the platform installs at
 * startup.
 */

package nexus

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------- responses

// JSON writes value as the whole response body.
//
// An encoding failure is logged rather than returned: the status line and
// headers are already on the wire by the time Encode runs, so there is no
// second answer left to give and a caller could do nothing with the error.
func JSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("failed to encode JSON response", "status", status, "error", err)
	}
}

// Error answers with {"error": message}.
//
// message is encoded rather than interpolated. Handlers used to build the body
// by concatenating a permission code or a failure reason into a JSON literal,
// which produced invalid JSON the moment the value contained a quote, a
// backslash or a newline — and the client saw a parse error instead of the
// reason it was refused.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}

// ------------------------------------------------------------------ context

// contextKey is unexported so nothing outside this package can write the values
// below. A module reads who the caller is; deciding it is the platform's, and a
// context key another package could construct would make that a suggestion.
type contextKey string

const (
	workspaceIDKey contextKey = "tenant_id"
	// allowedKey carries every organisation the caller's session is active in.
	//
	// workspaceIDKey stays the one they are acting in — the organisation a new row
	// is written into. This is the wider set they may read across, and it is
	// empty for almost every session.
	allowedKey     contextKey = "allowed_tenant_ids"
	userContextKey contextKey = "authenticated_user"
	// personScopeKey says the caller is a person with no organisation, which is
	// a different thing from an absent workspace: see WithPersonScope.
	personScopeKey contextKey = "person_scope"
)

// ErrWorkspaceMissing is returned when a context carries no acting organisation.
var ErrWorkspaceMissing = errors.New("tenant context is missing")

// ErrUnauthenticated is returned when a context carries no caller.
var ErrUnauthenticated = errors.New("unauthenticated")

// UserClaims is who the platform decided the caller is.
//
// A module reads this and never builds one: it is written by the session
// middleware from a session row, and a claim a handler could invent is not a
// claim.
type UserClaims struct {
	UserID      string `json:"user_id"`
	WorkspaceID string `json:"tenant_id"`
	Email       string `json:"email"`
	IsAdmin     bool   `json:"is_admin"`
	// AllowedWorkspaceIDs is every organisation this session reads across, and
	// WorkspaceID is always among them. Empty means only WorkspaceID, which is what
	// every session is until somebody asks for more.
	//
	// IsAdmin is deliberately not widened with it: being an administrator is a
	// role held in one organisation, and holding it in the parent says nothing
	// about the subsidiary.
	AllowedWorkspaceIDs []string `json:"allowed_tenant_ids,omitempty"`
	// Impersonated says this session belongs to a platform operator acting as
	// this person rather than to the person themselves.
	//
	// It travels with the claims rather than being looked up where it is
	// needed, because three separate things depend on it — the banner the
	// shell shows, the mark on every audit row, and the fact that /me reports
	// it — and a fact three consumers each fetch for themselves is a fact they
	// can disagree about.
	Impersonated bool `json:"impersonated,omitempty"`
	// ImpersonatedBy is the operator's id. Never their address, and never
	// serialised: this struct reaches the browser of somebody whose
	// organisation is being looked at, and who is looking is answered by the
	// organisation's own audit trail, in full, rather than by a claim in a
	// JSON body.
	ImpersonatedBy string `json:"-"`
}

// WithWorkspaceID injects the acting organisation into a context.
//
// Exported because the platform sets it and because a test needs to; a module
// calling this in a handler is a module deciding which organisation it is
// acting for, which is the one thing tenant isolation is.
func WithWorkspaceID(ctx context.Context, workspaceID string) context.Context {
	return context.WithValue(ctx, workspaceIDKey, workspaceID)
}

// WithAllowedWorkspaces records the organisations this request may read across.
//
// The caller is expected to have taken the list from the session row, which is
// the only place it is written and only after the same membership check that
// decides the acting tenant. Never build it from something a request said.
func WithAllowedWorkspaces(ctx context.Context, workspaceIDs []string) context.Context {
	if len(workspaceIDs) == 0 {
		return ctx
	}
	return context.WithValue(ctx, allowedKey, workspaceIDs)
}

// AllowedWorkspaces returns the organisations this request may read across, which
// is always at least the one it is acting in.
//
// Handlers that mean "this organisation" should keep using RequireTenant. This
// is for the few lists that are deliberately a group view.
func AllowedWorkspaces(ctx context.Context) []string {
	current, _ := ctx.Value(workspaceIDKey).(string)
	set, _ := ctx.Value(allowedKey).([]string)
	if len(set) == 0 {
		if current == "" {
			return nil
		}
		return []string{current}
	}
	return set
}

// WithoutWorkspace strips the tenant from a context, putting the caller back on
// the platform path — outside the row-level policies.
//
// It is for the handful of questions that are genuinely about a person rather
// than about a tenant. Reach for it only where crossing organisations is the
// point, and never with an id that arrived from a request without a membership
// check behind it.
//
// It clears the person scope as well, and that is the whole reason the two live
// in one file. "Which organisations does this account belong to" is asked by
// the workspace switcher, and it is asked most urgently by somebody who has
// just been accepted into their first one — a person still signed in with no
// workspace, whose connection is bound to the tenant role with no tenant, where
// a query about memberships matches nothing. Leaving the scope set would answer
// "none" to the one question whose answer had just changed.
func WithoutWorkspace(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, allowedKey, []string(nil))
	ctx = context.WithValue(ctx, personScopeKey, false)
	return context.WithValue(ctx, workspaceIDKey, "")
}

// WithPersonScope marks a request as one person acting for themselves, with no
// organisation in play.
//
// The platform's session middleware sets it, and only when a resolved session
// carries no workspace. What it buys is the difference between two things that
// look identical in a context — "nobody is signed in" and "somebody is signed
// in and belongs to no organisation" — which must not be bound to the database
// the same way. The first is the platform path and sees everything; the second
// is the least privileged caller on the system and is bound to the tenant role
// with no tenant, where every workspace policy refuses and only the rows keyed
// on the person themselves are readable.
func WithPersonScope(ctx context.Context) context.Context {
	return context.WithValue(ctx, personScopeKey, true)
}

// IsPersonScoped reports whether ctx was marked by WithPersonScope.
func IsPersonScoped(ctx context.Context) bool {
	scoped, _ := ctx.Value(personScopeKey).(bool)
	return scoped
}

// WorkspaceID extracts the acting organisation from a context.
//
// Handlers should reach for RequireTenant instead. This is for callers that
// have a context but no ResponseWriter to answer on.
func WorkspaceID(ctx context.Context) (string, error) {
	workspaceID, ok := ctx.Value(workspaceIDKey).(string)
	if !ok || workspaceID == "" {
		return "", ErrWorkspaceMissing
	}
	return workspaceID, nil
}

// RequireWorkspace resolves the caller's organisation, or answers 401 and reports
// false. A handler that gets false has already had its response written and
// must return.
//
// This is not middleware, and there is deliberately none: what guards a request
// is the session middleware, the only thing that puts a tenant into the
// context, and the app gate, which refuses the request when this fails.
func RequireWorkspace(w http.ResponseWriter, r *http.Request) (string, bool) {
	workspaceID, err := WorkspaceID(r.Context())
	if err != nil {
		Error(w, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	return workspaceID, true
}

// WithUser injects the caller's claims into a context. The platform's session
// middleware is what calls this.
func WithUser(ctx context.Context, claims UserClaims) context.Context {
	return context.WithValue(ctx, userContextKey, claims)
}

// UserFromContext returns who the platform decided the caller is.
//
// Claims with no user id count as absent. A zero-valued UserClaims reaching a
// handler would otherwise read as "an authenticated person whose id is the
// empty string", and every query scoped to that id would quietly match nothing
// instead of refusing.
func UserFromContext(ctx context.Context) (UserClaims, error) {
	claims, ok := ctx.Value(userContextKey).(UserClaims)
	if !ok || claims.UserID == "" {
		return UserClaims{}, ErrUnauthenticated
	}
	return claims, nil
}

// ----------------------------------------------------------------- database

// DB is the database as a module sees it.
//
// It is pgx's own shape rather than an abstraction over it, and deliberately:
// this platform is committed to pgx and hand-written SQL, and an interface that
// hid the driver would either leak it back through the row types or force every
// module to translate between two vocabularies for no benefit. `*pgxpool.Pool`
// satisfies this as it is.
//
// What the interface buys, then, is not portability but honesty about the
// contract: the platform hands a module a handle with these four methods, and a
// module cannot reach past it to the pool's configuration, its statistics or
// its Close.
//
// Every query is tenant-scoped by row-level security, decided from the tenant
// on the connection rather than from anything the module passes — see
// internal/kernel/dbguard. A module that forgets a WHERE tenant_id still
// cannot read another organisation's rows.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
	// BeginTx is here for one option in particular: pgx.ReadOnly. A module that
	// means "this cannot write" should be able to say so to the database rather
	// than only to the compiler — the report engine opens every run this way,
	// and a statement timeout needs a transaction to be SET LOCAL in anyway.
	BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error)
}

// --------------------------------------------------------------- permissions

// ErrForbidden is the refusal a permission check produces.
var ErrForbidden = errors.New("forbidden: insufficient permissions")

// PermissionStore answers what a person may do in an organisation.
//
// The implementation is the platform's — it reads roles, caches per tenant and
// is invalidated across replicas. A module holds one and asks it.
type PermissionStore interface {
	GetUserPermissions(ctx context.Context, workspaceID, userID string) (map[string]bool, error)
}

// RequirePermission is middleware that refuses a request the caller has no
// right to make.
//
// A tenant administrator passes without a lookup. That is not a shortcut around
// the check: the administrator role is defined as holding every permission in
// its organisation, so asking the store would return true for all of them, and
// a per-request query to learn that is a query with a known answer.
func RequirePermission(store PermissionStore, permissionCode string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := UserFromContext(r.Context())
			if err != nil {
				Error(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if claims.IsAdmin {
				next.ServeHTTP(w, r)
				return
			}
			permissions, err := store.GetUserPermissions(r.Context(), claims.WorkspaceID, claims.UserID)
			if err != nil || !permissions[permissionCode] {
				Error(w, http.StatusForbidden, "forbidden: permission "+permissionCode+" required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --------------------------------------------------------------------- audit

// AuditSink is where recorded events go. The platform installs one at startup;
// see UseAuditSink.
type AuditSink func(ctx context.Context, workspaceID, userID, action, resource string, details map[string]any)

// UseAuditSink installs the platform's audit recorder.
//
// It is a sink rather than an interface a module holds because an audit row is
// not a service a module may decline to use: everything that records one should
// reach the same table, and a module that was handed a recorder could be
// constructed without one.
//
// Deprecated: use Provide[AuditSink] instead. This is a wrapper over it and
// behaves identically, including UseAuditSink(nil) to withdraw. It stays for
// one major version so a distribution pinned to v1 keeps compiling, and goes in
// v2 — see docs/MODULES.md.
func UseAuditSink(sink AuditSink) { Provide[AuditSink](sink) }

// Audit records that something happened, for the log that answers "who read my
// data".
//
// Best-effort by design: an audit write must not fail the operation it is
// describing. With no sink installed the event is logged and dropped, which is
// what happens in a test that constructs a module without a platform.
func Audit(ctx context.Context, workspaceID, userID, action, resource string, details map[string]any) {
	sink, err := Capability[AuditSink]()
	if err != nil {
		slog.Info("AUDIT_EVENT_UNSUNK", "tenant_id", workspaceID, "user_id", userID,
			"action", action, "resource", resource)
		return
	}
	sink(ctx, workspaceID, userID, action, resource, details)
}

// -------------------------------------------------------------------- wiring

// Platform is what a module is handed when it is built.
//
// One argument rather than four, because the list will grow and a constructor
// signature that changes every time the platform lends a module something new
// is a signature every distribution has to chase. A module takes what it needs
// in its constructor and keeps that:
//
//	func New(p nexus.Platform) *Module {
//	    m := &Module{db: p.DB(), perms: p.Permissions()}
//	    nexus.Register(m)
//	    return m
//	}
type Platform interface {
	// DB is the tenant-scoped database handle.
	DB() DB
	// Permissions answers what a person may do, for RequirePermission.
	Permissions() PermissionStore
}

// NewPlatform assembles a Platform from its parts.
//
// Two callers have it: a test that builds one module without starting a server,
// and a distribution's main.go that assembles its own runtime. Both would
// otherwise have to declare a four-line type of their own, which is four lines
// that stop compiling the day this interface grows a method — the exact churn
// the single-argument constructor was meant to avoid.
func NewPlatform(db DB, permissions PermissionStore) Platform {
	return staticPlatform{db: db, permissions: permissions}
}

type staticPlatform struct {
	db          DB
	permissions PermissionStore
}

func (p staticPlatform) DB() DB                       { return p.db }
func (p staticPlatform) Permissions() PermissionStore { return p.permissions }

// ---------------------------------------------------------------- state rails

// StateRail is one of the state's systems, as this deployment is wired to it.
//
// Published here because a module has to be able to say what it is connected
// to. internal/apps/egov — the app-facing surface of the state integrations —
// imported internal/operator/staterail for this type and nothing else, and that
// package is eight lines of struct: a module was pinned into this repository by
// a type declaration it could have carried itself.
//
// The clients stay where they are. What travels is the shape of the answer:
// which rail, what it is called, whether it is live or mocked, and where it
// points. A module renders that; it does not dial it.
type StateRail struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Mode is "live" or "mock" — a deployment wired to the real registry and
	// one answering from fixtures look identical to a screen otherwise, which
	// is the difference an operator most needs to see.
	Mode     string `json:"mode"`
	Endpoint string `json:"endpoint,omitempty"`
}

// StateRails is read per call rather than captured: what this deployment is
// wired to can change while it is running, and a snapshot would be wrong from
// the moment it was taken.
type StateRails func() []StateRail

// ------------------------------------------------------------- rate limiting

// RateLimit is a per-caller rate limit, as middleware.
//
// Published because every module that exposes something expensive needs one and
// none of them should write it: a limiter written per app is a limiter that
// counts per app, so five apps behind one gateway enforce five separate budgets
// and the deployment has none.
//
// The platform's own implementation is installed by the platform;
// a deployment that installs none gets middleware that limits nothing rather
// than a nil panic, because a missing limiter must not take the route down.
func RateLimit(perMinute float64, burst int) func(http.Handler) http.Handler {
	limiter, err := Capability[RateLimiter]()
	if err != nil {
		slog.Warn("nexus: this deployment provides no rate limiter; the route is unlimited",
			"per_minute", perMinute, "burst", burst)
		return func(next http.Handler) http.Handler { return next }
	}
	return limiter.Limit(perMinute, burst)
}

// RateLimiter is what the platform installs for RateLimit to use.
type RateLimiter interface {
	// Limit returns middleware admitting perMinute requests per caller, with
	// burst allowed above the average.
	Limit(perMinute float64, burst int) func(http.Handler) http.Handler
}

// ------------------------------------------------------------------- quotas

// QuotaGate holds a route to the organisation's allowance for a metered act.
//
// Published for the same reason RateLimit is, and it is not the same thing: a
// rate limit is about the next second and is enforced per caller; a quota is
// about the month and is enforced per organisation, from what the control
// plane sold it. A module that enforced its own would be enforcing a number
// nobody sold.
//
// kind names the metered act — "ai" is the one this platform meters today. A
// deployment that meters nothing, or a kind nothing meters, gets middleware
// that admits everything rather than a nil panic: a missing meter must not take
// the route down, the same rule the rate limiter follows.
func QuotaGate(kind string) func(http.Handler) http.Handler {
	quota, err := Capability[Quota]()
	if err != nil {
		slog.Warn("nexus: this deployment enforces no quotas; the route is ungated", "kind", kind)
		return func(next http.Handler) http.Handler { return next }
	}
	return quota.Gate(kind)
}

// Quota is what the platform installs for QuotaGate to use.
type Quota interface {
	// Gate returns middleware that refuses a request once the organisation has
	// spent its allowance for kind this month, and admits everything for a kind
	// this deployment does not meter.
	Gate(kind string) func(http.Handler) http.Handler
}

// ------------------------------------------------------------------- metrics

// SignatureCounter counts signatures for the deployment's metrics.
//
// A module cannot import the platform's Prometheus registry — it is under
// internal/, and a module that could would be able to declare a metric with a
// tenant label, which is the one thing observability refuses. So the platform
// keeps the collector and publishes the act.
type SignatureCounter interface {
	// Signed records one signature attempt on a named rail.
	Signed(rail string, ok bool)
}

// RecordSigned counts one signature attempt, if this deployment is counting.
// Silent when nothing is: a metric that cannot be recorded must not fail the
// signature it is describing.
func RecordSigned(rail string, ok bool) {
	if counter, err := Capability[SignatureCounter](); err == nil {
		counter.Signed(rail, ok)
	}
}

// ---------------------------------------------------------- installed apps

// InstalledApps answers which apps an organisation has.
//
// The gate every module that lists something across apps needs: a report
// belonging to an app nobody installed is not listed and cannot be run, and the
// answer to "which apps" has to be the platform's one answer rather than a
// second, differently-stale copy.
//
// Published as an exported type on 2026-08-23 because it had been published as
// an internal one — internal/apps.InstalledApps — which a distribution cannot
// name and therefore cannot ask for. That is the same mistake the PDF signing
// rails made and it surfaced the same way: an app that had left this repository
// could not fetch the capability the platform was providing.
type InstalledApps func(ctx context.Context, workspaceID string) (map[string]bool, error)

// AppsOf returns which apps an organisation has, from whatever the platform
// provides.
func AppsOf(ctx context.Context, workspaceID string) (map[string]bool, error) {
	installed, err := Capability[InstalledApps]()
	if err != nil {
		return nil, err
	}
	return installed(ctx, workspaceID)
}
