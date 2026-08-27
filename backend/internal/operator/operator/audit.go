package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// §2.5 of the plan states the rule this file exists to make true: a write the
// control plane makes without an audit row does not happen at all.
//
// Stating it is easy and remembering it in every handler is not, so it is not
// left to handlers. Two things enforce it together:
//
//   - Do opens a transaction, runs the write inside it, and writes the audit
//     row into the same transaction. Either both land or neither does. There
//     is no ordering to get wrong and no failure mode where the action
//     succeeds and the record is lost.
//   - requireAudit (middleware.go) holds the response back until it has seen an
//     audit row recorded for that request. A handler that wrote something some
//     other way — a bare pool.Exec — answers 500 with nothing in its body,
//     rather than answering 200 for a change nobody can trace.
//
// The second is what makes the first a rule instead of a convention. It cannot
// undo a write that bypassed Do, and it is not meant to: it makes such a
// handler fail loudly the first time it is exercised, in development, which is
// where that mistake should be found.

// ErrReasonRequired is returned when a write is attempted without one.
//
// Every audit table has a "reason" column and most are empty, because nothing
// asks. This one asks: the field is what turns a log of what happened into an
// answer to why, and the moment to capture it is while the operator is looking
// at the confirmation dialog.
var ErrReasonRequired = errors.New("a reason is required for this action")

// Change describes one write, before and after.
type Change struct {
	// Action is a dotted verb — "tenant.suspend", "operator.session.begin".
	// It is what the audit search filters on, so it is a closed vocabulary in
	// practice; keep it stable when renaming things.
	Action     string
	TargetType string
	TargetID   string
	Reason     string
	Before     any
	After      any
}

// auditTicketKey marks a request as one whose audit row has been written.
type auditTicketKey struct{}

// auditTicket is the flag requireAudit reads after the handler returns. A
// pointer in the context rather than a returned value, because the middleware
// has no way to receive anything from a handler it only calls.
type auditTicket struct{ recorded bool }

func withAuditTicket(ctx context.Context) (context.Context, *auditTicket) {
	ticket := &auditTicket{}
	return context.WithValue(ctx, auditTicketKey{}, ticket), ticket
}

func ticketFrom(ctx context.Context) *auditTicket {
	ticket, _ := ctx.Value(auditTicketKey{}).(*auditTicket)
	return ticket
}

// Do performs a control-plane write and records it, atomically.
//
// fn receives the transaction, and must do all of its database work through it
// — a query made against the pool instead would be outside the transaction and
// would survive a rollback the audit row did not, which is the exact split this
// function exists to prevent.
func (c *Console) Do(ctx context.Context, sess Session, change Change, fn func(context.Context, pgx.Tx) error) error {
	if change.Reason == "" {
		return ErrReasonRequired
	}
	return c.Record(ctx, sess, change, fn)
}

// Record is Do without the reason requirement, for the writes that are their
// own reason: signing in, signing out, confirming a second factor. Asking an
// operator to explain why they are signing in would train them to type
// something meaningless into a field that matters elsewhere.
//
// Exported because every screen in this plane is its own package now and all of
// them write through here. Which of the two a screen calls is the decision
// about whether that change needs a sentence beside it.
func (c *Console) Record(ctx context.Context, sess Session, change Change, fn func(context.Context, pgx.Tx) error) error {
	ctx = Scoped(ctx)

	tx, err := c.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("control plane: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if fn != nil {
		if err := fn(ctx, tx); err != nil {
			return err
		}
	}
	if err := recordAudit(ctx, tx, sess, change, clientIPFrom(ctx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("control plane: commit: %w", err)
	}

	// Only after the commit. A ticket stamped before it would let a failed
	// transaction answer 200 to the middleware's question.
	if ticket := ticketFrom(ctx); ticket != nil {
		ticket.recorded = true
	}
	// The same line goes to the log, and from there to Loki, so that the trail
	// survives the database being the thing that is broken — §2.5 asks for
	// both.
	slog.Info("CONTROL_PLANE_AUDIT",
		"operator_id", sess.ID, "operator_email", sess.Email,
		"action", change.Action, "target_type", change.TargetType,
		"target_id", change.TargetID, "reason", change.Reason)
	return nil
}

// recordAudit writes the row. Unexported and taking a transaction, so that
// there is no way to write an audit row on its own — an audit trail that can be
// appended to without a corresponding act is one that can be furnished.
func recordAudit(ctx context.Context, tx pgx.Tx, sess Session, change Change, ip string) error {
	before, err := asJSON(change.Before)
	if err != nil {
		return err
	}
	after, err := asJSON(change.After)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO operator.operator_audit
		     (operator_id, operator_email, action, target_type, target_id, reason, before, after, ip)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		sess.ID, sess.Email, change.Action, change.TargetType, change.TargetID,
		change.Reason, before, after, ip); err != nil {
		return fmt.Errorf("control plane: record the audit row: %w", err)
	}
	return nil
}

// asJSON renders a before/after value for the jsonb columns. nil becomes an
// empty object rather than SQL NULL, so that reading the column never has to
// branch.
func asJSON(value any) ([]byte, error) {
	if value == nil {
		return []byte("{}"), nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("control plane: encode an audit value: %w", err)
	}
	return encoded, nil
}

// clientIPKey carries the caller's address from the middleware to the audit
// row, because Do has a context and no request.
type clientIPKey struct{}

func withClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey{}, ip)
}

func clientIPFrom(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey{}).(string)
	return ip
}

// AuditEntry is one row of the trail, as the console shows it.
type AuditEntry struct {
	ID            string          `json:"id"`
	OperatorID    string          `json:"operator_id"`
	OperatorEmail string          `json:"operator_email"`
	Action        string          `json:"action"`
	TargetType    string          `json:"target_type"`
	TargetID      string          `json:"target_id"`
	Reason        string          `json:"reason"`
	Before        json.RawMessage `json:"before"`
	After         json.RawMessage `json:"after"`
	IP            string          `json:"ip"`
	CreatedAt     time.Time       `json:"created_at"`
}

// AuditPageSize bounds one page of the trail.
const AuditPageSize = 100

// bufferedWriter holds a response until requireAudit decides whether it may be
// sent. It is the smallest thing that can do that: everything a handler writes
// is kept, and either replayed onto the real writer or dropped.
type bufferedWriter struct {
	header http.Header
	body   []byte
	status int
}

func newBufferedWriter() *bufferedWriter {
	return &bufferedWriter{header: make(http.Header), status: http.StatusOK}
}

func (b *bufferedWriter) Header() http.Header { return b.header }

func (b *bufferedWriter) WriteHeader(status int) {
	if b.status == http.StatusOK {
		b.status = status
	}
}

func (b *bufferedWriter) Write(p []byte) (int, error) {
	b.body = append(b.body, p...)
	return len(p), nil
}

// flushTo replays the held response, headers included — which is what makes the
// sign-in path work: its Set-Cookie was written into this buffer.
func (b *bufferedWriter) flushTo(w http.ResponseWriter) {
	for key, values := range b.header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(b.status)
	if len(b.body) > 0 {
		if _, err := w.Write(b.body); err != nil {
			slog.Warn("control plane: could not write the response", "error", err)
		}
	}
}
