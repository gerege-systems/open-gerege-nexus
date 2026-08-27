/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package reporting

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// The two records a reports screen shows and this platform keeps.
//
// Every statement here was lifted out of the app — internal/apps/reports and
// backend/domain/reports/postgres — unchanged apart from the type it fills.
// The move is the point: a schedule is acted on by the scheduler at three in
// the morning and a grant is checked by the consolidated run, so both are the
// platform's rows, and an app that wrote them directly was an app that could
// not leave this repository.
//
// What this layer decides is what an error means — no rows is a schedule that
// is not this organisation's, a violated partial unique index is an agreement
// that already exists — so that nothing above it has to know pgx.

// DB is the handle these adapters need, in pgx's own shape.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// AsSchedules presents report_schedules as nexus.ReportSchedules.
func AsSchedules(db DB) nexus.ReportSchedules { return scheduleRecords{db} }

type scheduleRecords struct{ db DB }

func (s scheduleRecords) List(ctx context.Context, tenantID string) ([]nexus.ReportSchedule, error) {
	rows, err := ListSchedules(nexus.WithWorkspaceID(ctx, tenantID), s.db, tenantID)
	if err != nil {
		return nil, err
	}
	schedules := make([]nexus.ReportSchedule, 0, len(rows))
	for _, row := range rows {
		schedules = append(schedules, nexus.ReportSchedule{
			// The stored parameters are a JSON object and the contract carries
			// strings, which is the same conversion the scheduler makes before
			// a run: a report binds from strings, so a schedule that could not
			// be turned into them could not be run either.
			ID: row.ID, ReportKey: row.ReportKey, Name: row.Name,
			Params: stringifyParams(row.Params),
			Cron:   row.Cron, Format: row.Format, Recipients: row.Recipients,
			Active: row.Active, LastRunAt: row.LastRunAt, LastStatus: row.LastStatus,
			LastError: row.LastError, CreatedAt: row.CreatedAt, Titles: row.Titles,
		})
	}
	return schedules, nil
}

func (s scheduleRecords) Create(ctx context.Context, tenantID string, schedule nexus.ReportSchedule) (string, error) {
	var id string
	err := s.db.QueryRow(nexus.WithWorkspaceID(ctx, tenantID), `
		INSERT INTO workspace.report_schedules
		    (tenant_id, report_key, name, params, cron, format, recipients, active, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, '')::uuid)
		RETURNING id`,
		tenantID, schedule.ReportKey, schedule.Name, schedule.Params, schedule.Cron,
		schedule.Format, schedule.Recipients, schedule.Active, schedule.CreatedBy).Scan(&id)
	return id, err
}

func (s scheduleRecords) Update(ctx context.Context, tenantID, id string, schedule nexus.ReportSchedule) (bool, error) {
	// The tenant clause is here as well as in the policy. `WHERE id = $1` alone
	// would be a schedule id from one organisation editing another's row, and
	// the row-level policy is the layer that catches it — not the only one.
	tag, err := s.db.Exec(nexus.WithWorkspaceID(ctx, tenantID), `
		UPDATE workspace.report_schedules
		   SET report_key = $3, name = $4, params = $5, cron = $6, format = $7,
		       recipients = $8, active = $9, updated_at = NOW()
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID, schedule.ReportKey, schedule.Name, schedule.Params, schedule.Cron,
		schedule.Format, schedule.Recipients, schedule.Active)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s scheduleRecords) Delete(ctx context.Context, tenantID, id string) (string, error) {
	var reportKey string
	err := s.db.QueryRow(nexus.WithWorkspaceID(ctx, tenantID),
		`DELETE FROM workspace.report_schedules WHERE id = $1 AND tenant_id = $2 RETURNING report_key`,
		id, tenantID).Scan(&reportKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nexus.ErrReportScheduleNotFound
	}
	return reportKey, err
}

// AsGrants presents report_grants — and the audit trail of what was read under
// them — as nexus.ReportGrants.
func AsGrants(db DB) nexus.ReportGrants { return grantRecords{db} }

type grantRecords struct{ db DB }

func (g grantRecords) List(ctx context.Context, tenantID string) ([]nexus.ReportGrant, error) {
	rows, err := ListGrants(ctx, g.db, tenantID)
	if err != nil {
		return nil, err
	}
	grants := make([]nexus.ReportGrant, 0, len(rows))
	for _, row := range rows {
		grants = append(grants, nexus.ReportGrant{
			ID: row.ID, ReportKey: row.ReportKey,
			GrantorWorkspaceID: row.GrantorTenantID, GrantorName: row.GrantorName,
			GranteeWorkspaceID: row.GranteeTenantID, GranteeName: row.GranteeName,
			Scope: row.Scope, CounterpartyRef: row.CounterpartyRef,
			ValidFrom: row.ValidFrom, ValidUntil: row.ValidUntil,
			RevokedAt: row.RevokedAt, AcceptedAt: row.AcceptedAt,
			Note: row.Note, CreatedAt: row.CreatedAt,
			Direction: row.Direction, Titles: row.Titles,
		})
	}
	return grants, nil
}

// History reads the audit trail rather than a table of its own.
//
// The act it reports — one organisation's rows read by another — is written
// there by the consolidated run on both sides, and a second record of the same
// fact would be a second thing to keep true.
func (g grantRecords) History(ctx context.Context, tenantID string) ([]nexus.ReportGrantUse, error) {
	rows, err := g.db.Query(nexus.WithWorkspaceID(ctx, tenantID), `
		SELECT a.created_at, a.resource, a.details, coalesce(t.name, '—')
		  FROM workspace.audit_events a
		  LEFT JOIN registry.tenants t
		    ON t.id = (a.details->>'grantee_tenant_id')::uuid
		 WHERE a.tenant_id = $1 AND a.action = 'reports.data_shared'
		 ORDER BY a.created_at DESC
		 LIMIT 200`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	uses := make([]nexus.ReportGrantUse, 0, 32)
	for rows.Next() {
		var use nexus.ReportGrantUse
		var details []byte
		if err := rows.Scan(&use.At, &use.ReportKey, &details, &use.ReaderName); err != nil {
			return nil, err
		}
		// The scope and the row count live in the audit entry's details, which
		// is a JSON document rather than columns: an audit row is written once
		// and read rarely, and giving every act's details their own columns is
		// how an audit table stops being one table.
		var payload struct {
			Scope string `json:"scope"`
			Rows  int    `json:"rows"`
		}
		if err := json.Unmarshal(details, &payload); err == nil {
			use.Scope, use.Rows = payload.Scope, payload.Rows
		}
		uses = append(uses, use)
	}
	return uses, rows.Err()
}

func (g grantRecords) Request(ctx context.Context, grant nexus.ReportGrant) (string, error) {
	var id string
	err := g.db.QueryRow(nexus.WithWorkspaceID(ctx, grant.GranteeWorkspaceID), `
		INSERT INTO workspace.report_grants
		    (grantor_tenant_id, grantee_tenant_id, report_key, scope,
		     counterparty_ref, valid_until, created_by, note)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, '')::uuid, $8)
		RETURNING id`,
		grant.GrantorWorkspaceID, grant.GranteeWorkspaceID, grant.ReportKey, grant.Scope,
		grant.CounterpartyRef, grant.ValidUntil, grant.CreatedBy, grant.Note).Scan(&id)
	// One live agreement per pair per report, held by a partial unique index.
	// Only that violation is the conflict: the handler this was lifted from
	// answered 409 to every failure, so a database that was down told the
	// operator their colleague had already asked.
	if sqlState(err) == "23505" {
		return "", nexus.ErrReportGrantExists
	}
	return id, err
}

func (g grantRecords) Accept(ctx context.Context, grantorTenantID, id, actorUserID string) (string, error) {
	// `grantor_tenant_id = $2` is the whole authorization for this statement:
	// the row-level policy lets both parties see the row, so without this
	// clause a grantee could accept their own request.
	var reportKey string
	err := g.db.QueryRow(nexus.WithWorkspaceID(ctx, grantorTenantID), `
		UPDATE workspace.report_grants
		   SET accepted_by = NULLIF($3, '')::uuid, accepted_at = NOW(), updated_at = NOW()
		 WHERE id = $1 AND grantor_tenant_id = $2 AND revoked_at IS NULL AND accepted_at IS NULL
		 RETURNING report_key`, id, grantorTenantID, actorUserID).Scan(&reportKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nexus.ErrReportGrantNotPending
	}
	return reportKey, err
}

func (g grantRecords) Revoke(ctx context.Context, tenantID, id string) (string, string, error) {
	var reportKey, side string
	err := g.db.QueryRow(nexus.WithWorkspaceID(ctx, tenantID), `
		UPDATE workspace.report_grants
		   SET revoked_at = NOW(), updated_at = NOW()
		 WHERE id = $1
		   AND (grantor_tenant_id = $2 OR grantee_tenant_id = $2)
		   AND revoked_at IS NULL
		 RETURNING report_key,
		           CASE WHEN grantor_tenant_id = $2 THEN 'given' ELSE 'received' END`,
		id, tenantID).Scan(&reportKey, &side)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nexus.ErrReportGrantNotFound
	}
	return reportKey, side, err
}

// OrganisationByRegistration deliberately looks outside the caller's own
// organisation, and narrowly: an exact registration number in, an id or
// ErrOrganisationNotFound out. Unbound, because a lookup scoped to the caller's
// own tenant could only ever find the caller.
func (g grantRecords) OrganisationByRegistration(ctx context.Context, registration string) (string, error) {
	if registration == "" {
		return "", nexus.ErrOrganisationNotFound
	}
	var tenantID string
	err := g.db.QueryRow(nexus.WithoutWorkspace(ctx),
		`SELECT tenant_id FROM workspace.tenant_profiles WHERE registration_number = $1`,
		registration).Scan(&tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nexus.ErrOrganisationNotFound
	}
	return tenantID, err
}

func (g grantRecords) RegistrationOf(ctx context.Context, tenantID string) (string, error) {
	var registration string
	err := g.db.QueryRow(nexus.WithWorkspaceID(ctx, tenantID),
		`SELECT registration_number FROM workspace.tenant_profiles WHERE tenant_id = $1`,
		tenantID).Scan(&registration)
	if errors.Is(err, pgx.ErrNoRows) {
		// Not having filled in a legal profile is a state, not a fault.
		return "", nil
	}
	return registration, err
}

// sqlState is how PostgreSQL says which rule was broken. Anything else — a
// closed connection, a timeout — has no state and is nobody's mistake.
func sqlState(err error) string {
	if err == nil {
		return ""
	}
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState()
	}
	return ""
}

// Compile-time proof that the two adapters answer the contracts.
var (
	_ nexus.ReportSchedules = scheduleRecords{}
	_ nexus.ReportGrants    = grantRecords{}
)
