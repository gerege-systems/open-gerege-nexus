/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import (
	"context"
	"log/slog"
	"sync"
)

// What an app lends the assistant.
//
// The platform's copilot used to declare two tools of its own — erp_summary and
// search_products — and answer them with SQL over products, contacts,
// warehouses and stock_levels. Those are commerce's tables and commerce is in
// business-gerege-nexus. The tables are still created by db/migrations, so
// nothing failed: on a deployment that never had commerce the copilot answered
// "0 products, 0 customers, 0 stock" and said it with the confidence of a
// figure it had counted. copilot.go's own comment says a half answer is worse
// than no answer; this was the case it described.
//
// So the core no longer knows what an assistant tool is called. A module
// declares its own, and a distribution's module declares tools this repository
// has never heard of:
//
//	func (m *Module) AssistantTools() []nexus.AssistantTool {
//	    return []nexus.AssistantTool{{
//	        Name:        "erp_summary",
//	        Description: "Get current tenant product, contact, warehouse and inventory totals.",
//	        Call:        m.summary,
//	    }}
//	}
//
//	nexus.ProvideAssistant(m.ID(), m)
//
// The contract speaks in strings, maps and a context, and in nothing from
// internal/ — the lesson meetings.go wrote down after an interface declared in
// internal types made its own module unmovable.
type AssistantTool struct {
	// Name is what the model calls. It is the app's word, not the platform's:
	// nothing here validates it against a list, because a list is the thing
	// being removed.
	Name string
	// Description is what the model reads to decide whether to call it. It is
	// the whole of the model's knowledge about this tool, so it is prose rather
	// than a label.
	Description string
	// Parameters is the tool's JSON Schema, or nil for a tool that takes none.
	// Passed to the model as declared; the platform does not inspect it.
	Parameters map[string]any
	// Call runs the tool for one organisation and returns what the model is
	// told. An error is turned into a refusal the model can report, not into a
	// number: a tool that could not answer must not look like one that
	// answered zero.
	Call func(ctx context.Context, workspaceID string, args map[string]any) (map[string]any, error)
}

// AssistantSource is a module with something to lend the assistant.
//
// Asked per request rather than read once, so a module may offer different
// tools to different organisations — which app is installed, which integration
// is connected — without the platform holding a snapshot that is wrong the
// moment either changes.
type AssistantSource interface {
	AssistantTools() []AssistantTool
}

var (
	assistantMu      sync.RWMutex
	assistantSources = map[string]AssistantSource{}
)

// ProvideAssistant registers a module as a source of assistant tools.
//
// Keyed by module id rather than by type, because unlike Link or DocumentFiler
// there is no reason for a deployment to have only one: every app with data
// worth asking about can lend some. Registering the same id twice replaces what
// it had, and is logged for the same reason Provide logs it.
//
// A nil source withdraws the registration.
func ProvideAssistant(moduleID string, source AssistantSource) {
	assistantMu.Lock()
	defer assistantMu.Unlock()
	if source == nil {
		delete(assistantSources, moduleID)
		return
	}
	if _, replaced := assistantSources[moduleID]; replaced {
		slog.Warn("a module registered assistant tools twice and the later ones win", "module", moduleID)
	}
	assistantSources[moduleID] = source
}

// AssistantToolset is every tool this deployment can offer the model, now.
//
// An empty result is an ordinary state and the honest one: a deployment with no
// app lending data has nothing for the assistant to look up, and the assistant
// should say so rather than count rows in tables nobody filled.
//
// Two modules claiming one tool name is a build mistake the way two modules
// claiming one id is, so the later registration wins and both are still
// returned to nobody's benefit — the platform does not arbitrate, it hands the
// model what the deployment declared.
func AssistantToolset() []AssistantTool {
	assistantMu.RLock()
	defer assistantMu.RUnlock()

	tools := make([]AssistantTool, 0, len(assistantSources))
	for _, source := range assistantSources {
		tools = append(tools, source.AssistantTools()...)
	}
	return tools
}
