"use client";

/**
 * The append-only operator ledger.
 *
 * It reads an API that has existed since CP-1. The vertical rule is not
 * decoration: it makes the ordering and the fact that entries only accumulate
 * legible before somebody opens the before/after payloads.
 */

import React, { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Filter, RefreshCw, ScrollText } from "lucide-react";

import { Badge, formatMoment } from "@/components/cp/ui";
import { cp, type AuditEntry } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

interface AuditFilters {
  action: string;
  target_type: string;
  target_id: string;
}

const EMPTY_FILTERS: AuditFilters = { action: "", target_type: "", target_id: "" };

export default function AuditTrail() {
  const { t, locale } = useI18n();
  const [entries, setEntries] = useState<AuditEntry[] | null>(null);
  const [draft, setDraft] = useState<AuditFilters>(EMPTY_FILTERS);
  const [filters, setFilters] = useState<AuditFilters>(EMPTY_FILTERS);
  const [failure, setFailure] = useState("");
  const [refreshing, setRefreshing] = useState(false);

  const load = useCallback(async () => {
    setRefreshing(true);
    try {
      setEntries((await cp.audit(filters)).entries);
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setRefreshing(false);
    }
  }, [filters]);

  useEffect(() => {
    void load();
  }, [load]);

  function submit(event: React.FormEvent) {
    event.preventDefault();
    setFilters(trimmed(draft));
  }

  function clear() {
    setDraft(EMPTY_FILTERS);
    setFilters(EMPTY_FILTERS);
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start gap-3">
        <div className="flex-1">
          <div className="flex items-center gap-2 text-amber-600">
            <ScrollText className="h-4 w-4" />
            <span className="text-xs font-bold uppercase tracking-[0.18em]">
              {t("cp.audit.append_only")}
            </span>
          </div>
          <h1 className="mt-2 text-2xl font-semibold text-slate-900">{t("cp.section.audit")}</h1>
          <p className="mt-1 max-w-2xl text-sm text-slate-500">{t("cp.audit.hint")}</p>
        </div>
        <button
          type="button"
          onClick={() => void load()}
          disabled={refreshing}
          className="inline-flex items-center gap-2 rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:bg-slate-50 disabled:opacity-60"
        >
          <RefreshCw className={`h-4 w-4 ${refreshing ? "animate-spin" : ""}`} />
          {t("cp.action.refresh")}
        </button>
      </div>

      <form
        onSubmit={submit}
        className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm"
      >
        <div className="mb-3 flex items-center gap-2 text-sm font-medium text-slate-900">
          <Filter className="h-4 w-4 text-slate-500" />
          {t("cp.audit.filter")}
        </div>
        <div className="grid gap-3 md:grid-cols-3">
          <FilterField
            label={t("cp.field.action")}
            placeholder="tenant.suspend"
            value={draft.action}
            onChange={(action) => setDraft((current) => ({ ...current, action }))}
          />
          <FilterField
            label={t("cp.field.target_type")}
            placeholder="tenant"
            value={draft.target_type}
            onChange={(target_type) => setDraft((current) => ({ ...current, target_type }))}
          />
          <FilterField
            label={t("cp.field.target_id")}
            placeholder="UUID"
            value={draft.target_id}
            onChange={(target_id) => setDraft((current) => ({ ...current, target_id }))}
          />
        </div>
        <div className="mt-4 flex gap-2">
          <button
            type="submit"
            className="rounded-lg bg-[var(--gerege-blue)] px-3 py-2 text-sm font-medium text-[var(--gerege-on-blue)] transition hover:brightness-105"
          >
            {t("cp.action.search")}
          </button>
          <button
            type="button"
            onClick={clear}
            className="rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 transition hover:bg-slate-50"
          >
            {t("cp.action.clear")}
          </button>
        </div>
      </form>

      {failure && (
        <p className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          {t("cp.message.load_failed")} {failure}
        </p>
      )}

      {entries === null && !failure && <div className="py-10 text-center text-slate-500">…</div>}

      {entries?.length === 0 && (
        <div className="rounded-xl border border-dashed border-slate-300 bg-white px-4 py-12 text-center">
          <ScrollText className="mx-auto h-6 w-6 text-slate-300" />
          <p className="mt-3 text-sm text-slate-500">{t("cp.audit.empty")}</p>
        </div>
      )}

      {entries && entries.length > 0 && (
        <ol className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
          {entries.map((entry) => (
            <AuditRow key={entry.id} entry={entry} locale={locale} />
          ))}
        </ol>
      )}
    </div>
  );
}

function FilterField({
  label,
  placeholder,
  value,
  onChange,
}: {
  label: string;
  placeholder: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="block text-sm">
      <span className="text-slate-600">{label}</span>
      <input
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 font-mono text-sm focus:border-slate-500 focus:outline-none focus:ring-2 focus:ring-slate-900/10"
      />
    </label>
  );
}

function AuditRow({ entry, locale }: { entry: AuditEntry; locale: string }) {
  const { t } = useI18n();
  const changed = hasValue(entry.before) || hasValue(entry.after);

  return (
    <li className="grid border-b border-slate-100 last:border-b-0 md:grid-cols-[11rem_minmax(0,1fr)]">
      <div className="bg-slate-50 px-4 py-4 text-xs text-slate-500 md:text-right">
        <time dateTime={entry.created_at} className="tabular-nums">
          {formatMoment(entry.created_at, locale)}
        </time>
        <span className="mt-1 block font-mono text-[11px] text-slate-400">{entry.ip || "—"}</span>
      </div>

      <div className="relative px-5 py-4 before:absolute before:inset-y-0 before:left-0 before:w-1 before:bg-amber-400">
        <div className="flex flex-wrap items-center gap-2">
          <strong className="font-mono text-sm text-slate-900">{entry.action}</strong>
          <Badge tone="slate">{entry.target_type}</Badge>
          <Target entry={entry} />
        </div>

        <p className="mt-2 text-sm leading-6 text-slate-700">{entry.reason || "—"}</p>
        <p className="mt-2 text-xs text-slate-500">
          <span className="font-medium text-slate-700">{entry.operator_email}</span>
          <span aria-hidden="true"> · </span>
          <span className="font-mono">{entry.operator_id}</span>
        </p>

        {changed && (
          <details className="mt-3 rounded-lg border border-slate-200 bg-slate-50 open:bg-white">
            <summary className="cursor-pointer px-3 py-2 text-xs font-medium text-slate-600 marker:text-amber-500">
              {t("cp.audit.change")}
            </summary>
            <div className="grid gap-px border-t border-slate-200 bg-slate-200 lg:grid-cols-2">
              <Snapshot label={t("cp.audit.before")} value={entry.before} />
              <Snapshot label={t("cp.audit.after")} value={entry.after} />
            </div>
          </details>
        )}
      </div>
    </li>
  );
}

function Target({ entry }: { entry: AuditEntry }) {
  const value = entry.target_id || "—";
  if (entry.target_type === "tenant" && entry.target_id) {
    return (
      <Link
        href={`/cp/tenants/${encodeURIComponent(entry.target_id)}`}
        className="max-w-full truncate font-mono text-xs text-slate-600 underline decoration-slate-300 underline-offset-2 hover:text-slate-900"
      >
        {value}
      </Link>
    );
  }
  return <span className="max-w-full truncate font-mono text-xs text-slate-500">{value}</span>;
}

function Snapshot({ label, value }: { label: string; value: unknown }) {
  return (
    <div className="min-w-0 bg-white p-3">
      <p className="mb-2 text-[11px] font-bold uppercase tracking-wider text-slate-400">{label}</p>
      <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-words text-xs leading-5 text-slate-700">
        {hasValue(value) ? JSON.stringify(value, null, 2) : "—"}
      </pre>
    </div>
  );
}

function hasValue(value: unknown): boolean {
  return value !== null && value !== undefined;
}

function trimmed(filters: AuditFilters): AuditFilters {
  return {
    action: filters.action.trim(),
    target_type: filters.target_type.trim(),
    target_id: filters.target_id.trim(),
  };
}
