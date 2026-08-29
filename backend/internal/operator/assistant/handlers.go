/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package assistant is the platform's own half of the AI assistant: the
// prompts it answers with and the knowledge it answers from, as they stand for
// every organisation on the deployment.
//
// The workspace used to carry this screen, one copy per organisation. It was
// the wrong plane for it: the model, its instructions and the corpus are the
// deployment's, paid for by the deployment's key, and an organisation editing
// the shared prompt was editing what every other organisation would be
// answered with. Here it is one screen, audited, behind the console's own
// capabilities — see docs/adr/0005-two-planes-one-origin-each.md.
//
// It stands on internal/operator/operator, which decides who is asking and
// records what they did; nothing in this plane stands on this package.
package assistant

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// PromptKeys are the two the copilot reads. A key outside this set would be a
// row nothing consults, so it is refused rather than stored.
var PromptKeys = []string{"scope", "instructions"}

// Prompt is one instruction the assistant carries into every conversation.
type Prompt struct {
	Key       string     `json:"key"`
	Content   string     `json:"content"`
	Active    bool       `json:"active"`
	UpdatedAt *time.Time `json:"updated_at"`
}

// Knowledge is one entry in the corpus the assistant answers from.
type Knowledge struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	SourceURL string    `json:"source_url"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListPrompts returns every key the copilot reads, whether or not the
// deployment has written one: a screen that showed only the rows that exist
// would offer no way to write the missing one.
func (s *Service) ListPrompts(ctx context.Context) ([]Prompt, error) {
	rows, err := s.db.Query(operator.Scoped(ctx),
		`SELECT prompt_key, content, active, updated_at
		   FROM workspace.ai_prompts
		  WHERE tenant_id IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("control plane: list the shared prompts: %w", err)
	}
	defer rows.Close()

	stored := map[string]Prompt{}
	for rows.Next() {
		var prompt Prompt
		var updated time.Time
		if err := rows.Scan(&prompt.Key, &prompt.Content, &prompt.Active, &updated); err != nil {
			return nil, fmt.Errorf("control plane: read a shared prompt: %w", err)
		}
		prompt.UpdatedAt = &updated
		stored[prompt.Key] = prompt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("control plane: read the shared prompts: %w", err)
	}

	prompts := make([]Prompt, 0, len(PromptKeys))
	for _, key := range PromptKeys {
		if prompt, ok := stored[key]; ok {
			prompts = append(prompts, prompt)
			continue
		}
		prompts = append(prompts, Prompt{Key: key, Active: true})
	}
	return prompts, nil
}

// SavePrompt writes one, for every organisation on the deployment.
//
// An UPDATE that falls back to an INSERT rather than ON CONFLICT: the table's
// uniqueness is UNIQUE (tenant_id, prompt_key), and two NULL tenant_ids are
// distinct to Postgres, so the conflict target never matches a shared row.
// Migration 00095 adds the partial unique index that makes this pair safe.
func (s *Service) SavePrompt(ctx context.Context, sess operator.Session, key, content string, active bool, reason string) error {
	if !knownKey(key) {
		return fmt.Errorf("%q is not a prompt this assistant reads", key)
	}
	if strings.TrimSpace(content) == "" {
		return errors.New("a prompt needs something to say")
	}

	return s.op.Do(ctx, sess, operator.Change{
		Action:     "assistant.prompt.save",
		TargetType: "ai_prompt",
		TargetID:   key,
		Reason:     reason,
		After:      map[string]any{"active": active, "length": len(content)},
	}, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE workspace.ai_prompts SET content = $2, active = $3, updated_at = NOW()
			  WHERE tenant_id IS NULL AND prompt_key = $1`, key, content, active)
		if err != nil {
			return err
		}
		if tag.RowsAffected() > 0 {
			return nil
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO workspace.ai_prompts (tenant_id, prompt_key, content, active)
			 VALUES (NULL, $1, $2, $3)`, key, content, active)
		return err
	})
}

// ListKnowledge returns the shared corpus, newest first.
func (s *Service) ListKnowledge(ctx context.Context) ([]Knowledge, error) {
	rows, err := s.db.Query(operator.Scoped(ctx),
		`SELECT id::text, title, content, source_url, updated_at
		   FROM workspace.ai_knowledge
		  WHERE tenant_id IS NULL
		  ORDER BY updated_at DESC LIMIT 100`)
	if err != nil {
		return nil, fmt.Errorf("control plane: list the shared knowledge: %w", err)
	}
	defer rows.Close()

	entries := make([]Knowledge, 0, 8)
	for rows.Next() {
		var entry Knowledge
		if err := rows.Scan(&entry.ID, &entry.Title, &entry.Content, &entry.SourceURL, &entry.UpdatedAt); err != nil {
			return nil, fmt.Errorf("control plane: read a knowledge entry: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// AddKnowledge writes one entry every organisation's assistant may answer from.
func (s *Service) AddKnowledge(ctx context.Context, sess operator.Session, entry Knowledge, reason string) error {
	if strings.TrimSpace(entry.Title) == "" || strings.TrimSpace(entry.Content) == "" {
		return errors.New("a knowledge entry needs a title and something in it")
	}

	return s.op.Do(ctx, sess, operator.Change{
		Action:     "assistant.knowledge.add",
		TargetType: "ai_knowledge",
		TargetID:   entry.Title,
		Reason:     reason,
		After:      map[string]any{"title": entry.Title, "source_url": entry.SourceURL},
	}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO workspace.ai_knowledge (tenant_id, title, content, source_url)
			 VALUES (NULL, $1, $2, $3)`, entry.Title, entry.Content, entry.SourceURL)
		return err
	})
}

// RemoveKnowledge deletes one. A corpus nobody can take a page out of is a
// corpus that grows a wrong answer and keeps it.
func (s *Service) RemoveKnowledge(ctx context.Context, sess operator.Session, id, reason string) error {
	return s.op.Do(ctx, sess, operator.Change{
		Action:     "assistant.knowledge.remove",
		TargetType: "ai_knowledge",
		TargetID:   id,
		Reason:     reason,
	}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM workspace.ai_knowledge WHERE id = $1::uuid AND tenant_id IS NULL`, id)
		return err
	})
}

func knownKey(key string) bool {
	for _, known := range PromptKeys {
		if known == key {
			return true
		}
	}
	return false
}

func (s *Service) handleListPrompts(w http.ResponseWriter, r *http.Request) {
	prompts, err := s.ListPrompts(r.Context())
	if err != nil {
		fail(w, err, "could not read the assistant's prompts")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"prompts": prompts})
}

func (s *Service) handleSavePrompt(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		Content string `json:"content"`
		Active  bool   `json:"active"`
		Reason  string `json:"reason"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.SavePrompt(r.Context(), sess, chi.URLParam(r, "key"), body.Content, body.Active, body.Reason); err != nil {
		fail(w, err, "could not save the prompt")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (s *Service) handleListKnowledge(w http.ResponseWriter, r *http.Request) {
	entries, err := s.ListKnowledge(r.Context())
	if err != nil {
		fail(w, err, "could not read the assistant's knowledge")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"knowledge": entries})
}

func (s *Service) handleAddKnowledge(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		Knowledge
		Reason string `json:"reason"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.AddKnowledge(r.Context(), sess, body.Knowledge, body.Reason); err != nil {
		fail(w, err, "could not add the knowledge entry")
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{"status": "added"})
}

func (s *Service) handleRemoveKnowledge(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body operator.Reasoned
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.RemoveKnowledge(r.Context(), sess, chi.URLParam(r, "id"), body.Reason); err != nil {
		fail(w, err, "could not remove the knowledge entry")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// Routes are this screen's, mounted on the console's signed-in group.
//
// Reading what the assistant says to everybody is a read any operator may
// make. Every write asks for the second factor as well as the capability, as
// the platform settings do: a prompt and a knowledge entry both decide what
// every organisation on the deployment is answered with, and a borrowed
// console session should not be able to change that quietly.
func (s *Service) Routes(r chi.Router) {
	r.With(s.op.RequireCapability(operator.CapTenantRead)).Get("/assistant/prompts", s.handleListPrompts)
	r.With(s.op.RequireCapability(operator.CapSettingsWrite), s.op.RequireStepUp).
		Put("/assistant/prompts/{key}", s.handleSavePrompt)
	r.With(s.op.RequireCapability(operator.CapTenantRead)).Get("/assistant/knowledge", s.handleListKnowledge)
	r.With(s.op.RequireCapability(operator.CapSettingsWrite), s.op.RequireStepUp).
		Post("/assistant/knowledge", s.handleAddKnowledge)
	r.With(s.op.RequireCapability(operator.CapSettingsWrite), s.op.RequireStepUp).
		Delete("/assistant/knowledge/{id}", s.handleRemoveKnowledge)
}
