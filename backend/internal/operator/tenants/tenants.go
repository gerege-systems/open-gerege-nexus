package tenants

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"

	"github.com/jackc/pgx/v5"
)

// What the console may know about an organisation, and how.
//
// §4 of the plan lists "a free query with row-level security switched off"
// among the things this console deliberately does not have. What it has instead
// is this file: a handful of statements, each written for one screen, each
// selecting named columns. The database enforces the same shape from the other
// side — migration 00049 grants the operator role SELECT on ten tables and
// nothing else — so a query written here that strays outside them fails rather
// than succeeding quietly.
//
// Everything in this file is metadata: how many people, which apps, when
// somebody was last here. No contacts, no invoices, no documents. Reading what
// an organisation actually keeps on the platform is impersonation, it needs
// consent and a reason, and it arrives in CP-2 with both.

// operator.QueryTimeout bounds one console query. The console is a handful of people
// looking at a list; a statement still running after this is one that will not
// be looked at, and holding a connection for it costs the platform's own
// traffic.

// Homes are not on this screen.
//
// A personal workspace is one person's own space, made for them the first time
// they sign in with no organisation to sign in to (migration 00085). It is a
// workspace by mechanism and not by purpose: nobody administers it, nobody is
// invited to it, and an operator scrolling for "which customer is suspended"
// should not be reading a list that is mostly citizens. The filter is on kind
// rather than on owner_user_id for the reason FirstOrganisationFor gives.
//
// This is also what keeps tenantPageSize honest. The bound below assumes the
// list is customers; homes would have grown it past 200 on the first busy day
// and the screen would have started lying by omission instead of paging.

// tenantPageSize bounds the list. Search narrows it; nothing pages yet, because
// a deployment with more organisations than this is one where the list screen
// needs rethinking rather than a second page.
const tenantPageSize = 200

// TenantSummary is one row of the organisation list.
type TenantSummary struct {
	ID                 string    `json:"id"`
	Slug               string    `json:"slug"`
	Name               string    `json:"name"`
	RegistrationNumber string    `json:"registration_number"`
	CreatedAt          time.Time `json:"created_at"`
	// Counts, named apart from the lists TenantDetail carries. They were both
	// called "apps" for an afternoon, and Go's JSON encoder resolves that
	// collision by silently dropping the shallower field — so the detail page
	// serialised its list and no count at all, with nothing anywhere saying so.
	UserCount      int        `json:"user_count"`
	AppCount       int        `json:"app_count"`
	LastActivityAt *time.Time `json:"last_activity_at"`
	// The lifecycle, in the list as well as on the detail page: an operator
	// scanning for "which of these is suspended" should not have to open
	// twenty pages to find out.
	SuspendedAt         *time.Time `json:"suspended_at"`
	SuspensionReason    string     `json:"suspension_reason"`
	DeletionScheduledAt *time.Time `json:"deletion_scheduled_at"`
	// MaintenanceAt is CP-3's read-only mode for this one organisation.
	MaintenanceAt *time.Time `json:"maintenance_at"`
}

// ListTenants answers the console's main screen.
//
// search matches the three things an operator has in front of them when
// somebody telephones: the name they call themselves, the slug in the URL, and
// the registration number on the letter.
func (s *Service) ListTenants(ctx context.Context, search string) ([]TenantSummary, error) {
	ctx, cancel := context.WithTimeout(operator.Scoped(ctx), operator.QueryTimeout)
	defer cancel()

	// The counts are subqueries rather than joins with a GROUP BY: the numbers
	// come from three different tables and grouping across them multiplies the
	// rows before it counts them, which is the classic way this list ends up
	// reporting a user count equal to users × apps.
	rows, err := s.db.Query(ctx,
		`SELECT t.id::text, t.slug, t.name,
		        COALESCE(p.registration_number, ''),
		        t.created_at,
		        (SELECT count(*) FROM workspace.memberships m WHERE m.tenant_id = t.id),
		        (SELECT count(*) FROM workspace.app_installations i
		          WHERE i.tenant_id = t.id AND i.enabled AND i.status = 'installed'),
		        (SELECT max(s.last_seen_at) FROM workspace.sessions s WHERE s.tenant_id = t.id),
		        t.suspended_at, t.suspension_reason, t.deletion_scheduled_at, t.maintenance_at
		   FROM registry.tenants t
		   LEFT JOIN workspace.tenant_profiles p ON p.tenant_id = t.id
		  WHERE t.kind = 'organisation'
		    AND ($1 = ''
		     OR t.name ILIKE '%' || $1 || '%'
		     OR t.slug ILIKE '%' || $1 || '%'
		     OR COALESCE(p.registration_number, '') ILIKE '%' || $1 || '%')
		  ORDER BY t.name, t.id
		  LIMIT $2`, search, tenantPageSize)
	if err != nil {
		return nil, fmt.Errorf("control plane: list the organisations: %w", err)
	}
	defer rows.Close()

	summaries := make([]TenantSummary, 0, 32)
	for rows.Next() {
		var row TenantSummary
		if err := rows.Scan(&row.ID, &row.Slug, &row.Name, &row.RegistrationNumber,
			&row.CreatedAt, &row.UserCount, &row.AppCount, &row.LastActivityAt,
			&row.SuspendedAt, &row.SuspensionReason, &row.DeletionScheduledAt,
			&row.MaintenanceAt); err != nil {
			return nil, fmt.Errorf("control plane: read an organisation: %w", err)
		}
		summaries = append(summaries, row)
	}
	return summaries, rows.Err()
}

// TenantApp is one app an organisation has.
type TenantApp struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Status      string    `json:"status"`
	Enabled     bool      `json:"enabled"`
	InstalledAt time.Time `json:"installed_at"`
}

// TenantMember is one person in an organisation, with the roles they hold
// there. Deliberately not their password state, their sessions or their
// telephone number: this screen answers "who can act here", not "who is this".
type TenantMember struct {
	UserID string   `json:"user_id"`
	Email  string   `json:"email"`
	Name   string   `json:"name"`
	Roles  []string `json:"roles"`
}

// TenantActivity is one line of the organisation's own audit trail.
type TenantActivity struct {
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

// TenantDetail is the organisation's page.
type TenantDetail struct {
	TenantSummary
	LegalName string           `json:"legal_name"`
	TaxNumber string           `json:"tax_number"`
	Apps      []TenantApp      `json:"apps"`
	Members   []TenantMember   `json:"members"`
	Activity  []TenantActivity `json:"activity"`
	// OperatorActions is what operators have done to this organisation. It is
	// on the same page as the organisation's own trail on purpose: the two
	// belong to the same story, and separating them is how "who suspended this
	// tenant" becomes a question somebody has to know where to ask.
	OperatorActions []operator.AuditEntry `json:"operator_actions"`
	// Quota is the limits and where they stand, so the page that can change
	// them does not need a second request to show them.
	Quota Quota `json:"quota"`
	// Impersonations is who has been inside this organisation, most recent
	// first. On the operator's page as well as the organisation's own, because
	// an operator about to go in should see who was there this morning.
	Impersonations []operator.Impersonation `json:"impersonations"`
}

// activityPageSize bounds the recent-activity list on the detail page.
const activityPageSize = 25

// GetTenant assembles one organisation's page.
//
// Five statements rather than one, because they answer five questions with
// different shapes and joining them would either multiply rows or need window
// functions to undo the multiplication. The console is not a hot path.
func (s *Service) GetTenant(ctx context.Context, tenantID string) (TenantDetail, error) {
	ctx, cancel := context.WithTimeout(operator.Scoped(ctx), operator.QueryTimeout)
	defer cancel()

	var detail TenantDetail
	err := s.db.QueryRow(ctx,
		`SELECT t.id::text, t.slug, t.name,
		        COALESCE(p.registration_number, ''), COALESCE(p.legal_name, ''), COALESCE(p.tax_number, ''),
		        t.created_at,
		        (SELECT count(*) FROM workspace.memberships m WHERE m.tenant_id = t.id),
		        (SELECT count(*) FROM workspace.app_installations i
		          WHERE i.tenant_id = t.id AND i.enabled AND i.status = 'installed'),
		        (SELECT max(s.last_seen_at) FROM workspace.sessions s WHERE s.tenant_id = t.id),
		        t.suspended_at, t.suspension_reason, t.deletion_scheduled_at, t.maintenance_at
		   FROM registry.tenants t
		   LEFT JOIN workspace.tenant_profiles p ON p.tenant_id = t.id
		  WHERE t.id = $1::uuid`, tenantID).
		Scan(&detail.ID, &detail.Slug, &detail.Name, &detail.RegistrationNumber,
			&detail.LegalName, &detail.TaxNumber, &detail.CreatedAt,
			&detail.UserCount, &detail.AppCount, &detail.LastActivityAt,
			&detail.SuspendedAt, &detail.SuspensionReason, &detail.DeletionScheduledAt,
			&detail.MaintenanceAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TenantDetail{}, operator.ErrTenantNotFound
	}
	if err != nil {
		return TenantDetail{}, fmt.Errorf("control plane: read the organisation: %w", err)
	}

	if detail.Apps, err = s.tenantApps(ctx, tenantID); err != nil {
		return TenantDetail{}, err
	}
	if detail.Members, err = s.tenantMembers(ctx, tenantID); err != nil {
		return TenantDetail{}, err
	}
	if detail.Activity, err = s.tenantActivity(ctx, tenantID); err != nil {
		return TenantDetail{}, err
	}
	if detail.OperatorActions, err = s.audit.ListAudit(ctx, "", "tenant", tenantID); err != nil {
		return TenantDetail{}, err
	}
	if detail.Quota, err = s.GetQuota(ctx, tenantID); err != nil {
		return TenantDetail{}, err
	}
	if detail.Impersonations, err = s.op.ListImpersonations(ctx, tenantID); err != nil {
		return TenantDetail{}, err
	}
	return detail, nil
}

func (s *Service) tenantApps(ctx context.Context, tenantID string) ([]TenantApp, error) {
	rows, err := s.db.Query(ctx,
		`SELECT i.app_id, COALESCE(a.name, i.app_id), i.installed_version, i.status, i.enabled, i.installed_at
		   FROM workspace.app_installations i
		   LEFT JOIN registry.apps a ON a.id = i.app_id
		  WHERE i.tenant_id = $1::uuid
		  ORDER BY COALESCE(a.name, i.app_id)`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("control plane: list the organisation's apps: %w", err)
	}
	defer rows.Close()

	apps := make([]TenantApp, 0, 8)
	for rows.Next() {
		var app TenantApp
		if err := rows.Scan(&app.ID, &app.Name, &app.Version, &app.Status, &app.Enabled, &app.InstalledAt); err != nil {
			return nil, fmt.Errorf("control plane: read an app installation: %w", err)
		}
		apps = append(apps, app)
	}
	return apps, rows.Err()
}

func (s *Service) tenantMembers(ctx context.Context, tenantID string) ([]TenantMember, error) {
	// The roles are aggregated in the database rather than by reading one row
	// per membership-role pair and folding them here — the fold is where a
	// person with two roles becomes two people in the list.
	rows, err := s.db.Query(ctx,
		`SELECT u.id::text, u.email, u.name,
		        COALESCE(ARRAY_AGG(r.code ORDER BY r.code) FILTER (WHERE r.code IS NOT NULL), '{}') AS roles
		   FROM workspace.memberships m
		   JOIN registry.users u ON u.id = m.user_id
		   LEFT JOIN workspace.membership_roles mr ON mr.membership_id = m.id
		   LEFT JOIN workspace.roles r ON r.id = mr.role_id AND r.tenant_id = m.tenant_id
		  WHERE m.tenant_id = $1::uuid
		  GROUP BY u.id, u.email, u.name
		  ORDER BY u.email`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("control plane: list the organisation's people: %w", err)
	}
	defer rows.Close()

	members := make([]TenantMember, 0, 16)
	for rows.Next() {
		var member TenantMember
		if err := rows.Scan(&member.UserID, &member.Email, &member.Name, &member.Roles); err != nil {
			return nil, fmt.Errorf("control plane: read a membership: %w", err)
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (s *Service) tenantActivity(ctx context.Context, tenantID string) ([]TenantActivity, error) {
	rows, err := s.db.Query(ctx,
		`SELECT action, resource, COALESCE(user_id, ''), created_at
		   FROM workspace.audit_events
		  WHERE tenant_id = $1::uuid
		  ORDER BY created_at DESC
		  LIMIT $2`, tenantID, activityPageSize)
	if err != nil {
		return nil, fmt.Errorf("control plane: read the organisation's activity: %w", err)
	}
	defer rows.Close()

	activity := make([]TenantActivity, 0, activityPageSize)
	for rows.Next() {
		var entry TenantActivity
		if err := rows.Scan(&entry.Action, &entry.Resource, &entry.UserID, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("control plane: read an activity row: %w", err)
		}
		activity = append(activity, entry)
	}
	return activity, rows.Err()
}
