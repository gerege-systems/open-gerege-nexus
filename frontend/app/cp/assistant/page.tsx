"use client";

/**
 * The assistant, as every organisation on the deployment meets it.
 *
 * The prompts here are the ones the copilot carries into a conversation when an
 * organisation has written none of its own, and the knowledge is the corpus it
 * may answer from anywhere. Both used to be edited one organisation at a time,
 * which meant a tenant administrator editing what every other tenant would be
 * answered with; they are the deployment's, so they are edited here, with a
 * reason, in the audit trail.
 */

import React, { useCallback, useEffect, useState } from "react";
import { BookOpen, BrainCircuit, Plus, Save, Trash2 } from "lucide-react";

import { useAction } from "@/components/cp/Action";
import { Card, formatMoment } from "@/components/cp/ui";
import { cp, type Knowledge, type Prompt } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

export default function Assistant() {
  const { t, locale } = useI18n();
  const action = useAction();
  const [prompts, setPrompts] = useState<Prompt[]>([]);
  const [knowledge, setKnowledge] = useState<Knowledge[]>([]);
  const [draft, setDraft] = useState({ title: "", content: "", source_url: "" });
  const [failure, setFailure] = useState("");

  const load = useCallback(async () => {
    try {
      const [prompted, corpus] = await Promise.all([cp.prompts(), cp.knowledge()]);
      setPrompts(prompted.prompts);
      setKnowledge(corpus.knowledge);
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-slate-900 flex items-center gap-2">
          <BrainCircuit className="w-6 h-6 text-[var(--gerege-blue)]" />
          {t("cp.section.assistant")}
        </h1>
        <p className="mt-1 text-sm text-slate-500">{t("cp.hint.assistant")}</p>
      </div>

      {failure && (
        <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
      )}

      <div className="grid grid-cols-1 xl:grid-cols-[minmax(0,1.35fr)_minmax(340px,.65fr)] gap-6 items-start">
        <Card title={t("ai.view.system_prompt")}>
          <div className="p-4 space-y-6">
            {prompts.map((prompt) => (
              <PromptEditor
                key={prompt.key}
                prompt={prompt}
                onSave={(content, active) =>
                  action.run({
                    title: t("base.action.save"),
                    detail: prompt.key,
                    perform: (reason) => cp.savePrompt(prompt.key, content, active, reason),
                    onDone: load,
                  })
                }
              />
            ))}
            {prompts.length === 0 && <p className="text-sm text-slate-500">{t("cp.message.no_activity")}</p>}
          </div>
        </Card>

        <Card title={t("ai.view.knowledge")}>
          <div className="p-4 space-y-4">
            <div className="grid gap-3">
              <input
                placeholder={t("ai.field.knowledge_title")}
                value={draft.title}
                onChange={(event) => setDraft({ ...draft, title: event.target.value })}
                className="rounded-lg border border-slate-300 px-3 py-2 text-sm"
              />
              <input
                placeholder={t("ai.field.source_url")}
                value={draft.source_url}
                onChange={(event) => setDraft({ ...draft, source_url: event.target.value })}
                className="rounded-lg border border-slate-300 px-3 py-2 text-sm"
              />
              <textarea
                placeholder={t("ai.field.knowledge_content")}
                value={draft.content}
                onChange={(event) => setDraft({ ...draft, content: event.target.value })}
                rows={6}
                className="rounded-lg border border-slate-300 px-3 py-2 text-sm"
              />
            </div>
            <button
              type="button"
              disabled={!draft.title.trim() || !draft.content.trim()}
              onClick={() =>
                action.run({
                  title: t("ai.action.add_knowledge"),
                  detail: draft.title,
                  perform: (reason) => cp.addKnowledge(draft, reason),
                  onDone: async () => {
                    setDraft({ title: "", content: "", source_url: "" });
                    await load();
                  },
                })
              }
              className="inline-flex items-center gap-2 rounded-lg bg-[var(--gerege-blue)] px-3 py-2 text-sm font-medium text-[var(--gerege-on-blue)] hover:brightness-105 disabled:opacity-50"
            >
              <Plus className="w-4 h-4" />
              {t("ai.action.add_knowledge")}
            </button>

            <div className="divide-y divide-slate-100 max-h-96 overflow-y-auto">
              {knowledge.map((entry) => (
                <article key={entry.id} className="py-3 flex items-start gap-3">
                  <BookOpen className="w-4 h-4 mt-0.5 text-slate-400 shrink-0" />
                  <div className="min-w-0 flex-1">
                    <h3 className="text-sm font-semibold text-slate-900 truncate">{entry.title}</h3>
                    <p className="text-xs text-slate-500 line-clamp-2">{entry.content}</p>
                    <p className="text-[11px] text-slate-400">{formatMoment(entry.updated_at, locale)}</p>
                  </div>
                  <button
                    type="button"
                    aria-label={t("base.action.delete")}
                    onClick={() =>
                      action.run({
                        title: t("base.action.delete"),
                        detail: entry.title,
                        danger: true,
                        perform: (reason) => cp.removeKnowledge(entry.id, reason),
                        onDone: load,
                      })
                    }
                    className="shrink-0 rounded-lg border border-slate-200 p-1.5 text-slate-500 hover:bg-red-50 hover:text-red-600"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </article>
              ))}
              {knowledge.length === 0 && <p className="py-3 text-sm text-slate-500">{t("cp.message.no_activity")}</p>}
            </div>
          </div>
        </Card>
      </div>

      {action.dialog}
    </div>
  );
}

/**
 * One prompt, with its own draft.
 *
 * The draft is local so that an operator editing the instructions does not lose
 * them when the list reloads after somebody else's save.
 */
function PromptEditor({ prompt, onSave }: { prompt: Prompt; onSave: (content: string, active: boolean) => void }) {
  const { t, locale } = useI18n();
  const [content, setContent] = useState(prompt.content);
  const [active, setActive] = useState(prompt.active);

  useEffect(() => {
    setContent(prompt.content);
    setActive(prompt.active);
  }, [prompt.content, prompt.active]);

  return (
    <div>
      <div className="flex items-center justify-between gap-3 mb-1.5">
        <label className="text-sm font-semibold text-slate-900 font-mono">{prompt.key}</label>
        <span className="text-xs text-slate-400">
          {prompt.updated_at ? formatMoment(prompt.updated_at, locale) : t("cp.state.never")}
        </span>
      </div>
      <textarea
        rows={7}
        value={content}
        onChange={(event) => setContent(event.target.value)}
        className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm font-mono"
      />
      <div className="mt-2 flex items-center gap-3">
        <button
          type="button"
          disabled={!content.trim()}
          onClick={() => onSave(content, active)}
          className="inline-flex items-center gap-2 rounded-lg bg-[var(--gerege-blue)] px-3 py-2 text-sm font-medium text-[var(--gerege-on-blue)] hover:brightness-105 disabled:opacity-50"
        >
          <Save className="w-4 h-4" />
          {t("base.action.save")}
        </button>
        <label className="flex items-center gap-2 text-sm text-slate-600">
          <input type="checkbox" checked={active} onChange={(event) => setActive(event.target.checked)} />
          {t("cp.field.active")}
        </label>
      </div>
    </div>
  );
}
