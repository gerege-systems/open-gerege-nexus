/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package announce is what the platform tells the organisations on it.
//
// It stands on internal/operator/operator, which decides who is asking and
// records what they did; nothing in this plane stands on this package.
package announce

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Announcement is one thing to tell people.
type Announcement struct {
	ID        string     `json:"id"`
	TenantID  *string    `json:"tenant_id"`
	Kind      string     `json:"kind"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	StartsAt  time.Time  `json:"starts_at"`
	EndsAt    *time.Time `json:"ends_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// ListAnnouncements returns them, newest first.
func (s *Service) ListAnnouncements(ctx context.Context) ([]Announcement, error) {
	rows, err := s.db.Query(operator.Scoped(ctx),
		`SELECT id::text, tenant_id::text, kind, title, body, starts_at, ends_at, created_at
		   FROM registry.announcements ORDER BY starts_at DESC LIMIT 100`)
	if err != nil {
		return nil, fmt.Errorf("control plane: list the announcements: %w", err)
	}
	defer rows.Close()

	announcements := make([]Announcement, 0, 8)
	for rows.Next() {
		var announcement Announcement
		if err := rows.Scan(&announcement.ID, &announcement.TenantID, &announcement.Kind,
			&announcement.Title, &announcement.Body, &announcement.StartsAt,
			&announcement.EndsAt, &announcement.CreatedAt); err != nil {
			return nil, fmt.Errorf("control plane: read an announcement: %w", err)
		}
		announcements = append(announcements, announcement)
	}
	return announcements, rows.Err()
}

// Announce broadcasts something, to everybody or to one organisation.
func (s *Service) Announce(ctx context.Context, sess operator.Session, announcement Announcement, reason string) error {
	if announcement.Title == "" {
		return errors.New("an announcement needs something to say")
	}
	switch announcement.Kind {
	case "info", "warning", "maintenance":
	case "":
		announcement.Kind = "info"
	default:
		return fmt.Errorf("%q is not a kind of announcement", announcement.Kind)
	}

	return s.op.Do(ctx, sess, operator.Change{
		Action:     "announcement.create",
		TargetType: "announcement",
		TargetID:   valueOr(announcement.TenantID, "all"),
		Reason:     reason,
		After:      map[string]any{"title": announcement.Title, "kind": announcement.Kind},
	}, func(ctx context.Context, tx pgx.Tx) error {
		starts := announcement.StartsAt
		if starts.IsZero() {
			starts = time.Now()
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO registry.announcements (tenant_id, kind, title, body, starts_at, ends_at, created_by)
			 VALUES (NULLIF($1, '')::uuid, $2, $3, $4, $5, $6, $7::uuid)`,
			valueOr(announcement.TenantID, ""), announcement.Kind, announcement.Title,
			announcement.Body, starts, announcement.EndsAt, sess.ID)
		return err
	})
}

// WithdrawAnnouncement removes one.
func (s *Service) WithdrawAnnouncement(ctx context.Context, sess operator.Session, id, reason string) error {
	return s.op.Do(ctx, sess, operator.Change{
		Action:     "announcement.withdraw",
		TargetType: "announcement",
		TargetID:   id,
		Reason:     reason,
	}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM registry.announcements WHERE id = $1::uuid`, id)
		return err
	})
}

// valueOr dereferences a pointer or gives a fallback, which the two callers
// above would otherwise each write for themselves.
func valueOr(value *string, fallback string) string {
	if value == nil || *value == "" {
		return fallback
	}
	return *value
}

func (s *Service) handleListAnnouncements(w http.ResponseWriter, r *http.Request) {
	announcements, err := s.ListAnnouncements(r.Context())
	if err != nil {
		fail(w, err, "could not read the announcements")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"announcements": announcements})
}

func (s *Service) handleAnnounce(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		Announcement
		Reason string `json:"reason"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.Announce(r.Context(), sess, body.Announcement, body.Reason); err != nil {
		fail(w, err, "could not publish the announcement")
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{"status": "published"})
}

func (s *Service) handleWithdrawAnnouncement(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body operator.Reasoned
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.WithdrawAnnouncement(r.Context(), sess, chi.URLParam(r, "id"), body.Reason); err != nil {
		fail(w, err, "could not withdraw the announcement")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "withdrawn"})
}

// Routes are this screen's, mounted on the console's signed-in group. The
// capability each one asks for is the decision about who may see or do it.
func (s *Service) Routes(r chi.Router) {

	r.With(s.op.RequireCapability(operator.CapTenantRead)).Get("/announcements", s.handleListAnnouncements)
	r.With(s.op.RequireCapability(operator.CapAnnounce)).Post("/announcements", s.handleAnnounce)
	r.With(s.op.RequireCapability(operator.CapAnnounce)).
		Delete("/announcements/{id}", s.handleWithdrawAnnouncement)
}
