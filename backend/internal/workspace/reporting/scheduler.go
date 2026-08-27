/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Scheduled reports: one goroutine, one tick a minute, no second process.
 */

package reporting

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/async"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// scheduleLockKey is the advisory lock every replica contends for before
// claiming a due schedule.
//
// A PostgreSQL session-level advisory lock rather than a row lock or a leader
// election: it is one number, it is released when the connection goes away
// however it goes away, and it costs nothing when uncontended. The constant is
// arbitrary but must be unique within the database — anything else taking the
// same number would block this sweep and be blocked by it.
const scheduleLockKey int64 = 0x6E657875735F7270 // "nexus_rp"

// Scheduler runs due report schedules.
//
// One goroutine on a one-minute ticker, in the API process. Not a second binary
// and not a cron container: this platform is one process by design, and a
// scheduler that is a separate deployable is a second thing to roll out, watch
// and wake up for.
type Scheduler struct {
	engine    *Engine
	deliverer Deliverer
}

// NewScheduler builds it. deliverer may be nil, and nil means a due report is
// still produced and still recorded — with "delivery not configured" as its
// outcome, which is visible on the schedule screen.
func NewScheduler(engine *Engine, deliverer Deliverer) *Scheduler {
	return &Scheduler{engine: engine, deliverer: deliverer}
}

// Start launches the ticker. It returns immediately and runs until ctx ends.
func (s *Scheduler) Start(ctx context.Context) {
	async.Go("report-scheduler", func() {
		// Aligned to the top of the minute. A ticker started at :37 would ask
		// "is this minute due" at :37 of every minute, which is the same set of
		// minutes — but a schedule created for 09:00 firing at 09:00:37 looks
		// wrong to whoever created it, and the alignment costs one sleep.
		select {
		case <-time.After(time.Until(time.Now().Truncate(time.Minute).Add(time.Minute))):
		case <-ctx.Done():
			return
		}

		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		s.sweep(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.sweep(ctx)
			}
		}
	})
	slog.Info("scheduled reports are running", "delivery_configured", s.deliverer != nil)
}

// sweep finds the schedules due this minute and runs them.
func (s *Scheduler) sweep(ctx context.Context) {
	// A minute's worth of work at most: the next tick is a minute away, and a
	// sweep still running then would overlap itself.
	sweepCtx, cancel := context.WithTimeout(ctx, 55*time.Second)
	defer cancel()

	// On the platform's clock: "0 8 * * 1" means eight on Monday morning in
	// Ulaanbaatar, which is what the person who typed it meant. Matches reads
	// the wall-clock fields of whatever it is given, so the zone has to be
	// decided here rather than left to the container's.
	due, err := s.claimDue(sweepCtx, nexus.Now())
	if err != nil {
		slog.Error("reports: could not read the schedules", "error", err)
		return
	}
	for _, schedule := range due {
		s.run(sweepCtx, schedule)
	}
}

// dueSchedule is one claimed row.
type dueSchedule struct {
	ID         string
	TenantID   string
	ReportKey  string
	Name       string
	Params     map[string]string
	Format     Format
	Recipients []string
	CreatedBy  string
}

// claimDue reads the active schedules and takes the ones whose expression
// matches this minute, marking them run before anything is produced.
//
// The order is deliberate: claim first, work second. A schedule marked after a
// successful send would be sent twice by a replica that restarted between the
// two, and a report arriving twice is worse than one arriving late — the second
// copy is indistinguishable from a real one and its numbers may differ.
//
// The whole claim runs under one advisory lock on one connection, so two
// replicas ticking in the same second do not both see the same row as unclaimed.
// The lock is taken with pg_try_advisory_lock: a replica that cannot get it
// does nothing this minute, which is correct — another one is already doing it.
// connAcquirer is the one pool capability this file needs and nexus.DB does not
// carry. See claimDue for why it is not on the module-facing handle.
type connAcquirer interface {
	Acquire(ctx context.Context) (*pgxpool.Conn, error)
}

func (s *Scheduler) claimDue(ctx context.Context, now time.Time) ([]dueSchedule, error) {
	// The platform path: this sweep crosses every tenant deliberately, so it
	// runs as the login role and outside the row-level policies, the way the
	// other housekeeping sweeps do.
	ctx = nexus.WithoutTenant(ctx)

	// A pinned connection, because pg_try_advisory_lock is session-scoped: taken
	// on one connection and released on another it would do nothing at all.
	//
	// Asked for by assertion rather than through nexus.DB, and deliberately.
	// Pinning a connection out of the pool is a platform capability — this sweep
	// crosses every tenant and holds a lock the whole deployment shares — and
	// putting Acquire on the handle app modules are given would offer every
	// module the ability to exhaust the pool. The reports app is where this
	// happens to be started from; it is not the reports app's power.
	pool, ok := s.engine.DB().(connAcquirer)
	if !ok {
		// Only a test double reaches this. Doing nothing is the correct sweep
		// for a handle that cannot hold a lock: the alternative is running it
		// unlocked on every replica at once.
		slog.Warn("reports: the schedule sweep needs a connection pool; skipping")
		return nil, nil
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("take a connection for the schedule sweep: %w", err)
	}
	defer conn.Release()

	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, scheduleLockKey).Scan(&locked); err != nil {
		return nil, fmt.Errorf("take the schedule lock: %w", err)
	}
	if !locked {
		return nil, nil
	}
	defer func() {
		if _, err := conn.Exec(context.WithoutCancel(ctx),
			`SELECT pg_advisory_unlock($1)`, scheduleLockKey); err != nil {
			// Not fatal: the lock is released when the connection closes, and
			// the pool closes connections. Worth a line because a lock left
			// held would stop every later sweep on every replica.
			slog.Warn("reports: could not release the schedule lock", "error", err)
		}
	}()

	rows, err := conn.Query(ctx, `
		SELECT id, tenant_id, report_key, name, params, cron, format, recipients,
		       coalesce(created_by::text, ''), last_run_at
		  FROM workspace.report_schedules
		 WHERE active`)
	if err != nil {
		return nil, err
	}

	minute := now.Truncate(time.Minute)
	var due []dueSchedule
	var claimed []string

	for rows.Next() {
		var (
			schedule   dueSchedule
			expression string
			format     string
			params     map[string]any
			lastRun    *time.Time
		)
		if err := rows.Scan(&schedule.ID, &schedule.TenantID, &schedule.ReportKey,
			&schedule.Name, &params, &expression, &format, &schedule.Recipients,
			&schedule.CreatedBy, &lastRun); err != nil {
			rows.Close()
			return nil, err
		}

		parsed, err := ParseCron(expression)
		if err != nil {
			// Stored expressions are validated on write, so this is a row that
			// predates the validation or was edited in SQL. Skipped and named
			// rather than crashing the sweep for every other schedule.
			slog.Warn("reports: a schedule has an unreadable expression",
				"schedule_id", schedule.ID, "cron", expression, "error", err)
			continue
		}
		if !parsed.Matches(minute) {
			continue
		}
		// Already run this minute — by this replica a moment ago, or by another
		// one before it took the lock.
		if lastRun != nil && !lastRun.Truncate(time.Minute).Before(minute) {
			continue
		}

		schedule.Format = Format(format)
		schedule.Params = stringifyParams(params)
		due = append(due, schedule)
		claimed = append(claimed, schedule.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(claimed) > 0 {
		if _, err := conn.Exec(ctx,
			`UPDATE workspace.report_schedules SET last_run_at = $2, updated_at = NOW() WHERE id = ANY($1)`,
			claimed, minute); err != nil {
			// Nothing is sent if the claim could not be written. Sending
			// anyway would be the double-delivery this whole function exists
			// to prevent.
			return nil, fmt.Errorf("claim the due schedules: %w", err)
		}
	}
	return due, nil
}

// run produces one scheduled report and delivers it.
func (s *Scheduler) run(ctx context.Context, schedule dueSchedule) {
	status, failure := "SENT", ""

	if err := s.produceAndDeliver(ctx, schedule); err != nil {
		status = "FAILED"
		failure = err.Error()
		slog.Error("reports: a scheduled report did not go out",
			"schedule_id", schedule.ID, "report", schedule.ReportKey, "error", err)
	}

	// Recorded whichever way it went. A schedule that has been failing for a
	// month is exactly the thing nobody notices: the report simply stops
	// arriving, and only the recipient knows.
	if _, err := s.engine.DB().Exec(nexus.WithoutTenant(context.WithoutCancel(ctx)),
		`UPDATE workspace.report_schedules SET last_status = $2, last_error = $3, updated_at = NOW() WHERE id = $1`,
		schedule.ID, status, truncate(failure, 500)); err != nil {
		slog.Error("reports: could not record a scheduled run", "schedule_id", schedule.ID, "error", err)
	}
}

func (s *Scheduler) produceAndDeliver(ctx context.Context, schedule dueSchedule) error {
	report, ok := Get(schedule.ReportKey)
	if !ok {
		return fmt.Errorf("report %q is not in this build", schedule.ReportKey)
	}

	params, err := Bind(report, schedule.Params, "mn")
	if err != nil {
		return fmt.Errorf("the stored parameters are no longer valid: %w", err)
	}

	result, err := s.engine.Run(ctx, schedule.TenantID, report, params)
	if err != nil {
		return fmt.Errorf("run the report: %w", err)
	}

	title := LocalizedTitle(report.Titles(), "mn", report.Key())
	payload, err := Export(schedule.Format, title, result, "mn")
	if err != nil {
		return fmt.Errorf("render the export: %w", err)
	}

	// Audited before delivery, and audited whether or not delivery is
	// configured: the fact that the numbers were read out of the database is
	// the auditable act. Who received them is in the same record.
	audit.Record(nexus.WithTenantID(ctx, schedule.TenantID), schedule.TenantID, schedule.CreatedBy,
		"reports.scheduled_run", schedule.ReportKey, map[string]any{
			"schedule_id": schedule.ID,
			"format":      string(schedule.Format),
			"rows":        len(result.Rows),
			"recipients":  len(schedule.Recipients),
		})

	if s.deliverer == nil {
		return ErrDeliveryNotConfigured
	}

	subject := title
	if schedule.Name != "" {
		subject = schedule.Name + " — " + title
	}
	body := fmt.Sprintf(
		"%s\n\nХугацаа: %s\nМөрийн тоо: %d\n\nЭнэ бол Gerege Nexus-ийн товлосон тайлан. Хавсралтыг үзнэ үү.\n",
		title, nexus.Now().Format("2006-01-02 15:04"), len(result.Rows))

	return s.deliverer.Deliver(ctx, schedule.Recipients, subject, body,
		Filename(report.Key(), schedule.Format), payload)
}

// stringifyParams turns a stored JSON parameter object back into the string
// form Bind validates.
//
// Round-tripping through strings rather than trusting the JSON types is what
// makes a schedule stored last year still safe to run: the report's declaration
// may have changed since, and Bind is the only thing that knows what it accepts
// now.
func stringifyParams(params map[string]any) map[string]string {
	raw := make(map[string]string, len(params))
	for key, value := range params {
		switch typed := value.(type) {
		case string:
			raw[key] = typed
		case bool:
			raw[key] = fmt.Sprint(typed)
		case float64:
			raw[key] = fmt.Sprintf("%v", typed)
		case nil:
			raw[key] = ""
		default:
			raw[key] = fmt.Sprint(typed)
		}
	}
	return raw
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	// Back up to the start of the rune the cut landed inside: a failure
	// message is prose, and Cyrillic prose cut mid-rune is stored as mojibake.
	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}
	return value[:limit]
}

// scanRecipients is here for the handlers, which read a schedule row back to
// the screen and need the same column list.
const scheduleColumns = `id, report_key, name, params, cron, format, recipients, active,
	last_run_at, last_status, last_error, created_at`

// Schedule row as the API returns it.
type ScheduleRow struct {
	ID         string            `json:"id"`
	ReportKey  string            `json:"report_key"`
	Name       string            `json:"name"`
	Params     map[string]any    `json:"params"`
	Cron       string            `json:"cron"`
	Format     string            `json:"format"`
	Recipients []string          `json:"recipients"`
	Active     bool              `json:"active"`
	LastRunAt  *time.Time        `json:"last_run_at,omitempty"`
	LastStatus string            `json:"last_status,omitempty"`
	LastError  string            `json:"last_error,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	Titles     map[string]string `json:"titles,omitempty"`
}

// ListSchedules returns one tenant's schedules.
func ListSchedules(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, tenantID string) ([]ScheduleRow, error) {

	rows, err := q.Query(ctx,
		`SELECT `+scheduleColumns+` FROM workspace.report_schedules
		  WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schedules := make([]ScheduleRow, 0, 8)
	for rows.Next() {
		var row ScheduleRow
		if err := rows.Scan(&row.ID, &row.ReportKey, &row.Name, &row.Params, &row.Cron,
			&row.Format, &row.Recipients, &row.Active, &row.LastRunAt,
			&row.LastStatus, &row.LastError, &row.CreatedAt); err != nil {
			return nil, err
		}
		if report, ok := Get(row.ReportKey); ok {
			row.Titles = report.Titles()
		}
		schedules = append(schedules, row)
	}
	return schedules, rows.Err()
}
