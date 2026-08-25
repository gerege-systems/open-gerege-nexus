/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package apps

import (
	"sort"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// Both empty since 2026-08-25: sso_clients was the last module in this
// repository and its two permissions went with it to the App Store.
//
// Өртөө's three were here until 2026-08-23 and are client-gerege-nexus's. The
// tables stay for the next module that arrives, because a permission that
// appears with nobody having stated its reach is exactly what this catches.
var everyModule = map[string]nexus.Module{}

var defaultGrants = map[string]string{}

func TestEveryPermissionSaysWhoItReaches(t *testing.T) {
	seen := map[string]bool{}
	for name, module := range everyModule {
		for _, perm := range module.Permissions() {
			seen[perm.Code] = true

			if err := perm.Validate(); err != nil {
				t.Errorf("%s: %v", name, err)
				continue
			}

			want, listed := defaultGrants[perm.Code]
			if !listed {
				t.Errorf(`%s declares the permission %q, which this table does not name.

Add it, with either "suffix" (the module says nothing and the deprecated
grammar rule decides) or the roles it declares. The line is the claim a
reviewer reads; a permission that appears without one is a permission whose
reach nobody stated.`, name, perm.Code)
				continue
			}

			got := "suffix"
			if perm.AdminOnly {
				got = "admin only"
			}
			if len(perm.DefaultRoles) > 0 {
				roles := append([]string(nil), perm.DefaultRoles...)
				sort.Strings(roles)
				got = strings.Join(roles, ",")
			}
			if got != want {
				t.Errorf("%s: %s reaches %q, recorded as %q", name, perm.Code, got, want)
			}
		}
	}

	for code := range defaultGrants {
		if !seen[code] {
			t.Errorf("the table names %q, which no module in this binary declares", code)
		}
	}
}
