/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package migrations_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Core SQL names its plane explicitly. The runtime keeps tenant,platform in
// search_path for modules from other repositories, so an unqualified core
// query could otherwise regress without failing locally. This text check is
// the review-time guard; database tests prove the qualified queries execute.
func TestCoreSQLQualifiesEveryOwnedTable(t *testing.T) {
	names := make([]string, 0, len(platformTables))
	for name := range platformTables {
		names = append(names, regexp.QuoteMeta(name))
	}
	sort.Strings(names)

	unqualified := regexp.MustCompile(`(?i)\b(?:FROM|JOIN|INTO|UPDATE|DELETE\s+FROM)\s+(` +
		strings.Join(names, "|") + `)\b`)
	comments := regexp.MustCompile(`(?s)/\*.*?\*/|(?m)//.*$`)

	root := filepath.Join("..", "..")
	var offences []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return err //nolint:wrapcheck // the walk error is the useful error
		}
		source, err := os.ReadFile(path) // #nosec G304 -- a walk of this repository
		if err != nil {
			return err //nolint:wrapcheck
		}
		code := comments.ReplaceAllString(string(source), "")
		for _, match := range unqualified.FindAllStringSubmatch(code, -1) {
			rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
			offences = append(offences, rel+" names "+strings.ToLower(match[1])+" without a schema")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(offences)
	for _, offence := range offences {
		t.Errorf(`%s

Core SQL must name its schema: workspace.<table>, registry.<table> or
operator.<table>. search_path remains only for module SQL compiled from other
repositories; relying on it here would make a query's plane invisible in review
and let a same-named table redirect it.`, offence)
	}
}
