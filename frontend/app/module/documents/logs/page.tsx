"use client";

import React, { useCallback, useEffect, useState } from "react";
import { Download, ScrollText, Search } from "lucide-react";
import { esign, saveBlob, type LogFilter, type SignatureLogEntry } from "@/lib/esign";
import { useI18n } from "@/lib/i18n";
import { Banner, EmptyState, Loading, PageHeader, cardClass, tableHeadClass } from "@/components/ui";
import { OutcomeBadge, Pager, useErrorMessage } from "@/components/esign/shared";

const PAGE_SIZE = 50;

/**
 * The signature log — every certificate check, ceremony and download with its
 * outcome.
 *
 * Failures are the point. The original log only ever recorded successes, so a
 * refused or expired signature left no trace, which is exactly the event an
 * auditor comes here looking for.
 */
export default function EsignLogsPage() {
  const { t } = useI18n();
  const describe = useErrorMessage();
  const [entries, setEntries] = useState<SignatureLogEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [filter, setFilter] = useState<LogFilter>({});
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(
    async (nextOffset: number, nextFilter: LogFilter) => {
      setLoading(true);
      try {
        const page = await esign.logs({ ...nextFilter, limit: PAGE_SIZE, offset: nextOffset });
        setEntries(page.items || []);
        setTotal(page.total);
        setOffset(nextOffset);
      } catch (err) {
        setError(describe(err, t("base.message.error")));
      } finally {
        setLoading(false);
      }
    },
    [describe, t],
  );

  useEffect(() => {
    void load(0, {});
  }, [load]);

  const applyFilter = (patch: Partial<LogFilter>) => {
    const next = { ...filter, ...patch };
    setFilter(next);
    // Any filter change resets to the first page; keeping the offset would
    // land on an empty page whenever the result set shrank.
    void load(0, next);
  };

  const submitSearch = (event: React.FormEvent) => {
    event.preventDefault();
    applyFilter({ q: search.trim() || undefined });
  };

  const exportCsv = async () => {
    try {
      saveBlob(await esign.exportLogs(filter), "esign-signature-log.csv");
    } catch (err) {
      setError(describe(err, t("base.message.error")));
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ScrollText className="w-7 h-7 text-indigo-600" />}
        title={t("esign.view.logs_title")}
        subtitle={t("esign.view.logs_subtitle")}
        actions={
          <button
            onClick={exportCsv}
            className="bg-surface-2 hover:bg-slate-200 text-foreground text-xs font-semibold px-4 py-2 rounded-lg flex items-center gap-2"
          >
            <Download className="w-4 h-4" />
            {t("esign.action.export_csv")}
          </button>
        }
      />

      {error && <Banner tone="error" message={error} onDismiss={() => setError(null)} />}

      <section className="bg-surface border border-line rounded-xl shadow-sm p-4 flex flex-wrap gap-3 items-end">
        <form onSubmit={submitSearch} className="flex-1 min-w-[220px]">
          <label htmlFor="log-search" className="block text-[11px] font-semibold uppercase tracking-wider text-muted mb-1">
            {t("base.action.search")}
          </label>
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted" />
            <input
              id="log-search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t("esign.field.search_placeholder")}
              className="w-full pl-9 pr-3 py-2 text-sm border border-input rounded-lg focus:ring-2 focus:ring-indigo-500"
            />
          </div>
        </form>

        <Select
          id="log-action"
          label={t("esign.field.action")}
          value={filter.action ?? ""}
          onChange={(value) => applyFilter({ action: value || undefined })}
          options={[
            ["", t("base.label.all")],
            ["SIGN", t("esign.action_type.sign")],
            ["SIGN_START", t("esign.action_type.sign_start")],
            ["BATCH_SIGN", t("esign.action_type.batch_sign")],
            ["CERT_CHECK", t("esign.action_type.cert_check")],
            ["DOWNLOAD", t("esign.action_type.download")],
          ]}
        />

        <Select
          id="log-outcome"
          label={t("esign.field.outcome")}
          value={filter.outcome ?? ""}
          onChange={(value) => applyFilter({ outcome: value || undefined })}
          options={[
            ["", t("base.label.all")],
            ["OK", t("esign.outcome.ok")],
            ["FAILED", t("esign.outcome.failed")],
            ["REJECTED", t("esign.outcome.rejected")],
            ["EXPIRED", t("esign.outcome.expired")],
            ["CANCELLED", t("esign.outcome.cancelled")],
          ]}
        />

        <Select
          id="log-provider"
          label={t("esign.field.provider")}
          value={filter.provider ?? ""}
          onChange={(value) => applyFilter({ provider: value || undefined })}
          options={[
            ["", t("base.label.all")],
            ["EID", "eID Mongolia"],
            ["HSM", "Gerege eSign HSM"],
          ]}
        />

        <div>
          <label htmlFor="log-from" className="block text-[11px] font-semibold uppercase tracking-wider text-muted mb-1">
            {t("esign.field.from")}
          </label>
          <input
            id="log-from"
            type="date"
            value={filter.from ?? ""}
            onChange={(event) => applyFilter({ from: event.target.value || undefined })}
            className="px-3 py-2 text-sm border border-input rounded-lg focus:ring-2 focus:ring-indigo-500"
          />
        </div>
        <div>
          <label htmlFor="log-to" className="block text-[11px] font-semibold uppercase tracking-wider text-muted mb-1">
            {t("esign.field.to")}
          </label>
          <input
            id="log-to"
            type="date"
            value={filter.to ?? ""}
            onChange={(event) => applyFilter({ to: event.target.value || undefined })}
            className="px-3 py-2 text-sm border border-input rounded-lg focus:ring-2 focus:ring-indigo-500"
          />
        </div>
      </section>

      <div className={`${cardClass} overflow-x-auto`}>
        {loading ? (
          <div className="p-6">
            <Loading />
          </div>
        ) : (
          <table className="w-full text-left text-xs text-muted">
            <thead className={tableHeadClass}>
              <tr>
                <th className="px-4 py-3">{t("base.field.date")}</th>
                <th className="px-4 py-3">{t("esign.field.action")}</th>
                <th className="px-4 py-3">{t("esign.field.outcome")}</th>
                <th className="px-4 py-3">{t("esign.field.document")}</th>
                <th className="px-4 py-3">{t("esign.field.signer")}</th>
                <th className="px-4 py-3">{t("esign.field.provider")}</th>
                <th className="px-4 py-3">{t("esign.field.detail")}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {entries.length === 0 && (
                <tr>
                  <td colSpan={7}>
                    <EmptyState message={t("esign.message.logs_empty")} />
                  </td>
                </tr>
              )}
              {entries.map((entry) => (
                <tr key={entry.id} className="hover:bg-surface-hover">
                  <td className="px-4 py-3 whitespace-nowrap text-muted">
                    {new Date(entry.created_at).toLocaleString()}
                  </td>
                  <td className="px-4 py-3 font-semibold text-foreground">{entry.action}</td>
                  <td className="px-4 py-3">
                    <OutcomeBadge outcome={entry.outcome} />
                  </td>
                  <td className="px-4 py-3">{entry.document_title || <span className="text-muted">—</span>}</td>
                  <td className="px-4 py-3">
                    {[entry.last_name, entry.first_name].filter(Boolean).join(" ") || entry.reg_no || (
                      <span className="text-muted">—</span>
                    )}
                    {entry.reg_no && (entry.first_name || entry.last_name) && (
                      <div className="text-[10px] text-muted font-mono">{entry.reg_no}</div>
                    )}
                  </td>
                  <td className="px-4 py-3 font-mono text-[11px]">{entry.provider}</td>
                  <td className="px-4 py-3 text-muted max-w-xs truncate" title={entry.detail}>
                    {entry.detail || <span className="text-muted">—</span>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <Pager
        total={total}
        offset={offset}
        pageSize={PAGE_SIZE}
        onPage={(next) => load(next, filter)}
      />
    </div>
  );
}

function Select({
  id,
  label,
  value,
  onChange,
  options,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: [string, string][];
}) {
  return (
    <div>
      <label htmlFor={id} className="block text-[11px] font-semibold uppercase tracking-wider text-muted mb-1">
        {label}
      </label>
      <select
        id={id}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="px-3 py-2 text-sm border border-input rounded-lg bg-surface focus:ring-2 focus:ring-indigo-500 h-[38px]"
      >
        {options.map(([optionValue, optionLabel]) => (
          <option key={optionValue} value={optionValue}>
            {optionLabel}
          </option>
        ))}
      </select>
    </div>
  );
}

