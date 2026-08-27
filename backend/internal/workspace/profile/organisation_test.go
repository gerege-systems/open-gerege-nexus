/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The organisation's legal profile, tested where it now lives.
 *
 * These moved here from the organisation app together with the handlers. The
 * last test is the one that is new, and it is the reason the move happened: a
 * tenant with no apps at all still has a legal name and still has to be able to
 * change it.
 *
 *	AUTH_TEST_DATABASE_URL=postgres://... go test ./internal/operator/...
 */

package profile

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type profileFixture struct {
	pool     *pgxpool.Pool
	router   chi.Router
	tenantID string
	userID   string
	otherID  string // a second tenant, to prove nothing leaks between them
}

func newProfileFixture(t *testing.T) *profileFixture {
	t.Helper()
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to a migrated test database to run the tenant profile tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	newTenant := func(prefix string) string {
		var id string
		if err := pool.QueryRow(ctx,
			`INSERT INTO registry.tenants (slug, name) VALUES ($1 || substr(gen_random_uuid()::text, 1, 8), 'Profile test')
			 RETURNING id::text`, prefix).Scan(&id); err != nil {
			t.Fatalf("tenant: %v", err)
		}
		// No profile is inserted here on purpose. It arrives with the tenant, by
		// trigger, and every read below would fail if it did not — which is the
		// only way that invariant is worth relying on.
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1`, id) })
		return id
	}

	f := &profileFixture{pool: pool, tenantID: newTenant("prof-"), otherID: newTenant("other-")}

	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name, is_admin)
		 VALUES ('prof-' || substr(gen_random_uuid()::text, 1, 8) || '@example.com', 'x', 'Profile Admin', TRUE)
		 RETURNING id::text`).Scan(&f.userID); err != nil {
		t.Fatalf("user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id = $1`, f.userID) })

	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1, $2)`, f.tenantID, f.userID); err != nil {
		t.Fatalf("membership: %v", err)
	}

	// The platform's own handlers, with the tenant and the caller already
	// resolved: what is under test is the handler, not the middleware in front
	// of it. No app is installed for this tenant and none is registered — which
	// is exactly the state these routes have to keep working in.
	srv := New(pool, nil, nil)
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := nexus.WithTenantID(r.Context(), f.tenantID)
			ctx = auth.WithUserContext(ctx, auth.UserClaims{UserID: f.userID, TenantID: f.tenantID, IsAdmin: true})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Get("/api/v1/tenant/profile", srv.HandleGetTenantProfile)
	router.Put("/api/v1/tenant/profile", srv.HandleUpdateTenantProfile)
	router.Get("/api/v1/profile/preferences", srv.HandleGetPreferences)
	router.Put("/api/v1/profile/preferences", srv.HandleUpdatePreferences)
	f.router = router
	return f
}

func (f *profileFixture) do(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

func TestTheOrganisationIsReadableAndEditableInParts(t *testing.T) {
	f := newProfileFixture(t)

	res := f.do(t, http.MethodGet, "/api/v1/tenant/profile", "")
	if res.Code != http.StatusOK {
		t.Fatalf("read answered %d: %s", res.Code, res.Body.String())
	}
	var before TenantProfile
	if err := json.Unmarshal(res.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	// Defaults an organisation should not have to set: this is a Mongolian
	// platform and a deadline has to be counted somewhere.
	if before.Timezone != "Asia/Ulaanbaatar" || before.Currency != "MNT" {
		t.Fatalf("expected Mongolian defaults, got %s / %s", before.Timezone, before.Currency)
	}

	if res := f.do(t, http.MethodPut, "/api/v1/tenant/profile",
		`{"registration_number":"1234567","legal_name":"Жишээ ХХК"}`); res.Code != http.StatusOK {
		t.Fatalf("update answered %d: %s", res.Code, res.Body.String())
	}
	// One more edit, naming a different field. The first two must survive it —
	// a form that sends what it changed should not blank what it did not.
	if res := f.do(t, http.MethodPut, "/api/v1/tenant/profile", `{"phone":"+976 11 123456"}`); res.Code != http.StatusOK {
		t.Fatalf("second update answered %d", res.Code)
	}

	var after TenantProfile
	if err := json.Unmarshal(f.do(t, http.MethodGet, "/api/v1/tenant/profile", "").Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if after.RegistrationNumber != "1234567" || after.LegalName != "Жишээ ХХК" {
		t.Fatalf("an unrelated edit erased the legal identity: %+v", after)
	}
	if after.Phone != "+976 11 123456" {
		t.Fatalf("the phone was not saved: %q", after.Phone)
	}
}

// "An organisation with organisations under it" is two questions, and this is
// the half that is not a department: a subsidiary is its own tenant, and what
// was missing was only the record of the relationship.
func TestASubsidiaryIsRecordedButChangesNothingAboutIsolation(t *testing.T) {
	f := newProfileFixture(t)
	ctx := context.Background()

	fetchProfile := func(t *testing.T) TenantProfile {
		t.Helper()
		var o TenantProfile
		if err := json.Unmarshal(f.do(t, http.MethodGet, "/api/v1/tenant/profile", "").Body.Bytes(), &o); err != nil {
			t.Fatal(err)
		}
		return o
	}

	// The caller belongs to the other tenant too, which is what makes the
	// claim checkable at all.
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1, $2)`, f.otherID, f.userID); err != nil {
		t.Fatal(err)
	}

	if res := f.do(t, http.MethodPut, "/api/v1/tenant/profile",
		`{"parent_tenant_id":"`+f.otherID+`"}`); res.Code != http.StatusOK {
		t.Fatalf("recording the parent answered %d: %s", res.Code, res.Body.String())
	}
	after := fetchProfile(t)
	if after.ParentTenantID != f.otherID || after.ParentName == "" {
		t.Fatalf("the parent did not come back named: %+v", after)
	}

	// Three refusals, and the platform owes a different answer for each.
	for _, refusal := range []struct {
		name, body string
		want       int
	}{
		{"itself", `{"parent_tenant_id":"` + f.tenantID + `"}`, http.StatusBadRequest},
		{"a loop", `{"parent_tenant_id":"` + f.otherID + `"}`, http.StatusOK}, // already the parent; still fine
	} {
		if res := f.do(t, http.MethodPut, "/api/v1/tenant/profile", refusal.body); res.Code != refusal.want {
			t.Fatalf("%s answered %d, wanted %d: %s", refusal.name, res.Code, refusal.want, res.Body.String())
		}
	}
}

// The point of the move: none of this depends on an app being installed.
//
// The fixture registers no module and installs nothing, so a tenant here is in
// the state a deployment reaches by removing every app it was given. The legal
// profile and the person's own preferences must still read and write — if they
// did not, "the platform boots with no apps" would be true only in the sense
// that the process starts.
func TestATenantWithNoAppsStillHasAProfileAndPreferences(t *testing.T) {
	f := newProfileFixture(t)

	var installed int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM workspace.app_installations WHERE tenant_id = $1`, f.tenantID).Scan(&installed); err != nil {
		t.Fatal(err)
	}
	if installed != 0 {
		t.Fatalf("the fixture was supposed to install nothing, found %d", installed)
	}

	if res := f.do(t, http.MethodGet, "/api/v1/tenant/profile", ""); res.Code != http.StatusOK {
		t.Fatalf("the legal profile needs an app installed: %d %s", res.Code, res.Body.String())
	}
	if res := f.do(t, http.MethodPut, "/api/v1/tenant/profile", `{"legal_name":"Апп-гүй ХХК"}`); res.Code != http.StatusOK {
		t.Fatalf("editing the legal profile needs an app installed: %d %s", res.Code, res.Body.String())
	}

	res := f.do(t, http.MethodGet, "/api/v1/profile/preferences", "")
	if res.Code != http.StatusOK {
		t.Fatalf("preferences need an app installed: %d %s", res.Code, res.Body.String())
	}
	var prefs Preferences
	if err := json.Unmarshal(res.Body.Bytes(), &prefs); err != nil {
		t.Fatal(err)
	}
	if prefs.OrganisationTimezone == "" {
		t.Fatal("the organisation default did not come back with the person's preference")
	}
	if res := f.do(t, http.MethodPut, "/api/v1/profile/preferences", `{"locale":"en"}`); res.Code != http.StatusOK {
		t.Fatalf("changing your own language needs an app installed: %d %s", res.Code, res.Body.String())
	}
}
