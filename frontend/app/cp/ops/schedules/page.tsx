"use client";

/**
 * Which scheduled report is the one that is broken.
 *
 * The front page counts them: how many have never run, how many failed. A
 * count says something is wrong without saying which, and the next question is
 * always "whose, and what did it say". The ones in trouble sort first, because
 * an operator opening this screen is looking for exactly them.
 */

import React, { useCallback, useEffect, useState } from "react";
import { CalendarClock, RefreshCw } from "lucide-react";
import Link from "next/link";

import { Badge, Card, formatMoment, Table } from "@/components/cp/ui";
import { cp, type ReportSchedule } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

export default function Schedules() {
  const { t, locale } = useI18n();
  const [schedules, setSchedules] = useState<ReportSchedule[]>([]);
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setBusy(true);
    try {
      setSchedules((await cp.reportSchedules()).schedules);
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

  const failing = schedules.filter((item) => item.active && item.last_status !== "" && item.last_status !== "ok").length;
  const never = schedules.filter((item) => item.active && !item.last_run_at).length;

  return (
    <div className="space-y-6">
      <div className="flex items-start gap-3">
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-slate-900 flex items-center gap-2">
            <CalendarClock className="w-6 h-6 text-[var(--gerege-blue)]" />
            {t("cp.section.schedules")}
          </h1>
          <p className="mt-1 text-sm text-slate-500">{t("cp.hint.schedules")}</p>
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

      {(failing > 0 || never > 0) && (
        <p className="text-sm rounded-xl bg-amber-50 border border-amber-200 text-amber-900 px-4 py-3">
          {t("cp.message.schedules_trouble", { failing: String(failing), never: String(never) })}
        </p>
      )}

      <Card title={t("cp.section.schedules")}>
        <Table
          head={[
            t("cp.field.organisation"),
            t("cp.field.report"),
            t("cp.field.cron"),
            t("cp.field.recipients"),
            t("cp.field.last_run"),
            t("cp.field.status"),
          ]}
          rows={schedules.map((schedule) => [
            <Link
              key="t"
              href={`/cp/tenants/${schedule.tenant_id}`}
              className="font-medium text-[var(--gerege-blue)] hover:underline"
            >
              {schedule.tenant_name || t("cp.state.deleted")}
            </Link>,
            <span key="r" className="min-w-0">
              <strong className="text-slate-900">{schedule.name || schedule.report_key}</strong>
              <span className="block text-xs text-slate-500 font-mono">{schedule.report_key} · {schedule.format}</span>
            </span>,
            <span key="c" className="font-mono text-xs">{schedule.cron}</span>,
            <span key="p" className="text-xs text-slate-500">{schedule.recipients.join(", ") || "—"}</span>,
            formatMoment(schedule.last_run_at, locale) || <span key="n" className="text-xs text-slate-400">{t("cp.state.never")}</span>,
            <Badge
              key="s"
              tone={!schedule.active ? "slate" : !schedule.last_run_at ? "amber" : schedule.last_status === "" || schedule.last_status === "ok" ? "emerald" : "red"}
            >
              {!schedule.active
                ? t("cp.state.off")
                : !schedule.last_run_at
                  ? t("cp.state.never")
                  : schedule.last_status === "" || schedule.last_status === "ok"
                    ? t("cp.state.normal")
                    : schedule.last_status}
            </Badge>,
          ])}
          empty={t("cp.message.no_schedules")}
        />
      </Card>
    </div>
  );
}
