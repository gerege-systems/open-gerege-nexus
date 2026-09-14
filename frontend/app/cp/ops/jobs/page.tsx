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

import { cp, type Overview } from "@/lib/cp";
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

export default function Jobs() {
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
          <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
            <Timer className="w-6 h-6 text-accent" />
            {t("cp.section.jobs")}
          </h1>
          <p className="mt-1 text-sm text-muted">{t("cp.hint.jobs")}</p>
        </div>
        <Button variant="outline" onClick={() => void load()} loading={busy} leadingIcon={<RefreshCw />}>
          {t("cp.action.refresh")}
        </Button>
      </div>

      {failure && (
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
      )}

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.background")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.job")}</TableHead>
              <TableHead>{t("cp.field.last_run")}</TableHead>
              <TableHead>{t("cp.field.status")}</TableHead>
              <TableHead>{t("cp.field.pending")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(health?.background ?? []).map((job, index) => (
              <TableRow key={index}>
                <TableCell>
                  <span className="min-w-0">
                    <strong className="text-foreground">{job.name}</strong>
                    {job.detail && <span className="block text-xs text-muted">{job.detail}</span>}
                  </span>
                </TableCell>
                <TableCell>
                  {formatMoment(job.last_run) || "—"}
                </TableCell>
                <TableCell>
                  <Badge tone={job.ok ? "success" : "danger"}>
                    {job.ok ? t("cp.state.normal") : t("cp.state.failing")}
                  </Badge>
                </TableCell>
                <TableCell>
                  {job.pending > 0 ? String(job.pending) : "—"}
                </TableCell>
              </TableRow>
            ))}
            {!health && !failure && (
              <TableRow>
                <TableCell colSpan={4}>
                  <div role="status" aria-busy="true" className="space-y-3 py-2">
                    <span className="sr-only">{t("base.message.loading")}</span>
                    <Skeleton className="h-4" />
                    <Skeleton className="h-4" />
                    <Skeleton className="h-4" />
                  </div>
                </TableCell>
              </TableRow>
            )}
            {health && health.background.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="py-8 text-center text-muted">
                  {t("cp.message.no_activity")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.tenant_trouble")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.organisation")}</TableHead>
              <TableHead>{t("cp.field.failures")}</TableHead>
              <TableHead>{t("cp.field.sample")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(health?.tenant_trouble ?? []).map((trouble, index) => (
              <TableRow key={index}>
                <TableCell>
                  <span className="font-medium text-foreground">{trouble.name}</span>
                </TableCell>
                <TableCell>
                  {String(trouble.failures)}
                </TableCell>
                <TableCell>
                  <span className="text-xs text-muted font-mono break-all">{trouble.sample}</span>
                </TableCell>
              </TableRow>
            ))}
            {!health && !failure && (
              <TableRow>
                <TableCell colSpan={3}>
                  <div role="status" aria-busy="true" className="space-y-3 py-2">
                    <span className="sr-only">{t("base.message.loading")}</span>
                    <Skeleton className="h-4" />
                    <Skeleton className="h-4" />
                    <Skeleton className="h-4" />
                  </div>
                </TableCell>
              </TableRow>
            )}
            {health && health.tenant_trouble.length === 0 && (
              <TableRow>
                <TableCell colSpan={3} className="py-8 text-center text-muted">
                  {t("cp.message.no_trouble")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>
    </div>
  );
}
