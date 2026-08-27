/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The two things this package has to be true about itself.
 *
 * It is an external test package (`nexus_test`) on purpose: it can only reach
 * what is exported, which is the same view a distribution repository has.
 */

package nexus_test

import (
	"context"
	"encoding/json"
	"go/build"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/go-chi/chi/v5"
)

// probeModule is what a module in another repository looks like.
//
// It is written against this package and nothing else — no import of anything
// under internal/ appears in this file — so if it compiles and registers, a
// third party's module can too. That is the claim the SDK exists to make, and
// before this package the claim was false: every module had to name
// internal.Module, which no other repository may import.
type probeModule struct{ id string }

func (p probeModule) ID() string      { return p.id }
func (p probeModule) Name() string    { return "Probe" }
func (p probeModule) Version() string { return "1.0.0" }

func (p probeModule) Dependencies() []nexus.Dependency {
	return []nexus.Dependency{{ID: "io.gerege.nexus.organisation", VersionConstraint: "^1.0.0"}}
}

func (p probeModule) Permissions() []nexus.PermissionDefinition {
	return []nexus.PermissionDefinition{
		{Code: "probe.read", Name: "Read"},
		{Code: "probe.secret", Name: "Secret", AdminOnly: true},
	}
}

func (p probeModule) Menus() []nexus.MenuDefinition {
	return []nexus.MenuDefinition{{
		ID: "probe_home", Label: "Probe", Path: "/probe", Icon: "boxes", Order: 10,
		Labels: map[string]string{"mn": "Туршилт"},
	}}
}

func (p probeModule) RegisterRoutes(r chi.Router, gate func(http.Handler) http.Handler) {
	r.Route("/api/v1/probe", func(pr chi.Router) {
		pr.Use(gate)
		pr.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	})
}

func TestAModuleDefinedOutsideThePlatformCanRegisterItself(t *testing.T) {
	const id = "mn.example.probe"
	nexus.Register(probeModule{id: id})

	got, ok := nexus.Get(id)
	if !ok {
		t.Fatalf("a registered module was not in the registry")
	}
	if got.Name() != "Probe" {
		t.Fatalf("the registry returned something else: %s", got.Name())
	}
	if err := nexus.VerifyModuleExists(id); err != nil {
		t.Fatalf("VerifyModuleExists disagrees with Get: %v", err)
	}
	if err := nexus.VerifyModuleExists("mn.example.absent"); err == nil {
		t.Fatal("an id nothing registered was reported as present")
	}

	var listed bool
	for _, m := range nexus.List() {
		if m.ID() == id {
			listed = true
		}
	}
	if !listed {
		t.Fatal("the module is in the registry but not in List()")
	}

	// The routes really mount. A module that registers and then cannot be
	// served is the failure this would otherwise only show at boot.
	router := chi.NewRouter()
	got.RegisterRoutes(router, func(next http.Handler) http.Handler { return next })
	if !router.Match(chi.NewRouteContext(), http.MethodGet, "/api/v1/probe/") {
		t.Fatal("the module's route did not mount")
	}
}

func TestALabelFallsBackToTheDefault(t *testing.T) {
	item := nexus.MenuDefinition{Label: "People", Labels: map[string]string{"mn": "Ажилтнууд", "fr": ""}}
	for _, c := range []struct{ locale, want string }{
		{"mn", "Ажилтнууд"},
		{"en", "People"},
		// An empty translation is a missing one. Left to win, it renders as a
		// blank menu entry — which is how a locale nobody finished shipping
		// produces a sidebar of nameless rows.
		{"fr", "People"},
		{"zh", "People"},
	} {
		if got := item.LocalizedLabel(c.locale); got != c.want {
			t.Errorf("%s: got %q, want %q", c.locale, got, c.want)
		}
	}
}

// The contract may not depend on the implementation.
//
// `pkg/nexus` is the one package in this repository that other repositories
// compile against. An import of anything under `internal/` — direct or through
// another of our packages — would make it uncompilable outside this module, and
// the error a third party would get names a package they have never heard of.
// Nothing else in the build would notice, because inside this module the import
// is perfectly legal.
func TestTheSDKDoesNotDependOnInternal(t *testing.T) {
	const modulePrefix = "github.com/gerege-systems/open-gerege-nexus/backend"

	seen := map[string]bool{}
	var walk func(importPath, via string)
	walk = func(importPath, via string) {
		if seen[importPath] {
			return
		}
		seen[importPath] = true

		if strings.HasPrefix(importPath, modulePrefix+"/internal") {
			t.Errorf("pkg/nexus reaches %s (via %s); the SDK cannot import internal/", importPath, via)
			return
		}
		// Only our own packages are walked. A third-party dependency cannot
		// import this module's internal packages — Go forbids it — so following
		// them would cost time and find nothing.
		if !strings.HasPrefix(importPath, modulePrefix) {
			return
		}

		pkg, err := build.Import(importPath, "", 0)
		if err != nil {
			t.Fatalf("resolve %s: %v", importPath, err)
		}
		for _, next := range pkg.Imports {
			walk(next, importPath)
		}
	}
	walk(modulePrefix+"/pkg/nexus", "the test")
}

// A module written against the SDK alone can do the job, not merely declare it.
//
// The first test proved a third party can be *registered*. This one proves the
// rest of the working day: name the organisation the request is for, name the
// caller, refuse the request when a permission is missing, answer in JSON, and
// leave an audit row. Every one of those used to require a package under
// internal/, which is to say it required a fork.
//
// Nothing here imports anything but this package, chi and the standard library
// — deliberately, and the import block is part of the assertion.
type workingModule struct{ p nexus.Platform }

func (workingModule) ID() string                       { return "mn.example.working" }
func (workingModule) Name() string                     { return "Working" }
func (workingModule) Version() string                  { return "1.0.0" }
func (workingModule) Dependencies() []nexus.Dependency { return nil }
func (workingModule) Menus() []nexus.MenuDefinition    { return nil }
func (workingModule) Permissions() []nexus.PermissionDefinition {
	return []nexus.PermissionDefinition{{Code: "working.read", Name: "Read"}}
}

func (m workingModule) RegisterRoutes(r chi.Router, gate func(http.Handler) http.Handler) {
	r.Route("/api/v1/working", func(wr chi.Router) {
		wr.Use(gate)
		wr.With(nexus.RequirePermission(m.p.Permissions(), "working.read")).Get("/", m.handle)
	})
}

func (m workingModule) handle(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := nexus.RequireWorkspace(w, r)
	if !ok {
		return
	}
	claims, err := nexus.UserFromContext(r.Context())
	if err != nil {
		nexus.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	nexus.Audit(r.Context(), workspaceID, claims.UserID, "working.read", "working", nil)
	nexus.JSON(w, http.StatusOK, map[string]string{"tenant": workspaceID, "user": claims.UserID})
}

// grants is a PermissionStore a test can hold. That the interface is small
// enough to implement in three lines is the point of it being an interface.
type grants map[string]bool

func (g grants) GetUserPermissions(context.Context, string, string) (map[string]bool, error) {
	return g, nil
}

func TestAModuleWrittenAgainstTheSDKCanServeARequest(t *testing.T) {
	var recorded []string
	nexus.Provide[nexus.AuditSink](func(_ context.Context, workspaceID, userID, action, _ string, _ map[string]any) {
		recorded = append(recorded, action+" "+workspaceID+" "+userID)
	})
	t.Cleanup(func() { nexus.Provide[nexus.AuditSink](nil) })

	call := func(permissions grants) *httptest.ResponseRecorder {
		module := workingModule{p: nexus.NewPlatform(nil, permissions)}
		router := chi.NewRouter()
		module.RegisterRoutes(router, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := nexus.WithWorkspaceID(r.Context(), "tenant-1")
				ctx = nexus.WithUser(ctx, nexus.UserClaims{UserID: "user-1", WorkspaceID: "tenant-1"})
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/working/", nil))
		return rec
	}

	// Without the permission the module declared, the request is refused — by
	// the SDK's own middleware, with the SDK's own JSON error shape.
	refused := call(grants{})
	if refused.Code != http.StatusForbidden {
		t.Fatalf("a caller without the permission got %d", refused.Code)
	}
	var problem struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(refused.Body.Bytes(), &problem); err != nil {
		t.Fatalf("the refusal was not JSON: %v", err)
	}
	if !strings.Contains(problem.Error, "working.read") {
		t.Fatalf("the refusal does not name the permission: %q", problem.Error)
	}
	if len(recorded) != 0 {
		t.Fatalf("a refused request recorded an audit event: %v", recorded)
	}

	// With it, the handler runs, reads both halves of the context and answers.
	allowed := call(grants{"working.read": true})
	if allowed.Code != http.StatusOK {
		t.Fatalf("a permitted caller got %d: %s", allowed.Code, allowed.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(allowed.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["tenant"] != "tenant-1" || body["user"] != "user-1" {
		t.Fatalf("the handler could not see who was asking: %+v", body)
	}
	if len(recorded) != 1 || recorded[0] != "working.read tenant-1 user-1" {
		t.Fatalf("the audit trail did not receive the event: %v", recorded)
	}
}
