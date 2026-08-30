/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package observability is the operator's view of how the deployment is
// holding up.
//
// It stands on internal/operator/operator, which decides who is asking and
// records what they did; nothing in this plane stands on this package.
package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// BackgroundJob is one thing the platform does on a timer, and how it went.
type BackgroundJob struct {
	Name    string     `json:"name"`
	LastRun *time.Time `json:"last_run"`
	OK      bool       `json:"ok"`
	Detail  string     `json:"detail"`
	// Pending is how many are waiting — scheduled reports that have never run,
	// for instance. Zero for jobs where the idea does not apply.
	Pending int `json:"pending"`
}

// backgroundJobs answers "is anything quietly not running".
//
// Silent failure is the failure mode of every one of these: a scheduled report
// nobody receives is noticed weeks later by the person who was expecting it,
// and a catalogue that has not synced for a month looks exactly like one that
// has nothing to fetch.
func (s *Service) backgroundJobs(ctx context.Context) []BackgroundJob {
	jobs := make([]BackgroundJob, 0, 3)
	ctx = operator.Scoped(ctx)

	var lastRun *time.Time
	var failures, pending int
	if err := s.db.QueryRow(ctx,
		`SELECT max(last_run_at),
		        count(*) FILTER (WHERE last_status NOT IN ('', 'ok') AND active),
		        count(*) FILTER (WHERE last_run_at IS NULL AND active)
		   FROM workspace.report_schedules`).Scan(&lastRun, &failures, &pending); err != nil {
		slog.Warn("control plane: could not read the scheduled reports", "error", err)
	} else {
		detail := ""
		if failures > 0 {
			detail = fmt.Sprintf("%d schedule(s) failed on their last run", failures)
		}
		jobs = append(jobs, BackgroundJob{
			Name: "scheduled_reports", LastRun: lastRun, OK: failures == 0,
			Detail: detail, Pending: pending,
		})
	}

	if s.catalogStatusFrom != nil {
		at, ok, detail := s.catalogStatusFrom()
		var lastSync *time.Time
		if !at.IsZero() {
			lastSync = &at
		}
		jobs = append(jobs, BackgroundJob{
			Name: "catalog_sync", LastRun: lastSync, OK: ok, Detail: detail,
		})
	}

	// The Өртөө channel used to be reported here — undelivered envelopes and
	// links that had gone quiet. The channel left this repository with its app
	// (client-gerege-nexus, modules/urtuu/channel), and so did the two tables
	// this read. A console that kept the panel would be reporting the health of
	// something this binary does not run; a deployment that carries the app
	// reports it through the app's own metrics.
	// The deletion sweep has no row of its own; what it leaves behind is the
	// organisations still counting down, which is the useful number anyway.
	var awaiting int
	if err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM registry.tenants WHERE deletion_scheduled_at IS NOT NULL`).Scan(&awaiting); err == nil {
		jobs = append(jobs, BackgroundJob{
			Name: "deletion_sweep", OK: true, Pending: awaiting,
		})
	}
	return jobs
}

// TenantTrouble is an organisation having a bad day.
type TenantTrouble struct {
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
	Failures int    `json:"failures"`
	Sample   string `json:"sample"`
}

// tenantTrouble is the per-organisation error view §E asks for.
//
// It comes from workspace.audit_events rather than from Prometheus, and that is a
// consequence of a decision made in the very first phase: **no tenant label on
// any metric**, because a label whose values are customers is a series count
// that only grows. The trade is that this question has to be answered from the
// database — which is a good answer anyway, since what an operator wants to
// know is "whose work is failing", and the audit trail records acts rather
// than requests.
func (s *Service) tenantTrouble(ctx context.Context) []TenantTrouble {
	rows, err := s.db.Query(operator.Scoped(ctx),
		`SELECT a.tenant_id::text, COALESCE(t.name, ''), count(*), min(a.action)
		   FROM workspace.audit_events a
		   LEFT JOIN registry.tenants t ON t.id = a.tenant_id
		  WHERE a.created_at > NOW() - INTERVAL '24 hours'
		    AND a.tenant_id IS NOT NULL
		    AND (a.action LIKE '%fail%' OR a.action LIKE '%error%' OR a.action LIKE '%denied%')
		  GROUP BY a.tenant_id, t.name
		 HAVING count(*) >= 5
		  ORDER BY count(*) DESC
		  LIMIT 10`)
	if err != nil {
		slog.Warn("control plane: could not read the per-organisation failures", "error", err)
		return []TenantTrouble{}
	}
	defer rows.Close()

	trouble := make([]TenantTrouble, 0, 4)
	for rows.Next() {
		var row TenantTrouble
		if err := rows.Scan(&row.TenantID, &row.Name, &row.Failures, &row.Sample); err != nil {
			slog.Warn("control plane: could not read a failure row", "error", err)
			return trouble
		}
		trouble = append(trouble, row)
	}
	// Хяналтын самбарын хувьд дутуу жагсаалт нь «асуудалгүй» гэсэн хариу:
	// уншилт тасарсныг чимээгүй өнгөрөөвөл тэр нь эрүүл байдлын дүр эсгэнэ.
	if err := rows.Err(); err != nil {
		slog.Warn("control plane: could not read every failure row", "error", err)
	}
	return trouble
}

// CatalogStatus is where the app catalogue came from and what is installed.
type CatalogStatus struct {
	LastSyncAt *time.Time     `json:"last_sync_at"`
	OK         bool           `json:"ok"`
	Detail     string         `json:"detail"`
	Apps       []AppInstalled `json:"apps"`
}

// AppInstalled is one app and how its versions are spread across organisations.
type AppInstalled struct {
	AppID    string         `json:"app_id"`
	Name     string         `json:"name"`
	Versions map[string]int `json:"versions"`
	Total    int            `json:"total"`
}

// CatalogStatus is where the catalogue came from and when it last answered.
func (s *Service) CatalogStatus(ctx context.Context) CatalogStatus {
	status := CatalogStatus{OK: true, Apps: []AppInstalled{}}
	if s.catalogStatusFrom != nil {
		at, ok, detail := s.catalogStatusFrom()
		if !at.IsZero() {
			status.LastSyncAt = &at
		}
		status.OK, status.Detail = ok, detail
	}

	// The version spread is what says whether a release actually landed. One
	// organisation left on the previous version of an app is invisible
	// everywhere else — it looks like a working deployment until somebody
	// telephones about a feature that is missing.
	rows, err := s.db.Query(operator.Scoped(ctx),
		`SELECT i.app_id, COALESCE(a.name, i.app_id), i.installed_version, count(*)
		   FROM workspace.app_installations i
		   LEFT JOIN registry.apps a ON a.id = i.app_id
		  WHERE i.enabled AND i.status = 'installed'
		  GROUP BY i.app_id, a.name, i.installed_version
		  ORDER BY i.app_id`)
	if err != nil {
		slog.Warn("control plane: could not read the installed versions", "error", err)
		return status
	}
	defer rows.Close()

	byApp := map[string]*AppInstalled{}
	for rows.Next() {
		var appID, name, version string
		var count int
		if err := rows.Scan(&appID, &name, &version, &count); err != nil {
			slog.Warn("control plane: could not read an installation row", "error", err)
			return status
		}
		app, known := byApp[appID]
		if !known {
			app = &AppInstalled{AppID: appID, Name: name, Versions: map[string]int{}}
			byApp[appID] = app
			status.Apps = append(status.Apps, AppInstalled{})
		}
		app.Versions[version] += count
		app.Total += count
	}

	status.Apps = status.Apps[:0]
	for _, app := range byApp {
		status.Apps = append(status.Apps, *app)
	}
	// Каталогийн дутуу жагсаалт нь «энэ апп хаана ч суугаагүй» гэж уншигдана.
	if err := rows.Err(); err != nil {
		slog.Warn("control plane: could not read every installed app", "error", err)
	}
	return status
}

// VersionInfo is what is actually running here.
type VersionInfo struct {
	Platform string `json:"platform"`
	Release  string `json:"release"`
	// Migration is the schema version the database is at, which is the number
	// that matters when a deployment half-landed.
	Migration int64      `json:"migration"`
	AppliedAt *time.Time `json:"migration_applied_at"`
}

// Version is what this binary claims to be.
func (s *Service) Version(ctx context.Context) VersionInfo {
	info := VersionInfo{
		Platform: s.platformVersion,
		Release:  firstNonEmpty(os.Getenv("RELEASE_VERSION"), os.Getenv("IMAGE_TAG")),
	}
	// goose's own table. Read rather than joined into anything: it is the one
	// place that says which migrations this database has actually seen, and a
	// deployment whose image is newer than its schema is a real and quiet
	// failure mode.
	if err := s.db.QueryRow(operator.Scoped(ctx),
		`SELECT version_id, tstamp FROM public.goose_db_version
		  WHERE is_applied ORDER BY id DESC LIMIT 1`).
		Scan(&info.Migration, &info.AppliedAt); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		slog.Warn("control plane: could not read the migration version", "error", err)
	}
	return info
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// Deployment.
//
// The console asks GitHub to run the workflow this repository already has. It
// does not ship anything, does not touch the server, and cannot: the token it
// uses is a fine-grained one with permission for exactly this workflow, and
// the console never sees the machine it runs on.

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	// No error path: Health degrades panel by panel and says which parts it
	// could not read. A console that answers 500 because Prometheus is down is
	// a console that is unavailable exactly when it is needed.
	httpx.JSON(w, http.StatusOK, s.Health(r.Context()))
}

// Routes are this screen's, mounted on the console's signed-in group. The
// capability each one asks for is the decision about who may see or do it.
func (s *Service) Routes(r chi.Router) {

	// The front page, and the operations behind it.
	r.With(s.op.RequireCapability(operator.CapTenantRead)).Get("/health", s.handleHealth)
	// Which scheduled report is the one the front page is counting.
	r.With(s.op.RequireCapability(operator.CapTenantRead)).Get("/report-schedules", s.handleSchedules)
}
