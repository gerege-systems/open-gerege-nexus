package auth

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The lockout is enforced entirely by the SQL in loginFailureStatement —
// the threshold, the window, and the restart after a lapsed lock are all
// decided by the statement rather than by Go — so it needs a real schema.
//
//	AUTH_TEST_DATABASE_URL=postgres://... go test ./internal/operator/...
//
// Without one these skip, so `go test ./...` stays green on a machine with no
// database.
func lockoutPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to a migrated test database to run the login lockout tests")
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

// lockoutUser inserts a throwaway account and returns its id.
func lockoutUser(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	email := "lockout+" + uuid.NewString() + "@identity.invalid"
	var id string
	if err := pool.QueryRow(ctx,
		`INSERT INTO platform.users(email, password_hash, name) VALUES($1, 'x', 'lockout probe') RETURNING id::text`,
		email).Scan(&id); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM platform.users WHERE id=$1`, id)
	})
	return id
}

// lockoutState reads the two columns the lockout is made of.
func lockoutState(t *testing.T, pool *pgxpool.Pool, userID string) (attempts int, locked bool) {
	t.Helper()
	if err := pool.QueryRow(context.Background(),
		`SELECT failed_login_attempts, COALESCE(locked_until > NOW(), FALSE) FROM platform.users WHERE id=$1`,
		userID).Scan(&attempts, &locked); err != nil {
		t.Fatalf("read lockout state: %v", err)
	}
	return attempts, locked
}

func TestLoginLockoutEngagesOnTheThreshold(t *testing.T) {
	pool := lockoutPool(t)
	server := &Handlers{db: pool}
	userID := lockoutUser(t, pool)

	for i := 1; i < maxLoginFailures; i++ {
		server.recordLoginFailure(context.Background(), userID)
		attempts, locked := lockoutState(t, pool, userID)
		if attempts != i {
			t.Fatalf("after %d failures: counted %d", i, attempts)
		}
		if locked {
			t.Fatalf("locked after %d failures; the threshold is %d", i, maxLoginFailures)
		}
	}

	server.recordLoginFailure(context.Background(), userID)
	attempts, locked := lockoutState(t, pool, userID)
	if attempts != maxLoginFailures || !locked {
		t.Fatalf("the threshold failure should lock: attempts=%d locked=%v", attempts, locked)
	}
}

// A lapsed lockout must start counting again. This is the regression: the
// counter used only ever to climb, so once it had reached maxLoginFailures the
// next single mistyped password met the threshold on its own and re-locked the
// account for another full window — indefinitely, and on demand for anybody who
// knew the address.
func TestLapsedLockoutDoesNotRelockOnASingleFailure(t *testing.T) {
	pool := lockoutPool(t)
	server := &Handlers{db: pool}
	userID := lockoutUser(t, pool)

	// An account that reached the threshold and whose window has since passed.
	if _, err := pool.Exec(context.Background(),
		`UPDATE platform.users SET failed_login_attempts=$2, locked_until=NOW() - INTERVAL '1 second' WHERE id=$1`,
		userID, maxLoginFailures); err != nil {
		t.Fatalf("arrange a lapsed lockout: %v", err)
	}

	server.recordLoginFailure(context.Background(), userID)

	attempts, locked := lockoutState(t, pool, userID)
	if locked {
		t.Fatal("one failure after a lapsed lockout re-locked the account")
	}
	if attempts != 1 {
		t.Fatalf("the count should restart at 1 after a lapsed lockout, got %d", attempts)
	}
}

// Reaching the threshold again after a lapse still locks, so restarting the
// count does not amount to switching the lockout off.
func TestLockoutStillEngagesAfterALapse(t *testing.T) {
	pool := lockoutPool(t)
	server := &Handlers{db: pool}
	userID := lockoutUser(t, pool)

	if _, err := pool.Exec(context.Background(),
		`UPDATE platform.users SET failed_login_attempts=$2, locked_until=NOW() - INTERVAL '1 second' WHERE id=$1`,
		userID, maxLoginFailures); err != nil {
		t.Fatalf("arrange a lapsed lockout: %v", err)
	}

	for range maxLoginFailures {
		server.recordLoginFailure(context.Background(), userID)
	}

	attempts, locked := lockoutState(t, pool, userID)
	if !locked {
		t.Fatalf("a fresh run of %d failures should lock again: attempts=%d", maxLoginFailures, attempts)
	}
}
