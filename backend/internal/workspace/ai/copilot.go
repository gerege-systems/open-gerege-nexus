package ai

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/credentials"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/gemini"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/settings"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

const maxToolRounds = 4

type CopilotRequest struct {
	Prompt   string        `json:"prompt"`
	TenantID string        `json:"tenant_id"`
	Lang     string        `json:"lang,omitempty"`
	History  []ChatMessage `json:"history,omitempty"`
	Audio    *Audio        `json:"audio,omitempty"`
}
type ChatMessage struct {
	Role string `json:"role"`
	Text string `json:"text"`
}
type Audio struct {
	Mime string `json:"mime"`
	Data string `json:"data"`
}
type Step struct {
	Tool string `json:"tool"`
}
type CopilotResponse struct {
	Answer     string         `json:"answer"`
	Reply      string         `json:"reply"`
	Intent     string         `json:"intent"`
	Data       map[string]any `json:"data,omitempty"`
	Actionable []string       `json:"actionable,omitempty"`
	Steps      []Step         `json:"steps,omitempty"`
	Degraded   bool           `json:"degraded,omitempty"`
}

type generator interface {
	GenerateContent(context.Context, gemini.Request) (gemini.Response, error)
}
type CopilotService struct {
	db nexus.DB
	// base is the address the clients are built against. It is the one thing
	// here that still cannot change at runtime.
	base string

	// chat and tts are rebuilt when the model or the key changes.
	//
	// The model has been a platform setting since settings.AIModel, so an
	// operator can move the deployment onto a newer Gemini without a deploy.
	// The key can now change under the same hand — it is a console credential
	// (credentials.GeminiAPIKey), which is what makes rotating it something
	// other than an ssh session. A client carries both, so "either changed"
	// means "build another one", and this is where that is noticed: once per
	// call, comparing two strings.
	chatMu    sync.Mutex
	chatModel string
	chatKey   string
	chat      generator

	ttsMu    sync.Mutex
	ttsModel string
	ttsKey   string
	tts      generator

	voice string
}

// apiKey is the credential as it stands now: what the console holds, then the
// environment.
func apiKey() string { return credentials.Get(credentials.GeminiAPIKey) }

// chatClient returns a generator for the model and key configured now.
func (s *CopilotService) chatClient() generator {
	model, key := settings.Get(settings.AIModel), apiKey()

	s.chatMu.Lock()
	defer s.chatMu.Unlock()
	if s.chat == nil || s.chatModel != model || s.chatKey != key {
		s.chat = observe(gemini.NewClient(s.base, key, model), "generate")
		s.chatModel, s.chatKey = model, key
	}
	return s.chat
}

// ttsClient is chatClient for the voice model.
func (s *CopilotService) ttsClient() generator {
	model, key := settings.Get(settings.AITTSModel), apiKey()

	s.ttsMu.Lock()
	defer s.ttsMu.Unlock()
	if s.tts == nil || s.ttsModel != model || s.ttsKey != key {
		s.tts = observe(gemini.NewClient(s.base, key, model), "tts")
		s.ttsModel, s.ttsKey = model, key
	}
	return s.tts
}

func NewCopilotService(db nexus.DB) *CopilotService {
	base := os.Getenv("GEMINI_API_BASE")
	voice := os.Getenv("GEMINI_VOICE")
	if voice == "" {
		voice = "Kore"
	}
	return &CopilotService{db: db, base: base, voice: voice}
}

func (s *CopilotService) Query(ctx context.Context, req CopilotRequest) (*CopilotResponse, error) {
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" && req.Audio == nil {
		return nil, fmt.Errorf("empty prompt")
	}
	if req.Lang == "" {
		req.Lang = "mn"
	}
	if req.Audio != nil {
		if err := validateAudio(req.Audio); err != nil {
			return nil, err
		}
	}
	contents := make([]gemini.Content, 0, len(req.History)+1)
	for _, h := range req.History {
		if (h.Role == "user" || h.Role == "model") && strings.TrimSpace(h.Text) != "" {
			contents = append(contents, gemini.Content{Role: h.Role, Parts: []gemini.Part{{Text: truncate(h.Text, 4000)}}})
		}
	}
	parts := []gemini.Part{}
	if req.Prompt != "" {
		parts = append(parts, gemini.Part{Text: truncate(req.Prompt, 4000)})
	}
	if req.Audio != nil {
		parts = append(parts, gemini.Part{InlineData: &gemini.Blob{MimeType: req.Audio.Mime, Data: req.Audio.Data}})
	}
	contents = append(contents, gemini.Content{Role: "user", Parts: parts})
	system := s.systemPrompt(ctx, req.TenantID, req.Lang)
	greq := gemini.Request{SystemInstruction: &gemini.Content{Parts: []gemini.Part{{Text: system}}}, Contents: contents, Tools: []gemini.Tool{{FunctionDeclarations: toolDeclarations()}}}
	steps := []Step{}
	for round := 0; round < maxToolRounds; round++ {
		out, err := s.chatClient().GenerateContent(ctx, greq)
		if err != nil {
			if errors.Is(err, gemini.ErrNotConfigured) || errors.Is(err, gemini.ErrUnavailable) {
				return s.localFallback(ctx, req), nil
			}
			return nil, err
		}
		calls := out.FunctionCalls()
		if len(calls) == 0 {
			answer := out.Text()
			if answer == "" {
				return s.localFallback(ctx, req), nil
			}
			return &CopilotResponse{Answer: answer, Reply: answer, Intent: "gemini", Steps: steps}, nil
		}
		greq.Contents = append(greq.Contents, out.ModelContent())
		responses := make([]gemini.Part, 0, len(calls))
		for _, call := range calls {
			result := s.executeTool(ctx, req.TenantID, call)
			steps = append(steps, Step{Tool: call.Name})
			responses = append(responses, gemini.Part{FunctionResponse: &gemini.FunctionResponse{Name: call.Name, Response: result}})
		}
		greq.Contents = append(greq.Contents, gemini.Content{Role: "user", Parts: responses})
	}
	return s.localFallback(ctx, req), nil
}

// systemPrompt builds what the model is told about itself.
//
// `{brand}` in a stored prompt becomes this deployment's name. The name used to
// be written into the row, which made a rebrand a data migration: 00012 rewrote
// it for the Gerege Nexus rename, and the Gerege Salus fork carried a migration
// of its own to do the same thing one distribution later. A deployment that
// differs by a name should not have to write SQL to say so, and a placeholder
// is the difference between a stored default and a stored decision — the first
// follows the deployment, the second is an operator's and must not be touched.
func (s *CopilotService) systemPrompt(ctx context.Context, tenantID, lang string) string {
	scope := "You are {brand} AI Copilot. Use approved tools for live data. Never expose data from another tenant."
	instructions := "Be concise, do not invent values, and reply in language code " + lang + "."
	if s.db != nil {
		rows, err := s.db.Query(ctx, `SELECT prompt_key, content FROM workspace.ai_prompts WHERE active AND (tenant_id IS NULL OR tenant_id=$1) ORDER BY tenant_id NULLS FIRST`, tenantID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var k, v string
				if rows.Scan(&k, &v) == nil {
					if k == "scope" {
						scope = v
					}
					if k == "instructions" {
						instructions = v
					}
				}
			}
			// Дундуур тасарвал суулгацын өөрийн заавар биш, файлд бичсэн
			// анхдагч нь хүчинтэй болно — тэр нь загварт өөр дүрэм өгнө.
			if err := rows.Err(); err != nil {
				slog.Warn("could not read the assistant prompts", "error", err)
			}
		}
	}
	// Applied to whatever won above — the shipped default, the global row, or a
	// tenant's own — so a tenant that wants the product named in its prompt
	// writes `{brand}` and stays right through a rename.
	prompt := strings.ReplaceAll(scope+"\n"+instructions, "{brand}", config.BrandName())

	// What this deployment can actually look up, said to the model in words.
	//
	// Code can stop the copilot counting rows in an app nobody installed; only
	// the prompt can stop it filling the gap from the shape of the question.
	// Both halves are needed, and this is the half that is easy to leave out —
	// the tools simply would not be declared, and a model asked about stock
	// would answer from what a system like this usually has.
	if len(nexus.AssistantToolset()) == 0 {
		prompt += "\n\nThis deployment has no app lending you organisation data: you can search " +
			"platform knowledge and nothing else. If you are asked about inventory, products, " +
			"customers, invoices or any other record, say that this deployment does not carry " +
			"that information. Never answer such a question with a number, and never answer it " +
			"with zero — zero is a count, and you have not counted anything."
	}
	return prompt
}

// searchKnowledge is the platform's own tool: the knowledge base an operator
// curates in ai_knowledge, which every deployment has because the platform has
// it. Everything else the model can call is lent by an app.
const searchKnowledge = "search_knowledge"

func toolDeclarations() []gemini.FunctionDeclaration {
	declarations := []gemini.FunctionDeclaration{{
		Name:        searchKnowledge,
		Description: "Search approved " + config.BrandName() + " platform knowledge.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{
			"query": map[string]any{"type": "string"}}, "required": []string{"query"}},
	}}
	for _, tool := range nexus.AssistantToolset() {
		declarations = append(declarations, gemini.FunctionDeclaration{
			Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters,
		})
	}
	return declarations
}

func (s *CopilotService) executeTool(ctx context.Context, tenantID string, call gemini.FunctionCall) map[string]any {
	if call.Name == searchKnowledge {
		return s.searchKnowledge(ctx, tenantID, call)
	}

	for _, tool := range nexus.AssistantToolset() {
		if tool.Name != call.Name || tool.Call == nil {
			continue
		}
		result, err := tool.Call(ctx, tenantID, call.Args)
		if err != nil {
			// An error, not an empty result. The model states whatever it is
			// handed as fact, so a tool that could not answer must not look
			// like one that answered zero — which is the failure this whole
			// change is about.
			return map[string]any{"error": tool.Name + " is unavailable"}
		}
		return result
	}
	return map[string]any{"error": "tool not allowed"}
}

func (s *CopilotService) searchKnowledge(ctx context.Context, tenantID string, call gemini.FunctionCall) map[string]any {
	if s.db == nil {
		return map[string]any{"error": "database unavailable"}
	}
	q := "%" + truncate(fmt.Sprint(call.Args["query"]), 200) + "%"
	rows, err := s.db.Query(ctx, `SELECT title,content,source_url FROM workspace.ai_knowledge WHERE (tenant_id IS NULL OR tenant_id=$1) AND (title ILIKE $2 OR content ILIKE $2) ORDER BY tenant_id NULLS LAST,updated_at DESC LIMIT 5`, tenantID, q)
	if err != nil {
		return map[string]any{"error": "knowledge unavailable"}
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var title, content, url string
		if err := rows.Scan(&title, &content, &url); err != nil {
			return map[string]any{"error": "knowledge unavailable"}
		}
		items = append(items, map[string]any{"title": title, "content": truncate(content, 1200), "source_url": url})
	}
	// A partial result is worse than none here: the model states whatever it is
	// handed as fact, so a stream that broke halfway would become an answer
	// rather than a question the user can retry.
	if rows.Err() != nil {
		return map[string]any{"error": "knowledge unavailable"}
	}
	return map[string]any{"items": items}
}

// localFallback is what comes back when the model cannot be reached.
//
// It used to answer one question itself — total stock — by querying
// stock_levels, which is commerce's table and not this binary's. That made the
// degraded path give a confident number on a deployment that has no inventory
// at all. There is nothing the platform can answer on its own about an
// organisation's data, and saying so is the honest degraded answer.
func (s *CopilotService) localFallback(_ context.Context, _ CopilotRequest) *CopilotResponse {
	res := &CopilotResponse{Intent: "general", Data: map[string]any{}, Degraded: true}
	res.Answer = "AI үйлчилгээ түр боломжгүй байна. GEMINI_API_KEY тохиргоог шалгана уу."
	res.Reply = res.Answer
	return res
}

func (s *CopilotService) Transcribe(ctx context.Context, audio Audio) (string, error) {
	if err := validateAudio(&audio); err != nil {
		return "", err
	}
	r, e := s.chatClient().GenerateContent(ctx, gemini.Request{Contents: []gemini.Content{{Role: "user", Parts: []gemini.Part{{Text: "Transcribe this audio exactly. Return only the transcript."}, {InlineData: &gemini.Blob{MimeType: audio.Mime, Data: audio.Data}}}}}})
	if e != nil {
		return "", e
	}
	return r.Text(), nil
}
func (s *CopilotService) Translate(ctx context.Context, text, target string) (string, error) {
	target = truncate(target, 12)
	r, e := s.chatClient().GenerateContent(ctx, gemini.Request{Contents: []gemini.Content{{Role: "user", Parts: []gemini.Part{{Text: "Translate the following text to language code " + target + ". Return only the translation:\n" + truncate(text, 8000)}}}}})
	if e != nil {
		return "", e
	}
	return r.Text(), nil
}
func (s *CopilotService) Speak(ctx context.Context, text string) (*Audio, error) {
	temp := 0.2
	r, e := s.ttsClient().GenerateContent(ctx, gemini.Request{Contents: []gemini.Content{{Role: "user", Parts: []gemini.Part{{Text: truncate(text, 2000)}}}}, GenerationConfig: &gemini.GenerationConfig{Temperature: &temp, ResponseModalities: []string{"AUDIO"}, SpeechConfig: &gemini.SpeechConfig{VoiceConfig: &gemini.VoiceConfig{PrebuiltVoiceConfig: &gemini.PrebuiltVoiceConfig{VoiceName: s.voice}}}}})
	if e != nil {
		return nil, e
	}
	blob := r.InlineAudio()
	if blob == nil {
		return nil, errors.New("tts returned no audio")
	}
	raw, e := base64.StdEncoding.DecodeString(blob.Data)
	if e != nil {
		return nil, e
	}
	wav := gemini.PCMToWAV(raw, gemini.PCMRateFromMime(blob.MimeType))
	return &Audio{Mime: "audio/wav", Data: base64.StdEncoding.EncodeToString(wav)}, nil
}

func validateAudio(a *Audio) error {
	if a == nil {
		return BadRequest("audio required")
	}
	allowed := map[string]bool{"audio/webm": true, "audio/ogg": true, "audio/wav": true, "audio/mp4": true, "audio/mpeg": true}
	mime := strings.ToLower(strings.Split(a.Mime, ";")[0])
	if !allowed[mime] {
		return BadRequest("unsupported audio format")
	}
	if len(a.Data) > 950000 {
		return BadRequest("audio is too large")
	}
	if _, e := base64.StdEncoding.DecodeString(a.Data); e != nil {
		return BadRequest("invalid audio data")
	}
	return nil
}
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
