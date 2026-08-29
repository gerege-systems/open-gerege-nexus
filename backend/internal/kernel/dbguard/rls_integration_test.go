package dbguard_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Tables that carry a tenant_id and are deliberately not isolated by it, with
// the reason each one is. A table here still has RLS forced; what it does not
// have is the `tenant_isolation` policy, because its rule is a different rule.
var notIsolatedByTenant = map[string]string{
	// The directory is a register of services one organisation publishes for
	// every other to find — a row is owned by a tenant and readable by all of
	// them (00090, `directory_is_public_but_owned`). Isolating it by tenant
	// would leave every deployment with a directory of one entry: its own.
	"service_directory": "published to every organisation on purpose",
}

// Every module table carrying tenant_id must be protected. This table-driven
// database invariant covers current and future modules without allowing a new
// tenant-scoped table to silently ship without a policy.
//
// It asked about schemas `tenant` and `platform` until this fix. 00079 split
// the tables into `registry` and `workspace` and 00084 finished the rename, and
// from that day the query matched no rows at all: the test kept reporting
// green having asserted nothing, which is the one failure mode an invariant
// test must not have. Naming the live schemas is not enough on its own — a
// schema renamed again would put it back in the same silence — so the test now
// also refuses to run on an empty result.
func TestEveryTenantTableHasForcedRLS(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	rows, err := pool.Query(context.Background(), `
		SELECT c.table_name, cls.relrowsecurity, cls.relforcerowsecurity,
		       EXISTS (SELECT 1 FROM pg_policies p WHERE p.schemaname=c.table_schema AND p.tablename=c.table_name AND p.policyname='tenant_isolation')
		FROM information_schema.columns c
		JOIN pg_class cls ON cls.relname=c.table_name
		JOIN pg_namespace n ON n.oid=cls.relnamespace AND n.nspname=c.table_schema
		WHERE c.table_schema IN ('registry', 'workspace', 'operator') AND c.column_name='tenant_id'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	checked := 0
	for rows.Next() {
		var table string
		var enabled, forced, policy bool
		if err := rows.Scan(&table, &enabled, &forced, &policy); err != nil {
			t.Fatal(err)
		}
		checked++
		if reason, listed := notIsolatedByTenant[table]; listed {
			if !enabled || !forced {
				t.Errorf("%s (%s): RLS enabled=%v forced=%v — a different rule is still a rule",
					table, reason, enabled, forced)
			}
			continue
		}
		if !enabled || !forced || !policy {
			t.Errorf("%s: RLS enabled=%v forced=%v policy=%v", table, enabled, forced, policy)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	// A schema this test cannot find is a schema whose tables it is not
	// protecting. The number is not asserted — tables come and go — but zero
	// means the query is asking about a database that is not this one.
	if checked == 0 {
		t.Fatal("no table with a tenant_id was found: this test is looking at the wrong schemas")
	}
}
