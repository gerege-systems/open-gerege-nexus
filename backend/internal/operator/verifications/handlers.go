/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package verifications is the address-verification ledger, across every
// organisation on the deployment.
//
// The workspace used to carry this screen one organisation at a time. The
// service behind it is the platform's — one credential, one quota, one
// provider — so "is it working" and "who has it written to" are the
// platform's questions, and an organisation's own administrator could only
// ever see a quarter of the answer.
//
// Read-only, deliberately: these rows are people's addresses and what they
// were asked to prove. Nothing here deletes one; housekeeping in the other
// plane ages them out on its own schedule.
package verifications

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/go-chi/chi/v5"
)

// Stats are the ledger in six numbers.
type Stats struct {
	Total       int     `json:"total"`
	Verified    int     `json:"verified"`
	Pending     int     `json:"pending"`
	Expired     int     `json:"expired"`
	Last24h     int     `json:"last_24h"`
	VerifiedPct float64 `json:"verified_pct"`
	Tenants     int     `json:"tenants"`
}

// Verification is one address the platform was asked to prove.
type Verification struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenant_id"`
	TenantName string     `json:"tenant_name"`
	Source     string     `json:"source"`
	Purpose    string     `json:"purpose"`
	Email      string     `json:"email"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	VerifiedAt *time.Time `json:"verified_at"`
}

// Overview is the screen.
type Overview struct {
	Stats  Stats          `json:"stats"`
	Recent []Verification `json:"recent"`
	Health Health         `json:"service"`
}

// Read builds it. limit bounds the ledger, not the counts: the numbers are of
// everything, the list is of the newest few.
func (s *Service) Read(ctx context.Context, limit int) (*Overview, error) {
	if limit <= 0 || limit > 200 {
		limit = 25
	}

	overview := &Overview{Recent: []Verification{}}

	scoped := operator.Scoped(ctx)
	// A pending row whose deadline has passed counts as expired even before
	// housekeeping rewrites it: the screen must never say somebody can still
	// act on a dead link. The workspace's own counter read it this way, and
	// the number would change meaning if this one did not.
	err := s.db.QueryRow(scoped, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE status = 'VERIFIED'),
		       COUNT(*) FILTER (WHERE status = 'PENDING' AND expires_at > NOW()),
		       COUNT(*) FILTER (WHERE status = 'EXPIRED' OR (status = 'PENDING' AND expires_at <= NOW())),
		       COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '24 hours'),
		       COUNT(DISTINCT tenant_id)
		  FROM workspace.email_verifications`).
		Scan(&overview.Stats.Total, &overview.Stats.Verified, &overview.Stats.Pending,
			&overview.Stats.Expired, &overview.Stats.Last24h, &overview.Stats.Tenants)
	if err != nil {
		return nil, fmt.Errorf("control plane: count the verifications: %w", err)
	}
	if overview.Stats.Total > 0 {
		overview.Stats.VerifiedPct = float64(overview.Stats.Verified) / float64(overview.Stats.Total) * 100
	}

	// LEFT JOIN, not JOIN: an organisation deleted after a verification was
	// written leaves the row behind, and a ledger that silently dropped those
	// would answer "who was written to" with less than the truth.
	rows, err := s.db.Query(scoped, `
		SELECT v.id::text, v.tenant_id::text, COALESCE(t.name, ''), v.source, v.purpose,
		       v.email, v.status, v.created_at, v.verified_at
		  FROM workspace.email_verifications v
		  LEFT JOIN registry.tenants t ON t.id = v.tenant_id
		 ORDER BY v.created_at DESC
		 LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("control plane: list the verifications: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item Verification
		if err := rows.Scan(&item.ID, &item.TenantID, &item.TenantName, &item.Source,
			&item.Purpose, &item.Email, &item.Status, &item.CreatedAt, &item.VerifiedAt); err != nil {
			return nil, fmt.Errorf("control plane: read a verification: %w", err)
		}
		overview.Recent = append(overview.Recent, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("control plane: read the verifications: %w", err)
	}

	if s.probe != nil {
		// Bounded here as well as in the prober: a slow provider should make
		// this screen say "unreachable", not make it hang.
		probeCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
		defer cancel()
		overview.Health = s.probe(probeCtx)
	}
	return overview, nil
}

func (s *Service) handleOverview(w http.ResponseWriter, r *http.Request) {
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	overview, err := s.Read(r.Context(), limit)
	if err != nil {
		fail(w, err, "could not read the verifications")
		return
	}
	httpx.JSON(w, http.StatusOK, overview)
}

// Routes are this screen's. One read, and the capability that governs reading
// what organisations have been doing.
func (s *Service) Routes(r chi.Router) {
	r.With(s.op.RequireCapability(operator.CapTenantRead)).
		Get("/email-verifications", s.handleOverview)
}
