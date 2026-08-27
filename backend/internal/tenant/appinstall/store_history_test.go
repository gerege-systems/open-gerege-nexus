package appinstall

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The history and overview screens are almost entirely SQL: a LATERAL join
// that reduces an installation to its most recent event, and a left join from
// a JSONB field to the users table that has to survive the value "system"
// sitting where a UUID otherwise would. Neither is observable without a schema.
//
//	AUTH_TEST_DATABASE_URL=postgres://... go test ./internal/operator/...
func historyPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to a migrated test database to run the store history tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatalf("ping: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// installedApp seeds a tenant, a user, an app, a version and an installation,
// and returns the tenant id, the user id, the app id and the installation id.
func installedApp(t *testing.T, pool *pgxpool.Pool) (tenantID, userID, appID, installID string) {
	t.Helper()
	ctx := context.Background()
	suffix := uuid.NewString()[:8]

	if err := pool.QueryRow(ctx,
		`INSERT INTO platform.tenants (name, slug) VALUES ($1, $2) RETURNING id::text`,
		"History Probe "+suffix, "history-"+suffix).Scan(&tenantID); err != nil {
		t.Fatalf("tenant: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO platform.users (email, password_hash, name) VALUES ($1,'x',$2) RETURNING id::text`,
		"history+"+suffix+"@identity.invalid", "Ноён Түүх").Scan(&userID); err != nil {
		t.Fatalf("user: %v", err)
	}

	appID = "io.test.history." + suffix
	if _, err := pool.Exec(ctx,
		`INSERT INTO platform.apps (id, slug, name) VALUES ($1, $2, $3)`,
		appID, "history-"+suffix, "History Probe"); err != nil {
		t.Fatalf("app: %v", err)
	}

	// Two versions, the newer one carrying a chronicle entry inside its
	// manifest exactly as SyncCatalog records it.
	manifest := func(version, summaryMN, summaryEN string, withNotes bool) []byte {
		m := map[string]any{"id": appID, "name": "History Probe", "version": version}
		if withNotes {
			m["release_notes"] = map[string]any{
				"kind":    "feature",
				"summary": map[string]string{"mn": summaryMN, "en": summaryEN},
				"details": map[string]string{"mn": "Дэлгэрэнгүй", "en": "Details"},
				"authors": []string{"craftzbay"},
			}
		}
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	for _, v := range []struct {
		version string
		notes   bool
	}{{"1.0.0", false}, {"1.1.0", true}} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO platform.app_versions (app_id, version, manifest) VALUES ($1,$2,$3)`,
			appID, v.version, manifest(v.version, "Шинэ зүйл", "Something new", v.notes)); err != nil {
			t.Fatalf("version %s: %v", v.version, err)
		}
	}

	if err := pool.QueryRow(ctx,
		`INSERT INTO tenant.app_installations (tenant_id, app_id, installed_version)
		 VALUES ($1,$2,'1.0.0') RETURNING id::text`, tenantID, appID).Scan(&installID); err != nil {
		t.Fatalf("installation: %v", err)
	}

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM platform.apps WHERE id=$1`, appID)
		_, _ = pool.Exec(bg, `DELETE FROM platform.tenants WHERE id=$1`, tenantID)
		_, _ = pool.Exec(bg, `DELETE FROM platform.users WHERE id=$1`, userID)
	})
	return tenantID, userID, appID, installID
}

func event(t *testing.T, pool *pgxpool.Pool, installID, kind string, details map[string]string) {
	t.Helper()
	raw, err := json.Marshal(details)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO tenant.installation_events (installation_id, event_type, details, created_at)
		 VALUES ($1,$2,$3, NOW())`, installID, kind, raw); err != nil {
		t.Fatalf("event %s: %v", kind, err)
	}
}

func TestReleaseHistoryCarriesTheChronicleInTheCallersLanguage(t *testing.T) {
	pool := historyPool(t)
	_, _, appID, _ := installedApp(t, pool)
	server := New(Deps{DB: pool})

	entries, err := server.appReleaseHistory(httptest.NewRequest("GET", "/", nil), appID, "mn")
	if err != nil {
		t.Fatalf("release history: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected both recorded versions, got %d", len(entries))
	}

	byVersion := map[string]historyEntry{}
	for _, e := range entries {
		byVersion[e.Version] = e
	}
	if got := byVersion["1.1.0"].Summary; got != "Шинэ зүйл" {
		t.Errorf("the chronicle did not reach the timeline in Mongolian: %q", got)
	}
	if got := byVersion["1.1.0"].Kind; got != "feature" {
		t.Errorf("kind %q", got)
	}
	// A version recorded before the chronicle existed still appears: it shipped,
	// and a history that hides it is wrong in the direction that matters.
	if byVersion["1.0.0"].Version == "" {
		t.Error("a version with no release notes fell out of the history")
	}
	if byVersion["1.0.0"].Summary != "" {
		t.Error("a version with no release notes invented one")
	}

	english, err := server.appReleaseHistory(httptest.NewRequest("GET", "/", nil), appID, "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range english {
		if e.Version == "1.1.0" && e.Summary != "Something new" {
			t.Errorf("english summary %q", e.Summary)
		}
	}
}

func TestInstallationHistoryTellsAPersonFromTheSweep(t *testing.T) {
	pool := historyPool(t)
	tenantID, userID, appID, installID := installedApp(t, pool)
	server := New(Deps{DB: pool})

	event(t, pool, installID, "installed", map[string]string{"version": "1.0.0", "user_id": userID})
	event(t, pool, installID, "upgraded", map[string]string{"from": "1.0.0", "to": "1.1.0", "user_id": "system"})

	entries, installed, err := server.appInstallationHistory(
		httptest.NewRequest("GET", "/", nil), tenantID, appID)
	if err != nil {
		t.Fatalf("installation history: %v", err)
	}
	if installed != "1.0.0" {
		t.Errorf("installed version %q", installed)
	}
	if len(entries) != 2 {
		t.Fatalf("expected two events, got %d", len(entries))
	}

	byType := map[string]historyEntry{}
	for _, e := range entries {
		byType[e.Type] = e
	}
	person := byType["installed"]
	if person.System {
		t.Error("a person's install was recorded as the sweep")
	}
	if person.ActorName != "Ноён Түүх" {
		t.Errorf("the actor's name did not resolve: %q", person.ActorName)
	}
	sweep := byType["upgraded"]
	if !sweep.System {
		t.Error("the sweep's upgrade was not marked as system")
	}
	// "system" is not a UUID, and the join has to survive that rather than
	// failing the whole query — which is the bug this test exists for.
	if sweep.ActorName != "" {
		t.Errorf("the sweep resolved to a person named %q", sweep.ActorName)
	}
	if sweep.From != "1.0.0" || sweep.Version != "1.1.0" {
		t.Errorf("upgrade recorded as %q → %q", sweep.From, sweep.Version)
	}
}

func TestHeldReadsTheLatestEventNotAnyEvent(t *testing.T) {
	pool := historyPool(t)
	tenantID, userID, appID, installID := installedApp(t, pool)
	server := New(Deps{DB: pool})

	event(t, pool, installID, "held", map[string]string{"reason": "permissions", "added": "contacts.manage"})
	held, err := server.heldApps(httptest.NewRequest("GET", "/", nil), tenantID)
	if err != nil {
		t.Fatalf("held: %v", err)
	}
	if !held[appID] {
		t.Fatal("an installation whose last event is a hold was not reported as held")
	}

	// Agreeing to the update ends the hold. Reading for the presence of a held
	// event anywhere in the history would keep claiming it for ever.
	event(t, pool, installID, "upgraded", map[string]string{"from": "1.0.0", "to": "1.1.0", "user_id": userID})
	held, err = server.heldApps(httptest.NewRequest("GET", "/", nil), tenantID)
	if err != nil {
		t.Fatalf("held after upgrade: %v", err)
	}
	if held[appID] {
		t.Error("an installation that has since been upgraded is still reported as held")
	}
}

func TestPickLocaleFallsBackThroughSourceThenEnglish(t *testing.T) {
	both := map[string]string{"mn": "Монгол", "en": "English"}
	if got := pickLocale(both, "mn"); got != "Монгол" {
		t.Errorf("mn: %q", got)
	}
	if got := pickLocale(both, "en"); got != "English" {
		t.Errorf("en: %q", got)
	}
	// A language the note was not written in falls back to the source rather
	// than rendering an empty line.
	if got := pickLocale(both, "fr"); got != "Монгол" {
		t.Errorf("fr should fall back to the source language, got %q", got)
	}
	if got := pickLocale(map[string]string{"en": "English"}, "fr"); got != "English" {
		t.Errorf("with no source language, english is the fallback, got %q", got)
	}
	if got := pickLocale(nil, "mn"); got != "" {
		t.Errorf("an absent field should render empty, got %q", got)
	}
}
