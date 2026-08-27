// Package dbguard binds a pooled database connection to the tenant the caller
// is acting for, so PostgreSQL row-level security can enforce the isolation the
// application layer already intends.
//
// Every query in this codebase carries `WHERE tenant_id = $1`, and that stays
// the primary control. This is the layer underneath it: a handler that forgets
// the clause — in a module written next year, by somebody who has not read
// every other module — returns nothing rather than another tenant's rows.
//
// How it works: pgxpool hands out a connection per query, and BeforeAcquire
// runs with the acquiring request's context. That is the one place that knows
// both which physical connection is about to be used and whose request is about
// to use it, so it is where the binding is made.
//
//	tenant in context     → SET ROLE gerege_nexus_tenant, app.current_tenant = <id>
//	no tenant in context  → SET ROLE NONE (the login role, not subject to the policies)
//
// The second case is not a gap, it is the platform path, and it is the reason
// this design needs no change to a single existing query. Signing in has no
// tenant yet; resolving a session is what discovers the tenant; the OAuth
// callback and the e-mail verification landing are reached by people who have
// not signed in; the housekeeping sweeps deliberately cross every tenant. All
// of those run as the login role and see everything, exactly as they do today.
package dbguard

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TenantRole is the unprivileged role the tenant-scoped policies are written
// for. Migration 00029 creates it as gerege_nexus_app and migration 00079 gives
// it the precise name below. "App" had come to mean three unrelated things: a
// database request role, a module somebody installs, and the platform binary.
// "Tenant" names only whose behalf the query runs on, and pairs with
// OperatorRole without that ambiguity. The role owns nothing.
const TenantRole = "gerege_nexus_tenant"

// OperatorRole is what the control plane's own queries run as. Migration 00049
// creates it with SELECT on a hand-written list of tables and read-only
// policies to match, so "the operator sees every organisation" is a set of
// named permissions rather than a switch that turns the isolation off.
//
// It is deliberately not the login role. The platform path (no tenant in
// context) is outside the policies entirely and can write anything; a console
// whose whole job is to look at other people's organisations must not run
// there, or the first handler with a typo in it becomes a cross-tenant write.
const OperatorRole = "gerege_nexus_operator"

type contextKey string

// operatorKey marks a request as the control plane's. It lives here rather than
// in a shared package because dbguard is what acts on it, and a marker any
// package could set is a marker no reviewer can trace.
const operatorKey contextKey = "dbguard_operator"

// AsOperator binds the queries made with this context to OperatorRole.
//
// Only the control plane's middleware calls it, and only after an operator
// session has been resolved. It carries no tenant: every control-plane query
// names the organisation it is asking about in its own WHERE clause, which is
// what makes each one reviewable on its own.
func AsOperator(ctx context.Context) context.Context {
	return context.WithValue(ctx, operatorKey, true)
}

// IsOperator reports whether ctx was marked by AsOperator.
func IsOperator(ctx context.Context) bool {
	marked, _ := ctx.Value(operatorKey).(bool)
	return marked
}

// bindStatement sets all three variables in one round trip.
//
// `role` is an ordinary GUC, so set_config assigns it exactly as SET ROLE would
// and the whole binding costs one message rather than two. Both values are
// parameters: the tenant id arrives from a session row, and building this
// string by concatenation would put a SET ROLE one quote away from user data.
const bindStatement = `SELECT set_config('role', $1, false), ` +
	`set_config('app.current_tenant', $2, false), set_config('app.allowed_tenants', $3, false)`

// allowedLiteral renders the read set as PostgreSQL's array literal, which is
// what the policy casts. Empty stays empty rather than becoming '{}': the
// policy reads that as "no organisations at all", where an unset value means
// "fall back to the one being acted in" — which is every session that has not
// asked for more.
func allowedLiteral(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return "{" + strings.Join(ids, ",") + "}"
}

// Guard installs the binding and reports whether it is live.
//
// It starts disabled. Enable is called after Probe has confirmed the database
// is actually carrying the policies, because a pool that tried to SET ROLE to a
// role that does not exist would fail every acquisition — an outage caused by
// the security control, on a deployment whose migrations had not caught up.
type Guard struct {
	enabled atomic.Bool
	// operatorReady is the same idea for OperatorRole, tracked apart because
	// the two originally arrived in different migrations. A deployment that has
	// 00029 but not 00049 serves tenants normally and has no control plane at all — which
	// is the honest answer, since the console's tables are not there either.
	operatorReady atomic.Bool
}

// Install attaches the binding to a pool configuration. It must be called
// before the pool is created, and takes effect only once Probe succeeds.
//
// PrepareConn rather than the older BeforeAcquire: the two cannot coexist —
// pgxpool ignores BeforeAcquire entirely when PrepareConn is set — so a library
// or a later change that reaches for the current hook would silently switch
// this one off, and the isolation would be gone with nothing to show for it.
func (g *Guard) Install(cfg *pgxpool.Config) {
	// The platform's clock, on every connection this pool hands out.
	//
	// It sits here rather than beside the pool because this is the one place
	// every pool in this codebase is configured — production's in
	// pkg/host/run.go and every database-backed test's — and a second call
	// somebody has to remember is how a timezone bug comes back. It is a
	// connection parameter rather than a statement, so it is set once when the
	// connection is made and costs nothing per query.
	//
	// What it decides: `created_at::date`, `CURRENT_DATE`, `date_trunc` and
	// every other reduction of an instant to a calendar. Postgres stores
	// timestamptz as an instant; the session's zone is what reads it. Left to
	// the server's default the answer would be UTC, and a daily figure for a
	// Mongolian office would end at eight in the morning — see
	// pkg/nexus/clock.go.
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["timezone"] = nexus.TimezoneName()

	cfg.PrepareConn = func(ctx context.Context, conn *pgx.Conn) (bool, error) {
		if !g.enabled.Load() {
			return true, nil
		}
		role, tenantID, allowed := "none", "", ""
		id, idErr := nexus.TenantID(ctx)
		switch {
		case IsOperator(ctx):
			// Refused rather than quietly served as the login role: falling
			// back would hand the console more access than the role it asked
			// for, which is the opposite of what an unmet precondition should
			// do. The control plane stops working until 00049 is applied, and
			// its tables do not exist until then either.
			if !g.operatorReady.Load() {
				return false, fmt.Errorf("dbguard: %s is not available (run the migrations up to 00049_control_plane)", OperatorRole)
			}
			role = OperatorRole
		case idErr == nil && id != "":
			role, tenantID = TenantRole, id
			// Only ever widened by the session, and only past the same
			// membership check that produced the acting tenant.
			allowed = allowedLiteral(nexus.AllowedTenants(ctx))
		}
		if _, err := conn.Exec(ctx, bindStatement, role, tenantID, allowed); err != nil {
			// False destroys the connection rather than handing over one whose
			// tenant binding is whatever the previous request left behind — the
			// failure this package exists to prevent. The error travels with it
			// so the query fails saying why, instead of pgxpool working through
			// the pool and reporting that it ran out of attempts.
			slog.Error("dbguard: could not bind the connection to a tenant",
				"role", role, "error", err)
			return false, fmt.Errorf("dbguard: could not bind the connection to tenant %q: %w",
				tenantID, err)
		}
		return true, nil
	}
}

// Enabled reports whether the binding is being applied.
func (g *Guard) Enabled() bool { return g.enabled.Load() }

// Probe checks that the database can enforce what the guard assumes, and turns
// the guard on if it can.
//
// A deployment whose migrations have not reached 00079 is a normal state during
// a rollout, and it is left running with the application-level filtering it has
// always had rather than being taken down. It is reported at WARN because a
// deployment that stays in that state is running with one layer, not two.
func (g *Guard) Probe(ctx context.Context, pool *pgxpool.Pool) error {
	var roleExists, policiesExist bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1),
		       EXISTS (SELECT 1 FROM pg_policies
		                WHERE schemaname = 'workspace' AND policyname = 'tenant_isolation')`,
		TenantRole).Scan(&roleExists, &policiesExist)
	if err != nil {
		return fmt.Errorf("dbguard: could not inspect the database: %w", err)
	}
	if !roleExists || !policiesExist {
		slog.Warn("dbguard: row-level tenant isolation is not installed; "+
			"the application filter is the only layer",
			"role_present", roleExists, "policies_present", policiesExist,
			"remedy", "run the database migrations up to 00079_two_schemas")
		return nil
	}

	// The role has to be reachable from whoever we connect as. If it is not,
	// enabling the guard would break every tenant-scoped query, so this is
	// checked before anything is switched on rather than discovered by traffic.
	//
	// One connection is held for both halves of the check. Two Exec calls would
	// be free to land on two different pooled connections, and the first of them
	// would go back to the pool still wearing the role — which nothing would
	// correct until the guard is on and starts rebinding on every acquisition.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("dbguard: could not take a connection to verify the role: %w", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, bindStatement, TenantRole, "", ""); err != nil {
		return fmt.Errorf("dbguard: cannot assume %s (grant it to the login role in DATABASE_URL): %w",
			TenantRole, err)
	}
	if _, err := conn.Exec(ctx, bindStatement, "none", "", ""); err != nil {
		return fmt.Errorf("dbguard: cannot return to the login role: %w", err)
	}

	g.enabled.Store(true)
	slog.Info("dbguard: row-level tenant isolation is active", "role", TenantRole)

	g.probeOperator(ctx, conn)
	return nil
}

// probeOperator decides whether control-plane contexts may bind, on the
// connection Probe is already holding.
//
// Failure is not an error returned to the caller: a deployment that has not run
// 00049 has no operator accounts, no console tables and nobody able to sign in
// to it, so the platform starting normally is the correct outcome. What must
// not happen is the console appearing to work while its queries run as the
// login role, and that is what the flag prevents.
//
// The connection is put back on the login role afterwards for the same reason
// Probe does it: a pooled connection wearing a role nothing asked for is the
// binding this package exists to prevent.
func (g *Guard) probeOperator(ctx context.Context, conn *pgxpool.Conn) {
	if _, err := conn.Exec(ctx, bindStatement, OperatorRole, "", ""); err != nil {
		slog.Info("dbguard: the control plane's database role is not available",
			"role", OperatorRole, "reason", err)
		return
	}
	if _, err := conn.Exec(ctx, bindStatement, "none", "", ""); err != nil {
		slog.Error("dbguard: could not return the probe connection to the login role", "error", err)
		return
	}
	g.operatorReady.Store(true)
	slog.Info("dbguard: the control plane may bind its own database role", "role", OperatorRole)
}

// OperatorReady reports whether control-plane contexts can be bound. The
// console's routes ask before they are mounted, so a deployment without 00049
// answers 404 rather than 500.
func (g *Guard) OperatorReady() bool { return g.operatorReady.Load() }

// ProbeUntilEnabled keeps probing until the guard is on or ctx is cancelled.
//
// It exists for the startup this process explicitly tolerates: the database
// being unreachable when the API comes up. /ready reports that, the pool
// reconnects on its own, and traffic starts working the moment the database
// does — but the guard is switched on by a probe, and a probe that ran once
// against nothing would leave the process serving that traffic for the rest of
// its life with the isolation switched off and nothing saying so.
//
// Connections acquired between the database returning and the probe succeeding
// are not bound. That window is bounded by interval and only exists on a
// degraded start; every acquisition after it is bound.
func (g *Guard) ProbeUntilEnabled(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if g.Enabled() {
			return
		}
		probeCtx, cancel := context.WithTimeout(ctx, interval)
		err := g.Probe(probeCtx, pool)
		cancel()
		if err != nil {
			slog.Warn("dbguard: still cannot enable tenant isolation", "error", err)
			continue
		}
		if g.Enabled() {
			return
		}
	}
}
