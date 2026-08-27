/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package canary

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/go-chi/chi/v5"
)

// A distribution constructs its module the way a product's main() does, and
// everything it declared is there afterwards.
//
// This is the assertion the golden file cannot make. api.txt records that
// Provide, Capability, Register, Migrations, RegisterReport and ProvideAssistant
// exist with those signatures; it says nothing about whether providing a
// capability makes it gettable, or whether a registered module is in the list.
// A refactor can keep every signature and break every one of those.
func TestADistributionsModuleIsWiredUpAfterConstruction(t *testing.T) {
	module := New(nexus.NewPlatform(nil, nil))

	if got, ok := nexus.Get(module.ID()); !ok || got.ID() != module.ID() {
		t.Errorf("the module registered itself and nexus.Get does not have it")
	}

	// The capability this distribution published, of a type the core has never
	// heard of.
	pricing, err := nexus.Capability[Pricing]()
	if err != nil {
		t.Fatalf("the module provided a Pricing and cannot get one: %v", err)
	}
	price, err := pricing.Quote(context.Background(), "tenant-1", "SKU-1")
	if err != nil || price != 4200 {
		t.Errorf("Quote = %d, %v; want 4200", price, err)
	}

	if _, ok := nexus.MigrationsOf(module.ID()); !ok {
		t.Error("the module registered migrations and MigrationsOf does not have them")
	}

	tools := nexus.AssistantToolset()
	if len(tools) != 1 || tools[0].Name != "canary_quote" {
		t.Errorf("the assistant was lent %v; want one canary_quote", tools)
	}
	result, err := tools[0].Call(context.Background(), "tenant-1", map[string]any{"sku": "SKU-1"})
	if err != nil || result["price"] != int64(4200) {
		t.Errorf("the tool answered %v, %v", result, err)
	}

	if got := nexus.MenuPermissionOf(module); got != "canary.read" {
		t.Errorf("menu permission: got %q, want canary.read", got)
	}
	if got := nexus.RoutePermissionPrefixOf(module); got != "canary" {
		t.Errorf("route permission prefix: got %q, want canary", got)
	}
}

// A capability nothing provides comes back as an error that names the type.
//
// The behaviour a distribution depends on and no signature records: a module
// asks for something the deployment does not have and gets an answer it can act
// on, rather than a zero value it will dereference.
func TestAMissingCapabilityIsAnErrorAndNotAZeroValue(t *testing.T) {
	type absent interface{ Absent() }

	value, err := nexus.Capability[absent]()
	if err == nil {
		t.Fatal("a capability nothing provides came back without an error")
	}
	if value != nil {
		t.Errorf("a missing capability also returned a value: %v", value)
	}
	if !strings.Contains(err.Error(), "absent") {
		t.Errorf("the error does not name the missing type: %v", err)
	}

	// And the sentinel accessors keep answering the way v1 promised.
	if _, err := nexus.Ring(); !errors.Is(err, nexus.ErrNoLink) {
		t.Errorf("Ring on a deployment with no link returned %v, want ErrNoLink", err)
	}
	if _, err := nexus.Documents(); !errors.Is(err, nexus.ErrNoDocumentFiler) {
		t.Errorf("Documents with no filer returned %v, want ErrNoDocumentFiler", err)
	}
	if _, err := nexus.Meetings(); err == nil {
		t.Error("Meetings with no booker returned no error")
	}
}

// A permission that says who it reaches, and one that contradicts itself.
//
// Both are v1 promises a distribution writes against: the first is the whole
// point of DefaultRoles, and the second is the refusal that stops a permission
// quietly reaching more people than intended.
func TestAPermissionsDeclaredReachIsHonoured(t *testing.T) {
	module := New(nexus.NewPlatform(nil, nil))

	for _, perm := range module.Permissions() {
		if err := perm.Validate(); err != nil {
			t.Errorf("%s: %v", perm.Code, err)
		}
	}

	contradiction := nexus.PermissionDefinition{
		Code: "canary.read", AdminOnly: true, DefaultRoles: []string{nexus.DefaultRoleUser},
	}
	if err := contradiction.Validate(); err == nil {
		t.Error("a permission that is both AdminOnly and granted by default was accepted")
	}
}

// The three calls every handler in every distribution starts with.
//
// api.txt records that RequireWorkspace, UserFromContext and WorkspaceOf exist
// with those signatures. It cannot say that a context carrying a workspace makes
// all three agree about which one, or that a context carrying none is refused —
// and those are the properties a handler is written against.
//
// This test is why .github/workflows/downstream.yml no longer clones a product
// from another repository to find out. That job did catch this surface, and it
// caught it by making the platform's pipeline depend on a repository the
// platform does not own: a deliberate major-version rename could not be merged
// here until somebody else's product had been updated and pushed, and a job
// that cannot tell an intended break from an accident will eventually be
// merged past rather than fixed.
func TestTheRequestScopedContractHoldsForAHandler(t *testing.T) {
	module := New(nexus.NewPlatform(nil, nil))
	const workspaceID, userID = "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"

	router := chi.NewRouter()
	module.RegisterRoutes(router, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := nexus.WithWorkspaceID(r.Context(), workspaceID)
			ctx = nexus.WithUser(ctx, nexus.UserClaims{UserID: userID, WorkspaceID: workspaceID})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	answered := httptest.NewRecorder()
	router.ServeHTTP(answered, httptest.NewRequest(http.MethodGet, "/api/v1/canary/mine", nil))
	if answered.Code != http.StatusOK {
		t.Fatalf("a request carrying a workspace was answered %d: %s", answered.Code, answered.Body.String())
	}
	if body := answered.Body.String(); !strings.Contains(body, workspaceID) || !strings.Contains(body, userID) {
		t.Errorf("the handler answered %s, which names neither the workspace nor the person", body)
	}

	// And the other half: no workspace in the context is refused rather than
	// served with an empty string, which is what a module would otherwise put
	// into a WHERE clause.
	bare := chi.NewRouter()
	module.RegisterRoutes(bare, func(next http.Handler) http.Handler { return next })
	refused := httptest.NewRecorder()
	bare.ServeHTTP(refused, httptest.NewRequest(http.MethodGet, "/api/v1/canary/mine", nil))
	if refused.Code == http.StatusOK {
		t.Errorf("a request with no workspace was answered %d: %s", refused.Code, refused.Body.String())
	}
}
