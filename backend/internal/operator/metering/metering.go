/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package metering counts what each organisation used, once a day.
//
// It exists for two readers. The console shows it, so an operator can see who
// is growing and who is about to cross a limit; and the quota checks read it,
// so a limit on AI calls or storage is enforced against a number somebody can
// point at rather than one computed differently in each handler.
//
// It counts organisations. A person's home is a workspace by mechanism and not
// by purpose — migration 00085 gives one to everybody who signs in, and since
// "everybody lands at home" that is everybody, employed or not. Counting them
// would put a row per citizen per day into the table a bill is read from, which
// is wrong twice: it grows with the population rather than with the customer
// list, and "how much was this platform used" starts answering with the
// country.
//
// How many citizens are active is a real question and a different one. It is a
// property of the deployment rather than of two hundred thousand workspaces,
// and when something needs it, it belongs on the deployment's own screen rather
// than as rows here.
//
// Three decisions shape it, and each is a road not taken:
//
//   - **Not from Prometheus.** The first phase decided that no metric would
//     carry a tenant label, because a label whose values are customers is a
//     series count that only ever grows. The price is that this counting
//     happens in SQL — and the profit is that it counts *acts* recorded in the
//     audit trail rather than HTTP requests, which is what a bill should be
//     based on.
//   - **Not an event per request.** A row per API call is a table nobody can
//     query by the second month. What is stored is the day's total, one row
//     per organisation per metric, written by a job that runs after midnight.
//   - **Not derived on read.** The numbers could be computed from the source
//     tables whenever somebody asks, and they would be correct until the day
//     an organisation is deleted or a retention sweep removes the rows they
//     were counted from. A usage record has to outlive what it counted.
package metering

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/async"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/usage"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Metrics is the list, in the order the console shows them.
func Metrics() []string {
	return []string{usage.ActiveUsers, usage.Actions, usage.AICalls, usage.ReportsSent, usage.StorageMB}
}

// Collector writes the daily rows.
type Collector struct{ db *pgxpool.Pool }

// NewCollector builds the nightly job that writes the daily rows.
func NewCollector(db *pgxpool.Pool) *Collector { return &Collector{db: db} }

// Start runs the collection shortly after every midnight, and once at startup.
//
// The startup run is what makes a deployment that was switched off overnight
// catch up: CollectDay is idempotent — the primary key is (tenant, day,
// metric) and the write is an upsert — so running it again for a day already
// counted rewrites the same numbers.
func (c *Collector) Start(ctx context.Context) {
	async.Go("usage-metering", func() {
		// Yesterday, at startup, because today is not over.
		c.CollectDay(ctx, time.Now().AddDate(0, 0, -1))
		for {
			wait := untilNextRun(time.Now())
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			c.CollectDay(ctx, time.Now().AddDate(0, 0, -1))
			// And today's numbers so far, so the console is not a day behind
			// for anybody looking at it in the afternoon. Rewritten on every
			// run until the day ends and the figure settles.
			c.CollectDay(ctx, time.Now())
		}
	})
}

// collectionHour is when the day's totals are taken: a little after midnight,
// late enough that anything still finishing at 23:59 has landed.
const collectionHour = 1

func untilNextRun(now time.Time) time.Duration {
	// On the platform's clock, so "a little after midnight" is the midnight the
	// people using this platform have rather than the one the container was
	// built with.
	now = now.In(nexus.Location())
	next := time.Date(now.Year(), now.Month(), now.Day(), collectionHour, 10, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next.Sub(now)
}

// CollectDay counts one day for every organisation.
//
// One statement per metric rather than one per organisation: a deployment with
// two hundred organisations would otherwise make a thousand round trips a
// night, and the aggregate is what SQL is for.
func (c *Collector) CollectDay(ctx context.Context, day time.Time) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	for metric, query := range queries {
		if err := c.collect(ctx, metric, query, day); err != nil {
			// One metric failing must not stop the others: a platform whose
			// storage query is slow should still know how many people signed
			// in.
			slog.Warn("metering: could not collect a metric",
				"metric", metric, "day", day.In(nexus.Location()).Format("2006-01-02"),
				"error", err)
		}
	}
}

// queries is the whole of the counting.
//
// Each takes the day as $1 and produces (tenant_id, value). They run on the
// platform path — the collector is not anybody's request — and every one of
// them is a plain aggregate over a table that carries a tenant_id.
//
// `$1::timestamptz::date` and not `$1::date`, and the difference is the whole
// bug this once had. A bare `$1::date` lets Postgres infer the parameter as a
// date, so the driver reduces the Go value to a calendar date using *its own*
// zone before the database ever sees it — which is the process's zone, not the
// platform's. Casting through timestamptz forces the parameter to be an
// instant, and the reduction then happens in the session's zone, the same one
// `created_at::date` uses. One rule, applied on one side of the wire.
var queries = map[string]string{
	// Somebody whose session was used that day. `last_seen_at` rather than
	// created_at: a person who signed in on Monday and worked through Friday
	// is active every one of those days, which is what an "active user" means
	// to whoever is paying for one.
	usage.ActiveUsers: `
		SELECT tenant_id, count(DISTINCT user_id)
		  FROM workspace.sessions
		 WHERE last_seen_at::date = $1::timestamptz::date
		 GROUP BY tenant_id`,

	usage.Actions: `
		SELECT tenant_id, count(*)
		  FROM workspace.audit_events
		 WHERE created_at::date = $1::timestamptz::date AND tenant_id IS NOT NULL
		 GROUP BY tenant_id`,

	usage.AICalls: `
		SELECT tenant_id, count(*)
		  FROM workspace.audit_events
		 WHERE created_at::date = $1::timestamptz::date AND tenant_id IS NOT NULL
		   AND action LIKE 'ai.%'
		 GROUP BY tenant_id`,

	usage.ReportsSent: `
		SELECT tenant_id, count(*)
		  FROM workspace.audit_events
		 WHERE created_at::date = $1::timestamptz::date AND tenant_id IS NOT NULL
		   AND action LIKE 'reports.%'
		 GROUP BY tenant_id`,

	// Storage is a state rather than an event: what is being kept *now*, not
	// what arrived that day. It is written against the day it was measured, so
	// the series reads as "how much they were holding on the 3rd".
	//
	// The signed documents are where the bytes are on this platform. The size
	// is a column the upload already wrote (esign_documents.byte_size), so
	// this is a sum over a number rather than over the blobs themselves —
	// which matters on a table whose rows are megabytes each.
	usage.StorageMB: `
		SELECT tenant_id, ceil(sum(byte_size) / 1048576.0)::bigint
		  FROM workspace.esign_documents
		 GROUP BY tenant_id`,
}

// collect writes one metric's rows for one day.
//
// The day and the metric name are parameters of the *outer* statement, so a
// counting query is free to ignore the day — the storage one does, because it
// measures what is being kept now rather than what happened then.
//
// Homes are dropped here rather than in the five counting queries, and this
// seam is the point of it: one join instead of five identical ones, a sixth
// metric added later inherits it, and there is no version of this that counts a
// citizen because somebody forgot a clause.
//
// The day is passed as an *instant* and reduced to a date by Postgres, not
// formatted here — see the note on `queries` for why the cast has to go through
// timestamptz for that to actually be true. Formatting it in Go used the
// process's zone while `created_at::date` used the database's, so on a machine
// east of UTC the collector spent the small hours counting a day the database
// had not reached: every figure came back zero, and nothing said why.
func (c *Collector) collect(ctx context.Context, metric, query string, day time.Time) error {
	// The insert and the count in one statement: the alternative reads every
	// row into Go to write it straight back, which is a round trip per
	// organisation for no reason.
	_, err := c.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO registry.usage_events (tenant_id, day, metric, value, recorded_at)
		SELECT counted.tenant_id, $1::timestamptz::date, $2, counted.value, NOW()
		  FROM (%s) AS counted(tenant_id, value)
		  JOIN registry.tenants t ON t.id = counted.tenant_id AND t.kind = 'organisation'
		ON CONFLICT (tenant_id, day, metric)
		DO UPDATE SET value = EXCLUDED.value, recorded_at = NOW()`, query),
		day, metric)
	return err
}
