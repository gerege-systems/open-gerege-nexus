/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/gemini"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

type source struct{ tools []nexus.AssistantTool }

func (s source) AssistantTools() []nexus.AssistantTool { return s.tools }

// With no app lending data, the copilot can search platform knowledge and
// nothing else.
//
// It used to declare erp_summary and search_products whatever was installed,
// and answer them with SQL over commerce's tables. Those tables are still
// created by db/migrations, so the query succeeded and returned zeros — and the
// model reported "0 products, 0 customers" as a count it had taken.
func TestWithNoSourceTheAssistantOffersOnlyPlatformKnowledge(t *testing.T) {
	declared := names(toolDeclarations())
	if len(declared) != 1 || declared[0] != searchKnowledge {
		t.Errorf("a deployment with no assistant source declares %v; want only %q", declared, searchKnowledge)
	}
	for _, gone := range []string{"erp_summary", "search_products"} {
		for _, name := range declared {
			if name == gone {
				t.Errorf("%s is still declared by the platform; it belongs to commerce", gone)
			}
		}
	}
}

// And the prompt says so, because code alone cannot stop the model filling the
// gap from the shape of the question.
func TestWithNoSourceThePromptForbidsInventingNumbers(t *testing.T) {
	var service CopilotService
	prompt := service.systemPrompt(context.Background(), "tenant-1", "en")
	for _, phrase := range []string{"no app lending you organisation data", "never answer it with zero"} {
		if !strings.Contains(prompt, phrase) {
			t.Errorf("the system prompt does not say %q:\n%s", phrase, prompt)
		}
	}
}

// An app's tool reaches the model, and the platform never learns its name.
func TestAProvidedToolIsDeclaredAndCalled(t *testing.T) {
	const appID = "io.example.commerce"
	t.Cleanup(func() { nexus.ProvideAssistant(appID, nil) })

	called := ""
	nexus.ProvideAssistant(appID, source{tools: []nexus.AssistantTool{{
		Name:        "erp_summary",
		Description: "Totals for this organisation.",
		Call: func(_ context.Context, tenantID string, _ map[string]any) (map[string]any, error) {
			called = tenantID
			return map[string]any{"products": 12}, nil
		},
	}}})

	declared := names(toolDeclarations())
	if len(declared) != 2 {
		t.Fatalf("declared %v; want platform knowledge plus the app's tool", declared)
	}

	var service CopilotService
	result := service.executeTool(context.Background(), "tenant-1", gemini.FunctionCall{Name: "erp_summary"})
	if result["products"] != 12 {
		t.Errorf("the tool's answer did not come back: %v", result)
	}
	if called != "tenant-1" {
		t.Errorf("the tool was called for %q, want tenant-1", called)
	}

	// The prompt stops warning once there is something to look up.
	if strings.Contains(service.systemPrompt(context.Background(), "tenant-1", "en"), "never answer it with zero") {
		t.Error("the no-data warning is still in the prompt after a source registered")
	}
}

// A tool that fails answers a refusal, not a number. The model states whatever
// it is handed as fact, so an empty result would become "you have none".
func TestAFailingToolIsNotAnEmptyAnswer(t *testing.T) {
	const appID = "io.example.broken"
	t.Cleanup(func() { nexus.ProvideAssistant(appID, nil) })
	nexus.ProvideAssistant(appID, source{tools: []nexus.AssistantTool{{
		Name: "erp_summary",
		Call: func(context.Context, string, map[string]any) (map[string]any, error) {
			return nil, context.DeadlineExceeded
		},
	}}})

	var service CopilotService
	result := service.executeTool(context.Background(), "tenant-1", gemini.FunctionCall{Name: "erp_summary"})
	if result["error"] == nil {
		t.Errorf("a failing tool answered %v rather than an error", result)
	}
}

// A name nothing declared is refused rather than guessed at.
func TestAnUnknownToolIsRefused(t *testing.T) {
	var service CopilotService
	result := service.executeTool(context.Background(), "tenant-1", gemini.FunctionCall{Name: "search_products"})
	if result["error"] != "tool not allowed" {
		t.Errorf("an undeclared tool answered %v", result)
	}
}

func names(declarations []gemini.FunctionDeclaration) []string {
	out := make([]string, 0, len(declarations))
	for _, d := range declarations {
		out = append(out, d.Name)
	}
	return out
}
