/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package migrations_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Policies that have no test binding the role they are written for, and why
// each one is still here.
//
// The list is the point of this file. Everything in it is a policy nobody has
// asserted from the role it governs, which is the state 00102 and 00106 were
// both written into and the state this test exists to stop growing.
var policiesWithoutATest = map[string]string{
	// 00049's console family, on eighteen tables between them. They are
	// read-only by construction — the operator role's grants are a
	// hand-written list — and every test that exercises the console today
	// runs as the login role, which is outside the policies entirely and so
	// would not notice if one of them were dropped.
	//
	// Writing them is one test per plane rather than eighteen: bind
	// gerege_nexus_operator, read one row of each table, assert it arrives.
	// It is not in this change because this change is about registry.
	"operator_read":   "00049's console reads; no test binds the operator role to them",
	"operator_write":  "00049's console writes; the same",
	"operator_shared": "00049's shared AI tables; the same",

	// The console's half of the service directory (00090). Its tenant half
	// is asserted in internal/person; this half is not.
	"directory_is_public_to_the_console": "00090; the console's half of the directory has no test",
}

// Every policy is asserted from the role it is written for.
//
// A policy is not code that fails when it is wrong. A missing one returns no
// rows and a too-wide one returns somebody else's, and both look exactly like
// a working screen to every test that connects as the login role — which is
// what TEST_DATABASE_URL is, and which is outside row-level security
// altogether. So a policy with no test that says `SET LOCAL ROLE` has not been
// tested at all, however green the suite is.
//
// `tenant_isolation` is excluded because it is not hand-written: 00037 and
// 00073 wrote it across every table with a tenant_id, dbguard's
// TestEveryTenantTableHasForcedRLS checks that every such table still carries
// it, and policy_shape_test.go checks the two shapes it may take. Everything
// else in this database is somebody's decision about one table, and this is
// what makes that decision arrive with evidence.
func TestEveryPolicyIsAssertedFromItsRole(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	// Only the platform's own tables, for the reason policy_shape_test.go
	// gives: CI runs every package against one database, and a table another
	// package's test created is in pg_policies too.
	rows, err := pool.Query(context.Background(), `
		SELECT DISTINCT policyname, tablename
		  FROM pg_policies
		 WHERE schemaname IN ('registry', 'workspace', 'operator')
		   AND policyname <> 'tenant_isolation'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	tables := map[string][]string{}
	for rows.Next() {
		var policy, table string
		if err := rows.Scan(&policy, &table); err != nil {
			t.Fatal(err)
		}
		if _, ours := platformTables[table]; !ours {
			continue
		}
		tables[policy] = append(tables[policy], table)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	// The lesson of the schema rename: an invariant that matches nothing
	// reports green having asserted nothing.
	if len(tables) == 0 {
		t.Fatal("no policies were found at all; this test is asserting nothing")
	}

	asserted := policiesNamedInRoleBoundTests(t)
	for policy, on := range tables {
		if _, excused := policiesWithoutATest[policy]; excused {
			continue
		}
		if asserted[policy] {
			continue
		}
		sort.Strings(on)
		t.Errorf(`%s (on %s) has no test that binds a database role.

A policy is only tested by a test that becomes the role it is written for:

    tx.Exec(ctx, "SET LOCAL ROLE gerege_nexus_tenant")   // or _operator

Anything else connects as the login role, which is outside row-level security,
and would pass with the policy dropped. Write that test — db/migrations already
has four files of them — and name %q in it so this test can find it. If the
policy genuinely cannot be asserted yet, add it to policiesWithoutATest with
the reason.`, policy, strings.Join(on, ", "), policy)
	}

	for policy := range policiesWithoutATest {
		if _, live := tables[policy]; !live {
			t.Errorf("policiesWithoutATest names %q, which is not a policy in this database", policy)
		}
	}
}

// policiesNamedInRoleBoundTests reads every _test.go in the backend and
// returns the policy names mentioned by the ones that bind a database role.
//
// Named rather than inferred: there is no way to ask PostgreSQL which policy a
// query was allowed by, so the link between a policy and its test is a
// sentence somebody wrote. Requiring the name to appear in a file that also
// says SET LOCAL ROLE is what makes that sentence checkable — it cannot prove
// the assertion is a good one, and it does stop a policy shipping with no
// role-bound test anywhere near it.
func policiesNamedInRoleBoundTests(t *testing.T) map[string]bool {
	t.Helper()
	named := map[string]bool{}
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return err //nolint:wrapcheck // the walk error is the useful error
		}
		source, err := os.ReadFile(path) // #nosec G304 -- a walk of this repository
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		text := string(source)
		// The two ways this repository becomes a role. set_config is what
		// dbguard itself uses, so a test that binds the way the application
		// binds counts too.
		if !strings.Contains(text, "SET LOCAL ROLE") && !strings.Contains(text, "set_config('role'") {
			return nil
		}
		for _, word := range strings.FieldsFunc(text, func(r rune) bool {
			return r != '_' && (r < 'a' || r > 'z') && (r < '0' || r > '9')
		}) {
			named[word] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return named
}
