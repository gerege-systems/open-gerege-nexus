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
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The console's four policies, asserted from the console's own role.
//
// These were the last entries in policy_coverage_test.go's policiesWithoutATest,
// and the reason they sat there is worth keeping: `operator_read`,
// `operator_write` and `operator_shared` are not one policy each but one policy
// name repeated across eighteen tables by a DO block, and every test that
// exercises the console today connects as the login role — which is outside
// row-level security altogether and so cannot tell a live policy from a dropped
// one.
//
// What makes that gap specific rather than theoretical is the shape of the
// failure. Drop `operator_read` from workspace.sessions and nothing raises:
// the operator role still holds the GRANT, the query still succeeds, and it
// returns no rows. The support screen goes blank and the logs stay quiet. That
// is the same silence 00049's own header warned about — "GRANT SELECT
// дангаараа операторт юу ч өгөхгүй" — and the tests below are what turns the
// warning into something that fails.
//
// So the proof has to be a row. Each table gets one, written by the owner
// inside a transaction that is rolled back, and then the console's role is
// asked for it back. A count of one is the policy working; a count of zero is
// the policy gone. Nothing here asserts a grant on its own, because a grant on
// its own is exactly the state that looks like success and is not.

func consolePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// consolePolicies are the policy names this file answers for.
//
// They are the console's, in the sense that they are the ones written TO
// gerege_nexus_operator. `tenant_isolation` is not here for the reason
// policy_coverage_test.go gives, and neither is anything written for the tenant
// role — registry_privilege_test.go and person_isolation_test.go have those.
var consolePolicies = map[string]bool{
	"operator_read":                      true,
	"operator_write":                     true,
	"operator_shared":                    true,
	"directory_is_public_to_the_console": true,
}

// world is the rows every seed below hangs off: one organisation, one person
// in it, and one app to install.
//
// Made once per test rather than per table because the point of the exercise
// is the console's view of a populated deployment, and eighteen organisations
// would prove the same thing while making the failures harder to read.
type world struct {
	tenant string
	user   string
	// A second person, because two of the console's writes are inserts rather
	// than updates and the first person is already a member.
	user2 string
	app   string
	flag  string
	stamp int64
}

func makeWorld(t *testing.T, tx pgx.Tx) world {
	t.Helper()
	ctx := context.Background()
	w := world{stamp: time.Now().UnixNano()}

	if err := tx.QueryRow(ctx,
		`INSERT INTO registry.tenants (slug, name) VALUES ($1, $1) RETURNING id::text`,
		fmt.Sprintf("console-policy-%d", w.stamp)).Scan(&w.tenant); err != nil {
		t.Fatalf("set up an organisation: %v", err)
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name)
		 VALUES ($1, 'x', 'Console Policy') RETURNING id::text`,
		fmt.Sprintf("console-policy-%d@isolation.test", w.stamp)).Scan(&w.user); err != nil {
		t.Fatalf("set up a person: %v", err)
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name)
		 VALUES ($1, 'x', 'Console Policy Two') RETURNING id::text`,
		fmt.Sprintf("console-policy-two-%d@isolation.test", w.stamp)).Scan(&w.user2); err != nil {
		t.Fatalf("set up a second person: %v", err)
	}
	w.app = fmt.Sprintf("test.console.policy.%d", w.stamp)
	if _, err := tx.Exec(ctx,
		`INSERT INTO registry.apps (id, slug, name) VALUES ($1, $2, 'Console Policy')`,
		w.app, fmt.Sprintf("console-policy-%d", w.stamp)); err != nil {
		t.Fatalf("set up an app: %v", err)
	}
	// feature_flag_overrides is an override *of* something: its key is a
	// foreign key into registry.feature_flags, and a seed that invented one
	// would fail on the reference rather than on the policy.
	w.flag = fmt.Sprintf("console.policy.%d", w.stamp)
	if _, err := tx.Exec(ctx,
		`INSERT INTO registry.feature_flags (key) VALUES ($1)`, w.flag); err != nil {
		t.Fatalf("set up a feature flag: %v", err)
	}
	return w
}

// A seed puts one row in one table and says how to find it again.
//
// The columns are written out rather than derived, and that is the cost this
// file is meant to carry: a table added to one of these policies later has no
// seed, TestEveryConsolePolicyTableHasASeed says so on the run after it
// appears, and somebody has to decide what a row in it looks like. A generated
// insert would have skipped that decision and, on a table with a foreign key,
// would have failed for a reason that had nothing to do with the policy.
type seed struct {
	// insert writes the row as the owner. The table is passed in already
	// schema-qualified, from pg_policies rather than from a list here, so a
	// table that moves schema does not need this file edited.
	insert func(ctx context.Context, tx pgx.Tx, table string, w world) error
	// find is the WHERE clause the console must be able to match, and its
	// arguments. It is run twice: once as the owner, to prove the row is
	// really there, and once as gerege_nexus_operator, which is the assertion.
	find func(w world) (string, []any)
}

var consoleSeeds = map[string]seed{
	// An upsert, because the row is already there: registry.tenants carries a
	// tenants_create_profile trigger, so creating the organisation above
	// created its profile too. A plain insert failed on the primary key, and
	// the failure read as a broken seed when what it actually was is one more
	// thing this table does that a list of columns does not say.
	"tenant_profiles": {
		insert: func(ctx context.Context, tx pgx.Tx, table string, w world) error {
			_, err := exec(ctx, tx,
				"INSERT INTO "+table+" (tenant_id, legal_name) VALUES ($1::uuid, 'Console Policy') "+
					"ON CONFLICT (tenant_id) DO UPDATE SET legal_name = EXCLUDED.legal_name", w.tenant)
			return err
		},
		find: func(w world) (string, []any) { return "tenant_id = $1::uuid", []any{w.tenant} },
	},
	"email_verifications": {
		insert: func(ctx context.Context, tx pgx.Tx, table string, w world) error {
			_, err := exec(ctx, tx,
				"INSERT INTO "+table+" (tenant_id, email, token_hash, expires_at) "+
					"VALUES ($1::uuid, $2, $3, NOW() + INTERVAL '1 hour')",
				w.tenant, fmt.Sprintf("verify-%d@isolation.test", w.stamp), w.hash())
			return err
		},
		find: func(w world) (string, []any) { return "token_hash = $1", []any{w.hash()} },
	},
	"report_schedules": {
		insert: func(ctx context.Context, tx pgx.Tx, table string, w world) error {
			_, err := exec(ctx, tx,
				"INSERT INTO "+table+" (tenant_id, report_key, cron) VALUES ($1::uuid, $2, '0 3 * * *')",
				w.tenant, w.code())
			return err
		},
		find: func(w world) (string, []any) {
			return "tenant_id = $1::uuid AND report_key = $2", []any{w.tenant, w.code()}
		},
	},
	"announcements": {
		insert: func(ctx context.Context, tx pgx.Tx, table string, w world) error {
			_, err := exec(ctx, tx,
				"INSERT INTO "+table+" (tenant_id, title) VALUES ($1::uuid, $2)", w.tenant, w.code())
			return err
		},
		find: func(w world) (string, []any) {
			return "tenant_id = $1::uuid AND title = $2", []any{w.tenant, w.code()}
		},
	},
	"feature_flag_overrides": {
		insert: func(ctx context.Context, tx pgx.Tx, table string, w world) error {
			_, err := exec(ctx, tx,
				"INSERT INTO "+table+" (flag_key, tenant_id, enabled) VALUES ($1, $2::uuid, TRUE)",
				w.flag, w.tenant)
			return err
		},
		find: func(w world) (string, []any) {
			return "flag_key = $1 AND tenant_id = $2::uuid", []any{w.flag, w.tenant}
		},
	},
	"memberships": {
		insert: func(ctx context.Context, tx pgx.Tx, table string, w world) error {
			_, err := exec(ctx, tx, "INSERT INTO "+table+" (tenant_id, user_id) VALUES ($1::uuid, $2::uuid)", w.tenant, w.user)
			return err
		},
		find: func(w world) (string, []any) {
			return "tenant_id = $1::uuid AND user_id = $2::uuid", []any{w.tenant, w.user}
		},
	},
	"roles": {
		insert: func(ctx context.Context, tx pgx.Tx, table string, w world) error {
			_, err := exec(ctx, tx, "INSERT INTO "+table+" (tenant_id, code, name) VALUES ($1::uuid, $2, 'Console Policy')", w.tenant, w.code())
			return err
		},
		find: func(w world) (string, []any) {
			return "tenant_id = $1::uuid AND code = $2", []any{w.tenant, w.code()}
		},
	},
	"app_installations": {
		insert: func(ctx context.Context, tx pgx.Tx, table string, w world) error {
			_, err := exec(ctx, tx,
				"INSERT INTO "+table+" (tenant_id, app_id, installed_version) VALUES ($1::uuid, $2, '1.0.0')",
				w.tenant, w.app)
			return err
		},
		find: func(w world) (string, []any) {
			return "tenant_id = $1::uuid AND app_id = $2", []any{w.tenant, w.app}
		},
	},
	"sessions": {
		insert: func(ctx context.Context, tx pgx.Tx, table string, w world) error {
			_, err := exec(ctx, tx,
				"INSERT INTO "+table+" (token_hash, user_id, tenant_id, expires_at) "+
					"VALUES ($1, $2::uuid, $3::uuid, NOW() + INTERVAL '1 hour')",
				w.hash(), w.user, w.tenant)
			return err
		},
		find: func(w world) (string, []any) { return "token_hash = $1", []any{w.hash()} },
	},
	"audit_events": {
		insert: func(ctx context.Context, tx pgx.Tx, table string, w world) error {
			_, err := exec(ctx, tx,
				"INSERT INTO "+table+" (tenant_id, action, resource) VALUES ($1::uuid, $2, 'console.policy')",
				w.tenant, w.code())
			return err
		},
		find: func(w world) (string, []any) {
			return "tenant_id = $1::uuid AND action = $2", []any{w.tenant, w.code()}
		},
	},
	"usage_events": {
		insert: func(ctx context.Context, tx pgx.Tx, table string, w world) error {
			_, err := exec(ctx, tx,
				"INSERT INTO "+table+" (tenant_id, day, metric, value) VALUES ($1::uuid, CURRENT_DATE, $2, 1)",
				w.tenant, w.code())
			return err
		},
		find: func(w world) (string, []any) {
			return "tenant_id = $1::uuid AND metric = $2", []any{w.tenant, w.code()}
		},
	},
	"tenant_quotas": {
		insert: func(ctx context.Context, tx pgx.Tx, table string, w world) error {
			_, err := exec(ctx, tx, "INSERT INTO "+table+" (tenant_id, max_users) VALUES ($1::uuid, 5)", w.tenant)
			return err
		},
		find: func(w world) (string, []any) { return "tenant_id = $1::uuid", []any{w.tenant} },
	},
	"operator_impersonations": {
		insert: func(ctx context.Context, tx pgx.Tx, table string, w world) error {
			_, err := exec(ctx, tx,
				"INSERT INTO "+table+" (operator_id, tenant_id, user_id, reason, handover_hash, handover_expires_at, ends_at) "+
					"VALUES (gen_random_uuid(), $1::uuid, $2::uuid, 'console policy test', $3, NOW() + INTERVAL '5 minutes', NOW() + INTERVAL '1 hour')",
				w.tenant, w.user, w.hash())
			return err
		},
		find: func(w world) (string, []any) { return "handover_hash = $1", []any{w.hash()} },
	},
	// The two assistant tables are the pair where the row has to be the right
	// row. `operator_shared` is not USING (true) like the rest of the console's
	// policies — 00095 wrote it as USING (tenant_id IS NULL), so what the
	// console may read is the platform's own prompts and knowledge and not an
	// organisation's. A seed with a tenant_id on it returns nothing here, and
	// would have been read as a missing policy; it is the policy working.
	// TestTheConsoleSeesOnlyTheSharedAssistantRows asserts the other half.
	"ai_prompts": {
		insert: func(ctx context.Context, tx pgx.Tx, table string, w world) error {
			_, err := exec(ctx, tx,
				"INSERT INTO "+table+" (tenant_id, prompt_key, content) VALUES (NULL, $1, 'console policy')",
				w.shortCode())
			return err
		},
		find: func(w world) (string, []any) {
			return "tenant_id IS NULL AND prompt_key = $1", []any{w.shortCode()}
		},
	},
	"ai_knowledge": {
		insert: func(ctx context.Context, tx pgx.Tx, table string, w world) error {
			_, err := exec(ctx, tx,
				"INSERT INTO "+table+" (tenant_id, title, content) VALUES (NULL, $1, 'console policy')",
				w.code())
			return err
		},
		find: func(w world) (string, []any) {
			return "tenant_id IS NULL AND title = $1", []any{w.code()}
		},
	},
	"service_directory": {
		insert: func(ctx context.Context, tx pgx.Tx, table string, w world) error {
			// The code may not be empty, may not exceed 128 characters and may
			// not begin `local.` — three CHECK constraints 00090 wrote, and a
			// seed that trips one of them would fail this test for a reason
			// that is not the policy.
			_, err := exec(ctx, tx,
				"INSERT INTO "+table+" (tenant_id, code, title) VALUES ($1::uuid, $2, 'Console Policy')",
				w.tenant, w.code())
			return err
		},
		find: func(w world) (string, []any) {
			return "tenant_id = $1::uuid AND code = $2", []any{w.tenant, w.code()}
		},
	},
}

// The statements the console must still be able to run, for the tables
// `operator_write` covers.
//
// Every one of them expects to affect exactly one row, and that number is the
// assertion rather than the absence of an error: without the policy an UPDATE
// matches nothing and reports success on nought rows, which is the same silence
// the reads have.
//
// audit_events is the exception in the data as well as here. 00050 gave it FOR
// INSERT and nothing else — "гарын үсэг зурсны дараа бол засагддаггүй бүртгэл" —
// so the statement is an insert, and an update would be refused by the
// immutability trigger rather than by the policy.
var consoleWrites = map[string]func(w world) (string, []any){
	"tenant_profiles": func(w world) (string, []any) {
		return "UPDATE %s SET legal_name = 'Renamed By Console' WHERE tenant_id = $1::uuid", []any{w.tenant}
	},
	// An insert, not an update, and that is 00049's decision rather than an
	// omission: the operator role's grants on these two are SELECT and INSERT
	// only. The console adds somebody to an organisation; it does not reach
	// back into a membership that already exists.
	//
	// The policy is wider than that — 00050 wrote operator_write FOR ALL — and
	// the grant is what actually stops the update. That pair is asserted from
	// both sides: here, that the permitted write works, and in
	// TestTheConsoleCannotRewriteAMembershipOrRole, that the other one does not.
	"memberships": func(w world) (string, []any) {
		return "INSERT INTO %s (tenant_id, user_id) VALUES ($1::uuid, $2::uuid)",
			[]any{w.tenant, w.user2}
	},
	"roles": func(w world) (string, []any) {
		return "INSERT INTO %s (tenant_id, code, name) VALUES ($1::uuid, $2, 'Added By Console')",
			[]any{w.tenant, w.code() + "-added"}
	},
	"sessions": func(w world) (string, []any) {
		return "UPDATE %s SET revoked_at = NOW() WHERE token_hash = $1", []any{w.hash()}
	},
	"audit_events": func(w world) (string, []any) {
		return "INSERT INTO %s (tenant_id, action, resource) VALUES ($1::uuid, 'console.write', 'console.policy')",
			[]any{w.tenant}
	},
	"tenant_quotas": func(w world) (string, []any) {
		return "UPDATE %s SET enforcement = 'hard' WHERE tenant_id = $1::uuid", []any{w.tenant}
	},
	"operator_impersonations": func(w world) (string, []any) {
		return "UPDATE %s SET ended_at = NOW() WHERE handover_hash = $1", []any{w.hash()}
	},
	"announcements": func(w world) (string, []any) {
		return "UPDATE %s SET kind = 'maintenance' WHERE tenant_id = $1::uuid AND title = $2",
			[]any{w.tenant, w.code()}
	},
	"feature_flag_overrides": func(w world) (string, []any) {
		return "UPDATE %s SET enabled = FALSE WHERE flag_key = $1 AND tenant_id = $2::uuid",
			[]any{w.flag, w.tenant}
	},
}

func (w world) code() string      { return fmt.Sprintf("console-policy-%d", w.stamp) }
func (w world) shortCode() string { return fmt.Sprintf("cp-%d", w.stamp%1_000_000_000) }

// A 64-character hash-shaped value, because token_hash and handover_hash are
// both CHAR(64) and a shorter string would be padded rather than rejected.
func (w world) hash() string {
	return fmt.Sprintf("%064d", w.stamp)
}

func exec(ctx context.Context, tx pgx.Tx, sql string, args ...any) (int64, error) {
	tag, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", sql, err)
	}
	return tag.RowsAffected(), nil
}

// seededTables is the list the assertions run over: the tables written down in
// consoleSeeds, sorted.
//
// It is deliberately not the catalogue. The first version of this file took the
// list from pg_policies, and dropping a policy made the test pass — the table
// left the query's result and the loop asserted nothing about it, which is the
// same "an invariant that matches nothing reports green" this package has been
// bitten by twice. The written-down list is what makes a dropped policy show up
// as a missing row rather than as a shorter loop; the catalogue's job is the
// other direction, in TestEveryConsolePolicyTableHasASeed, where it catches a
// table that was added.
func seededTables() []string {
	names := make([]string, 0, len(consoleSeeds))
	for table := range consoleSeeds {
		names = append(names, table)
	}
	sort.Strings(names)
	return names
}

// consolePolicyTables reads which tables each of the four policies is on.
//
// From the catalogue rather than from a list in this file, for the reason
// 00049 wrote its policies in a DO block: the set is a loop over an array in a
// migration, and a copy of it here would be right until the day somebody
// appended to that array.
func consolePolicyTables(t *testing.T, pool *pgxpool.Pool) map[string]string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT DISTINCT schemaname, tablename, policyname
		  FROM pg_policies
		 WHERE schemaname IN ('registry', 'workspace', 'operator')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	// table -> schema. A table carrying two of these policies (tenant_profiles
	// carries operator_read and operator_write) is one table with one seed.
	found := map[string]string{}
	for rows.Next() {
		var schema, table, policy string
		if err := rows.Scan(&schema, &table, &policy); err != nil {
			t.Fatal(err)
		}
		if !consolePolicies[policy] {
			continue
		}
		// Only the platform's own tables, for the reason policy_shape_test.go
		// gives: CI runs every package against one database.
		if _, ours := platformTables[table]; !ours {
			continue
		}
		found[table] = schema
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Fatal("none of the console's policies are on any table; this test is asserting nothing")
	}
	return found
}

// Every table one of the console's policies is written on has a row this file
// knows how to make.
//
// This is the drift guard, and it is the half of the change that keeps working
// after today: the policies are created by a loop over an array inside a
// migration, so the cheapest possible way to add a nineteenth table is to
// append a name to that array — at which point the policy exists, the console
// reads the table, and nothing anywhere asserts it. This test fails on the run
// after that append.
func TestEveryConsolePolicyTableHasASeed(t *testing.T) {
	pool := consolePool(t)
	var missing []string
	for table := range consolePolicyTables(t, pool) {
		if _, ok := consoleSeeds[table]; !ok {
			missing = append(missing, table)
		}
	}
	sort.Strings(missing)
	for _, table := range missing {
		t.Errorf(`%s carries one of the console's policies and has no seed in consoleSeeds.

A policy is only tested by a test that becomes the role it is written for, and
that test needs a row to look for: without one, a dropped policy and a live one
both return nothing. Add an entry to consoleSeeds saying what one row of %s
looks like — the columns are written out on purpose, so this is a decision
about the table rather than a generated insert that would fail on its foreign
keys.`, table, table)
	}

	// And the other direction: a seed for a table that no longer carries one
	// of these policies is a test asserting something nobody asked for.
	live := consolePolicyTables(t, pool)
	for table := range consoleSeeds {
		if _, ok := live[table]; !ok {
			t.Errorf("consoleSeeds names %q, which carries none of the console's policies any more", table)
		}
	}
	for table := range consoleWrites {
		if _, ok := consoleSeeds[table]; !ok {
			t.Errorf("consoleWrites names %q, which has no seed", table)
		}
	}

	// And the writes, which drift the same way and are the half that matters
	// more: a table the console may write to and nobody exercises is a screen
	// whose Save button can stop working without a single error.
	for table := range tablesCarrying(t, pool, "operator_write") {
		if _, ok := consoleWrites[table]; !ok {
			t.Errorf(`%s carries operator_write and has no statement in consoleWrites.

Reading it is not enough: operator_write is what lets the console change the
row, and an UPDATE the policy does not permit matches nothing and reports
success. Add the statement the console actually runs against %s.`, table, table)
		}
	}
}

// tablesCarrying reports which of the platform's tables one policy is on.
func tablesCarrying(t *testing.T, pool *pgxpool.Pool, policy string) map[string]string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT DISTINCT schemaname, tablename
		  FROM pg_policies
		 WHERE schemaname IN ('registry', 'workspace', 'operator')
		   AND policyname = $1`, policy)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	found := map[string]string{}
	for rows.Next() {
		var schema, table string
		if err := rows.Scan(&schema, &table); err != nil {
			t.Fatal(err)
		}
		if _, ours := platformTables[table]; !ours {
			continue
		}
		found[table] = schema
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Fatalf("%s is on no table at all; this test is asserting nothing", policy)
	}
	return found
}

// The console reads every table its policies cover, as the console's role.
//
// One transaction, rolled back. The rows are made by the owner — TEST_DATABASE_URL
// is the login role, which is outside row-level security, so the insert is not
// itself a test of anything — and then read back after SET LOCAL ROLE.
func TestTheConsoleReadsEveryTableItsPoliciesCover(t *testing.T) {
	pool := consolePool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	w := makeWorld(t, tx)

	names := seededTables()

	// Every row first, then the role. Becoming the operator part-way through
	// would leave the later inserts running as a role that may not hold the
	// grant, and the failure would read as a policy problem.
	for _, table := range names {
		qualified := pgx.Identifier{schemaOf(table), table}.Sanitize()
		if err := consoleSeeds[table].insert(ctx, tx, qualified, w); err != nil {
			t.Fatalf("seed %s: %v", qualified, err)
		}
	}

	// The owner sees what was just written. This is not the assertion — it is
	// what stops a broken seed being read as a broken policy.
	for _, table := range names {
		if n := countAs(t, tx, schemaOf(table), table, w); n != 1 {
			t.Fatalf("the owner sees %d rows in %s.%s after seeding one; the seed is wrong, not the policy",
				n, schemaOf(table), table)
		}
	}

	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_operator`); err != nil {
		t.Fatalf("become the operator role: %v", err)
	}

	for _, table := range names {
		n := countAs(t, tx, schemaOf(table), table, w)
		if n != 1 {
			t.Errorf(`the console reads %d rows from %s.%s, want 1.

The row is there — the owner just read it. What is missing is a policy letting
gerege_nexus_operator see it, and without one this table returns nothing to the
console with no error anywhere: the screen that reads it goes blank and the
logs stay quiet.`, n, schemaOf(table), table)
		}
	}
}

// The console writes where operator_write says it may, as the console's role.
//
// Reading is the half that goes quiet; writing is the half that goes quiet
// twice. An UPDATE the policy does not permit matches no rows and returns
// success, so the assertion is the row count and not the error.
func TestTheConsoleWritesWhereItsPolicySaysItMay(t *testing.T) {
	pool := consolePool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	w := makeWorld(t, tx)

	names := make([]string, 0, len(consoleWrites))
	for table := range consoleWrites {
		names = append(names, table)
	}
	sort.Strings(names)

	for _, table := range names {
		qualified := pgx.Identifier{schemaOf(table), table}.Sanitize()
		if err := consoleSeeds[table].insert(ctx, tx, qualified, w); err != nil {
			t.Fatalf("seed %s: %v", qualified, err)
		}
	}

	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_operator`); err != nil {
		t.Fatalf("become the operator role: %v", err)
	}

	for _, table := range names {
		statement, args := consoleWrites[table](w)
		qualified := pgx.Identifier{schemaOf(table), table}.Sanitize()

		// Each statement in its own savepoint. Without one the first refusal
		// aborts the transaction and every table after it reports 25P02 — six
		// failures for one cause, and the cause is not the one they name.
		nested, err := tx.Begin(ctx)
		if err != nil {
			t.Fatalf("savepoint: %v", err)
		}
		affected, err := exec(ctx, nested, fmt.Sprintf(statement, qualified), args...)
		_ = nested.Rollback(ctx)
		if err != nil {
			t.Errorf("the console cannot write to %s: %v", qualified, err)
			continue
		}
		if affected != 1 {
			t.Errorf(`the console's write to %s affected %d rows, want 1.

No error was raised, and that is the point: a statement the policy does not
permit matches nothing and reports success. Whatever screen runs this one would
show a saved change that was never written.`, qualified, affected)
		}
	}
}

// countAs runs the seed's own WHERE clause against the table, whoever the
// caller currently is.
func countAs(t *testing.T, tx pgx.Tx, schema, table string, w world) int {
	t.Helper()
	where, args := consoleSeeds[table].find(w)
	qualified := pgx.Identifier{schema, table}.Sanitize()
	var n int
	if err := tx.QueryRow(context.Background(),
		fmt.Sprintf("SELECT count(*) FROM %s WHERE %s", qualified, where), args...).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", qualified, err)
	}
	return n
}

// The console sees the platform's assistant rows and not an organisation's.
//
// The other half of `operator_shared`, and the reason it is worth its own test:
// the policy is the one place in the console's family that is not USING (true),
// so the read above proves it is live and this proves it is still narrow. A
// policy widened to USING (true) — the shape every other console policy has,
// and therefore the shape somebody would reach for — would pass every other
// test in this file while handing one organisation's prompts to a screen that
// shows them to everybody.
func TestTheConsoleSeesOnlyTheSharedAssistantRows(t *testing.T) {
	pool := consolePool(t)
	ctx := context.Background()
	tables := consolePolicyTables(t, pool)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	w := makeWorld(t, tx)

	owned := map[string]string{
		"ai_prompts":   "INSERT INTO %s (tenant_id, prompt_key, content) VALUES ($1::uuid, $2, 'an organisation''s own')",
		"ai_knowledge": "INSERT INTO %s (tenant_id, title, content) VALUES ($1::uuid, $2, 'an organisation''s own')",
	}
	find := map[string]string{
		"ai_prompts":   "tenant_id = $1::uuid AND prompt_key = $2",
		"ai_knowledge": "tenant_id = $1::uuid AND title = $2",
	}
	key := map[string]string{
		"ai_prompts":   w.shortCode(),
		"ai_knowledge": w.code(),
	}

	names := make([]string, 0, len(owned))
	for table := range owned {
		if _, ok := tables[table]; !ok {
			t.Errorf("%s no longer carries operator_shared; this test is asserting nothing", table)
			continue
		}
		names = append(names, table)
	}
	sort.Strings(names)

	for _, table := range names {
		qualified := pgx.Identifier{schemaOf(table), table}.Sanitize()
		if _, err := exec(ctx, tx, fmt.Sprintf(owned[table], qualified), w.tenant, key[table]); err != nil {
			t.Fatalf("seed %s: %v", qualified, err)
		}
	}

	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_operator`); err != nil {
		t.Fatalf("become the operator role: %v", err)
	}

	for _, table := range names {
		qualified := pgx.Identifier{schemaOf(table), table}.Sanitize()
		var n int
		if err := tx.QueryRow(ctx,
			fmt.Sprintf("SELECT count(*) FROM %s WHERE %s", qualified, find[table]),
			w.tenant, key[table]).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", qualified, err)
		}
		if n != 0 {
			t.Errorf(`the console reads %d of an organisation's own rows from %s, want 0.

operator_shared is USING (tenant_id IS NULL): what the console may read from
the assistant tables is the platform's own prompts and knowledge. A row with a
tenant_id on it belongs to one organisation, and the screen that lists these
shows them to every operator.`, n, qualified)
		}
	}
}

// The console adds a membership or a role; it does not rewrite one.
//
// The other side of the two writes above, and the reason both are worth
// asserting: `operator_write` on these tables is FOR ALL — it permits every
// statement — and what actually stops an UPDATE is 00049's grant list, which
// gives the operator role SELECT and INSERT and nothing else.
//
// That makes the boundary invisible in the place people look for it. A reader
// checking what the console may do to a membership finds a policy saying
// "anything", and the sentence that makes it untrue is a GRANT three
// migrations earlier. Widening the grant would need no policy change and would
// raise no error; this test is what would notice.
func TestTheConsoleCannotRewriteAMembershipOrRole(t *testing.T) {
	pool := consolePool(t)
	ctx := context.Background()
	tables := consolePolicyTables(t, pool)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	w := makeWorld(t, tx)
	for _, table := range []string{"memberships", "roles"} {
		if _, ok := tables[table]; !ok {
			t.Fatalf("%s carries none of the console's policies any more", table)
		}
		qualified := pgx.Identifier{schemaOf(table), table}.Sanitize()
		if err := consoleSeeds[table].insert(ctx, tx, qualified, w); err != nil {
			t.Fatalf("seed %s: %v", qualified, err)
		}
	}

	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_operator`); err != nil {
		t.Fatalf("become the operator role: %v", err)
	}

	for table, statement := range map[string]string{
		"memberships": "UPDATE %s SET active = FALSE WHERE tenant_id = $1::uuid",
		"roles":       "UPDATE %s SET name = 'Renamed By Console' WHERE tenant_id = $1::uuid",
	} {
		qualified := pgx.Identifier{schemaOf(table), table}.Sanitize()
		if !denied(t, tx, fmt.Sprintf(statement, qualified), w.tenant) {
			t.Errorf(`the console updated %s, which 00049's grants do not allow.

The policy on this table is FOR ALL and permits it; the grant is what refuses
it. If the console genuinely needs to change these rows, the change is a GRANT
in a migration and a line here saying so — not a widening that nothing records.`,
				qualified)
		}
	}
}
