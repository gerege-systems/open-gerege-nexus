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

import { cp, type PlatformUsage } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";
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
import { formatNumber, formatMoment } from "@/lib/datetime";

export default function Usage() {
  const { t } = useI18n();
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

  const metrics = report?.metrics ?? [];

  return (
    <div className="space-y-6">
      <div className="flex items-start gap-3">
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
            <BarChart3 className="w-6 h-6 text-accent" />
            {t("cp.section.usage")}
          </h1>
          <p className="mt-1 text-sm text-muted">
            {t("cp.hint.usage")} {report?.month && <span className="font-mono">{report.month}</span>}
          </p>
        </div>
        <Button variant="outline" onClick={() => void load()} loading={busy} leadingIcon={<RefreshCw />}>
          {t("cp.action.refresh")}
        </Button>
      </div>

      {failure && (
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
      )}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {metrics.map((metric) => (
          <div key={metric} className="rounded-lg border border-line bg-surface p-4">
            <p className="text-xs font-semibold uppercase tracking-wider text-muted">
              {t(`cp.metric.${metric}` as "cp.metric.storage_mb")}
            </p>
            <p className="mt-1 text-2xl font-semibold text-foreground">
              {formatNumber(report?.totals[metric] ?? 0)}
            </p>
          </div>
        ))}
      </div>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.by_organisation")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.organisation")}</TableHead>
              {metrics.map((metric) => (
                <TableHead key={metric}>{t(`cp.metric.${metric}` as "cp.metric.storage_mb")}</TableHead>
              ))}
              <TableHead>{t("cp.field.collected")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {lines.map((line) => (
              <TableRow key={line.tenant_id}>
                <TableCell>
                  <span className="min-w-0">
                    <Link href={`/cp/tenants/${line.tenant_id}`} className="font-medium text-accent hover:underline">
                      {line.tenant_name}
                    </Link>
                    <span className="block text-xs text-muted font-mono">{line.slug}</span>
                    {line.suspended && <Badge tone="danger">{t("cp.state.suspended")}</Badge>}
                  </span>
                </TableCell>
                {metrics.map((metric) => (
                  <TableCell key={metric} className="tabular-nums">
                    {formatNumber(line.metrics[metric] ?? 0)}
                  </TableCell>
                ))}
                <TableCell>
                  {line.collected ? formatMoment(line.collected) : (
                    <span className="text-xs text-muted">{t("cp.state.never_counted")}</span>
                  )}
                </TableCell>
              </TableRow>
            ))}
            {!report && !failure && (
              <TableRow>
                <TableCell colSpan={metrics.length + 2}>
                  <div role="status" aria-busy="true" className="space-y-3 py-2">
                    <span className="sr-only">{t("base.message.loading")}</span>
                    <Skeleton className="h-4" />
                    <Skeleton className="h-4" />
                    <Skeleton className="h-4" />
                  </div>
                </TableCell>
              </TableRow>
            )}
            {report && lines.length === 0 && (
              <TableRow>
                <TableCell colSpan={metrics.length + 2} className="py-8 text-center text-muted">
                  {t("cp.message.no_usage")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>
    </div>
  );
}
