/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package observability

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
)

// The scheduled reports, across every organisation.
//
// The front page already counts them — how many have never run, how many
// failed — because a scheduled report that stops arriving is noticed weeks
// later by the person who was expecting it. A count says something is wrong
// without saying which one, and the operator's next question is always "which
// organisation, and what did it say".

// Schedule is one organisation's one scheduled report.
type Schedule struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenant_id"`
	TenantName string     `json:"tenant_name"`
	Name       string     `json:"name"`
	ReportKey  string     `json:"report_key"`
	Cron       string     `json:"cron"`
	Format     string     `json:"format"`
	Recipients []string   `json:"recipients"`
	Active     bool       `json:"active"`
	LastRunAt  *time.Time `json:"last_run_at"`
	LastStatus string     `json:"last_status"`
}

// Schedules lists them, the ones in trouble first: never run, then failed,
// then the rest by organisation. An operator opening this screen is looking
// for what is broken, and sorting by name would hide it on page two.
func (s *Service) Schedules(ctx context.Context) ([]Schedule, error) {
	rows, err := s.db.Query(operator.Scoped(ctx), `
		SELECT r.id::text, r.tenant_id::text, COALESCE(t.name, ''), r.name, r.report_key,
		       r.cron, r.format, r.recipients, r.active, r.last_run_at,
		       COALESCE(r.last_status, '')
		  FROM workspace.report_schedules r
		  LEFT JOIN registry.tenants t ON t.id = r.tenant_id
		 ORDER BY (r.active AND r.last_run_at IS NULL) DESC,
		          (r.active AND COALESCE(r.last_status, '') NOT IN ('', 'ok')) DESC,
		          t.name, r.name
		 LIMIT 200`)
	if err != nil {
		return nil, fmt.Errorf("control plane: read the scheduled reports: %w", err)
	}
	defer rows.Close()

	schedules := make([]Schedule, 0, 16)
	for rows.Next() {
		var schedule Schedule
		if err := rows.Scan(&schedule.ID, &schedule.TenantID, &schedule.TenantName,
			&schedule.Name, &schedule.ReportKey, &schedule.Cron, &schedule.Format,
			&schedule.Recipients, &schedule.Active, &schedule.LastRunAt,
			&schedule.LastStatus); err != nil {
			return nil, fmt.Errorf("control plane: read a scheduled report: %w", err)
		}
		schedules = append(schedules, schedule)
	}
	return schedules, rows.Err()
}

func (s *Service) handleSchedules(w http.ResponseWriter, r *http.Request) {
	schedules, err := s.Schedules(r.Context())
	if err != nil {
		operator.Fail(w, err, "could not read the scheduled reports")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"schedules": schedules})
}
