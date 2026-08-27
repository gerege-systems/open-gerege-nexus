/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The two links the console hands out, redeemed here.
 *
 * They were beside the suspension tests while both were methods on one struct.
 * They are about credential_grants and operator_impersonations rather than
 * about suspension, and they travel with access_recovery.go.
 */

package access

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/memo"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

func recoveryServer(t *testing.T) (*Handlers, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to a migrated test database to run the recovery tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	sessions := auth.NewSessionStore(pool, time.Hour)
	// Redeeming either link makes a session, and making one asks whether the
	// organisation is closed first.
	authn := auth.New(auth.Deps{
		DB: pool, Sessions: sessions, Suspended: memo.New[bool](auth.SuspendedTTL),
	})
	return New(pool, nil, authn), pool
}

// tenantWithMember creates an organisation, somebody in it, and a live session.
func tenantWithMember(t *testing.T, pool *pgxpool.Pool) (tenantID, userID, token string) {
	t.Helper()
	ctx := context.Background()
	slug := fmt.Sprintf("recov-%d", time.Now().UnixNano())

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
		`INSERT INTO tenant.memberships (tenant_id, user_id) VALUES ($1::uuid, $2::uuid)`, tenantID, userID); err != nil {
		t.Fatalf("add the membership: %v", err)
	}

	token, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("mint a token: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenant.sessions (token_hash, user_id, tenant_id, expires_at)
		 VALUES ($1, $2::uuid, $3::uuid, NOW() + INTERVAL '1 hour')`,
		auth.HashSessionToken(token), userID, tenantID); err != nil {
		t.Fatalf("create the session: %v", err)
	}
	return tenantID, userID, token
}

// The invitation and reset link: one use, and the account's sessions go with
// the password change.
func TestACredentialLinkWorksOnceAndEndsTheSessions(t *testing.T) {
	server, pool := recoveryServer(t)
	_, userID, token := tenantWithMember(t, pool)
	ctx := context.Background()

	link, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("mint a link: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO registry.credential_grants (user_id, purpose, token_hash, expires_at)
		 VALUES ($1::uuid, 'invite', $2, NOW() + INTERVAL '1 hour')`,
		userID, hashRecoveryToken(link)); err != nil {
		t.Fatalf("issue the grant: %v", err)
	}

	redeem := func() *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"token": link, "password": "a good long password"})
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/credential/redeem",
			strings.NewReader(string(body)))
		recorder := httptest.NewRecorder()
		server.HandleCredentialRedeem(recorder, request)
		return recorder
	}

	if recorder := redeem(); recorder.Code != http.StatusOK {
		t.Fatalf("redeeming answered %d: %s", recorder.Code, recorder.Body.String())
	}
	// The session the account already had is gone: a password given to
	// somebody who was locked out is usually a password somebody else knew.
	if _, err := server.authn.Sessions().Resolve(ctx, token); err == nil {
		t.Fatal("a session survived the password being set")
	}
	// And the link is spent.
	if recorder := redeem(); recorder.Code != http.StatusGone {
		t.Fatalf("reusing the link answered %d, want 410", recorder.Code)
	}
}

// The impersonation handover: one use, a session that ends when the visit
// does, and claims that say whose it really is.
func TestAnImpersonationHandoverProducesAMarkedSession(t *testing.T) {
	server, pool := recoveryServer(t)
	tenantID, userID, _ := tenantWithMember(t, pool)
	ctx := context.Background()

	var operatorID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO operator.operator_accounts (email, name, role, password_hash)
		 VALUES ($1, 'Operator', 'support', 'x') RETURNING id::text`,
		fmt.Sprintf("imp-%d@controlplane.test", time.Now().UnixNano())).Scan(&operatorID); err != nil {
		t.Fatalf("create the operator: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operator.operator_accounts WHERE id=$1::uuid`, operatorID)
	})

	handover, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("mint a handover: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO registry.operator_impersonations
		     (operator_id, operator_email, tenant_id, user_id, reason,
		      handover_hash, handover_expires_at, ends_at)
		 VALUES ($1::uuid, 'operator@example.mn', $2::uuid, $3::uuid, 'a support call',
		         $4, NOW() + INTERVAL '1 minute', NOW() + INTERVAL '30 minutes')`,
		operatorID, tenantID, userID, hashRecoveryToken(handover)); err != nil {
		t.Fatalf("record the impersonation: %v", err)
	}

	redeem := func() *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"token": handover})
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/impersonation/redeem",
			strings.NewReader(string(body)))
		recorder := httptest.NewRecorder()
		server.HandleImpersonationRedeem(recorder, request)
		return recorder
	}

	recorder := redeem()
	if recorder.Code != http.StatusOK {
		t.Fatalf("the handover answered %d: %s", recorder.Code, recorder.Body.String())
	}

	var sessionToken string
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == auth.SessionCookieName {
			sessionToken = cookie.Value
		}
	}
	if sessionToken == "" {
		t.Fatal("no session cookie was set")
	}

	claims, err := server.authn.Sessions().Resolve(ctx, sessionToken)
	if err != nil {
		t.Fatalf("the borrowed session does not resolve: %v", err)
	}
	// The flag the banner is drawn from, and the mark every audit row written
	// from this session will carry.
	if !claims.Impersonated || claims.ImpersonatedBy != operatorID {
		t.Fatalf("the session does not know it is an impersonation: %+v", claims)
	}
	if claims.UserID != userID || claims.TenantID != tenantID {
		t.Fatalf("the session is for the wrong person: %+v", claims)
	}

	// The organisation is told, in its own trail, that somebody came in.
	var started int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM tenant.audit_events
		  WHERE tenant_id = $1::uuid AND action = 'security.impersonation.started'`,
		tenantID).Scan(&started); err != nil {
		t.Fatalf("read the organisation's trail: %v", err)
	}
	if started != 1 {
		t.Fatalf("the organisation's trail has %d rows for the visit, want 1", started)
	}

	// A handover is worth one journey.
	if second := redeem(); second.Code != http.StatusGone {
		t.Fatalf("reusing the handover answered %d, want 410", second.Code)
	}
}
