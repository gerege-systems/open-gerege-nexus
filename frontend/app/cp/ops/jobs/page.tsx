"use client";

/**
 * What is supposed to be running on its own.
 *
 * Every job here fails silently by nature: a catalogue that has not synced for
 * a month looks exactly like one with nothing to fetch, and a scheduled report
 * nobody receives is noticed weeks later by the person who was expecting it.
 * The organisations with repeated failures sit under them, because a job that
 * runs and always fails for one tenant is the same class of quiet.
 */

import React, { useCallback, useEffect, useState } from "react";
import { RefreshCw, Timer } from "lucide-react";

import { Badge, Card, formatMoment, Table } from "@/components/cp/ui";
import { cp, type Overview } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

export default function Jobs() {
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

  return (
    <div className="space-y-6">
      <div className="flex items-start gap-3">
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-slate-900 flex items-center gap-2">
            <Timer className="w-6 h-6 text-[var(--gerege-blue)]" />
            {t("cp.section.jobs")}
          </h1>
          <p className="mt-1 text-sm text-slate-500">{t("cp.hint.jobs")}</p>
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

      <Card title={t("cp.section.background")}>
        <Table
          head={[t("cp.field.job"), t("cp.field.last_run"), t("cp.field.status"), t("cp.field.pending")]}
          rows={(health?.background ?? []).map((job) => [
            <span key="n" className="min-w-0">
              <strong className="text-slate-900">{job.name}</strong>
              {job.detail && <span className="block text-xs text-slate-500">{job.detail}</span>}
            </span>,
            formatMoment(job.last_run, locale) || "—",
            <Badge key="s" tone={job.ok ? "emerald" : "red"}>
              {job.ok ? t("cp.state.normal") : t("cp.state.failing")}
            </Badge>,
            job.pending > 0 ? String(job.pending) : "—",
          ])}
          empty={t("cp.message.no_activity")}
        />
      </Card>

      <Card title={t("cp.section.tenant_trouble")}>
        <Table
          head={[t("cp.field.organisation"), t("cp.field.failures"), t("cp.field.sample")]}
          rows={(health?.tenant_trouble ?? []).map((trouble) => [
            <span key="n" className="font-medium text-slate-800">{trouble.name}</span>,
            String(trouble.failures),
            <span key="s" className="text-xs text-slate-500 font-mono break-all">{trouble.sample}</span>,
          ])}
          empty={t("cp.message.no_trouble")}
        />
      </Card>
    </div>
  );
}
