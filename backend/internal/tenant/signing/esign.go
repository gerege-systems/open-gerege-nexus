/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The PDF signing rails. Two of them share one document store:
 *
 *   HSM — Gerege eSign hardware module. The citizen proves a certificate with
 *         phone + civil ID, draws a signature, and the HSM stamps it. Fully
 *         synchronous.
 *
 *   EID — eID Mongolia qualified remote signing. The citizen's own device holds
 *         the private key and approves with PIN2, so the ceremony is
 *         asynchronous: start, show a verification code, poll, download.
 *
 * The rails are not interchangeable in law. Only the eID rail produces a
 * qualified electronic signature, which is why it is the default and why a
 * tenant can switch the HSM off entirely from the signing policy.
 *
 * This is not an app and has not been one since migration 00058 folded it into
 * `documents`: it registers no module, answers no ID(), Menus() or
 * Permissions(), and appears in no catalogue or manifest. What kept it looking
 * like one was a directory under internal/apps, a type called `Module` and a
 * method called `RegisterRoutes` — three imitations of a contract it does not
 * implement, and the reason the boundaries test needed an exception to allow
 * `documents` to import it. The precedent is internal/operator/staterail, where
 * egov.Rail moved for the same reason: the platform builds the value, so the
 * type is the platform's.
 */

package signing

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/identity/eidmongolia"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/identity/gerege"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/integration"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/go-chi/chi/v5"
)

// FileExporter is the part of the integration manager this module needs: a way
// to put a finished document somewhere outside the platform.
//
// It is an interface rather than the concrete manager so a test can sign a
// document without a Google account, and so the dependency reads as what esign
// actually wants — somewhere to file a PDF — rather than as "integrations".
type FileExporter interface {
	ExportFileToAll(ctx context.Context, tenantID, filename string, content []byte, reference string) []integration.ExportResult
	ExportFile(ctx context.Context, tenantID, integrationID, filename string, content []byte, reference string) (*integration.ExportResult, error)
}

type Rails struct {
	db      nexus.DB
	store   *store
	hsm     *gerege.EsignService
	eid     *eidmongolia.Service
	perms   nexus.PermissionStore
	exports FileExporter
}

// New builds the PDF rails. It does not register a module, and that is the
// whole of what changed when the two cards became one.
//
// This was `io.gerege.nexus.esign`, a second app in the store beside
// `io.gerege.nexus.documents`. Two cards, two installations, two menu entries,
// two permission namespaces — for one question a person actually has, which is
// "where are my documents and who has signed them". Nobody adopts a signature
// on its own; they adopt it because something has to be signed.
//
// So the rails moved inside the documents module, which owns the app's
// identity, permissions and menu, and calls Mount below. Everything
// here is unchanged apart from the permission codes: the paths under
// /api/v1/esign, the tables, the HSM and eID ceremonies and the signature log
// are all what they were, because none of them was ever the reason there were
// two apps.
func New(p nexus.Platform, hsm *gerege.EsignService, eid *eidmongolia.Service, exports FileExporter) *Rails {
	db := p.DB()
	m := &Rails{
		db:      db,
		store:   &store{db: db},
		hsm:     hsm,
		eid:     eid,
		perms:   p.Permissions(),
		exports: exports,
	}
	registerReports()
	return m
}

func (m *Rails) Mount(r chi.Router, tenantAuthMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/esign", func(er chi.Router) {
		er.Use(tenantAuthMiddleware)

		// Documents
		er.Get("/documents", m.listDocumentsHandler)
		er.Post("/documents", m.uploadDocumentHandler)
		er.Post("/documents/upload", m.uploadMultipartHandler)
		er.Get("/documents/{id}", m.getDocumentHandler)
		er.Delete("/documents/{id}", m.deleteDocumentHandler)
		er.Get("/documents/{id}/download", m.downloadDocumentHandler)
		er.Post("/documents/{id}/export", m.exportDocumentHandler)

		// HSM rail
		er.Post("/cert/check", m.checkCertHandler)
		er.Post("/documents/{id}/sign", m.signDocumentHandler)

		// eID Mongolia rail. The paths mirror the reference contract so the
		// signing view is portable between deployments.
		er.Post("/sign/init", m.signInitHandler)
		er.Get("/sign/{id}", m.signStatusHandler)
		er.Get("/sign/{id}/download", m.signDownloadHandler)
		er.Post("/sign/{id}/cancel", m.signCancelHandler)
		er.Get("/organizations", m.organizationsHandler)

		// Signature log
		er.Get("/logs", m.listLogsHandler)
		er.Get("/logs/export", m.exportLogsHandler)

		// Batch signing
		er.Get("/batches", m.listBatchesHandler)
		er.Post("/batches", m.createBatchHandler)
		er.Get("/batches/{id}", m.getBatchHandler)
		er.Post("/batches/{id}/run", m.runBatchHandler)
		er.Post("/batches/{id}/cancel", m.cancelBatchHandler)

		// Configuration
		er.Get("/settings", m.getSettingsHandler)
		er.Put("/settings/placement", m.updatePlacementHandler)
		er.Put("/settings/policy", m.updatePolicyHandler)
		er.Post("/settings/hsm/test", m.testHSMHandler)
	})
}

// ─── Request guards ──────────────────────────────────────────────────────────

// actorFrom resolves the caller into registry.permissions and, when their account is
// linked, their eID signing identity. It is the single place authorisation
// context is built, so no handler can forget to apply it.
func (m *Rails) actorFrom(r *http.Request) (string, Actor, error) {
	tenantID, err := nexus.TenantID(r.Context())
	if err != nil {
		return "", Actor{}, &Error{Code: "UNAUTHORIZED", Message: "unauthorized", Status: http.StatusUnauthorized}
	}
	claims, err := nexus.UserFromContext(r.Context())
	if err != nil {
		return "", Actor{}, &Error{Code: "UNAUTHORIZED", Message: "unauthorized", Status: http.StatusUnauthorized}
	}

	actor := Actor{UserID: claims.UserID, Email: claims.Email, IsAdmin: claims.IsAdmin}
	if !claims.IsAdmin {
		perms, err := m.perms.GetUserPermissions(r.Context(), tenantID, claims.UserID)
		if err != nil {
			return "", Actor{}, err
		}
		actor.Perms = perms
	}

	// A missing eID link is normal — password and DAN accounts have none — so
	// a lookup failure degrades the actor rather than failing the request.
	if identity, err := m.store.eidIdentityFor(r.Context(), claims.UserID); err == nil && identity != nil {
		actor.Etsi = identity.PersonEtsi
		actor.DocumentNumber = identity.DocumentNumber
		actor.FullName = strings.TrimSpace(identity.Surname + " " + identity.GivenName)
	} else if err != nil {
		slog.Warn("esign: could not read eID identity", "user_id", claims.UserID, "error", err)
	}

	return tenantID, actor, nil
}

// require is the guard every handler starts with. The platform's app gate only
// checks that the module is installed for the tenant; the three permissions
// this module declares mean nothing unless asserted here.
func (m *Rails) require(w http.ResponseWriter, r *http.Request, permission string) (string, Actor, bool) {
	tenantID, actor, err := m.actorFrom(r)
	if err != nil {
		writeDomainError(w, err)
		return "", Actor{}, false
	}
	if !actor.can(permission) {
		writeDomainError(w, forbidden("permission "+permission+" is required"))
		return "", Actor{}, false
	}
	return tenantID, actor, true
}

// log records an auditable event. Failures are logged and swallowed: losing an
// audit row must never fail a signature that has already been made, because
// the signed document is the record of consequence.
func (m *Rails) log(r *http.Request, entry logEntry) {
	if err := m.store.recordLog(r.Context(), entry); err != nil {
		slog.Error("esign: could not write the signature log",
			"action", entry.Action, "outcome", entry.Outcome, "error", err)
	}
}

// ─── Responses ───────────────────────────────────────────────────────────────

// writeDomainError maps a domain error onto its status and machine code. An
// unrecognised error is reported as a generic internal failure and logged,
// rather than having its text — which may quote an upstream body carrying
// citizen identifiers — echoed to the browser.
func writeDomainError(w http.ResponseWriter, err error) {
	var domain *Error
	if errors.As(err, &domain) {
		status := domain.Status
		if status == 0 {
			status = http.StatusBadRequest
		}
		nexus.JSON(w, status, map[string]string{"error": domain.Message, "code": domain.Code})
		return
	}
	slog.Error("esign: unhandled error", "error", err)
	nexus.JSON(w, http.StatusInternalServerError, map[string]string{
		"error": "internal error", "code": "INTERNAL",
	})
}

// ─── Query helpers ───────────────────────────────────────────────────────────

func pagination(r *http.Request, defaultLimit int) (limit, offset int) {
	limit = defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			offset = n
		}
	}
	return limit, offset
}

// parseTime accepts a date or a full RFC3339 timestamp, so a date picker and
// an API client can both filter the log.
func parseTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return &t
		}
	}
	return nil
}

func decodeJSON(r *http.Request, out any) error {
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(out); err != nil {
		return badRequest("INVALID_BODY", "the request body is not valid JSON")
	}
	return nil
}
