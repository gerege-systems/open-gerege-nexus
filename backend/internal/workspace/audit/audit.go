// Package audit records what was done, by whom, to what.
//
// A record goes two places. The log line is what it has always been: structured
// output that a collector ships to Loki, and the only trace left when the
// database is the thing that is broken. The row in audit_events is the one that
// can be searched a month later, filtered to an organisation, and shown to the
// people whose data was read — which is what §3.5 of the monitoring proposal
// requires before one tenant may see anything of another's.
//
// The database write is best effort. An audit row failing must not fail the act
// it is recording: refusing a signature because a log insert timed out would
// trade a durable trail for an outage, and the log line has already been
// written by then.
package audit

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditEvent struct {
	TenantID  string         `json:"tenant_id"`
	UserID    string         `json:"user_id"`
	Action    string         `json:"action"`
	Resource  string         `json:"resource"`
	Details   map[string]any `json:"details"`
	Timestamp time.Time      `json:"timestamp"`
}

// writeTimeout bounds one insert. Short on purpose: this runs on the request
// path, and a database that cannot answer in a second is a database whose
// answer the caller is no longer waiting for.
const writeTimeout = time.Second

// db is the pool the rows go to. Package level rather than a parameter because
// Record has sixty-eight call sites across every module, all of which pass a
// context and none of which hold a pool — threading one through would have been
// a change to sixty-eight signatures to add a side effect none of them care
// about. Nil until UseDatabase is called, and nil simply means the log line is
// the whole record, which is what this package did before the table existed.
var db *pgxpool.Pool

// UseDatabase gives the package somewhere to persist to. Called once, from
// server construction, before any request is served.
func UseDatabase(pool *pgxpool.Pool) { db = pool }

// impersonationKey marks a request as an operator acting as somebody else.
//
// A context value rather than a parameter, because Record has sixty-eight call
// sites and none of them should have to remember this — the whole value of the
// mark is that it cannot be forgotten by the one handler where it matters.
type impersonationKey struct{}

// MarkImpersonated says that everything recorded from this context was done by
// a platform operator wearing somebody's account.
//
// Called once, by the platform's authentication middleware, from the session
// row. From there every audit row the request writes carries the mark, so the
// organisation reading its own trail sees which entries were ours — which is
// the promise §3.B makes to them.
func MarkImpersonated(ctx context.Context, operatorID string) context.Context {
	return context.WithValue(ctx, impersonationKey{}, operatorID)
}

func impersonatedBy(ctx context.Context) string {
	operatorID, _ := ctx.Value(impersonationKey{}).(string)
	return operatorID
}

func Record(ctx context.Context, tenantID, userID, action, resource string, details map[string]any) {
	if operatorID := impersonatedBy(ctx); operatorID != "" {
		if details == nil {
			details = map[string]any{}
		}
		details["impersonated"] = true
		details["operator_id"] = operatorID
	}
	slog.Info("AUDIT_EVENT",
		"tenant_id", tenantID,
		"user_id", userID,
		"action", action,
		"resource", resource,
		"details", details,
		"timestamp", time.Now().Format(time.RFC3339),
	)
	persist(ctx, tenantID, userID, action, resource, details)
}

// persist writes the row, or says why it could not.
//
// The context keeps its values and loses its cancellation: the tenant binding
// dbguard reads lives in those values, so a bare context.Background() would
// write the row as the login role and skip the row-level check that proves the
// tenant is the one the caller is acting for. A client that has hung up, on the
// other hand, must not take the audit row with it — the act it describes
// already happened.
func persist(ctx context.Context, tenantID, userID, action, resource string, details map[string]any) {
	if db == nil {
		return
	}
	if details == nil {
		details = map[string]any{}
	}

	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()

	_, err := db.Exec(writeCtx,
		`INSERT INTO workspace.audit_events (tenant_id, user_id, action, resource, details)
		 VALUES (NULLIF($1, '')::uuid, NULLIF($2, ''), $3, $4, $5)`,
		tenantID, userID, action, resource, details)
	if err != nil {
		// Warn, not Error: the event itself is already in the log above, so
		// nothing has been lost that an operator cannot recover — what has been
		// lost is the ability to query it.
		slog.Warn("audit: could not persist the event",
			"action", action, "resource", resource, "error", err)
	}
}
