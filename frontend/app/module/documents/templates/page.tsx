"use client";

import React, { useRef, useState } from "react";
import { api } from "@/lib/api";
import { useLoadOnMount } from "@/lib/useResource";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { Banner, LoadingBlock, PageHeader, TableCard, fieldClass, rowActionClass, selectClass } from "@/components/ui";
import { ActionMessage } from "@/components/documents/shared";
import { Files, Plus, Save, Trash2, Wand2 } from "lucide-react";

interface Template {
  id: string;
  name: string;
  doc_type: string;
  title_pattern: string;
  active: boolean;
  created_at: string;
}

const DOC_TYPES = ["CONTRACT", "REQUEST", "APPROVAL"] as const;

/**
 * Document templates: the presets a document is started from. A document is a
 * title and a type, so that is what a template carries — the title may hold
 * {year}, {month} or {date}, which are filled in when the template is used.
 */
export default function DocumentTemplatesPage() {
  const { t } = useI18n();
  const { can } = useAccess();
  const mayManage = can("documents.manage");

  const [templates, setTemplates] = useState<Template[]>([]);
  const [loading, setLoading] = useState(true);
  // One id per row, not one shared string. Two handlers overlapping — Use on one row
  // while another row saves — had whichever finished first clear the flag for both, so
  // a row's Use button came back to life with its own POST still in flight and a second
  // click created a second document and routed it for approval.
  const [busy, setBusyIds] = useState<Set<string>>(new Set());
  const isBusy = (id: string) => busy.has(id);
  const setBusy = (id: string, working: boolean) =>
    setBusyIds((current) => {
      const next = new Set(current);
      if (working) next.add(id);
      else next.delete(id);
      return next;
    });
  const [message, setMessage] = useState<ActionMessage | null>(null);
  const [form, setForm] = useState({ name: "", doc_type: "CONTRACT", title_pattern: "" });

  // Rows with edits that have not been saved. The Use button acts on what the server
  // holds, so it must not be enabled by a tick the server has not seen.
  //
  // Mirrored in a ref because a load that started before an edit has to see the
  // edit when it resolves, and a closure captured at render time would not.
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const dirtyRef = useRef<Record<string, boolean>>({});
  const markDirty = (id: string, unsaved: boolean) => {
    dirtyRef.current = { ...dirtyRef.current, [id]: unsaved };
    setDirty(dirtyRef.current);
  };

  // Rows the operator has deleted. A load that started before the delete still has
  // them in its answer, and reconciling around it would put a row the operator
  // watched disappear back on the table — where Use and Save then answer 404.
  const removedRef = useRef<Set<string>>(new Set());

  // A failed load must not be reported as an empty list.
  const [loadFailed, setLoadFailed] = useState(false);

  const loadData = async () => {
    setLoading(true);
    try {
      const rows = (await api.getDocumentTemplates()) || [];
      // Reconciled, not replaced. A load that resolves after the operator has
      // created or edited a row must not throw their work away — and discarding
      // the whole response instead threw the server's OTHER rows away: a tenant
      // holding nine templates was shown the one row it had just created, with no
      // spinner and no error, as though that were the list.
      setTemplates((current) => {
        const live = rows.filter((row) => !removedRef.current.has(row.id));
        const served = new Set(live.map((row) => row.id));
        const kept = live.map((row) =>
          dirtyRef.current[row.id] ? current.find((row2) => row2.id === row.id) ?? row : row,
        );
        // A row created while this load was in flight is not in its answer yet.
        return [...kept, ...current.filter((row) => !served.has(row.id))];
      });
      setLoadFailed(false);
    } catch (err: any) {
      // Always recorded, and the footer says it for as long as it is true. The banner
      // is only used when there are no rows to carry the news — with rows showing it
      // would overwrite what the action that triggered this refresh had just reported.
      setLoadFailed(true);
      if (templates.length === 0) {
        setMessage({ type: "error", text: err?.message || t("documents.message.templates_failed") });
      }
    } finally {
      setLoading(false);
    }
  };

  useLoadOnMount(loadData);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy("create", true);
    setMessage(null);
    try {
      const created = (await api.createDocumentTemplate(form)) as Template | undefined;
      setForm({ name: "", doc_type: "CONTRACT", title_pattern: "" });
      setMessage({ type: "success", text: t("documents.message.template_saved") });
      // Appending keeps whatever the operator has typed into the other rows; a
      // reload here threw it away. A load still in flight will reconcile around
      // this row rather than overwrite it.
      if (created && created.id) {
        setTemplates((current) => [...current, created]);
      } else {
        await loadData();
      }
    } catch (err: any) {
      setMessage({ type: "error", text: err?.message || t("documents.message.templates_failed") });
    } finally {
      setBusy("create", false);
    }
  };

  const edit = (id: string, patch: Partial<Template>, saved = false) => {
    setTemplates((current) => current.map((t) => (t.id === id ? { ...t, ...patch } : t)));
    markDirty(id, !saved);
  };

  const handleSave = async (tpl: Template) => {
    setBusy(tpl.id, true);
    setMessage(null);
    try {
      const saved = (await api.updateDocumentTemplate(tpl.id, {
        name: tpl.name,
        doc_type: tpl.doc_type,
        title_pattern: tpl.title_pattern,
        active: tpl.active,
      })) as Template | undefined;
      setMessage({ type: "success", text: t("documents.message.template_saved") });
      // Only the row that was saved is replaced, with what the server stored.
      // Reloading the whole table reverted every other row the operator had typed
      // into, under a banner saying this one was saved.
      if (saved && saved.id) edit(tpl.id, saved, true);
    } catch (err: any) {
      // The draft stays on screen so the operator can fix what was refused — unless
      // the row itself is gone, which is not something they can fix by retyping.
      setMessage({ type: "error", text: err?.message || t("documents.message.templates_failed") });
      if (err?.status === 404) reconcile(tpl, err);
    } finally {
      setBusy(tpl.id, false);
    }
  };

  // What the server says about a row is applied TO that row. Reporting "this template
  // has been retired" in a banner while the row still shows an Active tick and an
  // enabled Use button leaves the screen contradicting itself in one breath — and the
  // operator clicking Use again gets the same refusal.
  const reconcile = (tpl: Template, err: any) => {
    if (err?.status === 404) {
      removedRef.current.add(tpl.id);
      setTemplates((current) => current.filter((row) => row.id !== tpl.id));
      return;
    }
    if (err?.status === 409 && tpl.active) edit(tpl.id, { active: false }, true);
  };

  const handleUse = async (tpl: Template) => {
    setBusy(tpl.id, true);
    setMessage(null);
    try {
      const doc: any = await api.useDocumentTemplate(tpl.id);
      setMessage({
        type: "success",
        text: t("documents.message.template_used", { title: doc?.title || tpl.name }),
      });
    } catch (err: any) {
      setMessage({ type: "error", text: err?.message || t("documents.message.templates_failed") });
      reconcile(tpl, err);
    } finally {
      setBusy(tpl.id, false);
    }
  };

  const handleDelete = async (tpl: Template) => {
    if (!confirm(t("documents.message.template_delete_confirm", { name: tpl.name }))) return;
    setBusy(tpl.id, true);
    setMessage(null);
    try {
      await api.deleteDocumentTemplate(tpl.id);
      setMessage({ type: "success", text: t("documents.message.template_deleted", { name: tpl.name }) });
      removedRef.current.add(tpl.id);
      setTemplates((current) => current.filter((row) => row.id !== tpl.id));
    } catch (err: any) {
      // A 404 means somebody else already deleted it, which is the outcome this click
      // was asking for. Reporting failure over a row that is genuinely gone left the
      // table asserting a template the tenant no longer holds — with its Active tick
      // and its buttons — and repeating Delete just repeated the 404.
      if (err?.status === 404) {
        setMessage({ type: "success", text: t("documents.message.template_deleted", { name: tpl.name }) });
        removedRef.current.add(tpl.id);
        setTemplates((current) => current.filter((row) => row.id !== tpl.id));
      } else {
        setMessage({ type: "error", text: err?.message || t("documents.message.templates_failed") });
      }
    } finally {
      setBusy(tpl.id, false);
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Files className="w-7 h-7 text-indigo-600" />}
        title={t("documents.menu.templates")}
        subtitle={t("documents.view.templates_hint")}
      />

      {message && <Banner tone={message.type} message={message.text} onDismiss={() => setMessage(null)} />}

      {mayManage && (
        <form onSubmit={handleCreate} className="bg-surface border border-line rounded-xl p-4 grid gap-3 md:grid-cols-4">
          <div className="md:col-span-1">
            <label className="block text-xs font-semibold text-foreground mb-1">{t("documents.field.template_name")}</label>
            <input
              type="text"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              className={fieldClass}
              required
            />
          </div>
          <div className="md:col-span-1">
            <label className="block text-xs font-semibold text-foreground mb-1">{t("documents.field.category")}</label>
            <select
              value={form.doc_type}
              onChange={(e) => setForm({ ...form, doc_type: e.target.value })}
              className={selectClass}
            >
              {DOC_TYPES.map((type) => (
                <option key={type} value={type}>
                  {type}
                </option>
              ))}
            </select>
          </div>
          <div className="md:col-span-1">
            <label className="block text-xs font-semibold text-foreground mb-1">
              {t("documents.field.title_pattern")}
            </label>
            <input
              type="text"
              placeholder={t("documents.field.title_pattern_placeholder")}
              value={form.title_pattern}
              onChange={(e) => setForm({ ...form, title_pattern: e.target.value })}
              className={fieldClass}
              required
            />
          </div>
          <div className="md:col-span-1 flex items-end">
            <button
              type="submit"
              disabled={isBusy("create") || !form.name.trim() || !form.title_pattern.trim()}
              className="w-full bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-semibold px-4 py-2 rounded-lg flex items-center justify-center space-x-2 disabled:opacity-50"
            >
              <Plus className="w-4 h-4" />
              <span>{t("documents.action.add_template")}</span>
            </button>
          </div>
          <p className="md:col-span-4 text-[11px] text-muted">{t("documents.message.title_pattern_hint")}</p>
        </form>
      )}

      {loading ? (
        <LoadingBlock label={t("documents.message.loading")} />
      ) : templates.length === 0 ? (
        // Only a load that succeeded may say the tenant has no templates; a failed one
        // says so in the banner instead, and an operator adding one to a list the page
        // called complete would be building on a claim it could not make.
        loadFailed ? null : (
          <div className="bg-surface border border-line rounded-xl p-8 text-center text-muted text-sm">
            {t("documents.message.no_templates")}
          </div>
        )
      ) : (
        <TableCard
          head={
            <tr>
              <th className="px-4 py-3">{t("documents.field.template_name")}</th>
              <th className="px-4 py-3">{t("base.field.type")}</th>
              <th className="px-4 py-3">{t("documents.field.title_pattern")}</th>
              <th className="px-4 py-3">{t("base.state.active")}</th>
              <th className="px-4 py-3 text-right">{t("base.field.actions")}</th>
            </tr>
          }
          footer={
            <>
            {/* A stale list says so for as long as it is stale — the banner can be
                dismissed, and a refresh that failed after an action must not be the only
                thing that says the rows are old. */}
            {loadFailed && (
              <div className="flex items-center justify-between gap-3 px-4 py-3 border-t border-amber-200 bg-amber-50">
                <p className="text-[11px] text-amber-800">{t("documents.message.stale_rows")}</p>
                <button
                  type="button"
                  disabled={loading}
                  onClick={() => loadData()}
                  className="text-[11px] font-semibold px-3 py-1.5 rounded-lg bg-surface border border-amber-300 text-amber-800 hover:bg-amber-100 disabled:opacity-50"
                >
                  {t("documents.action.retry")}
                </button>
              </div>
            )}
            </>
          }
        >
          {templates.map((tpl) => (
            <tr key={tpl.id} className={`hover:bg-surface-hover ${tpl.active ? "" : "opacity-60"}`}>
              <td className="px-4 py-3">
                <input
                  type="text"
                  value={tpl.name}
                  disabled={!mayManage}
                  onChange={(e) => edit(tpl.id, { name: e.target.value })}
                  className="w-full px-2 py-1.5 text-xs font-semibold border border-input rounded-lg focus:ring-2 focus:ring-indigo-500 disabled:border-transparent disabled:bg-transparent"
                />
              </td>
              <td className="px-4 py-3">
                <select
                  value={tpl.doc_type}
                  disabled={!mayManage}
                  onChange={(e) => edit(tpl.id, { doc_type: e.target.value })}
                  className="h-[30px] px-2 py-1.5 text-xs font-mono border border-input rounded-lg bg-surface focus:ring-2 focus:ring-indigo-500 disabled:border-transparent disabled:bg-transparent"
                >
                  {DOC_TYPES.map((type) => (
                    <option key={type} value={type}>
                      {type}
                    </option>
                  ))}
                </select>
              </td>
              <td className="px-4 py-3">
                <input
                  type="text"
                  value={tpl.title_pattern}
                  disabled={!mayManage}
                  onChange={(e) => edit(tpl.id, { title_pattern: e.target.value })}
                  className="w-full px-2 py-1.5 text-xs font-mono border border-input rounded-lg focus:ring-2 focus:ring-indigo-500 disabled:border-transparent disabled:bg-transparent"
                />
              </td>
              <td className="px-4 py-3">
                <input
                  type="checkbox"
                  checked={tpl.active}
                  disabled={!mayManage}
                  onChange={(e) => edit(tpl.id, { active: e.target.checked })}
                  title={t("documents.message.template_active_hint")}
                  className="w-4 h-4 accent-indigo-600"
                />
              </td>
              <td className="px-4 py-3 text-right">
                {mayManage ? (
                  <div className="flex items-center justify-end space-x-2">
                    <button
                      onClick={() => handleSave(tpl)}
                      disabled={isBusy(tpl.id) || !tpl.name.trim() || !tpl.title_pattern.trim()}
                      className="inline-flex items-center space-x-1 px-2.5 py-1.5 rounded-lg text-[11px] font-semibold border border-input text-foreground hover:bg-surface-hover disabled:opacity-50"
                    >
                      <Save className="w-3.5 h-3.5" />
                      <span>{t("base.action.save")}</span>
                    </button>
                    <button
                      onClick={() => handleUse(tpl)}
                      disabled={isBusy(tpl.id) || !tpl.active || dirty[tpl.id]}
                      title={
                        dirty[tpl.id]
                          ? t("documents.message.template_unsaved")
                          : tpl.active
                            ? undefined
                            : t("documents.message.template_inactive")
                      }
                      className={rowActionClass}
                    >
                      <Wand2 className="w-3.5 h-3.5" />
                      <span>{t("documents.action.use_template")}</span>
                    </button>
                    <button
                      onClick={() => handleDelete(tpl)}
                      disabled={isBusy(tpl.id)}
                      className="inline-flex items-center space-x-1 px-2.5 py-1.5 rounded-lg text-[11px] font-semibold border border-red-200 text-red-600 hover:bg-red-50 disabled:opacity-50"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                      <span>{t("base.action.delete")}</span>
                    </button>
                  </div>
                ) : (
                  <span className="text-subtle text-[11px]">—</span>
                )}
              </td>
            </tr>
          ))}
        </TableCard>
      )}
    </div>
  );
}
