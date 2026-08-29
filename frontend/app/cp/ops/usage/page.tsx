"use client";

/**
 * Who is using this deployment.
 *
 * The organisation screen answers "is this one near its limit". It cannot
 * answer the question behind a capacity decision or an invoice run, and
 * answering that by opening forty organisations in turn is how a platform
 * stops asking it.
 *
 * Each metric is rolled up the way that metric means: counted things are
 * summed, storage is the latest reading, and people are the month's peak —
 * summing daily active users would count one person once per day.
 */

import React, { useCallback, useEffect, useMemo, useState } from "react";
import { BarChart3, RefreshCw } from "lucide-react";
import Link from "next/link";

import { Badge, Card, formatMoment, Table } from "@/components/cp/ui";
import { cp, type PlatformUsage } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

export default function Usage() {
  const { t, locale } = useI18n();
  const [report, setReport] = useState<PlatformUsage | null>(null);
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setBusy(true);
    try {
      setReport(await cp.platformUsage());
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  // Busiest first: a report ordered by name makes the reader scan for the
  // number they came for.
  const lines = useMemo(() => {
    if (!report) return [];
    const weight = (metrics: Record<string, number>) =>
      Object.values(metrics).reduce((sum, value) => sum + value, 0);
    return [...report.tenants].sort((one, two) => weight(two.metrics) - weight(one.metrics));
  }, [report]);

  return (
    <div className="space-y-6">
      <div className="flex items-start gap-3">
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-slate-900 flex items-center gap-2">
            <BarChart3 className="w-6 h-6 text-[var(--gerege-blue)]" />
            {t("cp.section.usage")}
          </h1>
          <p className="mt-1 text-sm text-slate-500">
            {t("cp.hint.usage")} {report?.month && <span className="font-mono">{report.month}</span>}
          </p>
        </div>
        <button
          type="button"
          onClick={() => void load()}
          disabled={busy}
          className="inline-flex items-center gap-2 rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 hover:bg-slate-50 disabled:opacity-50"
        >
          <RefreshCw className={`w-4 h-4 ${busy ? "animate-spin" : ""}`} />
          {t("cp.action.refresh")}
        </button>
      </div>

      {failure && (
        <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
      )}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {(report?.metrics ?? []).map((metric) => (
          <div key={metric} className="rounded-xl border border-slate-200 bg-white p-4">
            <p className="text-xs font-semibold uppercase tracking-wider text-slate-400">
              {t(`cp.metric.${metric}` as "cp.metric.storage_mb")}
            </p>
            <p className="mt-1 text-2xl font-semibold text-slate-900">
              {(report?.totals[metric] ?? 0).toLocaleString(locale)}
            </p>
          </div>
        ))}
      </div>

      <Card title={t("cp.section.by_organisation")}>
        <Table
          head={[
            t("cp.field.organisation"),
            ...(report?.metrics ?? []).map((metric) => t(`cp.metric.${metric}` as "cp.metric.storage_mb")),
            t("cp.field.collected"),
          ]}
          rows={lines.map((line) => [
            <span key="n" className="min-w-0">
              <Link href={`/cp/tenants/${line.tenant_id}`} className="font-medium text-[var(--gerege-blue)] hover:underline">
                {line.tenant_name}
              </Link>
              <span className="block text-xs text-slate-500 font-mono">{line.slug}</span>
              {line.suspended && <Badge tone="red">{t("cp.state.suspended")}</Badge>}
            </span>,
            ...(report?.metrics ?? []).map((metric) => (
              <span key={metric} className="tabular-nums">
                {(line.metrics[metric] ?? 0).toLocaleString(locale)}
              </span>
            )),
            line.collected ? formatMoment(line.collected, locale) : (
              <span key="c" className="text-xs text-slate-400">{t("cp.state.never_counted")}</span>
            ),
          ])}
          empty={t("cp.message.no_usage")}
        />
      </Card>
    </div>
  );
}
