package backup

import (
	"context"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
)

// The front page shows the latest backup and the latest restore test, which
// answers "was there one last night". The history is what answers "has this
// been failing", and it has to carry both kinds and the size the script
// reported.
func TestTheHistoryCarriesBothKindsAndTheSize(t *testing.T) {
	pool := optest.Pool(t)
	op := operator.New(pool)
	service := New(op, Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	ctx := context.Background()

	var backupID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO operator.platform_backups (kind, size_bytes, ok, detail)
		 VALUES ('backup', 5242880, TRUE, 'nightly') RETURNING id::text`).Scan(&backupID); err != nil {
		t.Fatalf("write a backup row: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operator.platform_backups WHERE id = $1::uuid`, backupID)
	})

	if err := service.RecordRestoreTest(ctx, optest.Session(account),
		"restored onto a scratch database", "prove the history"); err != nil {
		t.Fatalf("record a restore test: %v", err)
	}

	history, err := service.History(ctx, 50)
	if err != nil {
		t.Fatalf("read the history: %v", err)
	}

	kinds := map[string]Entry{}
	for _, entry := range history {
		if _, seen := kinds[entry.Kind]; !seen {
			kinds[entry.Kind] = entry
		}
	}
	if _, ok := kinds["backup"]; !ok {
		t.Fatal("the history has no backup in it")
	}
	if _, ok := kinds["restore_test"]; !ok {
		t.Fatal("the history has no restore test in it")
	}
	if size := kinds["backup"].SizeMB; size < 4.9 || size > 5.1 {
		t.Fatalf("five megabytes came back as %v", size)
	}
	if kinds["restore_test"].RecordedBy == "" {
		t.Error("a restore test recorded by an operator does not say who")
	}
	// Newest first: the restore test was written last.
	if history[0].Kind != "restore_test" {
		t.Fatalf("the history starts with a %q row", history[0].Kind)
	}
}
