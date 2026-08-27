/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package appinstall

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// The platform must not decide anything by an app's permission code.
//
// internal/apps/boundaries_test.go:88 records what this costs when it goes
// wrong. The platform used to hold two switch statements keyed by app id,
// deciding which permission gated each app's menu and routes. They were not
// imports, so no compiler saw them, and they broke in the worst way available:
// when the App Store's modules moved out the switches stopped matching, and
// every route in that product became reachable by any member of a tenant.
//
// The five gov.* codes this installer named were the same shape of mistake at a
// smaller scale. gov-services went to gerege-gov and the grants stayed: a
// switch in this binary handing out permissions this binary does not have. It
// happened to be harmless because the codes matched nothing. The version of it
// that is not harmless is the one where they do.
//
// Audit event names are not permission codes and are not what this looks for:
// it looks for a *decision* — a comparison against a literal code — because
// that is the shape that breaks when a module leaves.
func TestNoAppPermissionCodeIsNamedInThePlatform(t *testing.T) {
	source, err := os.ReadFile("installer.go")
	if err != nil {
		t.Fatalf("read installer.go: %v", err)
	}

	decision := regexp.MustCompile(`(?:[=!]=\s*|case\s+)"([a-z_]+\.[a-z_]+)"`)
	var found []string
	for _, match := range decision.FindAllStringSubmatch(string(source), -1) {
		found = append(found, match[1])
	}
	sort.Strings(found)

	for _, code := range found {
		t.Errorf(`installer.go decides something by the permission code %q.

A permission belongs to the app that enforces it, and so does the answer to who
gets it by default: nexus.PermissionDefinition.DefaultRoles. A decision keyed by
an app's code lives in the platform, survives the app leaving, and reports
nothing when it stops matching — see internal/apps/boundaries_test.go:88 for
what that cost the last time.`, code)
	}
}

// The two ways a definition can contradict itself, refused rather than merged.
func TestAPermissionCannotBeAdminOnlyAndAlsoDefault(t *testing.T) {
	both := nexus.PermissionDefinition{
		Code: "thing.read", AdminOnly: true,
		DefaultRoles: []string{nexus.DefaultRoleUser},
	}
	if err := both.Validate(); err == nil {
		t.Error("a permission that is both AdminOnly and granted by default was accepted")
	}

	unknown := nexus.PermissionDefinition{Code: "thing.read", DefaultRoles: []string{"auditor"}}
	if err := unknown.Validate(); err == nil {
		t.Error("a permission asking for a role that does not exist was accepted")
	}

	ok := nexus.PermissionDefinition{Code: "thing.read", DefaultRoles: []string{nexus.DefaultRoleManager}}
	if err := ok.Validate(); err != nil {
		t.Errorf("an ordinary declaration was refused: %v", err)
	}
}

// The suffix rule keeps doing exactly what it did, for every module that has
// not said otherwise. This is the compatibility half of the change: the grants
// a tenant gets on a fresh install must not move because the mechanism did.
func TestTheSuffixFallbackGrantsWhatItAlwaysDid(t *testing.T) {
	for _, tc := range []struct {
		code string
		want []string
	}{
		{"documents.read", []string{nexus.DefaultRoleManager, nexus.DefaultRoleUser}},
		{"documents.manage", []string{nexus.DefaultRoleManager}},
		{"reports.read", []string{nexus.DefaultRoleManager, nexus.DefaultRoleUser}},
		// Neither suffix: nobody by default, which is what the platform did
		// with anything the grammar did not cover — apart from the five gov.*
		// codes it named by hand, and which are now gone.
		{"gov.apply", nil},
		{"gov.process", nil},
	} {
		got := defaultRolesFor(nexus.PermissionDefinition{Code: tc.code})
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%s falls back to %v, want %v", tc.code, got, tc.want)
		}
	}

	// A declaration wins over the grammar, including one that narrows.
	got := defaultRolesFor(nexus.PermissionDefinition{
		Code: "documents.read", DefaultRoles: []string{nexus.DefaultRoleManager},
	})
	if strings.Join(got, ",") != nexus.DefaultRoleManager {
		t.Errorf("a declared DefaultRoles was overruled by the suffix: %v", got)
	}
}
