/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package flags

import (
	"context"
	"fmt"
	"testing"
	"time"

	kernelflags "github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/flags"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/jackc/pgx/v5/pgxpool"
)

// flagService is this screen against the test database.
func flagService(t *testing.T, pool *pgxpool.Pool) *Service {
	t.Helper()
	store := kernelflags.NewStore(pool)
	if err := store.Load(context.Background()); err != nil {
		t.Fatalf("load the flags: %v", err)
	}
	return New(operator.New(pool), Deps{Flags: store})
}

// A kill switch, aimed at one organisation and then at everybody.
func TestAFlagCanBeAimedAtOneOrganisation(t *testing.T) {
	pool := optest.Pool(t)
	service := flagService(t, pool)
	account, _ := optest.Account(t, pool, operator.RoleOperator)
	sess := optest.Session(account)
	tenantID, _ := optest.Tenant(t, pool)
	ctx := context.Background()

	key := fmt.Sprintf("module.test-%d.disabled", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.feature_flags WHERE key = $1`, key)
	})

	if err := service.SaveFlag(ctx, sess, FlagInput{
		Key: key, Kind: kernelflags.KindKillSwitch, Owner: "platform",
		Enabled: false, Rollout: 100, Reason: "prepared during an incident",
	}); err != nil {
		t.Fatalf("save the flag: %v", err)
	}

	on := true
	if err := service.SetFlagOverride(ctx, sess, key, tenantID, &on,
		"this organisation is the one seeing the fault"); err != nil {
		t.Fatalf("set the override: %v", err)
	}

	// Read through the same store the platform reads: the override is on for
	// this organisation and the flag is still off for everybody else.
	kernelflags.UseStore(service.flags)
	t.Cleanup(func() { kernelflags.UseStore(nil) })
	if !kernelflags.Enabled(tenantContext(tenantID), key) {
		t.Fatal("the override did not reach the evaluation")
	}
	if kernelflags.Enabled(tenantContext("00000000-0000-0000-0000-000000000000"), key) {
		t.Fatal("a flag switched on for one organisation was on for another")
	}

	// Removing the override puts them back with everybody else.
	if err := service.SetFlagOverride(ctx, sess, key, tenantID, nil, "the fault is fixed"); err != nil {
		t.Fatalf("remove the override: %v", err)
	}
	if kernelflags.Enabled(tenantContext(tenantID), key) {
		t.Fatal("the override survived being removed")
	}

	if err := service.DeleteFlag(ctx, sess, key, "no longer needed"); err != nil {
		t.Fatalf("delete the flag: %v", err)
	}
	list, err := service.ListFlags(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, flag := range list {
		if flag.Key == key {
			t.Fatal("the flag survived being deleted")
		}
	}
}

// tenantContext is what the request middleware would have built.
func tenantContext(tenantID string) context.Context {
	return nexus.WithWorkspaceID(context.Background(), tenantID)
}
