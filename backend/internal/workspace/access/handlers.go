package access

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var roleCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{1,62}[a-z0-9]$`)

type accessRole struct {
	ID          string   `json:"id"`
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Active      bool     `json:"active"`
	System      bool     `json:"system"`
	Permissions []string `json:"permissions"`
}

type accessPermission struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	App         string `json:"app"`
}

type accessMember struct {
	MembershipID string   `json:"membership_id"`
	UserID       string   `json:"user_id"`
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	IsAdmin      bool     `json:"is_admin"`
	Roles        []string `json:"roles"`
}

func (h *Handlers) HandleAccessOverview(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := nexus.RequireTenant(w, r)
	if !ok {
		return
	}

	roles := make([]accessRole, 0)
	rows, err := h.db.Query(r.Context(), `
		SELECT r.id::text,r.code,r.name,r.description,r.active,r.is_system,
		       COALESCE(array_agg(p.code ORDER BY p.code) FILTER (WHERE p.code IS NOT NULL),'{}')
		FROM workspace.roles r
		LEFT JOIN workspace.role_permissions rp ON rp.role_id=r.id
		LEFT JOIN registry.permissions p ON p.id=rp.permission_id
		WHERE r.tenant_id=$1 GROUP BY r.id ORDER BY r.is_system DESC,r.name`, tenantID)
	if err != nil {
		httpx.Error(w, 500, "failed to load roles")
		return
	}
	for rows.Next() {
		var v accessRole
		if err := rows.Scan(&v.ID, &v.Code, &v.Name, &v.Description, &v.Active, &v.System, &v.Permissions); err != nil {
			rows.Close()
			httpx.Error(w, 500, "failed to read roles")
			return
		}
		roles = append(roles, v)
	}
	// A stream that fails partway through leaves rows.Next() returning false —
	// exactly like a clean end. Without this check the screen renders a short
	// list as if it were the whole one, and the administrator then saves that
	// truncated view back, silently revoking whatever fell off the end.
	rows.Close()
	if err := rows.Err(); err != nil {
		httpx.Error(w, 500, "failed to read roles")
		return
	}

	permissions := make([]accessPermission, 0)
	rows, err = h.db.Query(r.Context(), `SELECT code,name,description,split_part(code,'.',1) FROM registry.permissions ORDER BY split_part(code,'.',1),code`)
	if err != nil {
		httpx.Error(w, 500, "failed to load permissions")
		return
	}
	for rows.Next() {
		var v accessPermission
		if err := rows.Scan(&v.Code, &v.Name, &v.Description, &v.App); err != nil {
			rows.Close()
			httpx.Error(w, 500, "failed to read permissions")
			return
		}
		permissions = append(permissions, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpx.Error(w, 500, "failed to read permissions")
		return
	}

	members := make([]accessMember, 0)
	rows, err = h.db.Query(r.Context(), `
		SELECT m.id::text,u.id::text,u.name,u.email,
		       EXISTS (SELECT 1 FROM workspace.membership_roles amr JOIN workspace.roles ar ON ar.id=amr.role_id
		               WHERE amr.membership_id=m.id AND ar.tenant_id=m.tenant_id AND ar.code='admin' AND ar.active),
		       COALESCE(array_agg(r.id::text ORDER BY r.name) FILTER (WHERE r.id IS NOT NULL),'{}')
		FROM workspace.memberships m JOIN registry.users u ON u.id=m.user_id
		LEFT JOIN workspace.membership_roles mr ON mr.membership_id=m.id
		LEFT JOIN workspace.roles r ON r.id=mr.role_id AND r.tenant_id=m.tenant_id
		WHERE m.tenant_id=$1 GROUP BY m.id,u.id ORDER BY u.name,u.email`, tenantID)
	if err != nil {
		httpx.Error(w, 500, "failed to load members")
		return
	}
	for rows.Next() {
		var v accessMember
		if err := rows.Scan(&v.MembershipID, &v.UserID, &v.Name, &v.Email, &v.IsAdmin, &v.Roles); err != nil {
			rows.Close()
			httpx.Error(w, 500, "failed to read members")
			return
		}
		members = append(members, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpx.Error(w, 500, "failed to read members")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"roles": roles, "permissions": permissions, "members": members})
}

func (h *Handlers) HandleCreateRole(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := nexus.TenantID(r.Context())
	claims, _ := auth.UserFromContext(r.Context())
	var req struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		httpx.Error(w, 400, "invalid role payload")
		return
	}
	req.Code = strings.ToLower(strings.TrimSpace(req.Code))
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if !roleCodePattern.MatchString(req.Code) || req.Name == "" {
		httpx.Error(w, 400, "role code or name is invalid")
		return
	}
	id := uuid.NewString()
	_, err := h.db.Exec(r.Context(), `INSERT INTO workspace.roles(id,tenant_id,code,name,description) VALUES($1,$2,$3,$4,$5)`, id, tenantID, req.Code, req.Name, req.Description)
	if err != nil {
		httpx.Error(w, 409, "role code already exists")
		return
	}
	h.recordAccessChange(r, claims.UserID, "role.create", "role", id, nil, req)
	httpx.JSON(w, 201, map[string]string{"id": id})
}

func (h *Handlers) HandleUpdateRole(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := nexus.TenantID(r.Context())
	claims, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Active      bool   `json:"active"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Name) == "" {
		httpx.Error(w, 400, "invalid role payload")
		return
	}
	var system bool
	var beforeName, beforeDesc string
	var beforeActive bool
	if h.db.QueryRow(r.Context(), `SELECT is_system,name,description,active FROM workspace.roles WHERE id=$1 AND tenant_id=$2`, id, tenantID).Scan(&system, &beforeName, &beforeDesc, &beforeActive) != nil {
		httpx.Error(w, 404, "role not found")
		return
	}
	if system && !req.Active {
		httpx.Error(w, 409, "system roles cannot be disabled")
		return
	}
	_, err := h.db.Exec(r.Context(), `UPDATE workspace.roles SET name=$1,description=$2,active=$3 WHERE id=$4 AND tenant_id=$5`, strings.TrimSpace(req.Name), strings.TrimSpace(req.Description), req.Active, id, tenantID)
	if err != nil {
		httpx.Error(w, 500, "failed to update role")
		return
	}
	h.recordAccessChange(r, claims.UserID, "role.update", "role", id, map[string]any{"name": beforeName, "description": beforeDesc, "active": beforeActive}, req)
	httpx.JSON(w, 200, map[string]string{"status": "updated"})
}

func (h *Handlers) HandleDeleteRole(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := nexus.TenantID(r.Context())
	claims, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var system bool
	var code string
	if h.db.QueryRow(r.Context(), `SELECT is_system,code FROM workspace.roles WHERE id=$1 AND tenant_id=$2`, id, tenantID).Scan(&system, &code) != nil {
		httpx.Error(w, 404, "role not found")
		return
	}
	if system {
		httpx.Error(w, 409, "system roles cannot be deleted")
		return
	}
	if _, err := h.db.Exec(r.Context(), `DELETE FROM workspace.roles WHERE id=$1 AND tenant_id=$2`, id, tenantID); err != nil {
		httpx.Error(w, 500, "failed to delete role")
		return
	}
	h.recordAccessChange(r, claims.UserID, "role.delete", "role", id, map[string]string{"code": code}, nil)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handlers) HandleSetRolePermissions(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := nexus.TenantID(r.Context())
	claims, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req struct {
		Permissions []string `json:"permissions"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		httpx.Error(w, 400, "invalid permission payload")
		return
	}
	var code string
	if h.db.QueryRow(r.Context(), `SELECT code FROM workspace.roles WHERE id=$1 AND tenant_id=$2 AND active`, id, tenantID).Scan(&code) != nil {
		httpx.Error(w, 404, "active role not found")
		return
	}
	if code == "admin" {
		httpx.Error(w, 409, "administrator permissions are managed automatically")
		return
	}
	seen := map[string]bool{}
	clean := make([]string, 0, len(req.Permissions))
	for _, p := range req.Permissions {
		p = strings.TrimSpace(p)
		if p != "" && !seen[p] {
			seen[p] = true
			clean = append(clean, p)
		}
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		httpx.Error(w, 500, "failed to start permission update")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var valid int
	if len(clean) > 0 {
		if err = tx.QueryRow(r.Context(), `SELECT count(*) FROM registry.permissions WHERE code=ANY($1)`, clean).Scan(&valid); err != nil || valid != len(clean) {
			httpx.Error(w, 400, "one or more permissions are unknown")
			return
		}
	}
	before, err := collectStrings(r.Context(), tx, `SELECT p.code FROM workspace.role_permissions rp JOIN registry.permissions p ON p.id=rp.permission_id WHERE rp.role_id=$1 ORDER BY p.code`, id)
	if err != nil {
		httpx.Error(w, 500, "failed to read the current permissions")
		return
	}
	if _, err = tx.Exec(r.Context(), `DELETE FROM workspace.role_permissions WHERE role_id=$1`, id); err != nil {
		httpx.Error(w, 500, "failed to clear permissions")
		return
	}
	if len(clean) > 0 {
		_, err = tx.Exec(r.Context(), `INSERT INTO workspace.role_permissions(role_id,permission_id) SELECT $1,id FROM registry.permissions WHERE code=ANY($2)`, id, clean)
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		httpx.Error(w, 500, "failed to save permissions")
		return
	}
	h.recordAccessChange(r, claims.UserID, "role.permissions", "role", id, before, clean)
	httpx.JSON(w, 200, map[string]string{"status": "updated"})
}

func (h *Handlers) HandleSetMembershipRoles(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := nexus.TenantID(r.Context())
	claims, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req struct {
		Roles []string `json:"roles"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		httpx.Error(w, 400, "invalid role assignment payload")
		return
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		httpx.Error(w, 500, "failed to start role assignment")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var exists bool
	if tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM workspace.memberships WHERE id=$1 AND tenant_id=$2)`, id, tenantID).Scan(&exists) != nil || !exists {
		httpx.Error(w, 404, "membership not found")
		return
	}
	seen := map[string]bool{}
	clean := []string{}
	for _, v := range req.Roles {
		if !seen[v] {
			seen[v] = true
			clean = append(clean, v)
		}
	}
	var valid int
	if len(clean) > 0 {
		_ = tx.QueryRow(r.Context(), `SELECT count(*) FROM workspace.roles WHERE tenant_id=$1 AND active AND id=ANY($2)`, tenantID, clean).Scan(&valid)
		if valid != len(clean) {
			httpx.Error(w, 400, "one or more roles are invalid or inactive")
			return
		}
	}
	before, err := collectStrings(r.Context(), tx, `SELECT role_id::text FROM workspace.membership_roles WHERE membership_id=$1 ORDER BY role_id`, id)
	if err != nil {
		httpx.Error(w, 500, "failed to read the current role assignment")
		return
	}
	var removedAdmin bool
	if err = tx.QueryRow(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM workspace.membership_roles mr JOIN workspace.roles r ON r.id=mr.role_id WHERE mr.membership_id=$1 AND r.code='admin')
		   AND NOT EXISTS(SELECT 1 FROM workspace.roles r WHERE r.id=ANY($2) AND r.tenant_id=$3 AND r.code='admin')`, id, clean, tenantID).Scan(&removedAdmin); err != nil {
		httpx.Error(w, 500, "failed to validate administrator assignment")
		return
	}
	if removedAdmin {
		var otherAdmins int
		if err = tx.QueryRow(r.Context(), `SELECT count(DISTINCT mr.membership_id) FROM workspace.membership_roles mr JOIN workspace.roles r ON r.id=mr.role_id JOIN workspace.memberships m ON m.id=mr.membership_id WHERE r.tenant_id=$1 AND r.code='admin' AND r.active AND m.id<>$2`, tenantID, id).Scan(&otherAdmins); err != nil {
			httpx.Error(w, 500, "failed to validate administrators")
			return
		}
		if otherAdmins == 0 {
			httpx.Error(w, http.StatusConflict, "the tenant must keep at least one administrator")
			return
		}
	}
	if _, err = tx.Exec(r.Context(), `DELETE FROM workspace.membership_roles WHERE membership_id=$1`, id); err == nil && len(clean) > 0 {
		_, err = tx.Exec(r.Context(), `INSERT INTO workspace.membership_roles(membership_id,role_id) SELECT $1,id FROM workspace.roles WHERE tenant_id=$2 AND active AND id=ANY($3)`, id, tenantID, clean)
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		httpx.Error(w, 500, "failed to assign roles")
		return
	}
	h.recordAccessChange(r, claims.UserID, "membership.roles", "membership", id, before, clean)
	httpx.JSON(w, 200, map[string]string{"status": "updated"})
}

// querier is the part of pgxpool.Pool and pgx.Tx this file needs, so the
// before-state snapshots can run inside the same transaction as the write they
// describe.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// collectStrings reads a single-text-column result into a slice.
//
// The before-state snapshots that feed the access-change audit trail used to
// discard the query error, discard each scan error, and append the zero value
// regardless — so a failed read was recorded as "this role had no permissions
// beforehand", which is precisely the claim an audit trail must never invent.
func collectStrings(ctx context.Context, q querier, sql string, args ...any) ([]string, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// recordAccessChange writes the audit row for a change to who may do what, and
// drops the tenant's cached grants.
//
// The invalidation lives here rather than in each of the five handlers because
// every one of them already has to call this — a new mutation that forgets to
// invalidate would be a role edit that takes half a minute to bite, and the
// administrator would be looking at a screen that says it already has.
func (h *Handlers) recordAccessChange(r *http.Request, actor, action, resource, resourceID string, before, after any) {
	tenantID, _ := nexus.TenantID(r.Context())
	h.forgetGrants(tenantID)
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	_, _ = h.db.Exec(r.Context(), `INSERT INTO workspace.access_change_events(tenant_id,actor_user_id,action,resource_type,resource_id,before_state,after_state) VALUES($1,$2,$3,$4,$5,$6,$7)`, tenantID, actor, action, resource, resourceID, beforeJSON, afterJSON)
	audit.Record(r.Context(), tenantID, actor, "access."+action, resource, map[string]any{"resource_id": resourceID})
}
