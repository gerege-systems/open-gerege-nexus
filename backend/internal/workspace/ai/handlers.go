/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package ai

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/go-chi/chi/v5"
)

// Service provides the platform's AI capabilities.
type Service struct {
	db      nexus.DB
	copilot *CopilotService
}

// NewService builds the AI service.
func NewService(db nexus.DB) *Service {
	return &Service{
		db:      db,
		copilot: NewCopilotService(db),
	}
}

func (s *Service) HandleAICopilot(w http.ResponseWriter, r *http.Request) {
	telemetry.RecordAIRequest("copilot")
	s.recordAIUse(r, "copilot")
	tenantID, ok := nexus.RequireTenant(w, r)
	if !ok {
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Prompt == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid prompt")
		return
	}

	res, err := s.copilot.Query(r.Context(), CopilotRequest{
		Prompt:   req.Prompt,
		TenantID: tenantID,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Service) HandleAIChat(w http.ResponseWriter, r *http.Request) {
	telemetry.RecordAIRequest("chat")
	s.recordAIUse(r, "chat")
	tenantID, ok := nexus.RequireTenant(w, r)
	if !ok {
		return
	}
	var req CopilotRequest
	if err := httpx.DecodeLimited(r, &req, 1<<20); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid AI request")
		return
	}
	req.TenantID = tenantID
	res, err := s.copilot.Query(r.Context(), req)
	if err != nil {
		httpx.Error(w, aiStatus(err), err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (s *Service) HandleAISTT(w http.ResponseWriter, r *http.Request) {
	telemetry.RecordAIRequest("stt")
	s.recordAIUse(r, "stt")
	var req struct {
		Audio Audio `json:"audio"`
	}
	if err := httpx.DecodeLimited(r, &req, 1<<20); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid audio request")
		return
	}
	text, err := s.copilot.Transcribe(r.Context(), req.Audio)
	if err != nil {
		httpx.Error(w, aiStatus(err), err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"text": text})
}

func (s *Service) HandleAITTS(w http.ResponseWriter, r *http.Request) {
	telemetry.RecordAIRequest("tts")
	s.recordAIUse(r, "tts")
	var req struct {
		Text string `json:"text"`
	}
	if err := httpx.DecodeLimited(r, &req, 16<<10); err != nil || req.Text == "" {
		httpx.Error(w, http.StatusBadRequest, "text is required")
		return
	}
	audio, err := s.copilot.Speak(r.Context(), req.Text)
	if err != nil {
		httpx.Error(w, aiStatus(err), err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, audio)
}

func (s *Service) HandleAITranslate(w http.ResponseWriter, r *http.Request) {
	telemetry.RecordAIRequest("translate")
	s.recordAIUse(r, "translate")
	var req struct {
		Text   string `json:"text"`
		Audio  *Audio `json:"audio"`
		Target string `json:"target_lang"`
		Speak  bool   `json:"speak"`
	}
	if err := httpx.DecodeLimited(r, &req, 1<<20); err != nil || req.Target == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid translation request")
		return
	}
	if req.Text == "" && req.Audio != nil {
		var err error
		req.Text, err = s.copilot.Transcribe(r.Context(), *req.Audio)
		if err != nil {
			httpx.Error(w, aiStatus(err), err.Error())
			return
		}
	}
	translated, err := s.copilot.Translate(r.Context(), req.Text, req.Target)
	if err != nil {
		httpx.Error(w, aiStatus(err), err.Error())
		return
	}
	result := map[string]any{"source_text": req.Text, "translated": translated}
	if req.Speak {
		if sound, e := s.copilot.Speak(r.Context(), translated); e == nil {
			result["audio"] = sound
		}
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (s *Service) HandleAIListPrompts(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := nexus.RequireTenant(w, r)
	if !ok {
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT prompt_key,content,active,tenant_id IS NULL FROM workspace.ai_prompts WHERE tenant_id IS NULL OR tenant_id=$1 ORDER BY prompt_key,tenant_id NULLS FIRST`, tenantID)
	if err != nil {
		httpx.Error(w, 500, "failed to load AI prompts")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var key, content string
		var active, global bool
		if err := rows.Scan(&key, &content, &active, &global); err != nil {
			httpx.Error(w, 500, "failed to read AI prompts")
			return
		}
		items = append(items, map[string]any{"key": key, "content": content, "active": active, "global": global})
	}
	if err := rows.Err(); err != nil {
		httpx.Error(w, 500, "failed to read AI prompts")
		return
	}
	httpx.JSON(w, 200, items)
}

func (s *Service) HandleAIUpdatePrompt(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := nexus.RequireTenant(w, r)
	if !ok {
		return
	}
	key := chi.URLParam(r, "key")
	if key != "scope" && key != "instructions" {
		httpx.Error(w, 400, "invalid prompt key")
		return
	}
	var req struct {
		Content string `json:"content"`
		Active  bool   `json:"active"`
	}
	if httpx.DecodeLimited(r, &req, 32<<10) != nil || strings.TrimSpace(req.Content) == "" {
		httpx.Error(w, 400, "content is required")
		return
	}
	_, err := s.db.Exec(r.Context(), `INSERT INTO workspace.ai_prompts(tenant_id,prompt_key,content,active) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,prompt_key) DO UPDATE SET content=EXCLUDED.content,active=EXCLUDED.active,updated_at=NOW()`, tenantID, key, req.Content, req.Active)
	if err != nil {
		httpx.Error(w, 500, "failed to save AI prompt")
		return
	}
	httpx.JSON(w, 200, map[string]string{"status": "saved"})
}

func (s *Service) HandleAIListKnowledge(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := nexus.RequireTenant(w, r)
	if !ok {
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT id,title,content,source_url,updated_at FROM workspace.ai_knowledge WHERE tenant_id=$1 ORDER BY updated_at DESC LIMIT 100`, tenantID)
	if err != nil {
		httpx.Error(w, 500, "failed to load knowledge")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, title, content, url string
		var updated time.Time
		if err := rows.Scan(&id, &title, &content, &url, &updated); err != nil {
			httpx.Error(w, 500, "failed to read knowledge")
			return
		}
		items = append(items, map[string]any{"id": id, "title": title, "content": content, "source_url": url, "updated_at": updated})
	}
	if err := rows.Err(); err != nil {
		httpx.Error(w, 500, "failed to read knowledge")
		return
	}
	httpx.JSON(w, 200, items)
}

func (s *Service) HandleAICreateKnowledge(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := nexus.RequireTenant(w, r)
	if !ok {
		return
	}
	var req struct {
		Title     string `json:"title"`
		Content   string `json:"content"`
		SourceURL string `json:"source_url"`
	}
	if httpx.DecodeLimited(r, &req, 256<<10) != nil || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		httpx.Error(w, 400, "title and content are required")
		return
	}
	var id string
	err := s.db.QueryRow(r.Context(), `INSERT INTO workspace.ai_knowledge(tenant_id,title,content,source_url) VALUES($1,$2,$3,$4) RETURNING id`, tenantID, req.Title, req.Content, req.SourceURL).Scan(&id)
	if err != nil {
		httpx.Error(w, 500, "failed to save knowledge")
		return
	}
	httpx.JSON(w, 201, map[string]string{"id": id})
}

func aiStatus(error) int { return http.StatusBadGateway }

func (s *Service) HandleAIForecast(w http.ResponseWriter, r *http.Request) {
	telemetry.RecordAIRequest("forecast")
	s.recordAIUse(r, "forecast")
	tenantID, ok := nexus.RequireTenant(w, r)
	if !ok {
		return
	}

	for _, tool := range nexus.AssistantToolset() {
		if tool.Name != stockForecastTool || tool.Call == nil {
			continue
		}
		forecast, err := tool.Call(r.Context(), tenantID, nil)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "failed to generate forecast")
			return
		}
		httpx.JSON(w, http.StatusOK, forecast)
		return
	}

	httpx.Error(w, http.StatusNotFound, "no app on this deployment produces a stock forecast")
}

const stockForecastTool = "stock_forecast"

func (s *Service) recordAIUse(r *http.Request, kind string) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		return
	}
	audit.Record(r.Context(), claims.TenantID, claims.UserID, "ai."+kind, kind, nil)
}
