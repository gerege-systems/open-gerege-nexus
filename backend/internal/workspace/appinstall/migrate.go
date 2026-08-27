/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package appinstall

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// Running a module's own schema, before its installation is written.
//
// Before, and deliberately outside the installation transaction. Three reasons,
// in the order they matter:
//
//   - A module's tables are a property of the *deployment*, not of the tenant
//     installing it. Tenants share them and are kept apart by row-level policy,
//     so the second tenant to install an app finds the tables already there.
//     Tying their creation to one tenant's transaction would mean rolling that
//     tenant's installation back drops tables the other tenants are using.
//
//   - goose owns its own transactions. It takes a *sql.DB and runs each
//     migration in one, precisely so that a half-applied file cannot be
//     recorded as applied. Handing it a transaction somebody else opened would
//     mean giving that up, and the guarantee it gives is the one that matters
//     most here.
//
//   - The two failures want different answers. A migration that fails must stop
//     the install before any row claims the app is present; an install that
//     fails afterwards should leave the schema alone, because the schema is not
//     wrong — it is just unused, which is the state every deployment is in for
//     every app it has not installed.
//
// So: migrate, then begin. Idempotent by construction — goose applies only what
// its version table does not already record.
func (ai *AppInstaller) runModuleMigrations(ctx context.Context, appID string) error {
	fsys, ok := nexus.MigrationsOf(appID)
	if !ok {
		// The ordinary case. The platform's own apps still live in
		// db/migrations; a module with no schema of its own is not a mistake.
		return nil
	}

	table, err := versionTable(appID)
	if err != nil {
		return err
	}

	// One *sql.DB over the same pool, closed when this returns: goose speaks
	// database/sql and the platform speaks pgx, and stdlib is the bridge that
	// does not open a second set of connections to the database.
	db := stdlib.OpenDBFromPool(ai.db)
	defer func() { _ = db.Close() }()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys, goose.WithTableName(table))
	if err != nil {
		return fmt.Errorf("prepare migrations for %s: %w", appID, err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate %s: %w", appID, err)
	}
	return nil
}

// MigrateModules applies the schema of every module compiled into this binary.
//
// runModuleMigrations otherwise runs in exactly one place — the install path —
// so a module that gains a schema *after* its app is installed never gets its
// tables. That is not hypothetical: three apps left this repository on
// 2026-08-23 and took their schema with them, migration 00077 dropped the
// tables they had left behind, and on a deployment that had installed those
// apps days earlier nothing ran the modules' own migrations. The row still
// read "installed", the routes still mounted, and every request into them
// answered "relation ... does not exist". Nothing in the deploy looked wrong:
// the stack was healthy, the shell rendered, sign-in worked.
//
// By module rather than by installation, for the reason runModuleMigrations
// sits outside the installation transaction to begin with: a module's tables
// are a property of the deployment, not of the tenant. A schema nobody has
// installed is unused, which is the state every deployment is already in for
// every app it does not carry — and it is the state the second tenant to
// install an app finds anyway.
//
// Every module is attempted before the first failure is returned, so one
// module's broken migration does not hide the rest. Idempotent: goose applies
// only what its version table does not already record.
func (ai *AppInstaller) MigrateModules(ctx context.Context) error {
	var failures []error
	for _, m := range nexus.List() {
		if err := ai.runModuleMigrations(ctx, m.ID()); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// safeSlug is what may appear in an identifier this code builds. goose's table
// name is interpolated into DDL, so it is checked rather than trusted — a
// module id comes from a manifest, and a manifest can come from a registry.
var safeSlug = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// versionTable is where a module's applied versions are recorded.
//
// public.goose_db_version_<slug>, where the slug is the last segment of the app
// id — io.gerege.nexus.urtuu becomes public.goose_db_version_urtuu. The
// explicit schema matters after migration 00079 puts tenant first in the
// search_path: a module's tables belong in workspace, but migration history is the
// deployment's bookkeeping and stays beside public.goose_db_version. Without
// the qualifier an existing history would remain in public while a fresh
// deployment silently created a second one in tenant.
func versionTable(appID string) (string, error) {
	slug := appID
	if idx := strings.LastIndex(appID, "."); idx >= 0 {
		slug = appID[idx+1:]
	}
	slug = strings.ReplaceAll(slug, "-", "_")
	if !safeSlug.MatchString(slug) {
		return "", fmt.Errorf("app id %q does not yield a usable migration table name", appID)
	}
	return "public.goose_db_version_" + slug, nil
}
