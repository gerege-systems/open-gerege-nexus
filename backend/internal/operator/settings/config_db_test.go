package settings

import (
	"context"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/flags"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CP-3 from the console's side: a setting changes and can be put back, a flag
// can be aimed at one organisation, and — the fourth of the access-mode tests
// the plan asks for — an invitation still creates an account while the
// platform is closed to strangers.

func configService(t *testing.T, pool *pgxpool.Pool) *Service {
	t.Helper()
	store := settings.NewStore(pool)
	settings.UseStore(store)
	t.Cleanup(func() { settings.UseStore(nil) })
	if err := store.Load(context.Background()); err != nil {
		t.Fatalf("load the settings: %v", err)
	}

	flagStore := flags.NewStore(pool)
	if err := flagStore.Load(context.Background()); err != nil {
		t.Fatalf("load the flags: %v", err)
	}

	return &Service{op: operator.New(pool), db: pool, settings: store, flags: flagStore}
}

// A setting changes, is recorded, and can be put back — and the rollback is
// itself a change rather than an erasure.
func TestASettingChangesAndRollsBack(t *testing.T) {
	pool := optest.Pool(t)
	service := configService(t, pool)
	account, _ := optest.Account(t, pool, operator.RoleOperator)
	sess := optest.Session(account)
	ctx := context.Background()

	// The history is append-only in spirit and cumulative in fact, so a
	// previous run's rows are cleared before counting this one's.
	clear := func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM platform.platform_settings WHERE key = $1`, settings.SessionIdleTimeout)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM platform.platform_settings_history WHERE key = $1`, settings.SessionIdleTimeout)
	}
	clear()
	t.Cleanup(clear)

	if err := service.SetSetting(ctx, sess, settings.SessionIdleTimeout, "20m",
		"a customer asked for tighter sessions"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := settings.Duration(settings.SessionIdleTimeout); got != 20*time.Minute {
		t.Fatalf("the platform reads %v", got)
	}

	// A value the registry refuses never reaches the table.
	if err := service.SetSetting(ctx, sess, settings.SessionIdleTimeout, "soon", "typo"); err == nil {
		t.Fatal("a nonsense duration was accepted")
	}
	if got := settings.Duration(settings.SessionIdleTimeout); got != 20*time.Minute {
		t.Fatalf("a refused value changed the setting to %v", got)
	}

	changes, err := service.SettingHistory(ctx, settings.SessionIdleTimeout)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("the history has %d rows, want 1", len(changes))
	}

	if err := service.RollbackSetting(ctx, sess, changes[0].ID, "it was worse"); err != nil {
		t.Fatalf("roll back: %v", err)
	}
	// Back to the default, because that change was the first: there was no
	// previous value to return to.
	if got := settings.Duration(settings.SessionIdleTimeout); got != 90*time.Minute {
		t.Fatalf("after the rollback the platform reads %v", got)
	}
	// And the rollback is in the history rather than having removed anything.
	changes, err = service.SettingHistory(ctx, settings.SessionIdleTimeout)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("the history has %d rows after a rollback, want 2", len(changes))
	}
}
