"use client";

/**
 * Is it up.
 *
 * The console's front page answers "is anything wrong anywhere" in one screen;
 * this one answers "how is the deployment running" in numbers — the API's own
 * three, every external system it depends on, and the four gauges that fill up
 * silently until something stops.
 */

import React, { useCallback, useEffect, useState } from "react";
import { ExternalLink, Gauge as GaugeIcon, RefreshCw } from "lucide-react";

import { Badge, Card, Table } from "@/components/cp/ui";
import { cp, type Overview } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

export default function Metrics() {
  const { t } = useI18n();
  const [health, setHealth] = useState<Overview | null>(null);
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setBusy(true);
    try {
      setHealth(await cp.health());
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

  return (
    <div className="space-y-6">
      <div className="flex items-start gap-3">
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-slate-900 flex items-center gap-2">
            <GaugeIcon className="w-6 h-6 text-[var(--gerege-blue)]" />
            {t("cp.section.metrics")}
          </h1>
          <p className="mt-1 text-sm text-slate-500">{t("cp.hint.metrics")}</p>
        </div>
        {health?.grafana_url && (
          <a
            href={health.grafana_url}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 hover:bg-slate-50"
          >
            Grafana
            <ExternalLink className="w-3.5 h-3.5" />
          </a>
        )}
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

      {health && !health.monitoring && (
        <p className="text-sm rounded-xl bg-amber-50 border border-amber-200 text-amber-900 px-4 py-3">
          {t("cp.message.no_monitoring")}
        </p>
      )}

      <div className="grid gap-4 sm:grid-cols-3">
        <Stat label={t("cp.metric.rps")} value={health?.api.read ? health.api.requests_per_second.toFixed(1) : "—"} />
        <Stat
          label={t("cp.metric.error_rate")}
          value={health?.api.read ? `${(health.api.error_rate * 100).toFixed(2)}%` : "—"}
          tone={health && health.api.error_rate > 0.01 ? "red" : undefined}
        />
        <Stat
          label={t("cp.metric.p95")}
          value={health?.api.read ? `${Math.round(health.api.p95_seconds * 1000)} ms` : "—"}
        />
      </div>

      <Card title={t("cp.section.infrastructure")}>
        <Table
          head={[t("cp.field.gauge"), t("cp.field.value"), t("cp.field.warning_at"), t("cp.field.status")]}
          rows={(health?.infra ?? []).map((gauge) => [
            <span key="n" className="font-mono text-xs uppercase text-slate-600">{gauge.name}</span>,
            gauge.measured ? `${gauge.value.toFixed(1)}${gauge.unit}` : <Unmeasured key="v" />,
            `${gauge.warning}${gauge.unit}`,
            <StateBadge key="s" state={gauge.state} />,
          ])}
          empty={t("cp.message.no_monitoring")}
        />
      </Card>

      <Card title={t("cp.section.external")}>
        <Table
          head={[t("cp.field.system"), t("cp.metric.error_rate"), t("cp.metric.p95"), t("cp.field.status")]}
          rows={(health?.external ?? []).map((system) => [
            <span key="n" className="font-medium text-slate-800">{system.system}</span>,
            system.measured ? `${(system.error_rate * 100).toFixed(1)}%` : <Unmeasured key="e" />,
            system.measured ? `${Math.round(system.p95_seconds * 1000)} ms` : <Unmeasured key="p" />,
            <StateBadge key="s" state={system.state} />,
          ])}
          empty={t("cp.message.no_monitoring")}
        />
      </Card>
    </div>
  );
}

/**
 * The colour the backend decided, in the words this screen uses.
 *
 * The states are green, amber, red and unknown — "unknown" being a system
 * Prometheus holds no sample for. It is deliberately not green: an unmeasured
 * system reading as healthy is the failure this badge exists to stop.
 */
function StateBadge({ state }: { state: string }) {
  const { t } = useI18n();
  const tone = state === "green" ? "emerald" : state === "amber" ? "amber" : state === "red" ? "red" : "slate";
  return <Badge tone={tone}>{t(`cp.state.${state}` as "cp.state.green")}</Badge>;
}

/** A number nobody measured is a dash and a word, never a zero. */
function Unmeasured() {
  const { t } = useI18n();
  return <span className="text-xs text-slate-400">{t("cp.state.unmeasured")}</span>;
}

function Stat({ label, value, tone }: { label: string; value: string; tone?: "red" }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-4">
      <p className="text-xs font-semibold uppercase tracking-wider text-slate-400">{label}</p>
      <p className={`mt-1 text-2xl font-semibold ${tone === "red" ? "text-red-600" : "text-slate-900"}`}>{value}</p>
    </div>
  );
}
