package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/eid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CP-3 item 0, against a real database: in private mode nobody is provisioned
// however they authenticate, switching to public takes effect without a
// restart, and the sign-in screen is told which mode it is looking at.
//
// These use the real provisioning functions rather than the check they call,
// because the failure being guarded against is somebody adding a third way
// into the platform that does not consult the mode — and a test of
// MayProvisionAccount alone would pass while that happened.

// accessModeServer builds a server with a settings store on the test database,
// and puts the platform in a known mode.
func accessModeServer(t *testing.T, mode string) (*Handlers, *pgxpool.Pool) {
	t.Helper()
	pool := lockoutPool(t)

	store := settings.NewStore(pool)
	settings.UseStore(store)
	t.Cleanup(func() { settings.UseStore(nil) })

	setMode(t, pool, store, mode)
	return &Handlers{db: pool}, pool
}

// setMode writes the access mode and reloads, which is what the console's own
// write does after it commits.
func setMode(t *testing.T, pool *pgxpool.Pool, store *settings.Store, mode string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO registry.platform_settings (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		settings.AccessMode, mode); err != nil {
		t.Fatalf("write the access mode: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM registry.platform_settings WHERE key = $1`, settings.AccessMode)
	})
	if err := store.Load(context.Background()); err != nil {
		t.Fatalf("load the settings: %v", err)
	}
	if got := settings.Get(settings.AccessMode); got != mode {
		t.Fatalf("the mode is %q after writing %q", got, mode)
	}
}

// A first sign-in through eID creates nobody while the platform is private.
func TestAPrivatePlatformProvisionsNobodyThroughEID(t *testing.T) {
	server, pool := accessModeServer(t, settings.AccessPrivate)

	// A provisioning tenant is configured, so the only thing standing between
	// this identity and a new account is the mode.
	slug := fmt.Sprintf("jit-%d", time.Now().UnixNano())
	var tenantID string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO registry.tenants (slug, name) VALUES ($1, $1) RETURNING id::text`, slug).
		Scan(&tenantID); err != nil {
		t.Fatalf("create the provisioning organisation: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1::uuid`, tenantID)
	})
	t.Setenv("EID_JIT_TENANT_SLUG", slug)
	t.Setenv("EID_RP_SECRET", "test-linking-key")

	identity := &eid.EIDIdentity{
		CivilID:   fmt.Sprintf("ЖИТ%d", time.Now().UnixNano()),
		FirstName: "Бат",
		LastName:  "Дорж",
	}

	if _, _, err := server.ResolveOrProvisionEIDUser(context.Background(), identity); err == nil {
		t.Fatal("a private platform provisioned an account through eID")
	} else {
		var visible SignInError
		if !errors.As(err, &visible) {
			t.Fatalf("the refusal is not one the person can be shown: %v", err)
		}
	}

	var people int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM workspace.memberships WHERE tenant_id = $1::uuid`, tenantID).Scan(&people); err != nil {
		t.Fatalf("count the members: %v", err)
	}
	if people != 0 {
		t.Fatalf("%d accounts were created while the platform was private", people)
	}
}

// The same through a federated provider, which is the other way in.
