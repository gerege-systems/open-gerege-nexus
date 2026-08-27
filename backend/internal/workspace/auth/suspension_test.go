package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/memo"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The tenant-facing half of CP-2, against a real schema: a suspended
// organisation is refused everywhere, an invitation can be redeemed once, and
// an impersonation produces a session that says what it is.
//
// These run through the actual middleware and the actual handlers rather than
// against the queries, because what is being tested is that the checks are
// *wired in* — a suspension the console records and the platform never reads
// would pass every query-level test there is.

func suspensionServer(t *testing.T) (*Handlers, *pgxpool.Pool) {
	t.Helper()
	pool := lockoutPool(t)
	return &Handlers{
		db:        pool,
		sessions:  NewSessionStore(pool, time.Hour),
		suspended: memo.New[bool](SuspendedTTL),
	}, pool
}

// tenantWithMember creates an organisation, somebody in it, and a live session.
func tenantWithMember(t *testing.T, pool *pgxpool.Pool) (tenantID, userID, token string) {
	t.Helper()
	ctx := context.Background()
	slug := fmt.Sprintf("susp-%d", time.Now().UnixNano())

	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.tenants (slug, name) VALUES ($1, $1) RETURNING id::text`, slug).Scan(&tenantID); err != nil {
		t.Fatalf("create the organisation: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id=$1::uuid`, tenantID)
	})

	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name) VALUES ($1, 'x', 'Member') RETURNING id::text`,
		slug+"@identity.invalid").Scan(&userID); err != nil {
		t.Fatalf("create the person: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id=$1::uuid`, userID) })

	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1::uuid, $2::uuid)`, tenantID, userID); err != nil {
		t.Fatalf("add the membership: %v", err)
	}

	token, err := NewSessionToken()
	if err != nil {
		t.Fatalf("mint a token: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace.sessions (token_hash, user_id, tenant_id, expires_at)
		 VALUES ($1, $2::uuid, $3::uuid, NOW() + INTERVAL '1 hour')`,
		HashSessionToken(token), userID, tenantID); err != nil {
		t.Fatalf("create the session: %v", err)
	}
	return tenantID, userID, token
}

// A live session in a suspended organisation is refused by the middleware
// every other authenticated route sits behind.
func TestASuspendedOrganisationIsRefusedByEveryRoute(t *testing.T) {
	server, pool := suspensionServer(t)
	tenantID, _, token := tenantWithMember(t, pool)

	reached := false
	guarded := server.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/contacts", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	recorder := httptest.NewRecorder()
	guarded.ServeHTTP(recorder, request)
	if !reached || recorder.Code != http.StatusOK {
		t.Fatalf("a healthy organisation was refused: %d", recorder.Code)
	}

	if _, err := pool.Exec(context.Background(),
		`UPDATE registry.tenants SET suspended_at = NOW(), suspension_reason = 'unpaid' WHERE id = $1::uuid`,
		tenantID); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	server.suspended = memo.New[bool](SuspendedTTL) // what the invalidation bus does across replicas

	reached = false
	request = httptest.NewRequest(http.MethodGet, "/api/v1/contacts", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	recorder = httptest.NewRecorder()
	guarded.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("a suspended organisation answered %d, want 403", recorder.Code)
	}
	if reached {
		t.Fatal("the request reached the handler")
	}
	// The reason is shown: somebody being refused should know whether to call
	// their account manager or their administrator.
	if !strings.Contains(recorder.Body.String(), "unpaid") {
		t.Fatalf("the refusal did not carry the reason: %s", recorder.Body.String())
	}
}

// Signing in is refused at the one place every method of signing in passes
// through, so no route needs to remember.
func TestSigningInToASuspendedOrganisationIsRefused(t *testing.T) {
	server, pool := suspensionServer(t)
	tenantID, userID, _ := tenantWithMember(t, pool)

	if _, err := pool.Exec(context.Background(),
		`UPDATE registry.tenants SET suspended_at = NOW() WHERE id = $1::uuid`, tenantID); err != nil {
		t.Fatalf("suspend: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	if _, _, err := server.IssueSession(request, userID, tenantID, "password"); err == nil {
		t.Fatal("a session was issued for a suspended organisation")
	}
}
