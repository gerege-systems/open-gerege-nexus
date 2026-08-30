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

import { useConsole } from "@/components/cp/Console";
import { useAction } from "@/components/cp/Action";
import { Badge, Card, formatMoment, Table, type Tone } from "@/components/cp/ui";
import { cp, type Overview } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

export default function Health() {
  const { t, locale } = useI18n();
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
    return <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>;
  }
  if (!health) return <div className="text-slate-500">…</div>;

  const grafana = (path: string) =>
    health.grafana_url ? `${health.grafana_url}${path}` : "";

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start gap-3">
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-slate-900">{t("cp.section.health")}</h1>
          <p className="mt-1 text-sm text-slate-500">
            {health.version.platform}
            {health.version.release ? ` · ${health.version.release}` : ""}
            {health.version.migration ? ` · db ${health.version.migration}` : ""}
          </p>
        </div>
        {operator.role === "superadmin" && (
          <button
            type="button"
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
            className="inline-flex items-center gap-2 rounded-lg bg-[var(--gerege-blue)] px-3 py-2 text-sm font-medium text-[var(--gerege-on-blue)] hover:brightness-105"
          >
            <Rocket className="w-4 h-4" />
            {t("cp.action.deploy")}
          </button>
        )}
      </div>

      {health.warnings.map((warning) => (
        <p key={warning} className="text-sm rounded-xl bg-amber-50 border border-amber-200 text-amber-900 px-4 py-3">
          {warning}
        </p>
      ))}

      {!health.monitoring && (
        <p className="text-sm rounded-xl bg-slate-100 border border-slate-200 text-slate-700 px-4 py-3">
          {t("cp.message.no_monitoring")}
        </p>
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
            tone={health.api.error_rate > 0.01 ? "red" : "emerald"}
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
        <Card title={t("cp.section.alerts")}>
          <Table
            head={[t("cp.field.alert"), t("cp.field.severity"), t("cp.field.when"), ""]}
            rows={health.alerts.map((alert) => [
              <span key="n">
                <strong className="text-slate-900">{alert.name}</strong>
                <span className="block text-xs text-slate-500">{alert.summary}</span>
              </span>,
              <Badge key="s" tone={alert.severity === "page" ? "red" : "amber"}>
                {alert.severity}
              </Badge>,
              formatMoment(alert.starts_at, locale),
              alert.silenced ? <Badge key="q" tone="slate">{t("cp.state.silenced")}</Badge> : "",
            ])}
            empty={t("cp.message.no_alerts")}
          />
        </Card>
      )}

      {health.external.length > 0 && (
        <Card
          title={t("cp.section.external")}
          action={grafana("/d/nexus-external") ? <DeepLink href={grafana("/d/nexus-external")} /> : undefined}
        >
          <div className="p-4 flex flex-wrap gap-3">
            {health.external.map((system) => (
              <div key={system.system} className="rounded-xl border border-slate-200 px-3 py-2 min-w-[9rem]">
                <div className="flex items-center gap-2">
                  <span className={`w-2.5 h-2.5 rounded-full ${dot(system.state)}`} />
                  <strong className="text-sm text-slate-900">{system.system}</strong>
                </div>
                <p className="mt-1 text-xs text-slate-500 tabular-nums">
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
        <Card
          title={t("cp.section.infra")}
          action={grafana("/d/nexus-infra") ? <DeepLink href={grafana("/d/nexus-infra")} /> : undefined}
        >
          <div className="p-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {health.infra.map((gauge) => (
              <div key={gauge.name} className="rounded-xl border border-slate-200 px-3 py-2">
                <p className="text-xs uppercase tracking-wide text-slate-400">{gauge.name}</p>
                <p className={`mt-1 text-lg tabular-nums ${textFor(gauge.state)}`}>
                  {gauge.measured ? `${gauge.value.toFixed(1)}${gauge.unit}` : t("cp.state.unmeasured")}
                </p>
              </div>
            ))}
          </div>
        </Card>
      )}

      <Card title={t("cp.section.background")}>
        <Table
          head={[t("cp.field.job"), t("cp.field.last_run"), t("cp.field.state"), ""]}
          rows={health.background.map((job) => [
            // The job's own name when the dictionary has one; its key
            // otherwise, so a job added later shows up rather than rendering
            // an empty cell.
            jobName(job.name, t),
            formatMoment(job.last_run, locale) || "—",
            <Badge key="s" tone={job.ok ? "emerald" : "red"}>
              {job.ok ? t("cp.state.ok") : t("cp.state.failing")}
            </Badge>,
            <span key="d" className="text-xs text-slate-500">
              {job.detail}
              {job.pending > 0 ? ` · ${job.pending}` : ""}
            </span>,
          ])}
          empty={t("cp.message.no_activity")}
        />
      </Card>

      {health.tenant_trouble.length > 0 && (
        <Card title={t("cp.section.tenant_trouble")}>
          <Table
            head={[t("cp.field.organisation"), t("cp.field.failures"), t("cp.field.action")]}
            rows={health.tenant_trouble.map((row) => [
              <Link key="t" href={`/cp/tenants/${row.tenant_id}`} className="hover:underline text-slate-900">
                {row.name || row.tenant_id}
              </Link>,
              <span key="c" className="tabular-nums">{row.failures}</span>,
              <span key="s" className="text-xs text-slate-500 font-mono">{row.sample}</span>,
            ])}
            empty={t("cp.message.no_activity")}
          />
        </Card>
      )}

      <div className="grid gap-4 lg:grid-cols-2">
        <Card title={t("cp.section.backups")}>
          <div className="p-4 space-y-2 text-sm">
            {!health.backups.configured ? (
              <p className="rounded-lg bg-amber-50 border border-amber-200 text-amber-900 px-3 py-2">
                {t("cp.message.no_backups")}
              </p>
            ) : (
              <>
                <Row
                  label={t("cp.field.last_backup")}
                  value={`${formatMoment(health.backups.last_backup_at, locale)} · ${health.backups.last_size_mb.toFixed(1)} MB`}
                  tone={health.backups.last_ok ? undefined : "red"}
                />
                <Row
                  label={t("cp.field.last_restore_test")}
                  value={formatMoment(health.backups.last_restore_test_at, locale) || t("cp.message.never_tested")}
                  tone={health.backups.last_restore_test_at ? undefined : "amber"}
                />
              </>
            )}
            <button
              type="button"
              onClick={() =>
                action.run({
                  title: t("cp.action.record_restore_test"),
                  detail: t("cp.hint.restore_test"),
                  perform: (reason) => cp.recordRestoreTest(reason, reason),
                  onDone: load,
                })
              }
              className="mt-2 inline-flex items-center gap-2 rounded-lg border border-slate-300 px-3 py-2 text-sm hover:bg-slate-50"
            >
              <DatabaseBackup className="w-4 h-4" />
              {t("cp.action.record_restore_test")}
            </button>
          </div>
        </Card>

        <Card title={t("cp.section.catalog")}>
          <div className="p-4 space-y-2 text-sm">
            <Row
              label={t("cp.field.last_sync")}
              value={formatMoment(health.catalog.last_sync_at, locale) || health.catalog.detail || "—"}
              tone={health.catalog.ok ? undefined : "red"}
            />
            <div className="pt-2 space-y-1">
              {health.catalog.apps.map((app) => (
                <div key={app.app_id} className="flex items-center gap-2 text-xs">
                  <Boxes className="w-3 h-3 text-slate-400" />
                  <span className="flex-1 truncate text-slate-700">{app.name}</span>
                  <span className="text-slate-500 font-mono">
                    {Object.entries(app.versions)
                      .map(([version, count]) => `${version}×${count}`)
                      .join("  ")}
                  </span>
                </div>
              ))}
            </div>
            <button
              type="button"
              onClick={() =>
                action.run({
                  title: t("cp.action.sync_catalog"),
                  detail: t("cp.hint.sync_catalog"),
                  perform: (reason) => cp.syncCatalog(reason),
                  onDone: load,
                })
              }
              className="mt-2 inline-flex items-center gap-2 rounded-lg border border-slate-300 px-3 py-2 text-sm hover:bg-slate-50"
            >
              <RefreshCw className="w-4 h-4" />
              {t("cp.action.sync_catalog")}
            </button>
          </div>
        </Card>
      </div>

      {action.dialog}
    </div>
  );
}

function Stat({
  label,
  value,
  tone,
  icon,
}: {
  label: string;
  value: string;
  tone?: Tone;
  icon: React.ReactNode;
}) {
  return (
    <div className="bg-white rounded-xl border border-slate-200 shadow-sm px-4 py-3">
      <p className="text-xs uppercase tracking-wide text-slate-400 flex items-center gap-1.5">
        {icon}
        {label}
      </p>
      <p className={`mt-1 text-2xl tabular-nums ${tone === "red" ? "text-red-600" : "text-slate-900"}`}>{value}</p>
    </div>
  );
}

function Row({ label, value, tone }: { label: string; value: string; tone?: Tone }) {
  return (
    <div className="flex items-baseline gap-3">
      <span className="text-xs uppercase tracking-wide text-slate-400 w-40 shrink-0">{label}</span>
      <span className={tone === "red" ? "text-red-700" : tone === "amber" ? "text-amber-700" : "text-slate-800"}>
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
      className="inline-flex items-center gap-1 text-xs text-slate-500 hover:text-slate-900"
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
  if (state === "unknown") return "bg-slate-300";
  return state === "red" ? "bg-red-500" : state === "amber" ? "bg-amber-500" : "bg-emerald-500";
}

function textFor(state: string): string {
  if (state === "unknown") return "text-slate-400";
  return state === "red" ? "text-red-600" : state === "amber" ? "text-amber-700" : "text-slate-900";
}
