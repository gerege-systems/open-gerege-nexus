/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * What this organisation says it does, in public.
 *
 * Beside the legal profile because it is the same kind of thing: a statement
 * the organisation makes about itself, read by people who are not in it. The
 * difference is who it is for — the profile answers "who are you" for a consent
 * screen and a registry, and this answers "what do you do" for somebody looking
 * for a service.
 *
 * Publishing is a separate act from accepting the work. An organisation that
 * handles a kind of request has decided something about its own queue;
 * appearing in a deployment-wide directory is a promise to strangers. Migration
 * 00090 keeps those two apart by making this table the publication itself.
 */

package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
)

// ErrLocalCodeIsPrivate is somebody trying to publish a code from their own
// namespace. 00062 set that namespace aside for codes an organisation invents
// for itself; a deployment-wide directory is the one place they must not
// appear, because two organisations' local.x are not the same service and the
// list has no way to say so.
var ErrLocalCodeIsPrivate = errors.New("a local. code is this organisation's own and cannot be published")

// Service is one published offer, as its owner sees it.
type Service struct {
	ID    string `json:"id"`
	Code  string `json:"code"`
	Title string `json:"title"`
}

// PublishedServices is this organisation's own list.
//
// No tenant_id in the WHERE clause and no filter on suspension: an
// administrator looking at their own list should see what they published,
// including while the organisation is suspended and the public list is hiding
// it. What limits this to one organisation is the row-level policy.
func (h *Handlers) PublishedServices(ctx context.Context) ([]Service, error) {
	rows, err := h.db.Query(ctx, `
		SELECT id::text, code, title FROM registry.service_directory
		 WHERE tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid
		 ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("list the published services: %w", err)
	}
	defer rows.Close()

	list := make([]Service, 0, 8)
	for rows.Next() {
		var one Service
		if err := rows.Scan(&one.ID, &one.Code, &one.Title); err != nil {
			return nil, fmt.Errorf("read a published service: %w", err)
		}
		list = append(list, one)
	}
	return list, rows.Err()
}

// PublishService adds one, or renames one already there.
//
// The `local.` rule is not checked here on purpose. It is a CHECK constraint in
// 00090, and 00062 wrote down why that is the right place: a Go function that
// validates every code is forgotten one day, and a constraint is not. What this
// does is turn the database's refusal into words, because "violates check
// constraint" is not something to put in front of an administrator.
func (h *Handlers) PublishService(ctx context.Context, tenantID, actorUserID, code, title string) error {
	code = strings.TrimSpace(code)
	if strings.HasPrefix(strings.ToLower(code), "local.") {
		return ErrLocalCodeIsPrivate
	}
	if _, err := h.db.Exec(ctx,
		`INSERT INTO registry.service_directory (tenant_id, code, title, published_by)
		 VALUES ($1::uuid, $2, $3, $4::uuid)
		 ON CONFLICT (tenant_id, code) DO UPDATE SET title = EXCLUDED.title`,
		tenantID, code, strings.TrimSpace(title), actorUserID); err != nil {
		return fmt.Errorf("publish %q: %w", code, err)
	}
	return nil
}

// WithdrawService takes one back out of the directory.
//
// A delete rather than a flag: the row is the publication, so withdrawing it is
// removing the row. Nothing is lost that the organisation did not put there
// itself, and its own queue — whether it still accepts this kind of request —
// is a different thing that this never touched.
func (h *Handlers) WithdrawService(ctx context.Context, id string) error {
	if _, err := h.db.Exec(ctx,
		`DELETE FROM registry.service_directory WHERE id = $1::uuid`, id); err != nil {
		return fmt.Errorf("withdraw a published service: %w", err)
	}
	return nil
}

// HandlePublishedServices, HandlePublishService and HandleWithdrawService are
// the administrator's three. Kept together at the end of the file they belong
// to rather than in the shared handler file, because they are one screen.
func (h *Handlers) HandlePublishedServices(w http.ResponseWriter, r *http.Request) {
	list, err := h.PublishedServices(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to list published services")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"services": list})
}

func (h *Handlers) HandlePublishService(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Code  string `json:"code"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "malformed request body")
		return
	}
	switch err := h.PublishService(r.Context(), claims.WorkspaceID, claims.UserID, body.Code, body.Title); {
	case err == nil:
		httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
	case errors.Is(err, ErrLocalCodeIsPrivate):
		httpx.Error(w, http.StatusConflict, err.Error())
	default:
		httpx.Error(w, http.StatusInternalServerError, "failed to publish the service")
	}
}

func (h *Handlers) HandleWithdrawService(w http.ResponseWriter, r *http.Request) {
	if err := h.WithdrawService(r.Context(), chi.URLParam(r, "id")); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to withdraw the service")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
