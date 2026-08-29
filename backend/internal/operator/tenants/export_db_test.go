/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package tenants

import (
	"context"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/support"
)

// The export reads the schema an organisation's tables are actually in.
//
// This is the test the export never had, and its absence is why a schema
// rename could break the whole screen without anything going red: every claim
// worth making here — that the rows come back, that they are the right
// organisation's, that a sibling's are not in the bundle — is a claim about
// what PostgreSQL answers, and none of them can be made without a database.
func TestTheExportReturnsTheOrganisationsOwnRows(t *testing.T) {
	pool := optest.Pool(t)
	ctx := context.Background()
	op := operator.New(pool)
	service := New(op, Deps{DB: pool, Support: support.New(op, support.Deps{DB: pool})})
	account, _ := optest.Account(t, pool, operator.RoleOperator)
	sess := optest.Session(account)

	mine, _ := optest.Tenant(t, pool)
	theirs, _ := optest.Tenant(t, pool)

	// audit_events is the one table every deployment has rows in, carries a
	// tenant_id, and needs no module installed to write to.
	for _, tenant := range []string{mine, theirs} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO workspace.audit_events (tenant_id, action, resource, details)
			 VALUES ($1::uuid, 'export.test', 'test', '{}'::jsonb)`, tenant); err != nil {
			t.Fatalf("write an audit row: %v", err)
		}
	}

	bundle, err := service.ExportTenant(ctx, sess, mine)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	rows := bundle.Tables["audit_events"]
	if len(rows) == 0 {
		t.Fatal("the bundle has no audit rows; the export read nothing")
	}
	for _, row := range rows {
		if got := toString(row["tenant_id"]); got != mine {
			t.Fatalf("the bundle carries a row belonging to %s, not %s", got, mine)
		}
	}
}

// toString reads a uuid column back out of the generic row map, whatever
// concrete type the driver decoded it into.
func toString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case [16]byte:
		return uuidString(value)
	case interface{ String() string }:
		return value.String()
	default:
		return ""
	}
}

func uuidString(b [16]byte) string {
	const hexDigits = "0123456789abcdef"
	out := make([]byte, 0, 36)
	for i, c := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out = append(out, '-')
		}
		out = append(out, hexDigits[c>>4], hexDigits[c&0x0f])
	}
	return string(out)
}
