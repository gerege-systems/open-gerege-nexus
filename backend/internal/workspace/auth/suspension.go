/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * What the control plane's decisions mean on this side of the platform: a
 * suspended organisation cannot be used, and a full one cannot grow.
 */

package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/memo"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/usage"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SuspendedTTL bounds how long a replica keeps believing an organisation is
// running after another replica has suspended it.
//
// Thirty seconds, matching the app gate, and it is not the primary control:
// suspending revokes every live session in the same transaction, so the people
// already signed in are stopped immediately. This is what stops the *next*
// request from a client that had not noticed yet.
const SuspendedTTL = 30 * time.Second

// SuspendedCacheName is what the invalidation bus knows the cache as, so that
// resuming an organisation takes effect on every replica at once rather than
// after the slowest of them has waited out its own copy.
const SuspendedCacheName = "suspended"

// forgetSuspension drops one organisation's cached state, here and everywhere.
func (h *Handlers) ForgetSuspension(tenantID string) {
	h.bus.Invalidate(SuspendedCacheName, memo.Key(tenantID, ""))
}

// ErrTenantSuspended is what every path refuses with.
var ErrTenantSuspended = errors.New("this organisation has been suspended")

// TenantSuspended reports whether an organisation is closed.
//
// A cached read on the request path, like the app gate beside it. The query
// runs on the platform path deliberately: the caller may not have a tenant
// context yet — this is asked during sign-in, before any session exists — and
// `tenants` carries no tenant_id, so no policy applies to it either way.
func (h *Handlers) TenantSuspended(ctx context.Context, tenantID string) (bool, string) {
	// No organisation is not a suspended one. Since 00094 a session may carry
	// no workspace at all, and asking the database whether the empty string is
	// suspended got as far as "invalid input syntax for type uuid" — logged at
	// ERROR, on every request a citizen made, and failing open each time. There
	// is nothing here to suspend: what such a session can reach is the person's
	// own record, which no organisation owns and none can close.
	if tenantID == "" {
		return false, ""
	}

	key := memo.Key(tenantID, "")
	if suspended, cached := h.suspended.Get(key); cached {
		return suspended, h.suspensionReason(ctx, tenantID, suspended)
	}

	var suspended bool
	var reason string
	err := h.db.QueryRow(ctx,
		`SELECT suspended_at IS NOT NULL OR deletion_scheduled_at IS NOT NULL,
		        suspension_reason
		   FROM registry.tenants WHERE id = $1::uuid`, tenantID).Scan(&suspended, &reason)
	if errors.Is(err, pgx.ErrNoRows) {
		// An organisation that is not there is not one anybody may act in.
		// This is the state a session held across a completed deletion is in.
		return true, "this organisation no longer exists"
	}
	if err != nil {
		// Fail open, and say so loudly. The alternative — refusing every
		// request on this replica because one query failed — turns a database
		// hiccup into an outage for organisations that are perfectly fine.
		slog.Error("could not check whether the organisation is suspended",
			"tenant_id", tenantID, "error", err)
		return false, ""
	}

	h.suspended.Put(key, suspended)
	return suspended, reason
}

// suspensionReason fetches the sentence to show, only when there is one to
// show. The cache holds the boolean rather than the text so that a reason
// edited in the console is never served from memory.
func (h *Handlers) suspensionReason(ctx context.Context, tenantID string, suspended bool) string {
	if !suspended {
		return ""
	}
	var reason string
	_ = h.db.QueryRow(ctx, `SELECT suspension_reason FROM registry.tenants WHERE id = $1::uuid`,
		tenantID).Scan(&reason)
	return reason
}

// RefuseIfSuspended answers 403 and reports true when the organisation is
// closed. Layered into Middleware, so every authenticated route on the
// platform is covered by one check rather than by each handler remembering.
func (h *Handlers) RefuseIfSuspended(w http.ResponseWriter, r *http.Request, tenantID string) bool {
	suspended, reason := h.TenantSuspended(r.Context(), tenantID)
	if !suspended {
		return false
	}
	message := ErrTenantSuspended.Error()
	if reason != "" {
		message += ": " + reason
	}
	// 403 rather than 401: the session is valid and signing in again will not
	// help, and a client that treats 401 as "log in again" would otherwise put
	// somebody in a loop through a login screen that also refuses them.
	httpx.Error(w, http.StatusForbidden, message)
	return true
}

// Quotas.
//
// One limit is enforced today — the number of people — because it is the only
// one this platform can currently count. Storage and AI calls are recorded by
// the console and shown as not-yet-enforced; CP-5's usage_events is what gives
// them numbers, and wiring them then is a change to this file rather than to
// every handler.

// ErrQuotaExceeded is a hard limit refusing to be crossed.
var ErrQuotaExceeded = errors.New("this organisation has reached its limit")

// CheckUserQuota decides whether one more person may join an organisation.
//
// Soft mode logs and allows; hard mode refuses. The count and the limit are
// read in one statement, so two people joining at the same moment cannot both
// see the last free place — the check is still not a lock, and it does not
// need to be: a limit exceeded by one because of a race is a warning on a
// screen, not a breach.
func (h *Handlers) CheckUserQuota(ctx context.Context, tenantID string) error {
	var limit, current int
	var enforcement string
	err := h.db.QueryRow(ctx,
		`SELECT COALESCE(q.max_users, -1), COALESCE(q.enforcement, 'soft'),
		        (SELECT count(*) FROM workspace.memberships m WHERE m.tenant_id = $1::uuid)
		   FROM registry.tenants t
		   LEFT JOIN registry.tenant_quotas q ON q.tenant_id = t.id
		  WHERE t.id = $1::uuid`, tenantID).Scan(&limit, &enforcement, &current)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		// Same reasoning as the suspension check: a failure here must not stop
		// people joining organisations that are nowhere near their limit.
		slog.Error("could not check the organisation's limits", "tenant_id", tenantID, "error", err)
		return nil
	}
	if limit < 0 || current < limit {
		return nil
	}

	if enforcement != "hard" {
		slog.Warn("an organisation is over its user limit",
			"tenant_id", tenantID, "limit", limit, "users", current)
		return nil
	}
	return ErrQuotaExceeded
}

// QuotaRail publishes the platform's metered allowances to modules, and is what
// nexus.QuotaGate resolves to.
//
// It holds the pool rather than the Server because of when it is asked: a
// module is constructed before the server pointer it would otherwise close over
// is filled, and it asks for its gate in its constructor. The pool exists by
// then; the server does not.
//
// An unmetered kind gets middleware that admits everything. A module asking for
// a meter this deployment does not keep is not an error — it is a module that
// runs on more deployments than this one.
type QuotaRail struct{ db *pgxpool.Pool }

// NewQuotaRail publishes the platform's metered allowances to modules.
func NewQuotaRail(db *pgxpool.Pool) QuotaRail { return QuotaRail{db: db} }

func (q QuotaRail) Gate(kind string) func(http.Handler) http.Handler {
	if kind != "ai" {
		slog.Warn("a module asked for a quota this platform does not meter; the route is ungated", "kind", kind)
		return func(next http.Handler) http.Handler { return next }
	}
	return aiQuota(q.db)
}

// aiQuota refuses an AI request from an organisation that has spent its month.
//
// This is the enforcement CP-2 could not do: the limit was recorded then and
// the number to check it against did not exist until CP-5's metering. It sits
// in middleware rather than in each of the six handlers for the reason every
// gate on this platform does — the seventh handler is written by somebody who
// has not read the other six.
//
// The count comes from registry.usage_events, which is rewritten a few times a day, so
// an organisation can cross its limit by however many calls it makes between
// two collections. That is deliberate: the alternative is a counter written on
// every request, and an AI limit is a commercial boundary rather than a
// security one.
//
// The assistant left for internal/apps/ai on 2026-08-23 and this stayed: the
// allowance is the deployment's, sold by the control plane, and a module that
// enforced its own would be enforcing a number nobody sold.
func aiQuota(db *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, err := nexus.WorkspaceID(r.Context())
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			var limit int
			var enforcement string
			if err := db.QueryRow(r.Context(),
				`SELECT COALESCE(max_ai_calls_monthly, -1), enforcement
				   FROM registry.tenant_quotas WHERE tenant_id = $1::uuid`, tenantID).
				Scan(&limit, &enforcement); err != nil {
				// No row is the ordinary case: an organisation nobody has set a
				// limit for has no limit.
				if !errors.Is(err, pgx.ErrNoRows) {
					slog.Warn("could not read the AI limit", "tenant_id", tenantID, "error", err)
				}
				next.ServeHTTP(w, r)
				return
			}
			if limit < 0 {
				next.ServeHTTP(w, r)
				return
			}

			used, err := usage.MonthToDate(r.Context(), db, tenantID, usage.AICalls)
			if err != nil {
				slog.Warn("could not read the month's AI usage", "tenant_id", tenantID, "error", err)
				next.ServeHTTP(w, r)
				return
			}
			if used < int64(limit) {
				next.ServeHTTP(w, r)
				return
			}

			if enforcement != "hard" {
				slog.Warn("an organisation is over its monthly AI limit",
					"tenant_id", tenantID, "limit", limit, "used", used)
				next.ServeHTTP(w, r)
				return
			}
			// 429 rather than 403: the request is not forbidden, the allowance
			// is spent, and it refills next month.
			httpx.JSON(w, http.StatusTooManyRequests, map[string]any{
				"error": "Энэ байгууллагын сарын AI дуудлагын хязгаар дуусав.",
				"limit": limit, "used": used,
			})
		})
	}
}
