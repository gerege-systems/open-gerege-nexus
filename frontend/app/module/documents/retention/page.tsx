"use client";

import React, { useState } from "react";
import { api } from "@/lib/api";
import { useResource } from "@/lib/useResource";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { Banner, LoadingBlock, PageHeader, TableCard, rowActionClass } from "@/components/ui";
import { ActionMessage } from "@/components/documents/shared";
import { Archive, Save } from "lucide-react";

interface Rule {
  doc_type: string;
  retain_years: number;
  note: string;
  configured: boolean;
  updated_at?: string;
  /**
   * Absent when the server could not count — the save path treats a failed count as
   * non-fatal — so the row keeps whatever it already knew rather than claiming the
   * type has nothing filed under it.
   */
  expired?: number;
  total?: number;
}

/**
 * Retention rules: how long each document type is kept. Nothing is deleted on
 * this schedule — the screen reports what is past its term and a person decides,
 * which is the only safe reading of a rule nobody has reviewed yet.
 */
export default function DocumentRetentionPage() {
  const { t } = useI18n();
  const { can } = useAccess();
  const mayManage = can("documents.manage");

  const [busy, setBusy] = useState<string | null>(null);
  const [message, setMessage] = useState<ActionMessage | null>(null);

  // A failed load left the three tiles saying "0 filed · 0 past their term · 0 rules
  // set" — a claim about the tenant, made out of rows the page never received. That
  // is what `failed` is read for below.
  const {
    data: rules,
    loading,
    failed: loadFailed,
    setData: setRules,
  } = useResource(async () => (await api.getRetentionRules()) || [], {
    initial: [] as Rule[],
    onError: (err: any) =>
      setMessage({ type: "error", text: err?.message || t("documents.message.retention_failed") }),
  });

  const edit = (docType: string, patch: Partial<Rule>) =>
    setRules((current) => current.map((r) => (r.doc_type === docType ? { ...r, ...patch } : r)));

  const save = async (rule: Rule) => {
    setBusy(rule.doc_type);
    setMessage(null);
    try {
      // The saved rule comes back with the counts the server computed, so only this
      // row needs replacing. Reloading the table wiped the years an operator had
      // typed into the other rows — and, on the error path, the very value they were
      // being asked to correct.
      const saved = (await api.saveRetentionRule(rule.doc_type, {
        retain_years: rule.retain_years,
        note: rule.note,
      })) as Rule | undefined;
      setMessage({ type: "success", text: t("documents.message.retention_saved", { type: rule.doc_type }) });
      if (saved && saved.doc_type) {
        // `total` counts every document of the type, so a save cannot change it and
        // the row's own is still good if the server sent none.
        //
        // `expired` is counted AGAINST THE TERM, which is exactly what this save
        // changed — so the row's own belongs to a term that no longer applies, and
        // the direction is always the dangerous one: shortening a term can only
        // increase what is past it, and shortening is the common edit. Left absent,
        // the cell reads "—" instead of stating a stale zero over a type where
        // hundreds are now past their term.
        //
        // `expired` is NAMED in the patch even though it may be undefined: the server
        // omits the key entirely when it could not count, and `edit` merges, so a key
        // that is merely absent leaves the row's old number sitting there. Naming it
        // is what actually clears it.
        edit(rule.doc_type, {
          ...saved,
          expired: saved.expired,
          total: saved.total ?? rule.total,
        });
      }
    } catch (err: any) {
      setMessage({ type: "error", text: err?.message || t("documents.message.retention_failed") });
    } finally {
      setBusy(null);
    }
  };

  // A tile that adds up rows the server could not count states a total it does not
  // know. It is prefixed with "≥" then, because the number can only be short.
  const expiredTotal = rules.reduce((sum, rule) => sum + (rule.expired ?? 0), 0);
  const filedTotal = rules.reduce((sum, rule) => sum + (rule.total ?? 0), 0);
  const expiredPartial = rules.some((rule) => rule.expired === undefined);
  const filedPartial = rules.some((rule) => rule.total === undefined);
  const atLeast = (value: number, partial: boolean) =>
    loadFailed ? "—" : partial ? `≥${value}` : `${value}`;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Archive className="w-7 h-7 text-indigo-600" />}
        title={t("documents.menu.retention")}
        subtitle={t("documents.view.retention_hint")}
      />

      {message && <Banner tone={message.type} message={message.text} onDismiss={() => setMessage(null)} />}

      <section className="grid grid-cols-2 md:grid-cols-3 gap-3">
        <div className="p-4 bg-surface border border-line rounded-xl">
          <div className="text-2xl font-semibold text-foreground">{atLeast(filedTotal, filedPartial)}</div>
          <div className="text-[11px] text-muted leading-snug mt-1">{t("documents.stat.filed")}</div>
        </div>
        <div className="p-4 bg-surface border border-line rounded-xl">
          <div className={`text-2xl font-semibold ${expiredTotal > 0 ? "text-amber-600" : "text-foreground"}`}>
            {atLeast(expiredTotal, expiredPartial)}
          </div>
          <div className="text-[11px] text-muted leading-snug mt-1">{t("documents.stat.past_term")}</div>
        </div>
        <div className="p-4 bg-surface border border-line rounded-xl">
          <div className="text-2xl font-semibold text-indigo-600">
            {loadFailed ? "—" : rules.filter((r) => r.configured).length}
          </div>
          <div className="text-[11px] text-muted leading-snug mt-1">{t("documents.stat.rules_set")}</div>
        </div>
      </section>

      {loading ? (
        <LoadingBlock label={t("documents.message.loading")} />
      ) : loadFailed ? (
        // The server answers with a row for every document type, so an empty table
        // could only mean the load failed — and rendering the headers over nothing
        // reads as "this tenant has no document types".
        <div className="bg-surface border border-line rounded-xl p-8 text-center text-muted text-sm">
          {t("documents.message.retention_failed")}
        </div>
      ) : (
        <TableCard
          head={
            <tr>
              <th className="px-4 py-3">{t("base.field.type")}</th>
              <th className="px-4 py-3">{t("documents.field.retain_years")}</th>
              <th className="px-4 py-3">{t("documents.field.retention_note")}</th>
              <th className="px-4 py-3">{t("documents.stat.filed")}</th>
              <th className="px-4 py-3">{t("documents.stat.past_term")}</th>
              <th className="px-4 py-3 text-right">{t("base.field.actions")}</th>
            </tr>
          }
        >
          {rules.map((rule) => (
            <tr key={rule.doc_type} className="hover:bg-surface-hover">
              <td className="px-4 py-3 font-mono font-semibold text-foreground">{rule.doc_type}</td>
              <td className="px-4 py-3">
                <input
                  type="number"
                  min={1}
                  max={100}
                  value={rule.retain_years || ""}
                  disabled={!mayManage}
                  onChange={(e) => edit(rule.doc_type, { retain_years: Number(e.target.value) })}
                  className="w-20 px-2 py-1.5 text-xs border border-input rounded-lg focus:ring-2 focus:ring-indigo-500"
                />
              </td>
              <td className="px-4 py-3">
                <input
                  type="text"
                  value={rule.note}
                  disabled={!mayManage}
                  onChange={(e) => edit(rule.doc_type, { note: e.target.value })}
                  className="w-full px-2 py-1.5 text-xs border border-input rounded-lg focus:ring-2 focus:ring-indigo-500"
                />
              </td>
              <td className="px-4 py-3 text-muted">{rule.total ?? "—"}</td>
              <td className="px-4 py-3">
                {rule.expired === undefined ? (
                  <span className="text-muted">—</span>
                ) : rule.expired > 0 ? (
                  <span className="text-[11px] font-semibold px-2 py-0.5 rounded-full bg-amber-50 text-amber-700 border border-amber-200">
                    {rule.expired}
                  </span>
                ) : (
                  <span className="text-muted">0</span>
                )}
              </td>
              <td className="px-4 py-3 text-right">
                {mayManage ? (
                  <button
                    onClick={() => save(rule)}
                    disabled={busy === rule.doc_type || !rule.retain_years}
                    className={rowActionClass}
                  >
                    <Save className="w-3.5 h-3.5" />
                    <span>{t("base.action.save")}</span>
                  </button>
                ) : (
                  <span className="text-subtle text-[11px]">—</span>
                )}
              </td>
            </tr>
          ))}
        </TableCard>
      )}

      <p className="text-xs text-muted">{t("documents.message.retention_no_deletion")}</p>
    </div>
  );
}
