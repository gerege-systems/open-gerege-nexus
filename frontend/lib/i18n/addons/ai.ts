/**
 * ai — The AI copilot and the tenant's answering scope: its system prompt and
 * the knowledge the assistant is allowed to draw on.
 */
export const ai = {
  "ai.view.title": { mn: "AI Туслах", en: "AI Copilot" },
  "ai.view.subtitle": { mn: "{brand} AI туслах", en: "{brand} AI Assistant" },
  "ai.view.placeholder": { mn: "AI туслахаас асуу...", en: "Ask AI Copilot..." },
  // Shown under the panel title. Was a hard-coded English literal in
  // AICopilot.tsx; it names the model and the guarantee that tools stay inside
  // the current tenant, so it belongs in the dictionary like everything else.
  "ai.view.engine_note": { mn: "Gemini · tenant-д хязгаарлагдсан платформ хэрэгслүүд", en: "Gemini · tenant-safe platform tools" },
  "ai.view.tab_chat": { mn: "AI чат", en: "Chat" },
  "ai.view.tab_translate": { mn: "Орчуулга", en: "Translate" },
  "ai.view.translate_placeholder": { mn: "Орчуулах текст…", en: "Text to translate…" },
  "ai.view.settings_title": { mn: "AI тохиргоо", en: "AI settings" },
  "ai.view.settings_subtitle": {
    mn: "Тухайн байгууллагын AI хариулах хүрээ, заавар болон мэдлэгийн санг удирдана.",
    en: "The answering scope, instructions and knowledge base this tenant's assistant works from.",
  },
  "ai.view.system_prompt": { mn: "Системийн заавар", en: "System prompt" },
  "ai.view.knowledge": { mn: "Мэдлэгийн сан", en: "Knowledge base" },

  "ai.field.target_language": { mn: "Зорилтот хэл", en: "Target language" },
  "ai.field.knowledge_title": { mn: "Гарчиг", en: "Title" },
  "ai.field.source_url": { mn: "Эх сурвалж URL", en: "Source URL" },
  "ai.field.knowledge_content": { mn: "AI ашиглах баталгаатай мэдээлэл", en: "Verified information the assistant may use" },

  "ai.scope.global": { mn: "Үндсэн", en: "Default" },
  "ai.scope.tenant": { mn: "Байгууллагын", en: "Tenant" },

  "ai.action.listen": { mn: "Сонсох", en: "Listen" },
  "ai.action.send": { mn: "Илгээх", en: "Send" },
  "ai.action.record": { mn: "Дуу бичих", en: "Record voice" },
  "ai.action.stop_recording": { mn: "Бичлэг зогсоох", en: "Stop recording" },
  "ai.action.add_knowledge": { mn: "Мэдлэг нэмэх", en: "Add knowledge" },

  "ai.message.greeting": {
    mn: "Сайн байна уу. Байгууллагын өгөгдөл, бараа материал, харилцагч болон системийн ажиллагааны талаар асуугаарай.",
    en: "Hello. Ask me about your organization data, stock, contacts or how the system works.",
  },
  "ai.message.voice_message": { mn: "🎙 Дуут мессеж", en: "🎙 Voice message" },
  "ai.message.microphone_denied": { mn: "Микрофонд хандах боломжгүй байна.", en: "The microphone is not available." },
  "ai.message.processing": { mn: "AI боловсруулж байна…", en: "The assistant is working…" },
  "ai.message.prompt_saved": { mn: "AI заавар хадгалагдлаа", en: "The prompt was saved" },
  "ai.message.knowledge_added": { mn: "Мэдлэг нэмэгдлээ", en: "The knowledge entry was added" },
} as const;
