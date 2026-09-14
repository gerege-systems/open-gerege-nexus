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

import { cp, type ReportSchedule } from "@/lib/cp";
import {
  Badge,
  Button,
  Card,
  CardHeader,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@gerege-systems/ui";
import { formatMoment } from "@/lib/datetime";
import { useI18n } from "@/lib/i18n";

export default function Schedules() {
  const { t } = useI18n();
  const [schedules, setSchedules] = useState<ReportSchedule[]>([]);
  const [failure, setFailure] = useState("");
  // Until the first answer, an empty list is not yet a claim that nothing exists.
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setBusy(true);
    try {
      setSchedules((await cp.reportSchedules()).schedules);
      setLoaded(true);
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
          <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
            <CalendarClock className="w-6 h-6 text-accent" />
            {t("cp.section.schedules")}
          </h1>
          <p className="mt-1 text-sm text-muted">{t("cp.hint.schedules")}</p>
        </div>
        <Button variant="outline" onClick={() => void load()} loading={busy} leadingIcon={<RefreshCw />}>
          {t("cp.action.refresh")}
        </Button>
      </div>

      {failure && (
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
      )}

      {(failing > 0 || never > 0) && (
        <p className="text-sm rounded-lg bg-warning-soft border border-warning-border text-warning px-4 py-3">
          {t("cp.message.schedules_trouble", { failing: String(failing), never: String(never) })}
        </p>
      )}

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.schedules")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.organisation")}</TableHead>
              <TableHead>{t("cp.field.report")}</TableHead>
              <TableHead>{t("cp.field.cron")}</TableHead>
              <TableHead>{t("cp.field.recipients")}</TableHead>
              <TableHead>{t("cp.field.last_run")}</TableHead>
              <TableHead>{t("cp.field.status")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {schedules.map((schedule, index) => (
              <TableRow key={index}>
                <TableCell>
                  <Link
                    href={`/cp/tenants/${schedule.tenant_id}`}
                    className="font-medium text-accent hover:underline"
                  >
                    {schedule.tenant_name || t("cp.state.deleted")}
                  </Link>
                </TableCell>
                <TableCell>
                  <span className="min-w-0">
                    <strong className="text-foreground">{schedule.name || schedule.report_key}</strong>
                    <span className="block text-xs text-muted font-mono">{schedule.report_key} · {schedule.format}</span>
                  </span>
                </TableCell>
                <TableCell>
                  <span className="font-mono text-xs">{schedule.cron}</span>
                </TableCell>
                <TableCell>
                  <span className="text-xs text-muted">{schedule.recipients.join(", ") || "—"}</span>
                </TableCell>
                <TableCell>
                  {formatMoment(schedule.last_run_at) || <span className="text-xs text-muted">{t("cp.state.never")}</span>}
                </TableCell>
                <TableCell>
                  <Badge
                    tone={!schedule.active ? "neutral" : !schedule.last_run_at ? "warning" : schedule.last_status === "" || schedule.last_status === "ok" ? "success" : "danger"}
                  >
                    {!schedule.active
                      ? t("cp.state.off")
                      : !schedule.last_run_at
                        ? t("cp.state.never")
                        : schedule.last_status === "" || schedule.last_status === "ok"
                          ? t("cp.state.normal")
                          : schedule.last_status}
                  </Badge>
                </TableCell>
              </TableRow>
            ))}
            {!loaded && !failure && (
              <TableRow>
                <TableCell colSpan={6}>
                  <div role="status" aria-busy="true" className="space-y-3 py-2">
                    <span className="sr-only">{t("base.message.loading")}</span>
                    <Skeleton className="h-4" />
                    <Skeleton className="h-4" />
                    <Skeleton className="h-4" />
                  </div>
                </TableCell>
              </TableRow>
            )}
            {loaded && schedules.length === 0 && (
              <TableRow>
                <TableCell colSpan={6} className="py-8 text-center text-muted">
                  {t("cp.message.no_schedules")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>
    </div>
  );
}
