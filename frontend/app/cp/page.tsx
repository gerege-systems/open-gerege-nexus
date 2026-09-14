"use client";

/**
 * The console's front page: is the platform well?
 *
 * A summary and not a dashboard. Everything on it is either a number somebody
 * would want in the first five seconds — requests, errors, latency, the
 * government systems, what is alerting — or a fact about this deployment that
 * has no other home: what version is running, when the last backup was, which
 * background jobs have quietly stopped.
 *
 * Every panel that has a deeper version links into Grafana. This screen is
 * deliberately never the place an investigation happens, because a summary
 * that grows into a dashboard becomes a dashboard nobody maintains.
 */

import React, { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  Activity,
  AlertTriangle,
  Boxes,
  DatabaseBackup,
  ExternalLink,
  RefreshCw,
  Rocket,
  Server,
} from "lucide-react";
import {
  Alert,
  Badge,
  Button,
  Card,
  CardHeader,
  Spinner,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@gerege-systems/ui";

import { useConsole } from "@/components/cp/Console";
import { useAction } from "@/components/cp/Action";
import { cp, type Overview } from "@/lib/cp";
import { formatMoment } from "@/lib/datetime";
import { useI18n } from "@/lib/i18n";

export default function Health() {
  const { t } = useI18n();
  const { operator } = useConsole();
  const action = useAction();
  const [health, setHealth] = useState<Overview | null>(null);
  const [failure, setFailure] = useState("");

  const load = useCallback(async () => {
    try {
      setHealth(await cp.health());
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    }
  }, []);

  useEffect(() => {
    void load();
    // A minute. The alerting stack is what wakes anybody up; this screen is
    // what somebody watches while they work, and a page that re-renders every
    // few seconds is one that cannot be read.
    const timer = setInterval(() => void load(), 60_000);
    return () => clearInterval(timer);
  }, [load]);

  if (failure) {
    return <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>;
  }
  if (!health) {
    return (
      <p role="status" className="flex items-center gap-2 text-sm text-muted">
        <Spinner size="md" decorative />
        {t("base.message.loading")}
      </p>
    );
  }

  const grafana = (path: string) =>
    health.grafana_url ? `${health.grafana_url}${path}` : "";

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start gap-3">
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-foreground">{t("cp.section.health")}</h1>
          <p className="mt-1 text-sm text-muted">
            {health.version.platform}
            {health.version.release ? ` · ${health.version.release}` : ""}
            {health.version.migration ? ` · db ${health.version.migration}` : ""}
          </p>
        </div>
        {operator.role === "superadmin" && (
          <Button
            type="button"
            leadingIcon={<Rocket className="w-4 h-4" />}
            onClick={() =>
              action.run({
                title: t("cp.action.deploy"),
                detail: t("cp.hint.deploy"),
                danger: true,
                perform: async (reason) => {
                  const { url } = await cp.deploy("main", reason);
                  window.open(url, "_blank", "noopener");
                },
              })
            }
          >
            {t("cp.action.deploy")}
          </Button>
        )}
      </div>

      {health.warnings.map((warning) => (
        <Alert key={warning} variant="warning">
          {warning}
        </Alert>
      ))}

      {!health.monitoring && (
        <Alert variant="default">{t("cp.message.no_monitoring")}</Alert>
      )}

      {health.monitoring && (
        <div className="grid gap-4 sm:grid-cols-3">
          <Stat
            label={t("cp.stat.rps")}
            value={health.api.read ? health.api.requests_per_second.toFixed(1) : "—"}
            icon={<Activity className="w-4 h-4" />}
          />
          <Stat
            label={t("cp.stat.errors")}
            value={health.api.read ? `${(health.api.error_rate * 100).toFixed(2)}%` : "—"}
            tone={health.api.error_rate > 0.01 ? "danger" : "success"}
            icon={<AlertTriangle className="w-4 h-4" />}
          />
          <Stat
            label={t("cp.stat.p95")}
            value={health.api.read ? `${(health.api.p95_seconds * 1000).toFixed(0)} ms` : "—"}
            icon={<Server className="w-4 h-4" />}
          />
        </div>
      )}

      {health.alerts.length > 0 && (
        <Card padding="none" className="overflow-hidden">
          <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
            <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.alerts")}</h2>
          </CardHeader>
          <Table containerClassName="rounded-none border-0">
            <TableHeader>
              <TableRow>
                <TableHead>{t("cp.field.alert")}</TableHead>
                <TableHead>{t("cp.field.severity")}</TableHead>
                <TableHead>{t("cp.field.when")}</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {health.alerts.map((alert, index) => (
                <TableRow key={index}>
                  <TableCell>
                    <strong className="text-foreground">{alert.name}</strong>
                    <span className="block text-xs text-muted">{alert.summary}</span>
                  </TableCell>
                  <TableCell>
                    <Badge tone={alert.severity === "page" ? "danger" : "warning"}>{alert.severity}</Badge>
                  </TableCell>
                  <TableCell>{formatMoment(alert.starts_at)}</TableCell>
                  <TableCell>{alert.silenced && <Badge tone="neutral">{t("cp.state.silenced")}</Badge>}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}

      {health.external.length > 0 && (
        <Card padding="none" className="overflow-hidden">
          <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
            <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.external")}</h2>
            {grafana("/d/nexus-external") ? <DeepLink href={grafana("/d/nexus-external")} /> : undefined}
          </CardHeader>
          <div className="p-4 flex flex-wrap gap-3">
            {health.external.map((system) => (
              <div key={system.system} className="rounded-lg border border-line px-3 py-2 min-w-36">
                <div className="flex items-center gap-2">
                  <span className={`w-2.5 h-2.5 rounded-full ${dot(system.state)}`} />
                  <strong className="text-sm text-foreground">{system.system}</strong>
                </div>
                <p className="mt-1 text-xs text-muted tabular-nums">
                  {system.measured
                    ? `${(system.error_rate * 100).toFixed(1)}% · ${(system.p95_seconds * 1000).toFixed(0)} ms`
                    : t("cp.state.unmeasured")}
                </p>
              </div>
            ))}
          </div>
        </Card>
      )}

      {health.infra.length > 0 && (
        <Card padding="none" className="overflow-hidden">
          <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
            <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.infra")}</h2>
            {grafana("/d/nexus-infra") ? <DeepLink href={grafana("/d/nexus-infra")} /> : undefined}
          </CardHeader>
          <div className="p-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {health.infra.map((gauge) => (
              <div key={gauge.name} className="rounded-lg border border-line px-3 py-2">
                <p className="text-xs uppercase tracking-wide text-muted">{gauge.name}</p>
                <p className={`mt-1 text-lg tabular-nums ${textFor(gauge.state)}`}>
                  {gauge.measured ? `${gauge.value.toFixed(1)}${gauge.unit}` : t("cp.state.unmeasured")}
                </p>
              </div>
            ))}
          </div>
        </Card>
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
              <TableHead>{t("cp.field.state")}</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {health.background.map((job) => (
              <TableRow key={job.name}>
                <TableCell>
                  {/* The job's own name when the dictionary has one; its key
                      otherwise, so a job added later shows up rather than
                      rendering an empty cell. */}
                  {jobName(job.name, t)}
                </TableCell>
                <TableCell>{formatMoment(job.last_run) || "—"}</TableCell>
                <TableCell>
                  <Badge tone={job.ok ? "success" : "danger"}>
                    {job.ok ? t("cp.state.ok") : t("cp.state.failing")}
                  </Badge>
                </TableCell>
                <TableCell className="text-xs text-muted">
                  {job.detail}
                  {job.pending > 0 ? ` · ${job.pending}` : ""}
                </TableCell>
              </TableRow>
            ))}
            {health.background.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="py-8 text-center text-muted">
                  {t("cp.message.no_activity")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      {health.tenant_trouble.length > 0 && (
        <Card padding="none" className="overflow-hidden">
          <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
            <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.tenant_trouble")}</h2>
          </CardHeader>
          <Table containerClassName="rounded-none border-0">
            <TableHeader>
              <TableRow>
                <TableHead>{t("cp.field.organisation")}</TableHead>
                <TableHead>{t("cp.field.failures")}</TableHead>
                <TableHead>{t("cp.field.action")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {health.tenant_trouble.map((row) => (
                <TableRow key={row.tenant_id}>
                  <TableCell>
                    <Link href={`/cp/tenants/${row.tenant_id}`} className="hover:underline text-foreground">
                      {row.name || row.tenant_id}
                    </Link>
                  </TableCell>
                  <TableCell className="tabular-nums">{row.failures}</TableCell>
                  <TableCell className="text-xs text-muted font-mono">{row.sample}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}

      <div className="grid gap-4 lg:grid-cols-2">
        <Card padding="none" className="overflow-hidden">
          <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
            <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.backups")}</h2>
          </CardHeader>
          <div className="p-4 space-y-2 text-sm">
            {!health.backups.configured ? (
              <Alert variant="warning">{t("cp.message.no_backups")}</Alert>
            ) : (
              <>
                <Row
                  label={t("cp.field.last_backup")}
                  value={`${formatMoment(health.backups.last_backup_at)} · ${health.backups.last_size_mb.toFixed(1)} MB`}
                  tone={health.backups.last_ok ? undefined : "danger"}
                />
                <Row
                  label={t("cp.field.last_restore_test")}
                  value={formatMoment(health.backups.last_restore_test_at) || t("cp.message.never_tested")}
                  tone={health.backups.last_restore_test_at ? undefined : "warning"}
                />
              </>
            )}
            <Button
              type="button"
              variant="outline"
              className="mt-2"
              leadingIcon={<DatabaseBackup className="w-4 h-4" />}
              onClick={() =>
                action.run({
                  title: t("cp.action.record_restore_test"),
                  detail: t("cp.hint.restore_test"),
                  perform: (reason) => cp.recordRestoreTest(reason, reason),
                  onDone: load,
                })
              }
            >
              {t("cp.action.record_restore_test")}
            </Button>
          </div>
        </Card>

        <Card padding="none" className="overflow-hidden">
          <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
            <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.catalog")}</h2>
          </CardHeader>
          <div className="p-4 space-y-2 text-sm">
            <Row
              label={t("cp.field.last_sync")}
              value={formatMoment(health.catalog.last_sync_at) || health.catalog.detail || "—"}
              tone={health.catalog.ok ? undefined : "danger"}
            />
            <div className="pt-2 space-y-1">
              {health.catalog.apps.map((app) => (
                <div key={app.app_id} className="flex items-center gap-2 text-xs">
                  <Boxes className="w-3 h-3 text-muted" />
                  <span className="flex-1 truncate text-foreground">{app.name}</span>
                  <span className="text-muted font-mono">
                    {Object.entries(app.versions)
                      .map(([version, count]) => `${version}×${count}`)
                      .join("  ")}
                  </span>
                </div>
              ))}
            </div>
            <Button
              type="button"
              variant="outline"
              className="mt-2"
              leadingIcon={<RefreshCw className="w-4 h-4" />}
              onClick={() =>
                action.run({
                  title: t("cp.action.sync_catalog"),
                  detail: t("cp.hint.sync_catalog"),
                  perform: (reason) => cp.syncCatalog(reason),
                  onDone: load,
                })
              }
            >
              {t("cp.action.sync_catalog")}
            </Button>
          </div>
        </Card>
      </div>

      {action.dialog}
    </div>
  );
}

/** How loudly a figure asks to be read: the design system's status tones. */
type StatTone = "danger" | "warning" | "success";

function Stat({
  label,
  value,
  tone,
  icon,
}: {
  label: string;
  value: string;
  tone?: StatTone;
  icon: React.ReactNode;
}) {
  return (
    <div className="bg-surface rounded-lg border border-line px-4 py-3">
      <p className="text-xs uppercase tracking-wide text-muted flex items-center gap-1.5">
        {icon}
        {label}
      </p>
      <p className={`mt-1 text-2xl tabular-nums ${tone === "danger" ? "text-danger" : "text-foreground"}`}>{value}</p>
    </div>
  );
}

function Row({ label, value, tone }: { label: string; value: string; tone?: StatTone }) {
  return (
    <div className="flex items-baseline gap-3">
      <span className="text-xs uppercase tracking-wide text-muted w-40 shrink-0">{label}</span>
      <span className={tone === "danger" ? "text-danger" : tone === "warning" ? "text-warning" : "text-foreground"}>
        {value}
      </span>
    </div>
  );
}

function DeepLink({ href }: { href: string }) {
  const { t } = useI18n();
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="inline-flex items-center gap-1 text-xs text-muted hover:text-foreground"
    >
      {t("cp.action.open_grafana")}
      <ExternalLink className="w-3 h-3" />
    </a>
  );
}

type Translate = ReturnType<typeof useI18n>["t"];

function jobName(name: string, t: Translate): string {
  switch (name) {
    case "scheduled_reports":
      return t("cp.job.scheduled_reports");
    case "catalog_sync":
      return t("cp.job.catalog_sync");
    case "deletion_sweep":
      return t("cp.job.deletion_sweep");
    default:
      return name;
  }
}

function dot(state: string): string {
  if (state === "unknown") return "bg-line-strong";
  return state === "red" ? "bg-danger-solid" : state === "amber" ? "bg-warning-solid" : "bg-success-solid";
}

function textFor(state: string): string {
  if (state === "unknown") return "text-subtle";
  return state === "red" ? "text-danger" : state === "amber" ? "text-warning" : "text-foreground";
}
