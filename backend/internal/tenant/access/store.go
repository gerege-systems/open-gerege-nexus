/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * PostgreSQL-backed permission lookup.
 */

package access

import (
	"context"
	"maps"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/memo"
	"github.com/jackc/pgx/v5/pgxpool"
)

// grantTTL is how long a member's effective grant is reused before it is asked
// for again. It is short because it is an authorisation answer: an
// administrator who takes a permission away on another replica has to wait this
// long for it to be true everywhere, and that wait is the whole cost of not
// running this five-table join in front of every request.
const grantTTL = 30 * time.Second

// grants is deliberately package-level. Five different components build their
// own SQLPermissionStore — the platform, documents, gov_services, esign — and a
// cache held per store would mean an administrator's change dropped one of five
// copies. Sharing it is what makes Invalidate mean anything.
var grants = memo.New[map[string]bool](grantTTL)

// GrantCache exposes the shared cache so the invalidation bus can address it,
// and so a replica told by another replica that a tenant has changed can drop
// the same entries this process's own handlers would have.
func GrantCache() *memo.Cache[map[string]bool] { return grants }

// GrantCacheName is what the bus knows this cache as.
const GrantCacheName = "grants"

// TenantPrefix is the key prefix covering every grant held for one tenant.
// Handlers hand this to the bus rather than calling the cache directly, so the
// drop reaches the other replicas as well as this one.
func TenantPrefix(tenantID string) string { return memo.Key(tenantID, "") }

// InvalidateTenant drops every cached grant belonging to one tenant in this
// process only. Prefer routing through the bus; this remains for callers that
// have no bus, such as tests.
func InvalidateTenant(tenantID string) {
	grants.InvalidatePrefix(TenantPrefix(tenantID))
}

// SQLPermissionStore resolves the permission codes granted to a user inside one
// tenant, walking membership → membership_roles → role_permissions.
//
// RequirePermission has existed since the first migration but had no
// implementation behind its PermissionStore interface, so nothing could
// actually enforce a permission. This is that implementation.
type SQLPermissionStore struct {
	db *pgxpool.Pool
}

func NewSQLPermissionStore(db *pgxpool.Pool) *SQLPermissionStore {
	return &SQLPermissionStore{db: db}
}

func (s *SQLPermissionStore) GetUserPermissions(ctx context.Context, tenantID, userID string) (map[string]bool, error) {
	key := memo.Key(tenantID, userID)
	if cached, ok := grants.Get(key); ok {
		// A copy, because the map goes out to callers that keep it: gov_services
		// hangs it off an Actor for the length of a request. One of them writing
		// to it would be granting a permission to everybody sharing the entry.
		return maps.Clone(cached), nil
	}

	rows, err := s.db.Query(ctx,
		`SELECT p.code
		   FROM tenant.memberships m
		   JOIN tenant.membership_roles mr ON mr.membership_id = m.id
		   JOIN tenant.role_permissions rp ON rp.role_id = mr.role_id
		   JOIN registry.permissions p ON p.id = rp.permission_id
		   JOIN tenant.roles r ON r.id = mr.role_id AND r.tenant_id = m.tenant_id AND r.active
		  WHERE m.tenant_id = $1 AND m.user_id = $2`, tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	perms := make(map[string]bool)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		perms[code] = true
	}
	if err := rows.Err(); err != nil {
		// A stream that broke halfway is a shorter list of permissions, and
		// caching it would turn one failed read into thirty seconds of a member
		// being told they may not do their job.
		return nil, err
	}

	grants.Put(key, perms)
	return maps.Clone(perms), nil
}
