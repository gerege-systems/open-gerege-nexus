/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// The Go name moved to workspace; the wire name did not.
//
// On 2026-08-27 every `tenant` in this package's exported surface became
// `workspace` — 65 of api.txt's 554 lines. The JSON tags deliberately did not
// move with them, so a field now reads
//
//	WorkspaceID string `json:"tenant_id"`
//
// which looks like an oversight and is the opposite of one. The Go API and the
// HTTP surface are two contracts with two audiences and two version numbers:
// the SDK is compiled against by module authors who can recompile, while
// `tenant_id` is read by the web shell, four native clients and every external
// OAuth2 client that ever integrated. api.txt guards the first. This guards
// the second, because nothing else does — a rename tool asked to be thorough
// would take the tags too, every test here would still pass, and the failure
// would arrive as a field silently reading empty in somebody else's app.
//
// Renaming a wire field is a thing to do on purpose, with a release note and a
// transition, not as a side effect of tidying Go identifiers. Until that day
// this test fails on anything that tries.
func TestTheWireNamesDidNotMoveWithTheGoNames(t *testing.T) {
	for _, payload := range []any{
		nexus.UserClaims{WorkspaceID: "w", AllowedWorkspaceIDs: []string{"w"}},
		nexus.DirectoryPerson{WorkspaceID: "w", WorkspaceName: "n"},
		nexus.SSOClient{WorkspaceID: "w"},
		nexus.ReportGrant{GrantorWorkspaceID: "g", GranteeWorkspaceID: "e"},
	} {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal %T: %v", payload, err)
		}
		if strings.Contains(string(encoded), "workspace") {
			t.Errorf(`%T serialises a "workspace" key:

	%s

The Go field is named for the domain and the JSON tag is named for
compatibility. Changing the tag changes what the web shell, the native clients
and every registered OAuth2 client receive, which is a release with a note and
a transition rather than a rename.`, payload, encoded)
		}
	}
}
