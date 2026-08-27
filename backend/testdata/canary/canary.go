/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package canary is a distribution the size of a test.
//
// It exists because the real ones do not reach far enough. business-gerege-nexus
// is a genuine distribution and the downstream job builds it on every change,
// but it was written before Provide, Capability, Migrations and AssistantTool
// existed, so it touches none of them. Those are exactly the surfaces where a
// behaviour change would go unnoticed: nothing outside this repository compiles
// against them yet, and the golden file only records their signatures.
//
// So this is not a mock of a distribution. It is a distribution — small, but
// real code using the real contract the same way a product would:
//
//	nexus.Register        a module joins the binary
//	nexus.AccessPolicy    it says how it is gated
//	nexus.Provide         it publishes a capability of its own
//	nexus.Capability      it asks for one
//	nexus.Migrations      it brings its own schema
//	nexus.RegisterReport  it declares a report
//	nexus.AssistantTool   it lends the assistant something
//
// When one of those changes shape or behaviour, this fails to build or fails a
// test, on the pull request that changed it rather than in somebody's product a
// week later.
package canary

import (
	"context"
	"embed"
	"io/fs"
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/go-chi/chi/v5"
)

//go:embed migrations/*.sql
var schema embed.FS

// Pricing is a capability this distribution publishes and nothing in the core
// has heard of — the case Provide exists for.
type Pricing interface {
	Quote(ctx context.Context, tenantID, sku string) (int64, error)
}

type Module struct{ p nexus.Platform }

// New is what a distribution's main() calls, in the order run.go's comment
// describes: construct, register, then platform.Run.
func New(p nexus.Platform) *Module {
	m := &Module{p: p}
	nexus.Register(m)

	migrations, err := fs.Sub(schema, "migrations")
	if err != nil {
		panic("canary: embedded migrations: " + err.Error())
	}
	nexus.Migrations(m.ID(), migrations)

	nexus.Provide[Pricing](fixedPricing{})
	nexus.ProvideAssistant(m.ID(), m)
	nexus.RegisterReport(quoteVolume{})
	return m
}

func (m *Module) ID() string      { return "io.example.canary" }
func (m *Module) Name() string    { return "Canary" }
func (m *Module) Version() string { return "1.0.0" }

func (m *Module) Dependencies() []nexus.Dependency { return nil }

func (m *Module) Permissions() []nexus.PermissionDefinition {
	return []nexus.PermissionDefinition{
		{Code: "canary.read", Name: "Read", Description: "See the canary's rows"},
		{
			Code: "canary.quote", Name: "Quote", Description: "Price something",
			DefaultRoles: []string{nexus.DefaultRoleManager, nexus.DefaultRoleUser},
		},
	}
}

func (m *Module) Menus() []nexus.MenuDefinition {
	return []nexus.MenuDefinition{{
		ID: "canary", Label: "Canary", Path: "/canary", Icon: "bird", Order: 10,
	}}
}

func (m *Module) RegisterRoutes(r chi.Router, gate func(http.Handler) http.Handler) {
	r.Route("/api/v1/canary", func(cr chi.Router) {
		cr.Use(gate)
		cr.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
		cr.Get("/mine", m.mine)
	})
}

// mine is the request-scoped half of the contract: who is asking, and which
// workspace are they asking for.
//
// Nothing else in this file needs it — the module could publish a capability, a
// report and an assistant tool without ever reading a request — and that is
// exactly why it is here. Every handler in every distribution begins with these
// three calls, so they are the most-used part of the SDK and the part a rename
// reaches first. They were caught until now by cloning a real product in CI and
// building it, which pointed the platform's own pipeline at a repository it
// does not own; see .github/workflows/downstream.yml.
func (m *Module) mine(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := nexus.RequireWorkspace(w, r)
	if !ok {
		return
	}
	claims, err := nexus.UserFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	// The same answer by two routes. A handler reads the acting workspace from
	// the claims it already has; a report has no ResponseWriter to refuse with
	// and takes the context alone. Both are contract, so the canary asserts
	// they agree rather than picking one.
	if claims.WorkspaceID != workspaceID || nexus.WorkspaceOf(r.Context()) != workspaceID {
		http.Error(w, "the workspace this request acts for is read three ways and they disagree", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"workspace_id":"` + workspaceID + `","user_id":"` + claims.UserID + `"}`))
}

func (m *Module) MenuPermission() string        { return "canary.read" }
func (m *Module) RoutePermissionPrefix() string { return "canary" }

// Quote asks for the capability rather than holding it, which is the property
// link.go:130-132 records: a module may be built before the platform has
// provided what it needs.
func (m *Module) Quote(ctx context.Context, tenantID, sku string) (int64, error) {
	pricing, err := nexus.Capability[Pricing]()
	if err != nil {
		return 0, err
	}
	return pricing.Quote(ctx, tenantID, sku)
}

// Meetings is here to keep an accessor a distribution would plausibly use in the
// build. It is expected to fail: nothing provides a booker in this module.
func (m *Module) Meetings() error {
	_, err := nexus.Meetings()
	return err
}

func (m *Module) AssistantTools() []nexus.AssistantTool {
	return []nexus.AssistantTool{{
		Name:        "canary_quote",
		Description: "Price one SKU for this organisation.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{
			"sku": map[string]any{"type": "string"}}, "required": []string{"sku"}},
		Call: func(ctx context.Context, tenantID string, args map[string]any) (map[string]any, error) {
			sku, _ := args["sku"].(string)
			price, err := m.Quote(ctx, tenantID, sku)
			if err != nil {
				return nil, err
			}
			return map[string]any{"sku": sku, "price": price}, nil
		},
	}}
}

type fixedPricing struct{}

func (fixedPricing) Quote(context.Context, string, string) (int64, error) { return 4200, nil }

// quoteVolume is a report declared against the SDK's contract, including the
// sharing opt-in.
type quoteVolume struct{}

func (quoteVolume) Key() string { return "canary.quote_volume" }
func (quoteVolume) App() string { return "io.example.canary" }

// Scopes is the sharing opt-in. Declared with the SDK's own constant rather
// than the string, which is the point: until ReportScopeFull was published a
// distribution could not say this at all.
func (quoteVolume) Scopes() []string { return []string{nexus.ReportScopeFull} }

func (quoteVolume) Titles() map[string]string {
	return map[string]string{"mn": "Үнийн санал", "en": "Quote volume"}
}

func (quoteVolume) Params() []nexus.ParamSpec { return nil }

func (quoteVolume) Columns() []nexus.ColumnSpec {
	return []nexus.ColumnSpec{
		{Key: "sku", Kind: nexus.ColumnText, Titles: map[string]string{"en": "SKU"}},
		{Key: "price", Kind: nexus.ColumnMoney, Titles: map[string]string{"en": "Price"}, Total: true},
	}
}

func (quoteVolume) Run(context.Context, nexus.Querier, nexus.Params) (nexus.Result, error) {
	return nexus.Result{Columns: quoteVolume{}.Columns(), Rows: []map[string]any{}}, nil
}
