"use client";

/**
 * What one organisation used.
 *
 * Ninety days, five metrics, and the limit each is measured against. The
 * numbers come from the nightly count in the database rather than from
 * Prometheus — no metric on this platform carries a tenant label, by a
 * decision made in the first phase and kept since — so what is charted here is
 * *acts recorded*, which is also the thing a bill should be based on.
 *
 * Two of the five are not sums, and the screen says so: people is a peak, and
 * storage is a reading. Adding up daily storage would produce a number that
 * grows for ever and means nothing.
 */

import React, { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft, Download } from "lucide-react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { Badge, Card, formatMoment } from "@/components/cp/ui";
import { cp, type Usage, type UsageSeries } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

export default function UsageScreen() {
  const { t, locale } = useI18n();
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const [usage, setUsage] = useState<Usage | null>(null);
  const [failure, setFailure] = useState("");

  const load = useCallback(async () => {
    try {
      setUsage(await cp.usage(id));
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    }
  }, [id]);

  useEffect(() => {
    if (id) void load();
  }, [id, load]);

  if (failure) {
    return <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>;
  }
  if (!usage) return <div className="text-slate-500">…</div>;

  return (
    <div className="space-y-6">
      <Link
        href={`/cp/tenants/${id}`}
        className="inline-flex items-center gap-1.5 text-sm text-slate-600 hover:text-slate-900"
      >
        <ArrowLeft className="w-4 h-4" />
        {t("cp.action.back")}
      </Link>

      <div className="flex flex-wrap items-start gap-3">
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-slate-900">{t("cp.section.usage")}</h1>
          <p className="mt-1 text-sm text-slate-500">
            {usage.collected
              ? `${t("cp.field.counted")}: ${formatMoment(usage.collected, locale)}`
              : t("cp.message.never_counted")}
          </p>
        </div>
        <a
          href={cp.usageCSVURL(id)}
          className="inline-flex items-center gap-2 rounded-lg border border-slate-300 px-3 py-2 text-sm hover:bg-slate-50"
        >
          <Download className="w-4 h-4" />
          CSV
        </a>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        {usage.series.map((series) => (
          <MetricCard key={series.metric} series={series} />
        ))}
      </div>
    </div>
  );
}

function MetricCard({ series }: { series: UsageSeries }) {
  const { t } = useI18n();
  const over = series.limit !== null && series.month_to_date >= series.limit;
  // Storage is a level over time and the rest are daily counts: a line for the
  // first, bars for the others, because a line implies a continuous quantity
  // and bars imply things that happened.
  const asLine = series.metric === "storage_mb";

  return (
    <Card
      title={metricName(series.metric, t)}
      action={
        series.limit !== null ? (
          <Badge tone={over ? "red" : "slate"}>
            {series.month_to_date} / {series.limit}
            {series.enforced ? "" : ` · ${t("cp.state.not_enforced")}`}
          </Badge>
        ) : (
          <span className="text-xs text-slate-400">{series.total}</span>
        )
      }
    >
      <div className="p-4 h-48">
        {series.points.length === 0 ? (
          <p className="text-sm text-slate-500">{t("cp.message.no_usage")}</p>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            {asLine ? (
              <LineChart data={series.points}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                <XAxis dataKey="day" tick={{ fontSize: 10 }} minTickGap={24} />
                <YAxis tick={{ fontSize: 10 }} width={40} />
                <Tooltip />
                <Line type="monotone" dataKey="value" stroke="#0f172a" dot={false} strokeWidth={2} />
              </LineChart>
            ) : (
              <BarChart data={series.points}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                <XAxis dataKey="day" tick={{ fontSize: 10 }} minTickGap={24} />
                <YAxis tick={{ fontSize: 10 }} width={40} />
                <Tooltip />
                <Bar dataKey="value" fill="#0f172a" radius={[2, 2, 0, 0]} />
              </BarChart>
            )}
          </ResponsiveContainer>
        )}
      </div>
    </Card>
  );
}

type Translate = ReturnType<typeof useI18n>["t"];

/** The dictionary's name for a metric, or its key for one added later. */
function metricName(metric: string, t: Translate): string {
  switch (metric) {
    case "active_users":
      return t("cp.metric.active_users");
    case "actions":
      return t("cp.metric.actions");
    case "ai_calls":
      return t("cp.metric.ai_calls");
    case "reports_sent":
      return t("cp.metric.reports_sent");
    case "storage_mb":
      return t("cp.metric.storage_mb");
    default:
      return metric;
  }
}
