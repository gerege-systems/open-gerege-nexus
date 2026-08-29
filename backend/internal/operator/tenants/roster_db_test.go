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

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
)

// An organisation nobody has set limits for is a line with no limits, not a
// missing line: "nobody has set limits here" is what the screen is for, and a
// query that inner-joined the quota table would answer it by saying nothing.
func TestEveryOrganisationHasALineInTheLimits(t *testing.T) {
	pool := optest.Pool(t)
	op := operator.New(pool)
	service := New(op, Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	unlimited, _ := optest.Tenant(t, pool)
	limited, _ := optest.Tenant(t, pool)
	ctx := context.Background()

	users := 5
	if err := service.SetQuota(ctx, optest.Session(account), limited, Quota{
		MaxUsers: &users, Enforcement: EnforcementHard,
	}, "prove the roster"); err != nil {
		t.Fatalf("set the limits: %v", err)
	}

	lines, err := service.ListQuotas(ctx)
	if err != nil {
		t.Fatalf("list the limits: %v", err)
	}
	found := map[string]QuotaLine{}
	for _, line := range lines {
		found[line.TenantID] = line
	}

	if line, ok := found[unlimited]; !ok {
		t.Fatal("an organisation with no limits is missing from the list")
	} else if line.MaxUsers != nil || line.Enforcement != EnforcementSoft {
		t.Errorf("an organisation with no limits reads %+v", line)
	}
	if line, ok := found[limited]; !ok {
		t.Fatal("the organisation just given limits is missing")
	} else if line.MaxUsers == nil || *line.MaxUsers != users || line.Enforcement != EnforcementHard {
		t.Errorf("the limits read %+v", line)
	} else if line.TenantName == "" {
		t.Error("the list does not name the organisation")
	}
}

// The catalogue screen counts versions across the platform, which says two
// organisations are behind without saying which two. This is that list, so it
// has to name both the app and the organisation.
func TestTheInstallationsListNamesBothSides(t *testing.T) {
	pool := optest.Pool(t)
	service := New(operator.New(pool), Deps{DB: pool})
	tenantID, _ := optest.Tenant(t, pool)
	ctx := context.Background()

	appID := fmt.Sprintf("mn.test.roster%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx,
		`INSERT INTO registry.apps (id, slug, name) VALUES ($1, $1, 'Roster Probe')`, appID); err != nil {
		t.Fatalf("register an app: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.apps WHERE id = $1`, appID) })
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace.app_installations (tenant_id, app_id, installed_version)
		 VALUES ($1::uuid, $2, '1.2.3')`, tenantID, appID); err != nil {
		t.Fatalf("install it: %v", err)
	}

	installations, err := service.ListInstallations(ctx)
	if err != nil {
		t.Fatalf("list the installations: %v", err)
	}
	for _, item := range installations {
		if item.AppID != appID {
			continue
		}
		if item.TenantID != tenantID || item.TenantName == "" {
			t.Errorf("the installation does not name its organisation: %+v", item)
		}
		if item.AppName != "Roster Probe" || item.Version != "1.2.3" || !item.Enabled {
			t.Errorf("the installation reads %+v", item)
		}
		return
	}
	t.Fatalf("the installation just written is not in the list of %d", len(installations))
}
