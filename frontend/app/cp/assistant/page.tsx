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
import { Button, Card, CardHeader, Checkbox, IconButton, Input, Spinner, Textarea } from "@gerege-systems/ui";

import { useAction } from "@/components/cp/Action";
import { cp, type Knowledge, type Prompt } from "@/lib/cp";
import { formatMoment } from "@/lib/datetime";
import { useI18n } from "@/lib/i18n";

export default function Assistant() {
  const { t } = useI18n();
  const action = useAction();
  const [prompts, setPrompts] = useState<Prompt[]>([]);
  const [knowledge, setKnowledge] = useState<Knowledge[]>([]);
  const [draft, setDraft] = useState({ title: "", content: "", source_url: "" });
  const [failure, setFailure] = useState("");
  const [loaded, setLoaded] = useState(false);

  const load = useCallback(async () => {
    try {
      const [prompted, corpus] = await Promise.all([cp.prompts(), cp.knowledge()]);
      setPrompts(prompted.prompts);
      setKnowledge(corpus.knowledge);
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setLoaded(true);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
          <BrainCircuit className="w-6 h-6 text-accent" />
          {t("cp.section.assistant")}
        </h1>
        <p className="mt-1 text-sm text-muted">{t("cp.hint.assistant")}</p>
      </div>

      {failure && (
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
      )}

      <div className="grid grid-cols-1 xl:grid-cols-[minmax(0,1.35fr)_minmax(340px,.65fr)] gap-6 items-start">
        <Card padding="none" className="overflow-hidden min-w-0">
          <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
            <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("ai.view.system_prompt")}</h2>
          </CardHeader>
          <div className="p-4 space-y-6">
            {!loaded && (
              <p role="status" className="flex items-center gap-2 text-sm text-muted">
                <Spinner size="sm" decorative />
                {t("base.message.loading")}
              </p>
            )}
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
            {loaded && prompts.length === 0 && <p className="text-sm text-muted">{t("cp.message.no_activity")}</p>}
          </div>
        </Card>

        <Card padding="none" className="overflow-hidden min-w-0">
          <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
            <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("ai.view.knowledge")}</h2>
          </CardHeader>
          <div className="p-4 space-y-4">
            <div className="grid gap-3">
              <Input
                label={t("ai.field.knowledge_title")}
                hideLabel
                placeholder={t("ai.field.knowledge_title")}
                value={draft.title}
                onChange={(event) => setDraft({ ...draft, title: event.target.value })}
              />
              <Input
                label={t("ai.field.source_url")}
                hideLabel
                placeholder={t("ai.field.source_url")}
                value={draft.source_url}
                onChange={(event) => setDraft({ ...draft, source_url: event.target.value })}
              />
              <Textarea
                label={t("ai.field.knowledge_content")}
                hideLabel
                placeholder={t("ai.field.knowledge_content")}
                value={draft.content}
                onChange={(event) => setDraft({ ...draft, content: event.target.value })}
                rows={6}
              />
            </div>
            <Button
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
              leadingIcon={<Plus />}
            >
              {t("ai.action.add_knowledge")}
            </Button>

            <div className="divide-y divide-line max-h-96 overflow-y-auto">
              {knowledge.map((entry) => (
                <article key={entry.id} className="py-3 flex items-start gap-3">
                  <BookOpen className="w-4 h-4 mt-0.5 text-muted shrink-0" />
                  <div className="min-w-0 flex-1">
                    <h3 className="text-sm font-semibold text-foreground truncate">{entry.title}</h3>
                    <p className="text-xs text-muted line-clamp-2">{entry.content}</p>
                    <p className="text-xs text-muted">{formatMoment(entry.updated_at)}</p>
                  </div>
                  <IconButton
                    variant="ghost"
                    size="sm"
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
                    className="shrink-0 hover:text-danger"
                    icon={<Trash2 />}
                  />
                </article>
              ))}
              {loaded && knowledge.length === 0 && <p className="py-3 text-sm text-muted">{t("cp.message.no_activity")}</p>}
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
  const { t } = useI18n();
  const [content, setContent] = useState(prompt.content);
  const [active, setActive] = useState(prompt.active);

  useEffect(() => {
    setContent(prompt.content);
    setActive(prompt.active);
  }, [prompt.content, prompt.active]);

  return (
    <div>
      <div className="flex items-center justify-between gap-3 mb-1.5">
        <label htmlFor={`prompt-${prompt.key}`} className="text-sm font-semibold text-foreground font-mono">
          {prompt.key}
        </label>
        <span className="text-xs text-muted">
          {prompt.updated_at ? formatMoment(prompt.updated_at) : t("cp.state.never")}
        </span>
      </div>
      <Textarea
        id={`prompt-${prompt.key}`}
        rows={7}
        value={content}
        onChange={(event) => setContent(event.target.value)}
        className="[&_textarea]:font-mono"
      />
      <div className="mt-2 flex items-center gap-3">
        <Button disabled={!content.trim()} onClick={() => onSave(content, active)} leadingIcon={<Save />}>
          {t("base.action.save")}
        </Button>
        <Checkbox
          id={`prompt-${prompt.key}-active`}
          label={t("cp.field.active")}
          checked={active}
          onCheckedChange={(checked) => setActive(checked === true)}
        />
      </div>
    </div>
  );
}
