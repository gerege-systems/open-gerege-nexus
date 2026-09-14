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

export default function Alerts() {
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

  const alerts = health?.alerts ?? [];
  const warnings = health?.warnings ?? [];

  return (
    <div className="space-y-6">
      <div className="flex items-start gap-3">
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
            <BellRing className="w-6 h-6 text-accent" />
            {t("cp.section.alerts")}
          </h1>
          <p className="mt-1 text-sm text-muted">{t("cp.hint.alerts")}</p>
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
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.firing")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.alert")}</TableHead>
              <TableHead>{t("cp.field.severity")}</TableHead>
              <TableHead>{t("cp.field.since")}</TableHead>
              <TableHead></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {alerts.map((alert, index) => (
              <TableRow key={index}>
                <TableCell>
                  <span className="min-w-0">
                    <strong className="text-foreground flex items-center gap-1.5">
                      {alert.name}
                      {alert.silenced && <VolumeX className="w-3.5 h-3.5 text-muted" aria-label={t("cp.state.silenced")} />}
                    </strong>
                    {alert.summary && <span className="block text-xs text-muted">{alert.summary}</span>}
                  </span>
                </TableCell>
                <TableCell>
                  <Badge tone={alert.severity === "critical" ? "danger" : alert.severity === "warning" ? "warning" : "neutral"}>
                    {alert.severity || "—"}
                  </Badge>
                </TableCell>
                <TableCell>
                  {formatMoment(alert.starts_at)}
                </TableCell>
                <TableCell>
                  {alert.runbook ? (
                    <a
                      href={alert.runbook}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 text-xs text-accent hover:underline"
                    >
                      {t("cp.action.runbook")}
                      <ExternalLink className="w-3 h-3" />
                    </a>
                  ) : (
                    <span className="text-xs text-muted">—</span>
                  )}
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
            {health && alerts.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="py-8 text-center text-muted">
                  {t("cp.message.nothing_firing")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.warnings")}</h2>
        </CardHeader>
        <div className="p-4 space-y-2">
          {warnings.length === 0 && <p className="text-sm text-muted">{t("cp.message.no_warnings")}</p>}
          {warnings.map((warning) => (
            <p key={warning} className="flex items-start gap-2 text-sm rounded-lg bg-warning-soft border border-warning-border text-warning px-3 py-2">
              <ShieldAlert className="w-4 h-4 mt-0.5 shrink-0" />
              {warning}
            </p>
          ))}
        </div>
      </Card>
    </div>
  );
}
