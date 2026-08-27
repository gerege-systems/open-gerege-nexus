/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package ssoprovider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/dbguard"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/tenant/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A client belongs to the organisation that registered it and signs in the
// users of every other one — that is what federating a separate deployment
// means, and issueAuthCode says so where it stores the user's tenant rather
// than the client's. The consent endpoints sit behind the session middleware,
// where dbguard binds the connection to the caller's tenant and row-level
// security hides every client but that tenant's own. Reading the client on the
// tenant path there answered "unknown or disabled client" to every first
// consent across a tenant boundary, and no such sign-in ever completed.
//
// This needs the guarded pool to reproduce, so it builds its own rather than
// using newFixture's.
func TestConsentPromptFindsAnotherTenantsClient(t *testing.T) {
	dsn := os.Getenv("OAUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set OAUTH_TEST_DATABASE_URL to a migrated test database")
	}
	ctx := context.Background()

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	guard := &dbguard.Guard{}
	guard.Install(cfg)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := guard.Probe(probeCtx, pool); err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !guard.Enabled() {
		t.Skip("the test database carries no row-level security policies")
	}

	owner := makeTenant(t, pool)  // registers the client
	caller := makeTenant(t, pool) // signs in to it

	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name)
		 VALUES ('consent-'||$1||'@example.com', 'x', 'Consent Tester') RETURNING id::text`,
		NewIdentifier(8)).Scan(&userID); err != nil {
		t.Fatalf("user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id = $1`, userID) })

	provider := &SSOProvider{store: NewStore(pool), issuer: testIssuer}
	provider.AttachSessions(&fakeSessions{claims: auth.UserClaims{
		UserID: userID, TenantID: caller, Email: "consent@example.com",
	}})

	client, err := provider.store.CreateClient(ctx, &Client{
		TenantID:     owner,
		ClientID:     "app_cross_" + NewIdentifier(8),
		ClientName:   "Another Tenant's App",
		ClientType:   clientTypeConfidential,
		RedirectURIs: []string{"https://client.test/callback"},
		GrantTypes:   SupportedGrantTypes,
		Scopes:       []string{"openid", "profile", "email"},
	}, HashSecret("sec_"+NewIdentifier(48)), userID)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	q := url.Values{
		"client_id":             {client.ClientID},
		"redirect_uri":          {client.RedirectURIs[0]},
		"scope":                 {"openid profile email"},
		"state":                 {"state-123"},
		"code_challenge":        {"E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"},
		"code_challenge_method": {"S256"},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/oauth2/consent?"+q.Encode(), nil)
	// What the session middleware puts there, and what dbguard binds the
	// connection to.
	req = req.WithContext(nexus.WithTenantID(req.Context(), caller))
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "test-session"})
	rec := httptest.NewRecorder()
	provider.HandleConsentPrompt(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("the consent screen could not describe another tenant's client: %d — %s",
			rec.Code, rec.Body.String())
	}
}

func makeTenant(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO registry.tenants (slug, name)
		 VALUES ('consent-'||substr(gen_random_uuid()::text, 1, 8), 'Consent test') RETURNING id::text`).
		Scan(&id); err != nil {
		t.Fatalf("tenant: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1`, id) })
	return id
}
