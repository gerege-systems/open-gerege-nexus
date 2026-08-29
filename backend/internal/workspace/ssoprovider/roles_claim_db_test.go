package ssoprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// The `roles` scope, and the one claim on it that grants real authority.
//
// Grafana is told `platform_admin && 'GrafanaAdmin' || 'Viewer'`, so whatever
// this claim says is who owns the monitoring stack. What makes it worth a test
// of its own is that "an admin" and "the admin" look identical from inside a
// single organisation: every tenant on a deployment has an `admin` role, and if
// this claim were true for all of them then anybody who could get an
// organisation created here would become a server administrator of Grafana.

func withRolesScope(c *Client) {
	c.Scopes = append(c.Scopes, "roles")
}

func TestRolesClaimNamesTheDeploymentsFirstAdministrator(t *testing.T) {
	f := newFixture(t, withRolesScope)
	ctx := context.Background()
	makeTenantAdmin(t, f)

	// An organisation older than the fixture's. Inserted rather than assumed:
	// the shared development database has years of them and a CI database has
	// none, so a test that relies on what is already there passes in one place
	// and fails in the other — which is how the first version of this went red.
	insertTenant(t, f, "organisation", "1970-01-01T00:00:00Z", nil)

	t.Run("an admin of a later organisation is not the platform admin", func(t *testing.T) {
		claims := verifyIDToken(t, f, idTokenWithRoles(t, f))

		if roles := stringsOf(claims["roles"]); !slices.Contains(roles, "admin") {
			t.Errorf("roles claim is %v, expected it to contain \"admin\"", claims["roles"])
		}
		if claims["platform_admin"] != false {
			t.Errorf("platform_admin is %v for an organisation that is not the first one",
				claims["platform_admin"])
		}
	})

	t.Run("the admin of the first organisation is", func(t *testing.T) {
		// Older than the one above, so the fixture's organisation becomes the
		// deployment's first.
		if _, err := f.pool.Exec(ctx,
			`UPDATE registry.tenants SET created_at = '1960-01-01T00:00:00Z' WHERE id = $1::uuid`,
			f.tenantID); err != nil {
			t.Fatalf("backdate the organisation: %v", err)
		}

		// And a *personal* workspace older than everything, which is a row in
		// the same table and must not count. Without the kind filter in
		// grantedRoles this makes the assertion below fail — which is the point
		// of creating it.
		owner := insertUser(t, f)
		insertTenant(t, f, "personal", "1950-01-01T00:00:00Z", &owner)

		claims := verifyIDToken(t, f, idTokenWithRoles(t, f))
		if claims["platform_admin"] != true {
			t.Errorf("platform_admin is %v for the deployment's first organisation",
				claims["platform_admin"])
		}
	})

}

// userinfo has to answer the same thing as the id_token. Grafana reads whichever
// it can: the two disagreeing would make an operator an administrator or not
// depending on which endpoint won a race.
func TestUserInfoCarriesTheSameRoles(t *testing.T) {
	f := newFixture(t, withRolesScope)
	makeTenantAdmin(t, f)

	accessToken, _ := exchangeForTokens(t, f, "openid profile email roles")

	req := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	f.provider.HandleUserInfo(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("userinfo returned %d: %s", rec.Code, rec.Body.String())
	}

	var info map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &info)
	if roles := stringsOf(info["roles"]); !slices.Contains(roles, "admin") {
		t.Errorf("userinfo roles is %v, expected it to contain \"admin\"", info["roles"])
	}
	if _, present := info["platform_admin"]; !present {
		t.Error("userinfo carried no platform_admin claim")
	}
}

// Without the scope, neither claim appears at all. A relying party that was
// never granted this must not be able to read the org chart out of a token it
// asked for something else with.
func TestRolesAreAbsentWithoutTheScope(t *testing.T) {
	f := newFixture(t)
	makeTenantAdmin(t, f)

	claims := verifyIDToken(t, f, idTokenWithRoles(t, f, "openid profile email"))
	if _, present := claims["roles"]; present {
		t.Errorf("roles leaked into a token that was not granted the scope: %v", claims["roles"])
	}
	if _, present := claims["platform_admin"]; present {
		t.Error("platform_admin leaked into a token that was not granted the roles scope")
	}
}

// helpers

// insertUser adds an account with no membership anywhere.
func insertUser(t *testing.T, f *fixture) string {
	t.Helper()
	var id string
	if err := f.pool.QueryRow(context.Background(),
		`INSERT INTO registry.users (email, password_hash, name)
		 VALUES ('roles-' || substr(gen_random_uuid()::text, 1, 8) || '@example.com', 'x', 'Roles Test')
		 RETURNING id::text`).Scan(&id); err != nil {
		t.Fatalf("user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id = $1::uuid`, id)
	})
	return id
}

// insertTenant adds an organisation or a personal workspace with an explicit
// creation time, so "first" is decided by this test rather than by whatever the
// database already holds.
func insertTenant(t *testing.T, f *fixture, kind, createdAt string, owner *string) string {
	t.Helper()
	var id string
	if err := f.pool.QueryRow(context.Background(),
		`INSERT INTO registry.tenants (slug, name, kind, owner_user_id, created_at)
		 VALUES ('roles-' || substr(gen_random_uuid()::text, 1, 8), 'Roles test', $1, $2, $3::timestamptz)
		 RETURNING id::text`, kind, owner, createdAt).Scan(&id); err != nil {
		t.Fatalf("tenant (%s): %v", kind, err)
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1::uuid`, id)
	})
	return id
}

// makeTenantAdmin gives the fixture's user the `admin` role in the fixture's
// organisation — the three rows tenants.ensureAdmin writes for the first
// account on a deployment.
func makeTenantAdmin(t *testing.T, f *fixture) {
	t.Helper()
	ctx := context.Background()

	var membershipID string
	if err := f.pool.QueryRow(ctx,
		`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1::uuid, $2::uuid)
		 ON CONFLICT (tenant_id, user_id) DO UPDATE SET tenant_id = EXCLUDED.tenant_id
		 RETURNING id::text`, f.tenantID, f.userID).Scan(&membershipID); err != nil {
		t.Fatalf("membership: %v", err)
	}

	var roleID string
	if err := f.pool.QueryRow(ctx,
		`INSERT INTO workspace.roles (tenant_id, code, name) VALUES ($1::uuid, 'admin', 'Tenant Admin')
		 ON CONFLICT (tenant_id, code) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id::text`, f.tenantID).Scan(&roleID); err != nil {
		t.Fatalf("role: %v", err)
	}

	if _, err := f.pool.Exec(ctx,
		`INSERT INTO workspace.membership_roles (membership_id, role_id) VALUES ($1::uuid, $2::uuid)
		 ON CONFLICT DO NOTHING`, membershipID, roleID); err != nil {
		t.Fatalf("grant: %v", err)
	}
}

// idTokenWithRoles runs one authorization-code exchange asking for the given
// scopes, defaulting to the set Grafana asks for.
func idTokenWithRoles(t *testing.T, f *fixture, scope ...string) string {
	t.Helper()
	requested := "openid profile email roles"
	if len(scope) > 0 {
		requested = scope[0]
	}
	_, idToken := exchangeForTokens(t, f, requested)
	return idToken
}

func stringsOf(claim any) []string {
	raw, ok := claim.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
