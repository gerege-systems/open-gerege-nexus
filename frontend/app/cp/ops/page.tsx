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
          <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
            <GaugeIcon className="w-6 h-6 text-accent" />
            {t("cp.section.metrics")}
          </h1>
          <p className="mt-1 text-sm text-muted">{t("cp.hint.metrics")}</p>
        </div>
        {health?.grafana_url && (
          <Button variant="outline" asChild>
            <a href={health.grafana_url} target="_blank" rel="noopener noreferrer">
              Grafana
              <ExternalLink className="w-3.5 h-3.5" />
            </a>
          </Button>
        )}
        <Button variant="outline" onClick={() => void load()} loading={busy} leadingIcon={<RefreshCw />}>
          {t("cp.action.refresh")}
        </Button>
      </div>

      {failure && (
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
      )}

      {health && !health.monitoring && (
        <p className="text-sm rounded-lg bg-warning-soft border border-warning-border text-warning px-4 py-3">
          {t("cp.message.no_monitoring")}
        </p>
      )}

      <div className="grid gap-4 sm:grid-cols-3">
        <Stat label={t("cp.metric.rps")} value={health?.api.read ? health.api.requests_per_second.toFixed(1) : "—"} />
        <Stat
          label={t("cp.metric.error_rate")}
          value={health?.api.read ? `${(health.api.error_rate * 100).toFixed(2)}%` : "—"}
          tone={health && health.api.error_rate > 0.01 ? "danger" : undefined}
        />
        <Stat
          label={t("cp.metric.p95")}
          value={health?.api.read ? `${Math.round(health.api.p95_seconds * 1000)} ms` : "—"}
        />
      </div>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.infrastructure")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.gauge")}</TableHead>
              <TableHead>{t("cp.field.value")}</TableHead>
              <TableHead>{t("cp.field.warning_at")}</TableHead>
              <TableHead>{t("cp.field.status")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(health?.infra ?? []).map((gauge, index) => (
              <TableRow key={index}>
                <TableCell>
                  <span className="font-mono text-xs uppercase text-muted">{gauge.name}</span>
                </TableCell>
                <TableCell>
                  {gauge.measured ? `${gauge.value.toFixed(1)}${gauge.unit}` : <Unmeasured />}
                </TableCell>
                <TableCell>
                  {`${gauge.warning}${gauge.unit}`}
                </TableCell>
                <TableCell>
                  <StateBadge state={gauge.state} />
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
            {health && health.infra.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="py-8 text-center text-muted">
                  {t("cp.message.no_monitoring")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.external")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.system")}</TableHead>
              <TableHead>{t("cp.metric.error_rate")}</TableHead>
              <TableHead>{t("cp.metric.p95")}</TableHead>
              <TableHead>{t("cp.field.status")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(health?.external ?? []).map((system, index) => (
              <TableRow key={index}>
                <TableCell>
                  <span className="font-medium text-foreground">{system.system}</span>
                </TableCell>
                <TableCell>
                  {system.measured ? `${(system.error_rate * 100).toFixed(1)}%` : <Unmeasured />}
                </TableCell>
                <TableCell>
                  {system.measured ? `${Math.round(system.p95_seconds * 1000)} ms` : <Unmeasured />}
                </TableCell>
                <TableCell>
                  <StateBadge state={system.state} />
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
            {health && health.external.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="py-8 text-center text-muted">
                  {t("cp.message.no_monitoring")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
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
  const tone = state === "green" ? "success" : state === "amber" ? "warning" : state === "red" ? "danger" : "neutral";
  return <Badge tone={tone}>{t(`cp.state.${state}` as "cp.state.green")}</Badge>;
}

/** A number nobody measured is a dash and a word, never a zero. */
function Unmeasured() {
  const { t } = useI18n();
  return <span className="text-xs text-muted">{t("cp.state.unmeasured")}</span>;
}

function Stat({ label, value, tone }: { label: string; value: string; tone?: "danger" }) {
  return (
    <div className="rounded-lg border border-line bg-surface p-4">
      <p className="text-xs font-semibold uppercase tracking-wider text-muted">{label}</p>
      <p className={`mt-1 text-2xl font-semibold ${tone === "danger" ? "text-danger" : "text-foreground"}`}>{value}</p>
    </div>
  );
}
