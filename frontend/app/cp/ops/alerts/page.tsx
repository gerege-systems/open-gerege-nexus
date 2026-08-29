"use client";

/**
 * What is firing, and what this deployment is complaining about.
 *
 * Two lists that are easy to confuse and must not be: an alert is Alertmanager
 * saying something is wrong now, a warning is the platform saying it was
 * configured in a way that will go wrong later. Both are here because both are
 * read at the same moment — the one where somebody asks "why did it do that".
 */

import React, { useCallback, useEffect, useState } from "react";
import { BellRing, ExternalLink, RefreshCw, ShieldAlert, VolumeX } from "lucide-react";

import { Badge, Card, formatMoment, Table } from "@/components/cp/ui";
import { cp, type Overview } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

export default function Alerts() {
  const { t, locale } = useI18n();
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

  const alerts = health?.alerts ?? [];
  const warnings = health?.warnings ?? [];

  return (
    <div className="space-y-6">
      <div className="flex items-start gap-3">
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-slate-900 flex items-center gap-2">
            <BellRing className="w-6 h-6 text-[var(--gerege-blue)]" />
            {t("cp.section.alerts")}
          </h1>
          <p className="mt-1 text-sm text-slate-500">{t("cp.hint.alerts")}</p>
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

      <Card title={t("cp.section.firing")}>
        <Table
          head={[t("cp.field.alert"), t("cp.field.severity"), t("cp.field.since"), ""]}
          rows={alerts.map((alert) => [
            <span key="n" className="min-w-0">
              <strong className="text-slate-900 flex items-center gap-1.5">
                {alert.name}
                {alert.silenced && <VolumeX className="w-3.5 h-3.5 text-slate-400" aria-label={t("cp.state.silenced")} />}
              </strong>
              {alert.summary && <span className="block text-xs text-slate-500">{alert.summary}</span>}
            </span>,
            <Badge key="s" tone={alert.severity === "critical" ? "red" : alert.severity === "warning" ? "amber" : "slate"}>
              {alert.severity || "—"}
            </Badge>,
            formatMoment(alert.starts_at, locale),
            alert.runbook ? (
              <a
                key="r"
                href={alert.runbook}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 text-xs text-[var(--gerege-blue)] hover:underline"
              >
                {t("cp.action.runbook")}
                <ExternalLink className="w-3 h-3" />
              </a>
            ) : (
              <span key="r" className="text-xs text-slate-400">—</span>
            ),
          ])}
          empty={t("cp.message.nothing_firing")}
        />
      </Card>

      <Card title={t("cp.section.warnings")}>
        <div className="p-4 space-y-2">
          {warnings.length === 0 && <p className="text-sm text-slate-500">{t("cp.message.no_warnings")}</p>}
          {warnings.map((warning) => (
            <p key={warning} className="flex items-start gap-2 text-sm rounded-lg bg-amber-50 border border-amber-200 text-amber-900 px-3 py-2">
              <ShieldAlert className="w-4 h-4 mt-0.5 shrink-0" />
              {warning}
            </p>
          ))}
        </div>
      </Card>
    </div>
  );
}
