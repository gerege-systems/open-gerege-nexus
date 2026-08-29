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

// Tables whose foreign key is deliberately left without an index, and why.
//
// Both are the probes a migration writes to prove itself: a handful of rows
// that exist so a test can ask whether the isolation is on. An index on a
// table with five rows costs more to keep than the scan it saves.
var foreignKeysWithoutAnIndex = map[string]string{
	"dbguard_probe(tenant_id)":   "a probe of five rows",
	"reporting_probe(tenant_id)": "a probe of five rows",
}

// Text columns that are deliberately unbounded, and why.
//
// Empty today, and that is the point: it is the place a reviewer writes the
// reason when a column genuinely has no length worth naming, so that "no
// length" is a decision rather than an omission.
var textColumnsWithoutALength = map[string]string{}

func hygienePool(t *testing.T) *pgxpool.Pool {
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

// PostgreSQL indexes the parent side of a foreign key and not the child, so a
// parent row that is deleted or whose key is updated reads every child table in
// full. On this platform that is not theoretical: deleting an organisation
// after its grace period cascades into forty-odd tables.
func TestEveryForeignKeyCanBeLookedUp(t *testing.T) {
	pool := hygienePool(t)
	rows, err := pool.Query(context.Background(), `
		SELECT cls.relname || '(' || a.attname || ')'
		  FROM pg_constraint c
		  JOIN pg_class cls ON cls.oid = c.conrelid
		  JOIN pg_namespace n ON n.oid = cls.relnamespace
		  JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = c.conkey[1]
		 WHERE c.contype = 'f'
		   AND n.nspname IN ('registry', 'workspace', 'operator')
		   AND array_length(c.conkey, 1) = 1
		   AND NOT EXISTS (SELECT 1 FROM pg_index i
		                    WHERE i.indrelid = c.conrelid AND i.indkey[0] = c.conkey[1])
		 ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			t.Fatal(err)
		}
		if _, listed := foreignKeysWithoutAnIndex[key]; listed {
			continue
		}
		t.Errorf(`%s has no index.

Deleting the parent row reads this table in full, once per parent. Add

    CREATE INDEX idx_<table>_<column> ON <schema>.<table> (<column>);

or name it in foreignKeysWithoutAnIndex with the reason it does not need one.`, key)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

// An unbounded text column is the database saying "as many characters as you
// like". The Go side validates at the boundary and stays the primary control;
// this is the last wall, and it is the one that holds when a handler forgets.
func TestEveryTextColumnSaysHowLongItMayBe(t *testing.T) {
	pool := hygienePool(t)
	rows, err := pool.Query(context.Background(), `
		SELECT c.table_name || '.' || c.column_name
		  FROM information_schema.columns c
		  JOIN information_schema.tables t
		    ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		 WHERE c.table_schema IN ('registry', 'workspace')
		   AND c.data_type = 'text'
		   AND t.table_type = 'BASE TABLE'
		   AND NOT EXISTS (
		        SELECT 1 FROM pg_constraint con
		          JOIN pg_class cl ON cl.oid = con.conrelid
		          JOIN pg_namespace n ON n.oid = cl.relnamespace AND n.nspname = c.table_schema
		         WHERE cl.relname = c.table_name AND con.contype = 'c'
		           AND pg_get_constraintdef(con.oid) ILIKE '%length(' || c.column_name || ')%')
		 ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	checked := 0
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		checked++
		if _, listed := textColumnsWithoutALength[column]; listed {
			continue
		}
		t.Errorf(`%s is text with no length.

Add a bound to the table:

    CONSTRAINT <table>_<column>_is_bounded CHECK (length(<column>) <= N)

or name it in textColumnsWithoutALength with the reason it has none.`, column)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	_ = checked
}
