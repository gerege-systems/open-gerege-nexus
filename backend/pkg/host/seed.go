/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Idempotent demo data seeder for local development environments.
 */

package host

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
)

// resolveCatalogPath locates catalog/apps.json.
//
// The documented quick start is `cd backend && go run ./cmd/api`, but the
// catalog lives at the repository root, so the relative default resolved to
// backend/catalog/apps.json and the server refused to start. The parent
// directory is therefore tried as a fallback before giving up.
func resolveCatalogPath(configured string) string {
	if configured != "" {
		return configured
	}

	const defaultPath = "catalog/apps.json"
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}
	if parent := filepath.Join("..", defaultPath); fileExists(parent) {
		slog.Info("using app catalog from parent directory", "path", parent)
		return parent
	}
	return defaultPath
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

const (
	demoTenantID = "00000000-0000-0000-0000-000000000001"
	demoUserID   = "00000000-0000-0000-0000-000000000002"
	demoRoleID   = "00000000-0000-0000-0000-000000000003"
	demoEmail    = "admin@example.com"
	demoPassword = "Password123!"

	// A second organisation for the same person, because one is not enough to
	// exercise anything: the tenant switcher, the row-level isolation and the
	// per-tenant permission set all behave identically on a single-tenant
	// deployment and identically wrongly if they are broken. Its ids are fixed
	// for the same reason the demo tenant's are — a seeder that ran twice must
	// not leave two of it.
	secondTenantID = "00000000-0000-0000-0000-000000000004"
	secondRoleID   = "00000000-0000-0000-0000-000000000005"
)

// demoTenants are seeded in order, and the first is the one a demo sign-in
// lands in: login picks the oldest membership.
var demoTenants = []struct {
	id     string
	slug   string
	name   string
	roleID string
	// The apps installed for this tenant. They differ deliberately — after
	// switching, the sidebar and the app rail must visibly be somebody else's.
	//
	// Only apps this binary compiles. The list used to name products,
	// inventory and billing, which left for the commerce distribution; seeding
	// went on asking for them and logged a warning per app per boot. A seed
	// that routinely fails is a seed nobody reads, and the day it fails for a
	// real reason nobody will notice either.
	apps []string
}{
	// "documents" and "egov" were here until 2026-08-23, when both left for
	// client-gerege-nexus and their slugs stopped naming anything this binary
	// carries — the same failure the note above describes, for the third time.
	// There is no default app to fall back on any more either, so a seed that
	// names nothing real leaves both tenants with an empty sidebar rather than
	// merely a shorter one.
	// "urtuu" and then "reports" left this list on 2026-08-23 with their apps —
	// both are client-gerege-nexus's now and neither slug names anything this
	// binary carries. The same failure the note above describes, arriving for
	// the fifth time in one day, which is what a list of app slugs in the
	// platform is for.
	{demoTenantID, "demo", "Demo Corporation", demoRoleID, []string{"ai", "integrations"}},
	// The two tenants keep different sets: without the connectors, this one's
	// sidebar is still visibly not the first one's.
	{secondTenantID, "demo-trade", "Demo Trade LLC", secondRoleID, []string{"ai"}},
}

// seedingEnabled reports whether the documented demo account should be
// created. The account has a published password, so it is never seeded into a
// production environment unless the operator asks for it explicitly.
func seedingEnabled() bool { return config.SeedingEnabled() }

// appIntaller is what the seeder needs from the platform server: the catalogue
// has to be in the apps table before an installation row can reference it, and
// that is done while the server is built.
type appInstaller interface {
	InstallAppForTenant(ctx context.Context, tenantID, appSlug, userID string) error
}

// seedInitialData creates the demo tenants, the admin user, their admin roles
// and the apps each tenant runs, if they are missing. Every statement is
// idempotent, so it is safe on every boot.
func seedInitialData(ctx context.Context, db *pgxpool.Pool, installer appInstaller) {
	if !seedingEnabled() {
		return
	}

	userID, err := ensureDemoUser(ctx, db)
	if err != nil {
		slog.Error("failed to seed the demo account", "error", err)
		return
	}

	for _, want := range demoTenants {
		tenantID, err := ensureTenant(ctx, db, want.id, want.slug, want.name)
		if err != nil {
			slog.Error("failed to seed tenant", "slug", want.slug, "error", err)
			continue
		}
		ensureDemoMembership(ctx, db, tenantID, userID, want.roleID)
		for _, slug := range want.apps {
			// An app already installed is updated in place by the installer, so
			// this stays idempotent; a failure is logged and the rest continue,
			// because a demo missing one app is better than a boot that stops.
			if err := installer.InstallAppForTenant(ctx, tenantID, slug, userID); err != nil {
				slog.Warn("failed to install demo app", "tenant", want.slug, "app", slug, "error", err)
			}
		}
	}
	slog.Info("seeded demo account", "email", demoEmail, "tenants", len(demoTenants))
}

// ensureTenant returns the id of the tenant with this slug, creating it at the
// fixed id when it is not there. The slug is what identifies it, not the id: a
// deployment that once created the tenant by hand keeps the row it has.
func ensureTenant(ctx context.Context, db *pgxpool.Pool, id, slug, name string) (string, error) {
	var existing string
	if err := db.QueryRow(ctx, `SELECT id::text FROM registry.tenants WHERE slug = $1`, slug).Scan(&existing); err == nil {
		return existing, nil
	}
	if _, err := db.Exec(ctx,
		`INSERT INTO registry.tenants (id, slug, name) VALUES ($1, $2, $3) ON CONFLICT (slug) DO NOTHING`,
		id, slug, name); err != nil {
		return "", err
	}
	var created string
	if err := db.QueryRow(ctx, `SELECT id::text FROM registry.tenants WHERE slug = $1`, slug).Scan(&created); err != nil {
		return "", err
	}
	return created, nil
}

func ensureDemoUser(ctx context.Context, db *pgxpool.Pool) (string, error) {
	var userID string
	if err := db.QueryRow(ctx, `SELECT id::text FROM registry.users WHERE email = $1`, demoEmail).Scan(&userID); err == nil {
		return userID, nil
	}

	passHash, err := auth.HashPassword(demoPassword)
	if err != nil {
		return "", err
	}
	if _, err := db.Exec(ctx,
		`INSERT INTO registry.users (id, email, password_hash, name, is_admin)
		 VALUES ($1, $2, $3, 'System Admin', FALSE)
		 ON CONFLICT (email) DO NOTHING`, demoUserID, demoEmail, passHash); err != nil {
		return "", err
	}
	if err := db.QueryRow(ctx, `SELECT id::text FROM registry.users WHERE email = $1`, demoEmail).Scan(&userID); err != nil {
		return "", err
	}
	return userID, nil
}

// ensureDemoMembership puts the user in the tenant and makes them its
// administrator. roleID is a parameter rather than a constant because roles are
// per-tenant rows: seeding a second tenant with the first one's role id fails
// on the primary key, which is not the conflict the ON CONFLICT below covers.
func ensureDemoMembership(ctx context.Context, db *pgxpool.Pool, tenantID, userID, roleID string) {
	if _, err := db.Exec(ctx,
		`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1, $2)
		 ON CONFLICT (tenant_id, user_id) DO NOTHING`, tenantID, userID); err != nil {
		slog.Error("failed to seed membership", "error", err)
		return
	}

	if _, err := db.Exec(ctx,
		`INSERT INTO workspace.roles (id, tenant_id, code, name) VALUES ($1, $2, 'admin', 'Tenant Admin')
		 ON CONFLICT (tenant_id, code) DO NOTHING`, roleID, tenantID); err != nil {
		slog.Error("failed to seed admin role", "error", err)
		return
	}

	var membershipID, adminRoleID string
	if err := db.QueryRow(ctx,
		`SELECT id::text FROM workspace.memberships WHERE tenant_id = $1 AND user_id = $2`,
		tenantID, userID).Scan(&membershipID); err != nil {
		return
	}
	if err := db.QueryRow(ctx,
		`SELECT id::text FROM workspace.roles WHERE tenant_id = $1 AND code = 'admin'`, tenantID).Scan(&adminRoleID); err != nil {
		return
	}

	if _, err := db.Exec(ctx,
		`INSERT INTO workspace.membership_roles (membership_id, role_id) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`, membershipID, adminRoleID); err != nil {
		slog.Error("failed to grant admin role", "error", err)
	}
}
