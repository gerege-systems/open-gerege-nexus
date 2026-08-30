/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// The assistant. Its data sources are being pulled out of the core.

import { request } from "./client";

export const aiApi = {
  queryAICopilot: (prompt: string) =>
    request<{ answer: string; intent: string; data?: any; actionable?: string[] }>("/ai/copilot", {
      method: "POST",
      body: JSON.stringify({ prompt }),
    }),

  chatAI: (data: {
    prompt?: string;
    lang?: string;
    history?: Array<{ role: "user" | "model"; text: string }>;
    audio?: { mime: string; data: string };
  }) => request<{ answer: string; reply: string; steps?: Array<{ tool: string }>; degraded?: boolean }>("/ai/chat", {
    method: "POST", body: JSON.stringify(data),
  }),

  speakAI: (text: string) => request<{ mime: string; data: string }>("/ai/tts", {
    method: "POST", body: JSON.stringify({ text }),
  }),

  translateAI: (data: { text?: string; audio?: { mime: string; data: string }; target_lang: string; speak?: boolean }) =>
    request<{ source_text: string; translated: string; audio?: { mime: string; data: string } }>("/ai/translate", {
      method: "POST", body: JSON.stringify(data),
    }),

  // The shared prompts and the knowledge base are administered in the console
  // now — /api/platform/v1/assistant — because they belong to the deployment
  // rather than to any one organisation.

  getAIForecast: () =>
    request<
      Array<{
        product_id: string;
        sku: string;
        product_name: string;
        current_stock: number;
        recommended_min: number;
        reorder_alert: boolean;
        suggested_reorder: number;
      }>
    >("/ai/stock-forecast"),

  // e-Government Link — the ХУР registry, reached through the egov app.
  //
  // A tenant without that app installed gets 403 here rather than a result.
  // That is a state a caller has to handle rather than an error to report: see
  // egovInstalled below, and the contacts screen, which hides the button.
};
