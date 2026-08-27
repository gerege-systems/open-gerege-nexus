/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package tenants

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
	platformsettings "github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/settings"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/support"
	"github.com/jackc/pgx/v5/pgxpool"
)

// privateService is this screen with the platform closed to strangers, which is
// the state the test below is about.
func privateService(t *testing.T, pool *pgxpool.Pool) (*Service, *platformsettings.Service) {
	t.Helper()
	store := settings.NewStore(pool)
	settings.UseStore(store)
	t.Cleanup(func() { settings.UseStore(nil) })
	if err := store.Load(context.Background()); err != nil {
		t.Fatalf("load the settings: %v", err)
	}
	op := operator.New(pool)
	return New(op, Deps{DB: pool, Support: support.New(op, support.Deps{DB: pool})}),
		platformsettings.New(op, platformsettings.Deps{DB: pool, Settings: store})
}

// The line the access mode does not cross: somebody an operator invited is
// already registered, so creating them is not just-in-time provisioning and is
// not what private mode refuses.
func TestAnInvitationStillWorksWhileThePlatformIsPrivate(t *testing.T) {
	pool := optest.Pool(t)
	service, config := privateService(t, pool)
	account, _ := optest.Account(t, pool, operator.RoleOperator)
	sess := optest.Session(account)
	ctx := context.Background()

	if err := config.SetSetting(ctx, sess, settings.AccessMode, settings.AccessPrivate,
		"closing the platform"); err != nil {
		t.Fatalf("set the access mode: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM registry.platform_settings WHERE key = $1`, settings.AccessMode)
	})
	if got := settings.Get(settings.AccessMode); got != settings.AccessPrivate {
		t.Fatalf("the platform is %q", got)
	}

	slug := fmt.Sprintf("invited-%d", time.Now().UnixNano())
	email := slug + "@example.mn"
	created, err := service.CreateTenant(ctx, sess, NewTenant{
		Name:       "Invited Organisation",
		Slug:       slug,
		AdminEmail: email,
		Reason:     "a customer signed a contract",
	})
	if err != nil {
		t.Fatalf("a private platform refused an invited organisation: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1::uuid`, created.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE email = $1`, email)
	})

	// The account exists and is an administrator of the new organisation.
	var admin bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (
		    SELECT 1 FROM registry.users u
		      JOIN workspace.memberships m ON m.user_id = u.id AND m.tenant_id = $2::uuid
		      JOIN workspace.membership_roles mr ON mr.membership_id = m.id
		      JOIN workspace.roles r ON r.id = mr.role_id AND r.code = 'admin'
		     WHERE u.email = $1)`, email, created.ID).Scan(&admin); err != nil {
		t.Fatalf("look for the administrator: %v", err)
	}
	if !admin {
		t.Fatal("the invited administrator was not created")
	}
	// Nothing was sent, because no mail is configured in a test — and the
	// console says so rather than pretending.
	if created.Invited {
		t.Fatal("an invitation was reported as sent with no mail configured")
	}
}
