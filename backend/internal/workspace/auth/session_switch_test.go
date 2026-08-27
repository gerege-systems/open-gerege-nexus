package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedTenant leaves one tenant behind, with a membership for userID only when
// asked — a tenant somebody is not in is half of what switching has to refuse.
func seedTenant(t *testing.T, pool *pgxpool.Pool, userID string, member bool) string {
	t.Helper()
	ctx := context.Background()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]

	var tenantID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.tenants (name, slug) VALUES ($1,$1) RETURNING id::text`,
		"switchtest-"+suffix).Scan(&tenantID); err != nil {
		t.Fatal(err)
	}
	if member {
		if _, err := pool.Exec(ctx,
			`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1,$2)`, tenantID, userID); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id=$1`, tenantID)
	})
	return tenantID
}

// The point of the feature: a person in two organisations can reach the second
// one, and the token they were carrying stops working when they do.
func TestSwitchingTenantsRotatesTheToken(t *testing.T) {
	pool := openPool(t)
	userID, first := seedMember(t, pool)
	second := seedTenant(t, pool, userID, true)
	store := auth.NewSessionStore(pool, time.Hour*12)
	ctx := context.Background()

	token, expiresAt, err := store.Create(ctx, userID, first, "password", "test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	next, nextExpiry, err := store.SwitchTenant(ctx, token, second)
	if err != nil {
		t.Fatalf("switching to a tenant the user belongs to failed: %v", err)
	}
	if next == token {
		t.Fatal("the session token was not rotated")
	}

	claims, err := store.Resolve(ctx, next)
	if err != nil {
		t.Fatalf("the new token did not resolve: %v", err)
	}
	if claims.TenantID != second {
		t.Fatalf("tenant=%s want=%s", claims.TenantID, second)
	}
	if claims.UserID != userID {
		t.Fatalf("user=%s want=%s", claims.UserID, userID)
	}

	// A switch is not a sign-in, so it must not push the expiry out. Compared
	// to the second: both values come from the same stored timestamp, and only
	// a deliberate extension would move it.
	if !nextExpiry.Truncate(time.Second).Equal(expiresAt.Truncate(time.Second)) {
		t.Fatalf("expiry moved: %s -> %s", expiresAt, nextExpiry)
	}

	if _, err := store.Resolve(ctx, token); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("the old token still resolves: %v", err)
	}
}

// The other half: asking for a tenant you hold no membership in is refused, and
// refused without disturbing the session you already have.
func TestSwitchingToATenantYouAreNotInIsRefused(t *testing.T) {
	pool := openPool(t)
	userID, first := seedMember(t, pool)
	stranger := seedTenant(t, pool, userID, false)
	store := auth.NewSessionStore(pool, time.Hour*12)
	ctx := context.Background()

	token, _, err := store.Create(ctx, userID, first, "password", "test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := store.SwitchTenant(ctx, token, stranger); !errors.Is(err, auth.ErrNotAMember) {
		t.Fatalf("err=%v want=%v", err, auth.ErrNotAMember)
	}

	claims, err := store.Resolve(ctx, token)
	if err != nil {
		t.Fatalf("the refused switch cost the caller their session: %v", err)
	}
	if claims.TenantID != first {
		t.Fatalf("tenant=%s want=%s", claims.TenantID, first)
	}
}

// A token that is no longer good cannot be traded for one in another tenant.
func TestSwitchingWithADeadSessionFails(t *testing.T) {
	pool := openPool(t)
	userID, first := seedMember(t, pool)
	second := seedTenant(t, pool, userID, true)
	store := auth.NewSessionStore(pool, time.Hour*12)
	ctx := context.Background()

	token, _, err := store.Create(ctx, userID, first, "password", "test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Revoke(ctx, token); err != nil {
		t.Fatal(err)
	}

	if _, _, err := store.SwitchTenant(ctx, token, second); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("err=%v want=%v", err, auth.ErrSessionInvalid)
	}
}
