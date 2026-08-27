/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package platform_test

import (
	"reflect"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/host"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/platform"
)

// The forwarding name has to be the same type, not a copy of it.
//
// This is the failure the alias exists to prevent: someone replaces
// `type Options = host.Options` with a struct that lists the fields, eleven
// distributions keep compiling, and the field added to host.Options after that
// is set by a distribution and never reaches the server. reflect compares the
// types at run time, so the check survives the alias being turned into a
// wrapper — which is the only version of this file that can break.
func TestTheOldNameIsStillTheSameType(t *testing.T) {
	old, current := reflect.TypeOf(platform.Options{}), reflect.TypeOf(host.Options{})
	if old != current {
		t.Errorf("platform.Options is %s and host.Options is %s.\n"+
			"A distribution setting a field on the old name is setting it on a different "+
			"struct, and whatever pkg/host reads is the zero value. Keep the alias.",
			old, current)
	}
}
