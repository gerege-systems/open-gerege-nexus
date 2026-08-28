/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package migrations_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Tables whose tenant_isolation policy is deliberately the narrow form, and
// why each one is.
//
// It sits outside the test because the first five entries are also the
// boundary between the two planes, and ownership_test.go checks its own list
// of them against this one. Two lists of the same five names drift.
var narrowPolicies = map[string]string{
	// Read-only views for the console and the tenant's own quota screen.
	// One organisation's figures, shown to that organisation; widening them
	// would show an operator's impersonation record to a sibling tenant.
	"announcements":           "console, FOR SELECT",
	"feature_flag_overrides":  "console, FOR SELECT",
	"operator_impersonations": "console, FOR SELECT",
	"tenant_quotas":           "console, FOR SELECT",
	"usage_events":            "console, FOR SELECT",

	// Device-scoped. A terminal is enrolled into one organisation and reads
	// as that organisation; there is no "this person also belongs to" for a
	// till or a kiosk.
	"device_enrollment_codes": "device scope",
	"device_telemetry":        "device scope",
	"devices":                 "device scope",
	"push_tokens":             "device scope",
	"staff_pin_credentials":   "device scope",

	// person_items was here, in the narrow form, while it was keyed by the
	// citizen's own workspace. Migration 00093 keyed it by the citizen instead
	// and moved it to registry, so it has no tenant_isolation policy to have a
	// shape: it isolates by app.current_user. The reasoning that put it here
	// survived the move — no sibling workspace has any business in one person's
	// list — and is now enforced by a policy that cannot express the wide form
	// at all, which is stronger than a narrow policy somebody could widen.

	// TODO: unreviewed. These four took the narrow form by copying, not by
	// deciding, and each needs its own answer before it is widened — an
	// audit trail and a signed file are not obviously things a sibling
	// organisation should read just because one person belongs to both.
	"audit_events": "unreviewed",
	// document_files was here — a signed file is not obviously something a
	// sibling organisation should read — and it went to
	// client-gerege-nexus with the documents app. Its policy is declared in
	// that repository's own migration, in the same narrow form and for the
	// same unreviewed reason.
	"report_grants":    "unreviewed",
	"report_schedules": "unreviewed",
}

// Two tenant-isolation policies with different shapes, written down.
//
// 00037 widened the read half of the rule: a session belonging to several
// organisations reads across all of them, and still writes only into the active
// one. It did that by walking every table with a tenant_id, so on the day it ran
// there was one shape. Tables created since have been copied from whichever
// neighbour was open at the time, and about a quarter of them took the pre-00037
// form — including the nine Өртөө tables, until 00073.
//
// The narrow form errs closed, so nothing has leaked; what it does is hide
// rows from somebody entitled to see them, on some tables and not others,
// with nothing saying which. The remaining sixteen are listed above with a
// reason each, so the question "why is this one different" has an answer that
// is not "whoever wrote it copied the wrong file".
//
// A new table takes the wide form or is named here. Either is fine; being
// neither is how the split got this far.
func TestTenantPoliciesHaveTheShapeOnRecord(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	// Only the platform's own tables, which is why this test lives beside the
	// list of them. CI runs every package against one database, so a table
	// another package's test created is in pg_policies too — and its policy is
	// that test's business, not this one's.
	rows, err := pool.Query(context.Background(), `
		SELECT tablename, qual LIKE '%allowed_tenants%'
		  FROM pg_policies
		 WHERE schemaname IN ('workspace', 'registry', 'operator') AND policyname = 'tenant_isolation'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	seen := map[string]bool{}
	for rows.Next() {
		var table string
		var wide bool
		if err := rows.Scan(&table, &wide); err != nil {
			t.Fatal(err)
		}
		if _, ours := platformTables[table]; !ours {
			continue
		}
		seen[table] = true
		reason, listed := narrowPolicies[table]
		switch {
		case wide && listed:
			t.Errorf("%s reads across organisations but is listed as narrow (%q); remove the entry", table, reason)
		case !wide && !listed:
			t.Errorf(`%s uses the pre-00037 tenant_isolation policy:

    tenant_id = current_setting('app.current_tenant')

A session belonging to several organisations will not see this table's rows for
any of them but the active one. That errs closed, so nothing leaks — it hides
rows from somebody entitled to see them, which is a bug that looks like an empty
screen.

Use the form 00037 and 00073 write, or add %q to the narrow list in this test
with the reason it is different.`, table, table)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	for table := range narrowPolicies {
		if !seen[table] {
			t.Errorf("the narrow list names %q, which has no tenant_isolation policy", table)
		}
	}
}
