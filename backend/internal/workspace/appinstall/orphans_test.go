/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package appinstall

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// An app a tenant has and this binary does not carry is said out loud.
//
// The state itself is legitimate — an app moved to another repository and the
// installation row outlived it — and the failure it produced was that nothing
// mentioned it: the routes answered 404, the app vanished from every sidebar,
// and the first report was somebody asking where their menu had gone.
func TestAnInstalledAppThisBinaryCannotServeIsReported(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a migrated test database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	tenantID := uuid.NewString()
	if _, err := pool.Exec(ctx,
		`INSERT INTO registry.tenants (id, name, slug) VALUES ($1, 'Uncarried', $2)`,
		tenantID, "uncarried-"+tenantID[:8]); err != nil {
		t.Fatalf("tenant: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1`, tenantID) })

	const departed = "io.example.left.for.another.repository"
	if _, err := pool.Exec(ctx,
		`INSERT INTO registry.apps (id, slug, name, description, icon_url, category)
		 VALUES ($1, 'departed', 'Departed', '', '', 'Test') ON CONFLICT (id) DO NOTHING`,
		departed); err != nil {
		t.Fatalf("app row: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.apps WHERE id = $1`, departed) })
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace.app_installations (tenant_id, app_id, installed_version, status, enabled)
		 VALUES ($1, $2, '1.0.0', 'installed', TRUE)`, tenantID, departed); err != nil {
		t.Fatalf("installation: %v", err)
	}

	var log bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	NewAppInstaller(pool, nil, "1.0.0").ReportUncarriedApps(ctx)

	if !strings.Contains(log.String(), departed) {
		t.Errorf("the report does not name the app nobody can reach:\n%s", log.String())
	}
}
