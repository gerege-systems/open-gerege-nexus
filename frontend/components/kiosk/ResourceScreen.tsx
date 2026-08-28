"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import { kioskList, kioskCreate, kioskUpdate, kioskRemove } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import type { KioskResource } from "@/lib/kiosk/types";
import { Plus, Pencil, Trash2, X, ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight, RefreshCw } from "lucide-react";
import { MediaCell, MediaViewer } from "./MediaCell";

const PAGE_SIZES = [25, 50, 100, 200, 500];
const DEFAULT_PAGE_SIZE = 50;

/**
 * The screen every declared kiosk resource renders through: list, create, edit
 * and delete, driven entirely by the resource definition.
 *
 * Errors are shown rather than swallowed. Most kiosk endpoints proxy other
 * Gerege services, so "cannot reach Core" is a common and very different thing
 * from "no records" — an empty table for both would be a lie.
 */
export default function ResourceScreen({ resource }: { resource: KioskResource }) {
  const { t } = useI18n();
  const idKey = resource.idKey ?? "id";
  const base = resource.app ?? "kiosk";

  const [rows, setRows] = useState<Record<string, any>[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
  // False when the endpoint answered with the whole table; the slice is then
  // ours to take rather than the server's.
  const [serverPaged, setServerPaged] = useState(true);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const [editing, setEditing] = useState<Record<string, any> | null>(null);
  const [isNew, setIsNew] = useState(false);
  const [form, setForm] = useState<Record<string, any>>({});
  const [formError, setFormError] = useState("");
  const [confirming, setConfirming] = useState<Record<string, any> | null>(null);
  const [viewing, setViewing] = useState<{ url: string; name?: string } | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      // Sent unconditionally. Every module built on the go_grc paginator
      // defaults to 50 whether or not it is asked, so a screen that stayed
      // silent got a silent truncation — 837 terminals reported and 50 shown,
      // with no way to reach the rest. The endpoints that ignore these
      // parameters are unaffected by them.
      const params: Record<string, string | number> = {
        ...(resource.listParams ?? {}),
        page_size: pageSize,
        page_number: page,
      };
      const res = await kioskList(resource.list, params, base);
      setRows(res.items || []);
      setTotal(res.total || 0);
      setServerPaged(res.serverPaged);
    } catch (err: any) {
      setError(err?.message || t("kiosk.message.load_failed"));
      setRows([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [resource, base, page, pageSize, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const openCreate = () => {
    const blank: Record<string, any> = {};
    for (const f of resource.fields ?? []) {
      if (f.readOnly) continue;
      blank[f.key] = f.type === "boolean" ? false : "";
    }
    setForm(blank);
    setEditing({});
    setIsNew(true);
    setFormError("");
  };

  const openEdit = (row: Record<string, any>) => {
    const filled: Record<string, any> = {};
    for (const f of resource.fields ?? []) {
      const v = row[f.key];
      filled[f.key] = f.type === "json" && v && typeof v === "object" ? JSON.stringify(v, null, 2) : v ?? "";
    }
    filled[idKey] = row[idKey];
    setForm(filled);
    setEditing(row);
    setIsNew(false);
    setFormError("");
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setFormError("");
    try {
      const payload: Record<string, any> = {};
      for (const f of resource.fields ?? []) {
        if (f.readOnly) continue;
        let v = form[f.key];
        if (f.type === "number") v = v === "" || v === null ? undefined : Number(v);
        if (f.type === "json" && typeof v === "string" && v.trim() !== "") {
          // Rejected here rather than sent, so a typo reads as a form error and
          // not as an opaque failure from the API.
          try {
            v = JSON.parse(v);
          } catch {
            throw new Error(`${t(f.label)}: ${t("kiosk.message.invalid_json")}`);
          }
        }
        if (v !== undefined) payload[f.key] = v;
      }
      if (!isNew) payload[idKey] = form[idKey];

      if (isNew) await kioskCreate(resource.create!, payload, base);
      else await kioskUpdate(resource.update!, payload, base);

      setEditing(null);
      await load();
    } catch (err: any) {
      setFormError(err?.message || t("kiosk.message.save_failed"));
    } finally {
      setBusy(false);
    }
  };

  const doRemove = async () => {
    if (!confirming) return;
    setBusy(true);
    try {
      await kioskRemove(resource.remove!, confirming[idKey], idKey, base);
      setConfirming(null);
      await load();
    } catch (err: any) {
      setError(err?.message || t("kiosk.message.delete_failed"));
      setConfirming(null);
    } finally {
      setBusy(false);
    }
  };

  const pages = Math.max(1, Math.ceil(total / pageSize));
  // A server-paged response is already the page. A bare array is the whole
  // table, so the slice happens here.
  const visible = serverPaged ? rows : rows.slice((page - 1) * pageSize, page * pageSize);
  const firstShown = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const lastShown = Math.min(page * pageSize, total);

  const canWrite = Boolean(resource.create || resource.update || resource.remove);
  const actionCount = useMemo(
    () => (resource.update ? 1 : 0) + (resource.remove ? 1 : 0),
    [resource.update, resource.remove],
  );

  return (
    // The AI Copilot is a fixed button in the bottom-right corner, and the
    // pager is the last thing on the page — scrolled to the end, the two landed
    // on top of each other and the page buttons could not be clicked. The
    // padding keeps the end of the content clear of it.
    <div className="space-y-6 pb-24">
      <div className="flex items-start justify-between border-b border-slate-200 pb-4 gap-4">
        <div className="min-w-0">
          <h1 className="text-2xl font-bold text-slate-900">{t(resource.title)}</h1>
          <p className="text-sm text-slate-500">{t(resource.description)}</p>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <button
            onClick={() => void load()}
            className="border border-slate-300 text-slate-700 hover:bg-slate-50 font-medium text-sm py-2 px-3 rounded-lg flex items-center gap-2"
            aria-label={t("kiosk.action.refresh")}
          >
            <RefreshCw className="w-4 h-4" />
          </button>
          {resource.create && (
            <button
              onClick={openCreate}
              className="bg-indigo-600 hover:bg-indigo-700 text-white font-medium text-sm py-2 px-4 rounded-lg flex items-center gap-2"
            >
              <Plus className="w-4 h-4" />
              <span>{t("kiosk.action.create")}</span>
            </button>
          )}
        </div>
      </div>

      {!canWrite && resource.readOnlyReason && (
        <div className="p-3 bg-slate-50 border border-slate-200 text-slate-600 text-sm rounded-lg">
          {t(resource.readOnlyReason)}
        </div>
      )}

      {error && (
        <div className="p-4 bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg break-words">{error}</div>
      )}

      {loading ? (
        <div className="py-8 text-slate-500 text-sm">{t("kiosk.message.loading")}</div>
      ) : rows.length === 0 && !error ? (
        <div className="bg-white border border-slate-200 rounded-xl p-12 text-center text-slate-500 text-sm">
          {t("kiosk.message.empty")}
        </div>
      ) : rows.length > 0 ? (
        <>
          <div className="bg-white border border-slate-200 rounded-xl overflow-x-auto shadow-sm">
            <table className="w-full text-left border-collapse text-sm">
              <thead>
                <tr className="bg-slate-50 border-b border-slate-200 text-slate-600 font-semibold text-xs uppercase tracking-wider">
                  {resource.media && <th className="py-3 px-4 w-px">{t("kiosk.field.preview")}</th>}
                  {resource.columns.map((c) => (
                    <th key={c.key} className="py-3 px-4 whitespace-nowrap">{t(c.label)}</th>
                  ))}
                  {actionCount > 0 && <th className="py-3 px-4 w-px" />}
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {visible.map((row, i) => (
                  <tr key={String(row[idKey] ?? i)} className="hover:bg-slate-50">
                    {resource.media && (
                      <td className="py-2 px-4">
                        <MediaCell
                          url={String(row[resource.media.urlKey] ?? "")}
                          name={resource.media.nameKey ? String(row[resource.media.nameKey] ?? "") : undefined}
                          onOpen={() =>
                            setViewing({
                              url: String(row[resource.media!.urlKey] ?? ""),
                              name: resource.media!.nameKey ? String(row[resource.media!.nameKey] ?? "") : undefined,
                            })
                          }
                        />
                      </td>
                    )}
                    {resource.columns.map((c) => (
                      <td key={c.key} className="py-3.5 px-4 text-slate-700 max-w-xs truncate">
                        {c.render ? c.render(row) : format(row[c.key])}
                      </td>
                    ))}
                    {actionCount > 0 && (
                      <td className="py-3.5 px-4">
                        <div className="flex items-center gap-1 justify-end">
                          {resource.update && (
                            <button
                              onClick={() => openEdit(row)}
                              className="p-1.5 rounded-lg text-slate-500 hover:bg-slate-100 hover:text-slate-800"
                              aria-label={t("kiosk.action.edit")}
                            >
                              <Pencil className="w-4 h-4" />
                            </button>
                          )}
                          {resource.remove && (
                            <button
                              onClick={() => setConfirming(row)}
                              className="p-1.5 rounded-lg text-slate-500 hover:bg-red-50 hover:text-red-600"
                              aria-label={t("kiosk.action.delete")}
                            >
                              <Trash2 className="w-4 h-4" />
                            </button>
                          )}
                        </div>
                      </td>
                    )}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="flex flex-wrap items-center justify-between gap-3 text-sm text-slate-600">
            <div className="flex items-center gap-3">
              <span>
                {firstShown.toLocaleString()}–{lastShown.toLocaleString()} / {total.toLocaleString()}
              </span>
              <label className="flex items-center gap-1.5">
                <span className="text-slate-500">{t("kiosk.label.per_page")}</span>
                <select
                  value={pageSize}
                  onChange={(e) => { setPageSize(Number(e.target.value)); setPage(1); }}
                  className="border border-slate-300 rounded-lg px-2 py-1 text-sm bg-white"
                >
                  {PAGE_SIZES.map((n) => <option key={n} value={n}>{n}</option>)}
                </select>
              </label>
            </div>
            {pages > 1 && (
              <div className="flex items-center gap-1.5">
                <button
                  disabled={page <= 1}
                  onClick={() => setPage(1)}
                  className="p-1.5 rounded-lg border border-slate-300 disabled:opacity-40 hover:bg-slate-50"
                  aria-label={t("kiosk.action.first_page")}
                >
                  <ChevronsLeft className="w-4 h-4" />
                </button>
                <button
                  disabled={page <= 1}
                  onClick={() => setPage((p) => p - 1)}
                  className="p-1.5 rounded-lg border border-slate-300 disabled:opacity-40 hover:bg-slate-50"
                  aria-label={t("kiosk.action.prev_page")}
                >
                  <ChevronLeft className="w-4 h-4" />
                </button>
                <span className="px-1 tabular-nums">{page} / {pages.toLocaleString()}</span>
                <button
                  disabled={page >= pages}
                  onClick={() => setPage((p) => p + 1)}
                  className="p-1.5 rounded-lg border border-slate-300 disabled:opacity-40 hover:bg-slate-50"
                  aria-label={t("kiosk.action.next_page")}
                >
                  <ChevronRight className="w-4 h-4" />
                </button>
                <button
                  disabled={page >= pages}
                  onClick={() => setPage(pages)}
                  className="p-1.5 rounded-lg border border-slate-300 disabled:opacity-40 hover:bg-slate-50"
                  aria-label={t("kiosk.action.last_page")}
                >
                  <ChevronsRight className="w-4 h-4" />
                </button>
              </div>
            )}
          </div>
        </>
      ) : null}

      {editing && resource.fields && (
        <div className="fixed inset-0 bg-slate-900/50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-xl max-w-lg w-full shadow-xl border border-slate-200 max-h-[90vh] flex flex-col">
            <div className="flex items-center justify-between p-6 pb-4">
              <h2 className="text-xl font-bold text-slate-900">
                {isNew ? t("kiosk.action.create") : t("kiosk.action.edit")} — {t(resource.title)}
              </h2>
              <button onClick={() => setEditing(null)} aria-label={t("kiosk.action.cancel")}>
                <X className="w-5 h-5 text-slate-400 hover:text-slate-700" />
              </button>
            </div>
            <form onSubmit={submit} className="px-6 pb-6 space-y-4 overflow-y-auto">
              {formError && (
                <div className="p-3 bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg break-words">
                  {formError}
                </div>
              )}
              {resource.fields.filter((f) => !f.readOnly).map((f) => (
                <div key={f.key}>
                  <label className="block text-xs font-semibold text-slate-700 mb-1">
                    {t(f.label)}{f.required && " *"}
                  </label>
                  {f.type === "boolean" ? (
                    <input
                      type="checkbox"
                      checked={Boolean(form[f.key])}
                      onChange={(e) => setForm({ ...form, [f.key]: e.target.checked })}
                      className="w-4 h-4"
                    />
                  ) : f.type === "textarea" || f.type === "json" ? (
                    <textarea
                      value={form[f.key] ?? ""}
                      onChange={(e) => setForm({ ...form, [f.key]: e.target.value })}
                      required={f.required}
                      rows={f.type === "json" ? 6 : 3}
                      placeholder={f.placeholder}
                      className="w-full px-3 py-2 text-sm border border-slate-300 rounded-lg focus:ring-2 focus:ring-indigo-500 font-mono"
                    />
                  ) : (
                    <input
                      type={f.type === "number" ? "number" : f.type === "date" ? "date" : "text"}
                      value={form[f.key] ?? ""}
                      onChange={(e) => setForm({ ...form, [f.key]: e.target.value })}
                      required={f.required}
                      placeholder={f.placeholder}
                      className="w-full px-3 py-2 text-sm border border-slate-300 rounded-lg focus:ring-2 focus:ring-indigo-500"
                    />
                  )}
                </div>
              ))}
              <div className="flex items-center gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setEditing(null)}
                  className="w-1/2 bg-slate-100 hover:bg-slate-200 text-slate-700 font-medium py-2 rounded-lg text-sm"
                >
                  {t("kiosk.action.cancel")}
                </button>
                <button
                  type="submit"
                  disabled={busy}
                  className="w-1/2 bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white font-medium py-2 rounded-lg text-sm"
                >
                  {busy ? t("kiosk.message.saving") : t("kiosk.action.save")}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {viewing && <MediaViewer url={viewing.url} name={viewing.name} onClose={() => setViewing(null)} />}

      {confirming && (
        <div className="fixed inset-0 bg-slate-900/50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-xl max-w-sm w-full p-6 shadow-xl border border-slate-200">
            <h2 className="text-lg font-bold text-slate-900 mb-2">{t("kiosk.action.delete")}</h2>
            <p className="text-sm text-slate-600 mb-5">{t("kiosk.message.confirm_delete")}</p>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setConfirming(null)}
                className="w-1/2 bg-slate-100 hover:bg-slate-200 text-slate-700 font-medium py-2 rounded-lg text-sm"
              >
                {t("kiosk.action.cancel")}
              </button>
              <button
                onClick={() => void doRemove()}
                disabled={busy}
                className="w-1/2 bg-red-600 hover:bg-red-700 disabled:opacity-50 text-white font-medium py-2 rounded-lg text-sm"
              >
                {t("kiosk.action.delete")}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function format(value: unknown): React.ReactNode {
  if (value === null || value === undefined || value === "") return "—";
  if (typeof value === "boolean") return value ? "✓" : "—";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}
