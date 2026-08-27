/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The boundary at usage_events, from both sides.
 *
 * The platform writes the daily rows (internal/operator/metering) and a tenant
 * reads its own month to check a limit against — one of the five tables
 * ownership_test.go marks as the meeting point of the two planes. The names and
 * the read live here because both planes need them and neither owns the other:
 * a second copy of the metric names is a second closed list, and the first
 * quota checked against the wrong string passes for ever.
 *
 * The write stays with the collector. Kernel holds the crossing, not the table.
 */

package usage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The metrics, as a closed list. A name that is not here is never written, so
// the console and the quota checks can rely on what they find.
const (
	// ActiveUsers is how many distinct people used the organisation that day.
	ActiveUsers = "active_users"
	// Actions is every act recorded in the organisation's audit trail — the
	// closest honest answer to "how much did they use the platform".
	//
	// Deliberately not called api_calls: it does not count requests, it counts
	// the things worth recording, and naming it after what it is avoids an
	// invoice line nobody can reconcile.
	Actions = "actions"
	// AICalls is copilot, chat, transcription, speech and translation.
	AICalls = "ai_calls"
	// ReportsSent is scheduled reports delivered.
	ReportsSent = "reports_sent"
	// StorageMB is what the organisation is keeping — the signed documents and
	// the files behind them, which is where the bytes on this platform are.
	StorageMB = "storage_mb"
)

// MonthToDate is what a monthly quota is checked against.
//
// The current month, from the first to today, for one organisation and one
// metric. Today's own row is included and is rewritten by every collection, so
// a limit is enforced against numbers that are hours old at worst — which is
// the right trade for a check that runs on the request path.
func MonthToDate(ctx context.Context, db *pgxpool.Pool, tenantID, metric string) (int64, error) {
	var total int64
	if err := db.QueryRow(ctx,
		`SELECT COALESCE(sum(value), 0) FROM registry.usage_events
		  WHERE tenant_id = $1::uuid AND metric = $2
		    AND day >= date_trunc('month', CURRENT_DATE)`,
		tenantID, metric).Scan(&total); err != nil {
		return 0, fmt.Errorf("metering: read the month's %s: %w", metric, err)
	}
	return total, nil
}
