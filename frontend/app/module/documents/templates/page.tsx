"use client";

import React, { useRef, useState } from "react";
import { api } from "@/lib/api";
import { useLoadOnMount } from "@/lib/useResource";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { PageHeader } from "@/components/ui";
import { ActionMessage, ListEmpty, ListSkeleton, SelectField, StaleNotice } from "@/components/documents/shared";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  ConfirmationDialog,
  Input,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@gerege-systems/ui";
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
const DOC_TYPE_OPTIONS = DOC_TYPES.map((type) => ({ value: type, label: type }));

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
  const [deleting, setDeleting] = useState<Template | null>(null);
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

  // Asked in the confirmation dialog below, which calls this only on confirm.
  const handleDelete = async (tpl: Template) => {
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
        icon={<Files className="w-7 h-7 text-accent" />}
        title={t("documents.menu.templates")}
        subtitle={t("documents.view.templates_hint")}
      />

      {message && (
        <Alert
          variant={message.type === "error" ? "danger" : message.type}
          live
          dismissible
          onDismiss={() => setMessage(null)}
        >
          {message.text}
        </Alert>
      )}

      {mayManage && (
        <Card padding="sm" asChild>
          <form onSubmit={handleCreate} className="grid gap-3 md:grid-cols-4 items-end">
            <Input
              id="template-name"
              type="text"
              label={t("documents.field.template_name")}
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              required
            />
            <SelectField
              id="template-type"
              label={t("documents.field.category")}
              value={form.doc_type}
              onValueChange={(doc_type) => setForm({ ...form, doc_type })}
              options={DOC_TYPE_OPTIONS}
            />
            <Input
              id="template-pattern"
              type="text"
              label={t("documents.field.title_pattern")}
              placeholder={t("documents.field.title_pattern_placeholder")}
              value={form.title_pattern}
              onChange={(e) => setForm({ ...form, title_pattern: e.target.value })}
              required
            />
            <Button
              type="submit"
              loading={isBusy("create")}
              disabled={!form.name.trim() || !form.title_pattern.trim()}
              leadingIcon={<Plus />}
              className="w-full"
            >
              {t("documents.action.add_template")}
            </Button>
            <p className="md:col-span-4 text-xs text-muted">{t("documents.message.title_pattern_hint")}</p>
          </form>
        </Card>
      )}

      {loading ? (
        <ListSkeleton label={t("documents.message.loading")} />
      ) : templates.length === 0 ? (
        // Only a load that succeeded may say the tenant has no templates; a failed one
        // says so in the banner instead, and an operator adding one to a list the page
        // called complete would be building on a claim it could not make.
        loadFailed ? null : <ListEmpty icon={<Files />} title={t("documents.message.no_templates")} />
      ) : (
        <Card padding="none" className="overflow-hidden">
          <Table containerClassName="rounded-none border-0" className="text-xs" scrollLabel={t("documents.menu.templates")}>
            <TableHeader>
              <TableRow>
                <TableHead>{t("documents.field.template_name")}</TableHead>
                <TableHead>{t("base.field.type")}</TableHead>
                <TableHead>{t("documents.field.title_pattern")}</TableHead>
                <TableHead>{t("base.state.active")}</TableHead>
                <TableHead align="right">{t("base.field.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {templates.map((tpl) => (
                <TableRow key={tpl.id} className={tpl.active ? "" : "opacity-60"}>
                  <TableCell>
                    <Input
                      type="text"
                      size="sm"
                      label={t("documents.field.template_name")}
                      hideLabel
                      value={tpl.name}
                      disabled={!mayManage}
                      onChange={(e) => edit(tpl.id, { name: e.target.value })}
                      className="min-w-40 [&_input]:font-semibold"
                    />
                  </TableCell>
                  <TableCell>
                    <SelectField
                      label={t("base.field.type")}
                      hideLabel
                      size="sm"
                      value={tpl.doc_type}
                      disabled={!mayManage}
                      onValueChange={(doc_type) => edit(tpl.id, { doc_type })}
                      options={DOC_TYPE_OPTIONS}
                      className="min-w-[140px] font-mono"
                    />
                  </TableCell>
                  <TableCell>
                    <Input
                      type="text"
                      size="sm"
                      label={t("documents.field.title_pattern")}
                      hideLabel
                      value={tpl.title_pattern}
                      disabled={!mayManage}
                      onChange={(e) => edit(tpl.id, { title_pattern: e.target.value })}
                      className="min-w-50 [&_input]:font-mono"
                    />
                  </TableCell>
                  <TableCell>
                    <Checkbox
                      checked={tpl.active}
                      disabled={!mayManage}
                      onCheckedChange={(checked) => edit(tpl.id, { active: checked === true })}
                      aria-label={`${t("base.state.active")} — ${tpl.name}`}
                      title={t("documents.message.template_active_hint")}
                    />
                  </TableCell>
                  <TableCell align="right">
                    {mayManage ? (
                      <div className="flex items-center justify-end gap-2">
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleSave(tpl)}
                          disabled={isBusy(tpl.id) || !tpl.name.trim() || !tpl.title_pattern.trim()}
                          leadingIcon={<Save />}
                        >
                          {t("base.action.save")}
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          className="text-accent"
                          onClick={() => handleUse(tpl)}
                          disabled={isBusy(tpl.id) || !tpl.active || dirty[tpl.id]}
                          title={
                            dirty[tpl.id]
                              ? t("documents.message.template_unsaved")
                              : tpl.active
                                ? undefined
                                : t("documents.message.template_inactive")
                          }
                          leadingIcon={<Wand2 />}
                        >
                          {t("documents.action.use_template")}
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          className="text-danger"
                          onClick={() => setDeleting(tpl)}
                          disabled={isBusy(tpl.id)}
                          leadingIcon={<Trash2 />}
                        >
                          {t("base.action.delete")}
                        </Button>
                      </div>
                    ) : (
                      <span className="text-subtle text-xs">—</span>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>

          {/* A stale list says so for as long as it is stale — the banner can be
              dismissed, and a refresh that failed after an action must not be the only
              thing that says the rows are old. */}
          {loadFailed && <StaleNotice inset busy={loading} onRetry={() => loadData()} />}
        </Card>
      )}

      {deleting && (
        <ConfirmationDialog
          open
          onOpenChange={(open) => {
            if (!open) setDeleting(null);
          }}
          title={deleting.name}
          description={t("documents.message.template_delete_confirm", { name: deleting.name })}
          confirmLabel={t("base.action.delete")}
          cancelLabel={t("base.action.cancel")}
          confirmVariant="destructive"
          onConfirm={() => {
            const tpl = deleting;
            setDeleting(null);
            void handleDelete(tpl);
          }}
        />
      )}
    </div>
  );
}
