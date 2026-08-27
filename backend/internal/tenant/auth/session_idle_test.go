package auth_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedMember leaves one user holding a membership in one tenant.
func seedMember(t *testing.T, pool *pgxpool.Pool) (userID, tenantID string) {
	t.Helper()
	ctx := context.Background()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]

	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.tenants (name, slug) VALUES ($1,$1) RETURNING id::text`,
		"sesstest-"+suffix).Scan(&tenantID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name) VALUES ($1,'x','t') RETURNING id::text`,
		"sesstest-"+suffix+"@example.mn").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenant.memberships (tenant_id, user_id) VALUES ($1,$2)`, tenantID, userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id=$1`, tenantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id=$1`, userID)
	})
	return userID, tenantID
}

// A session nobody has used stops working before its twelve hours are up. The
// clock is moved rather than waited on, which is the only way to test a
// ninety-minute rule in a test suite.
func TestASessionLeftAloneStopsWorking(t *testing.T) {
	t.Setenv("SESSION_IDLE_TIMEOUT", "30m")
	pool := openPool(t)
	userID, tenantID := seedMember(t, pool)
	store := auth.NewSessionStore(pool, time.Hour*12)
	ctx := context.Background()

	token, _, err := store.Create(ctx, userID, tenantID, "password", "test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(ctx, token); err != nil {
		t.Fatalf("a fresh session did not resolve: %v", err)
	}

	// Idle for longer than the timeout.
	if _, err := pool.Exec(ctx,
		`UPDATE tenant.sessions SET last_seen_at = NOW() - INTERVAL '31 minutes' WHERE user_id=$1`,
		userID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(ctx, token); err == nil {
		t.Error("a session idle past the timeout still resolved")
	}

	// The absolute lifetime has not run out, so this really was the idle rule.
	var expiresInFuture bool
	if err := pool.QueryRow(ctx,
		`SELECT expires_at > NOW() FROM tenant.sessions WHERE user_id=$1`, userID).Scan(&expiresInFuture); err != nil {
		t.Fatal(err)
	}
	if !expiresInFuture {
		t.Error("the session had also expired outright; the test proved nothing")
	}
}

// Using a session keeps it alive — otherwise a person working steadily would be
// signed out mid-task, which is the failure that makes people disable timeouts.
func TestUsingASessionKeepsItAlive(t *testing.T) {
	t.Setenv("SESSION_IDLE_TIMEOUT", "30m")
	pool := openPool(t)
	userID, tenantID := seedMember(t, pool)
	store := auth.NewSessionStore(pool, time.Hour*12)
	ctx := context.Background()

	token, _, err := store.Create(ctx, userID, tenantID, "password", "test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	// Idle for longer than the touch interval but well inside the timeout.
	if _, err := pool.Exec(ctx,
		`UPDATE tenant.sessions SET last_seen_at = NOW() - INTERVAL '20 minutes' WHERE user_id=$1`,
		userID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(ctx, token); err != nil {
		t.Fatalf("a session used inside the timeout was refused: %v", err)
	}

	var idleSeconds float64
	if err := pool.QueryRow(ctx,
		`SELECT EXTRACT(EPOCH FROM (NOW() - last_seen_at)) FROM tenant.sessions WHERE user_id=$1`,
		userID).Scan(&idleSeconds); err != nil {
		t.Fatal(err)
	}
	if idleSeconds > 60 {
		t.Errorf("resolving did not refresh last_seen_at: still %.0fs idle", idleSeconds)
	}
}

// The write is throttled, or every authenticated read would carry one.
func TestResolvingDoesNotWriteOnEveryRequest(t *testing.T) {
	t.Setenv("SESSION_IDLE_TIMEOUT", "30m")
	pool := openPool(t)
	userID, tenantID := seedMember(t, pool)
	store := auth.NewSessionStore(pool, time.Hour*12)
	ctx := context.Background()

	token, _, err := store.Create(ctx, userID, tenantID, "password", "test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	var first time.Time
	if err := pool.QueryRow(ctx, `SELECT last_seen_at FROM tenant.sessions WHERE user_id=$1`, userID).Scan(&first); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		if _, err := store.Resolve(ctx, token); err != nil {
			t.Fatal(err)
		}
	}
	var after time.Time
	if err := pool.QueryRow(ctx, `SELECT last_seen_at FROM tenant.sessions WHERE user_id=$1`, userID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if !after.Equal(first) {
		t.Errorf("five resolves inside the touch interval wrote to the row: %v → %v", first, after)
	}
}

// Turning the timeout off has to actually turn it off: a deployment whose
// clients sit open all day is a supported configuration.
func TestTheIdleTimeoutCanBeTurnedOff(t *testing.T) {
	t.Setenv("SESSION_IDLE_TIMEOUT", "0")
	pool := openPool(t)
	userID, tenantID := seedMember(t, pool)
	store := auth.NewSessionStore(pool, time.Hour*12)
	ctx := context.Background()

	token, _, err := store.Create(ctx, userID, tenantID, "password", "test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE tenant.sessions SET last_seen_at = NOW() - INTERVAL '30 days' WHERE user_id=$1`,
		userID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(ctx, token); err != nil {
		t.Errorf("the timeout was off but a long-idle session was refused: %v", err)
	}
}

// Signing out everywhere has to include the session it was asked from.
func TestRevokingEverySessionLeavesNoneBehind(t *testing.T) {
	pool := openPool(t)
	userID, tenantID := seedMember(t, pool)
	store := auth.NewSessionStore(pool, time.Hour*12)
	ctx := context.Background()

	tokens := make([]string, 0, 3)
	for range 3 {
		token, _, err := store.Create(ctx, userID, tenantID, "password", "test", "127.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, token)
	}

	revoked, err := store.RevokeAllForUser(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if revoked != 3 {
		t.Errorf("revoked %d sessions, want 3", revoked)
	}
	for i, token := range tokens {
		if _, err := store.Resolve(ctx, token); err == nil {
			t.Errorf("session %d still resolved after signing out everywhere", i)
		}
	}
}

// One person signing out everywhere must not sign anybody else out.
func TestSigningOutEverywhereIsScopedToOnePerson(t *testing.T) {
	pool := openPool(t)
	first, tenantID := seedMember(t, pool)
	second, otherTenant := seedMember(t, pool)
	store := auth.NewSessionStore(pool, time.Hour*12)
	ctx := context.Background()

	_, _, err := store.Create(ctx, first, tenantID, "password", "test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	othersToken, _, err := store.Create(ctx, second, otherTenant, "password", "test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.RevokeAllForUser(ctx, first); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(ctx, othersToken); err != nil {
		t.Errorf("somebody else's session was revoked too: %v", err)
	}
}
