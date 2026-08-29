/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package metering

import (
	"context"
	"fmt"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
)

// The same numbers as UsageFor, one row per organisation instead of one chart
// for one of them.
//
// A per-organisation screen answers "is this one near its limit". It cannot
// answer "who is using this deployment", which is the question behind a
// capacity decision or an invoice run, and answering it by opening every
// organisation in turn is how a platform with forty of them stops asking.

// TenantUsage is one organisation's line in the platform report.
type TenantUsage struct {
	TenantID   string           `json:"tenant_id"`
	TenantName string           `json:"tenant_name"`
	Slug       string           `json:"slug"`
	Suspended  bool             `json:"suspended"`
	Metrics    map[string]int64 `json:"metrics"`
	// Collected is when anything was last counted for this organisation. Null
	// is "never counted", which reads differently from a row of zeroes.
	Collected *time.Time `json:"collected"`
}

// PlatformUsage is the report: every organisation, month to date.
type PlatformUsage struct {
	Month   string           `json:"month"`
	Metrics []string         `json:"metrics"`
	Tenants []TenantUsage    `json:"tenants"`
	Totals  map[string]int64 `json:"totals"`
}

// PlatformUsageReport rolls the current month up across every organisation.
//
// Each metric is aggregated the way its own meaning demands, exactly as the
// per-organisation screen does: counted metrics are summed, storage is the
// latest reading, and active users is the month's peak. Summing daily active
// users across a month would count one person thirty times and make the
// biggest organisation look thirty times bigger than it is.
func (s *Service) PlatformUsageReport(ctx context.Context) (PlatformUsage, error) {
	ctx = operator.Scoped(ctx)
	month := time.Now().Format("2006-01")
	report := PlatformUsage{
		Month:   month,
		Metrics: Metrics(),
		Tenants: []TenantUsage{},
		Totals:  map[string]int64{},
	}

	rows, err := s.db.Query(ctx, `
		SELECT t.id::text, t.name, t.slug, t.suspended_at IS NOT NULL,
		       COALESCE(u.metric, ''),
		       COALESCE(u.total, 0),
		       u.collected
		  FROM registry.tenants t
		  LEFT JOIN (
		        SELECT tenant_id, metric,
		               CASE
		                 WHEN metric = 'storage_mb'   THEN MAX(value)
		                 WHEN metric = 'active_users' THEN MAX(value)
		                 ELSE SUM(value)
		               END AS total,
		               MAX(recorded_at) AS collected
		          FROM registry.usage_events
		         WHERE to_char(day, 'YYYY-MM') = $1
		         GROUP BY tenant_id, metric
		  ) u ON u.tenant_id = t.id
		 ORDER BY t.name, u.metric`, month)
	if err != nil {
		return PlatformUsage{}, fmt.Errorf("control plane: read the platform usage: %w", err)
	}
	defer rows.Close()

	// One row per organisation per metric arrives; the report wants one line
	// per organisation, so they are folded here rather than in the query,
	// which would need a pivot naming every metric and a migration to add one.
	index := map[string]int{}
	for rows.Next() {
		var id, name, slug, metric string
		var suspended bool
		var total int64
		var collected *time.Time
		if err := rows.Scan(&id, &name, &slug, &suspended, &metric, &total, &collected); err != nil {
			return PlatformUsage{}, fmt.Errorf("control plane: read a usage row: %w", err)
		}
		position, seen := index[id]
		if !seen {
			report.Tenants = append(report.Tenants, TenantUsage{
				TenantID: id, TenantName: name, Slug: slug, Suspended: suspended,
				Metrics: map[string]int64{},
			})
			position = len(report.Tenants) - 1
			index[id] = position
		}
		if metric == "" {
			continue
		}
		report.Tenants[position].Metrics[metric] = total
		if collected != nil {
			if at := report.Tenants[position].Collected; at == nil || collected.After(*at) {
				report.Tenants[position].Collected = collected
			}
		}
		report.Totals[metric] += total
	}
	return report, rows.Err()
}
