/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package assistant

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator/optest"
)

// Saving a shared prompt has to work twice: the first save may have no row to
// update and the second must not make a second one. The table's own uniqueness
// is UNIQUE (tenant_id, prompt_key), which two NULLs slip past, so this is the
// test that would fail if migration 00095's partial index were dropped.
func TestSavingASharedPromptTwiceLeavesOneRow(t *testing.T) {
	pool := optest.Pool(t)
	op := operator.New(pool)
	service := New(op, Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	sess := optest.Session(account)
	ctx := context.Background()

	first := fmt.Sprintf("answer only about this deployment (%d)", time.Now().UnixNano())
	if err := service.SavePrompt(ctx, sess, "scope", first, true, "prove the first save"); err != nil {
		t.Fatalf("save the shared prompt: %v", err)
	}
	second := first + " — and say so plainly"
	if err := service.SavePrompt(ctx, sess, "scope", second, false, "prove the second save"); err != nil {
		t.Fatalf("save it again: %v", err)
	}

	var rows int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM workspace.ai_prompts WHERE tenant_id IS NULL AND prompt_key = 'scope'`).
		Scan(&rows); err != nil {
		t.Fatalf("count the shared prompts: %v", err)
	}
	if rows != 1 {
		t.Fatalf("%d shared 'scope' prompts, want 1", rows)
	}

	prompts, err := service.ListPrompts(ctx)
	if err != nil {
		t.Fatalf("list the prompts: %v", err)
	}
	if len(prompts) != len(PromptKeys) {
		t.Fatalf("%d prompts, want one per key the copilot reads", len(prompts))
	}
	var scope Prompt
	for _, prompt := range prompts {
		if prompt.Key == "scope" {
			scope = prompt
		}
	}
	if scope.Content != second || scope.Active {
		t.Fatalf("the shared prompt reads %+v, want the second save with active off", scope)
	}
}

// A key the copilot does not read is a row nothing consults.
func TestAnUnknownPromptKeyIsRefused(t *testing.T) {
	pool := optest.Pool(t)
	op := operator.New(pool)
	service := New(op, Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleSuperadmin)

	err := service.SavePrompt(context.Background(), optest.Session(account),
		"tone-of-voice", "be cheerful", true, "prove the guard")
	if err == nil {
		t.Fatal("an unknown prompt key was accepted")
	}
}

// Knowledge written from the console is shared: every organisation's assistant
// may answer from it, which is what tenant_id IS NULL means here.
func TestKnowledgeAddedFromTheConsoleIsShared(t *testing.T) {
	pool := optest.Pool(t)
	op := operator.New(pool)
	service := New(op, Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	sess := optest.Session(account)
	ctx := context.Background()

	title := fmt.Sprintf("shared-entry-%d", time.Now().UnixNano())
	entry := Knowledge{Title: title, Content: "the platform's own answer", SourceURL: "https://example.test/policy"}
	if err := service.AddKnowledge(ctx, sess, entry, "prove the write"); err != nil {
		t.Fatalf("add the knowledge entry: %v", err)
	}

	entries, err := service.ListKnowledge(ctx)
	if err != nil {
		t.Fatalf("list the knowledge: %v", err)
	}
	var found *Knowledge
	for index := range entries {
		if entries[index].Title == title {
			found = &entries[index]
		}
	}
	if found == nil {
		t.Fatalf("the entry just written is not in the list of %d", len(entries))
	}

	var shared bool
	if err := pool.QueryRow(context.Background(),
		`SELECT tenant_id IS NULL FROM workspace.ai_knowledge WHERE id = $1::uuid`, found.ID).Scan(&shared); err != nil {
		t.Fatalf("read the entry back: %v", err)
	}
	if !shared {
		t.Fatal("the console wrote an entry that belongs to one organisation")
	}

	if err := service.RemoveKnowledge(ctx, sess, found.ID, "prove the delete"); err != nil {
		t.Fatalf("remove the entry: %v", err)
	}
	var left int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM workspace.ai_knowledge WHERE id = $1::uuid`, found.ID).Scan(&left); err != nil {
		t.Fatalf("count the entry: %v", err)
	}
	if left != 0 {
		t.Fatal("the entry survived its removal")
	}
}

// Every write is audited: the console's Do wraps them, and this is the check
// that a screen added later cannot quietly skip it.
func TestPromptAndKnowledgeWritesAreAudited(t *testing.T) {
	pool := optest.Pool(t)
	op := operator.New(pool)
	service := New(op, Deps{DB: pool})
	account, _ := optest.Account(t, pool, operator.RoleSuperadmin)
	ctx := context.Background()

	reason := fmt.Sprintf("audit probe %d", time.Now().UnixNano())
	if err := service.SavePrompt(ctx, optest.Session(account), "instructions", "be brief", true, reason); err != nil {
		t.Fatalf("save the prompt: %v", err)
	}

	var recorded int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM operator.operator_audit WHERE action = 'assistant.prompt.save' AND reason = $1`, reason).
		Scan(&recorded); err != nil {
		t.Fatalf("read the audit trail: %v", err)
	}
	if recorded != 1 {
		t.Fatalf("%d audit rows for the save, want 1", recorded)
	}
}
