/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package ai

import (
	"encoding/json"
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
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
	tenantID, ok := nexus.RequireWorkspace(w, r)
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
		fail(w, err, "answer a question")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Service) HandleAIChat(w http.ResponseWriter, r *http.Request) {
	telemetry.RecordAIRequest("chat")
	s.recordAIUse(r, "chat")
	tenantID, ok := nexus.RequireWorkspace(w, r)
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
		fail(w, err, "answer a question")
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
		fail(w, err, "transcribe audio")
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
		fail(w, err, "speak an answer")
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
			fail(w, err, "transcribe audio")
			return
		}
	}
	translated, err := s.copilot.Translate(r.Context(), req.Text, req.Target)
	if err != nil {
		fail(w, err, "translate")
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

// The assistant's shared prompts and its knowledge are administered in the
// console now — internal/operator/assistant — because they are the
// deployment's rather than any one organisation's. What stays here is the
// reading side: the copilot still takes the shared row first and an
// organisation's own after it.

func (s *Service) HandleAIForecast(w http.ResponseWriter, r *http.Request) {
	telemetry.RecordAIRequest("forecast")
	s.recordAIUse(r, "forecast")
	tenantID, ok := nexus.RequireWorkspace(w, r)
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
	audit.Record(r.Context(), claims.WorkspaceID, claims.UserID, "ai."+kind, kind, nil)
}
