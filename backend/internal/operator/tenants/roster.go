/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package tenants

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
)

// Two reads across every organisation at once.
//
// Both answer questions the per-organisation pages cannot: which limits are
// set where, and which app is installed where. Asking them one organisation at
// a time is how a platform with forty of them stops asking.

// QuotaLine is one organisation's limits, with the organisation named.
type QuotaLine struct {
	Quota
	TenantName string `json:"tenant_name"`
	Slug       string `json:"slug"`
	Suspended  bool   `json:"suspended"`
}

// ListQuotas reads every organisation's limits in one pass.
//
// An organisation with no row in tenant_quotas is listed with no limits rather
// than left out: "nobody has set limits here" is the answer the screen is for.
func (s *Service) ListQuotas(ctx context.Context) ([]QuotaLine, error) {
	rows, err := s.db.Query(operator.Scoped(ctx), `
		SELECT t.id::text, t.name, t.slug, t.suspended_at IS NOT NULL,
		       q.max_users, q.max_storage_mb, q.max_ai_calls_monthly,
		       COALESCE(q.enforcement, 'soft'), COALESCE(q.updated_at, t.created_at),
		       (SELECT count(*) FROM workspace.memberships m WHERE m.tenant_id = t.id)
		  FROM registry.tenants t
		  LEFT JOIN registry.tenant_quotas q ON q.tenant_id = t.id
		 ORDER BY t.name`)
	if err != nil {
		return nil, fmt.Errorf("control plane: list the limits: %w", err)
	}
	defer rows.Close()

	lines := make([]QuotaLine, 0, 16)
	for rows.Next() {
		line := QuotaLine{Quota: Quota{Enforced: []string{"users"}}}
		if err := rows.Scan(&line.TenantID, &line.TenantName, &line.Slug, &line.Suspended,
			&line.MaxUsers, &line.MaxStorageMB, &line.MaxAICallsMonthly,
			&line.Enforcement, &line.UpdatedAt, &line.Users); err != nil {
			return nil, fmt.Errorf("control plane: read a limit: %w", err)
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

// Installation is one app in one organisation.
type Installation struct {
	TenantID    string    `json:"tenant_id"`
	TenantName  string    `json:"tenant_name"`
	Slug        string    `json:"slug"`
	AppID       string    `json:"app_id"`
	AppName     string    `json:"app_name"`
	Version     string    `json:"installed_version"`
	Status      string    `json:"status"`
	Enabled     bool      `json:"enabled"`
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ListInstallations reads which app is installed where.
//
// The catalogue screen counts versions across the platform, which says that
// two organisations are behind without saying which two. This is that list.
func (s *Service) ListInstallations(ctx context.Context) ([]Installation, error) {
	rows, err := s.db.Query(operator.Scoped(ctx), `
		SELECT i.tenant_id::text, t.name, t.slug,
		       i.app_id, COALESCE(a.name, i.app_id),
		       i.installed_version, i.status, i.enabled, i.installed_at, i.updated_at
		  FROM workspace.app_installations i
		  JOIN registry.tenants t ON t.id = i.tenant_id
		  LEFT JOIN registry.apps a ON a.id = i.app_id
		 ORDER BY t.name, COALESCE(a.name, i.app_id)`)
	if err != nil {
		return nil, fmt.Errorf("control plane: list the installations: %w", err)
	}
	defer rows.Close()

	installations := make([]Installation, 0, 32)
	for rows.Next() {
		var item Installation
		if err := rows.Scan(&item.TenantID, &item.TenantName, &item.Slug,
			&item.AppID, &item.AppName, &item.Version, &item.Status, &item.Enabled,
			&item.InstalledAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("control plane: read an installation: %w", err)
		}
		installations = append(installations, item)
	}
	return installations, rows.Err()
}

func (s *Service) handleListQuotas(w http.ResponseWriter, r *http.Request) {
	quotas, err := s.ListQuotas(r.Context())
	if err != nil {
		fail(w, err, "could not read the limits")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"quotas": quotas})
}

func (s *Service) handleListInstallations(w http.ResponseWriter, r *http.Request) {
	installations, err := s.ListInstallations(r.Context())
	if err != nil {
		fail(w, err, "could not read the installations")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"installations": installations})
}
