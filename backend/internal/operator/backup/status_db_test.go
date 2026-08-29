/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package backup

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The screen nobody reads until the morning they need it.
//
// Everything here is a claim about what the database answers — which row is
// "the latest", whether an unconfigured deployment is distinguishable from a
// healthy one, and whether a failed backup is allowed to look like no backup.
// The last one is the one that matters: a panel that shows nothing on the
// morning after a failure is worse than a panel that shows a failure.

// backupRow writes one line of history and takes it away afterwards.
func backupRow(t *testing.T, pool *pgxpool.Pool, kind string, at time.Time, size *int64, ok bool, detail string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO operator.platform_backups (kind, started_at, size_bytes, ok, detail)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id::text`, kind, at, size, ok, detail).Scan(&id); err != nil {
		t.Fatalf("write a %s row: %v", kind, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operator.platform_backups WHERE id = $1::uuid`, id)
	})
	return id
}

// A deployment that never installed the cron job has to be told so, rather than
// shown an empty panel that reads like "all is well".
func TestADeploymentThatHasNeverBackedUpSaysSo(t *testing.T) {
	pool := optest.Pool(t)
	service := New(operator.New(pool), Deps{DB: pool})

	// Any history at all makes this deployment configured, so the claim can
	// only be made about one with none — which is the state a fresh test
	// database is in unless another test left a row behind.
	var rows int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM operator.platform_backups WHERE kind = 'backup'`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows > 0 {
		t.Skip("this database already has a backup history")
	}

	status := service.StatusOf(context.Background())
	if status.Configured {
		t.Error("a deployment with no backups reported itself configured")
	}
	if status.LastBackupAt != nil {
		t.Errorf("a deployment with no backups has a last backup at %v", status.LastBackupAt)
	}
}

// "Latest" is ORDER BY started_at DESC, and the row that matters is the last
// one — including when the last one failed.
func TestTheStatusShowsTheLatestRunEvenWhenItFailed(t *testing.T) {
	pool := optest.Pool(t)
	service := New(operator.New(pool), Deps{DB: pool})
	ctx := context.Background()

	size := int64(5 * 1024 * 1024)
	backupRow(t, pool, "backup", time.Now().Add(-48*time.Hour), &size, true, "two nights ago")
	backupRow(t, pool, "backup", time.Now().Add(-24*time.Hour), nil, false, "pg_dump exited 1")

	status := service.StatusOf(ctx)
	if !status.Configured {
		t.Fatal("a deployment with a history reported itself unconfigured")
	}
	if status.LastOK {
		t.Error("a failed backup is being shown as a successful one")
	}
	if status.LastDetail != "pg_dump exited 1" {
		t.Errorf("the detail is from another run: %q", status.LastDetail)
	}
	// The failed row reported no size, and a screen that carried the previous
	// run's megabytes into it would say a backup was written when none was.
	if status.LastSizeMB != 0 {
		t.Errorf("a backup that wrote nothing is shown as %.1f MB", status.LastSizeMB)
	}
}

// Bytes on the way in, megabytes on the way out — the conversion is the only
// arithmetic on this screen, and an off-by-1024 turns 5 MB into 5 GB.
func TestTheSizeIsReportedInMegabytes(t *testing.T) {
	pool := optest.Pool(t)
	service := New(operator.New(pool), Deps{DB: pool})

	size := int64(5 * 1024 * 1024)
	backupRow(t, pool, "backup", time.Now(), &size, true, "nightly")

	if got := service.StatusOf(context.Background()).LastSizeMB; got != 5 {
		t.Errorf("5 MiB was reported as %v MB", got)
	}
}

// An untested backup is not a backup. The restore test is a row somebody
// writes by hand, it is audited, and it must not be mistaken for a backup.
func TestARestoreTestIsRecordedAuditedAndKeptApartFromTheBackups(t *testing.T) {
	pool := optest.Pool(t)
	op := operator.New(pool)
	service := New(op, Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	sess := optest.Session(account)
	ctx := context.Background()

	size := int64(1024 * 1024)
	backupRow(t, pool, "backup", time.Now().Add(-time.Hour), &size, true, "nightly")

	if err := service.RecordRestoreTest(ctx, sess, "restored onto staging, 12 minutes", "quarterly drill"); err != nil {
		t.Fatalf("record the restore test: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM operator.platform_backups WHERE kind = 'restore_test' AND recorded_by = $1::uuid`, account.ID)
	})

	status := service.StatusOf(ctx)
	if status.LastRestoreTestAt == nil {
		t.Fatal("the restore test was not recorded")
	}
	// It is not a backup: the backup line must still be the nightly one.
	if status.LastDetail != "nightly" {
		t.Errorf("the restore test was counted as the latest backup: %q", status.LastDetail)
	}

	if got := optest.AuditCount(t, pool, account.ID, "backup.restore_test"); got != 1 {
		t.Errorf("the restore test wrote %d audit rows, want 1", got)
	}

	// And it appears in the history, which is what answers "when did we last
	// prove this works".
	history, err := service.History(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, entry := range history {
		if entry.Kind == "restore_test" && entry.RecordedBy == account.ID {
			found = true
			if entry.Detail != "restored onto staging, 12 minutes" {
				t.Errorf("the history lost the detail: %q", entry.Detail)
			}
		}
	}
	if !found {
		t.Error("the restore test is not in the history")
	}
}

// deploy_test.go already asks whether a deployment with no token is refused.
// This asks the half that needs a database: that the refusal leaves no audit
// row behind, because the trail is what a review reads afterwards and a
// deployment that never happened must not appear in it.
func TestARefusedDeploymentIsNotInTheTrail(t *testing.T) {
	pool := optest.Pool(t)
	op := operator.New(pool)
	service := New(op, Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	sess := optest.Session(account)

	t.Setenv("GITHUB_DEPLOY_TOKEN", "")
	t.Setenv("GITHUB_REPOSITORY", "")

	if _, err := service.TriggerDeploy(context.Background(), sess, "main", "release"); !errors.Is(err, ErrDeployNotConfigured) {
		t.Fatalf("a deployment with no token answered %v", err)
	}
	if got := optest.AuditCount(t, pool, account.ID, "deploy.trigger"); got != 0 {
		t.Errorf("a refused deployment wrote %d audit rows", got)
	}
}
