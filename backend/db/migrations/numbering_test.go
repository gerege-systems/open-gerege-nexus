/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package migrations_test

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Two migrations with one number is a deployment that runs one of them.
//
// goose records the number, not the file: whichever ran first marks the version
// applied, and the second is skipped in silence on every database that had
// already seen the first. It is not a hypothetical — two branches open at once
// each add "the next number", and both are right until they meet.
//
// The failure is quiet and it is remote: locally the developer who wrote the
// second file has already applied their own, so `migrate up` says there is
// nothing to do and the schema it needed never arrives. This test is the place
// that says so at review time instead.
var migrationName = regexp.MustCompile(`^(\d{5})_[a-z0-9_]+\.sql$`)

func TestMigrationNumbersAreUniqueAndUnbroken(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	owner := map[int]string{}
	numbers := make([]int, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		match := migrationName.FindStringSubmatch(entry.Name())
		if match == nil {
			t.Errorf(`%s is not named like a migration.

Migrations are NNNNN_lower_case_words.sql — goose sorts by the number and the
name is what a reviewer reads in a diff.`, entry.Name())
			continue
		}
		number, err := strconv.Atoi(match[1])
		if err != nil {
			t.Fatal(err)
		}
		if first, taken := owner[number]; taken {
			t.Errorf(`%05d is used twice: %s and %s.

goose records the number rather than the file, so a database that applied one
of them skips the other for good. Renumber the newer one to the next free
number.`, number, first, entry.Name())
			continue
		}
		owner[number] = entry.Name()
		numbers = append(numbers, number)
	}

	if len(numbers) == 0 {
		t.Fatal("no migrations were found: this test is looking at the wrong directory")
	}
	sort.Ints(numbers)

	// A gap is not fatal to goose, which applies what it finds in order. It is
	// still reported, because the usual way one appears is a file that was
	// renamed or deleted in a merge — and the second most usual is the file
	// that was never added to the commit.
	var gaps []string
	for i := 1; i < len(numbers); i++ {
		if step := numbers[i] - numbers[i-1]; step != 1 {
			gaps = append(gaps, fmt.Sprintf("%05d → %05d", numbers[i-1], numbers[i]))
		}
	}
	if len(gaps) > 0 {
		t.Errorf(`the numbering skips: %s.

A gap is usually a file that a merge renamed, or one that never made it into
the commit. If the number was deliberately retired, say so here.`, strings.Join(gaps, ", "))
	}
}

// Every migration says how to go back. `migrate down` is what a deployment
// reaches for when a release goes wrong, and a file with no Down section is one
// it cannot use.
func TestEveryMigrationCanBeRolledBack(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		body, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if !strings.Contains(text, "-- +goose Up") {
			t.Errorf("%s has no `-- +goose Up` section", entry.Name())
		}
		if !strings.Contains(text, "-- +goose Down") {
			t.Errorf("%s has no `-- +goose Down` section: a release that has to be undone cannot be", entry.Name())
		}
	}
}
