"use client";

/**
 * Which app is installed where.
 *
 * The catalogue screen counts versions across the platform — "three
 * organisations on 1.2.0, one on 1.1.0" — which says something is behind
 * without saying which one. This is that list, and it is also the answer to
 * "who is actually using this app" on the morning somebody proposes retiring
 * it.
 */

import React, { useCallback, useEffect, useMemo, useState } from "react";
import { PackageCheck } from "lucide-react";
import Link from "next/link";

import { Badge, Card, formatMoment, Table } from "@/components/cp/ui";
import { cp, type Installation } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

export default function Installations() {
  const { t, locale } = useI18n();
  const [installations, setInstallations] = useState<Installation[]>([]);
  const [app, setApp] = useState("");
  const [failure, setFailure] = useState("");

  const load = useCallback(async () => {
    try {
      setInstallations((await cp.installations()).installations);
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const apps = useMemo(() => {
    const names = new Map<string, string>();
    installations.forEach((item) => names.set(item.app_id, item.app_name));
    return [...names.entries()].sort((one, two) => one[1].localeCompare(two[1]));
  }, [installations]);

  // Which versions of one app are in the field. Two rows here is the state the
  // catalogue screen's count is trying to describe.
  const versions = useMemo(() => {
    const counts = new Map<string, number>();
    installations
      .filter((item) => !app || item.app_id === app)
      .forEach((item) => counts.set(item.installed_version, (counts.get(item.installed_version) ?? 0) + 1));
    return [...counts.entries()].sort((one, two) => two[1] - one[1]);
  }, [installations, app]);

  const shown = installations.filter((item) => !app || item.app_id === app);

  return (
    <div className="space-y-6">
      <div className="flex items-start gap-3">
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-slate-900 flex items-center gap-2">
            <PackageCheck className="w-6 h-6 text-[var(--gerege-blue)]" />
            {t("cp.section.installations")}
          </h1>
          <p className="mt-1 text-sm text-slate-500">{t("cp.hint.installations")}</p>
        </div>
        <select
          value={app}
          onChange={(event) => setApp(event.target.value)}
          aria-label={t("cp.field.app")}
          className="rounded-lg border border-slate-300 px-3 py-2 text-sm"
        >
          <option value="">{t("cp.state.every_app")}</option>
          {apps.map(([id, name]) => (
            <option key={id} value={id}>{name}</option>
          ))}
        </select>
      </div>

      {failure && (
        <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
      )}

      {versions.length > 1 && (
        <p className="text-sm rounded-xl bg-amber-50 border border-amber-200 text-amber-900 px-4 py-3">
          {t("cp.message.versions_in_the_field", {
            versions: versions.map(([version, count]) => `${version} × ${count}`).join(", "),
          })}
        </p>
      )}

      <Card title={t("cp.section.installations")}>
        <Table
          head={[
            t("cp.field.organisation"),
            t("cp.field.app"),
            t("cp.field.version"),
            t("cp.field.status"),
            t("cp.field.when"),
          ]}
          rows={shown.map((item) => [
            <span key="n" className="min-w-0">
              <Link href={`/cp/tenants/${item.tenant_id}`} className="font-medium text-[var(--gerege-blue)] hover:underline">
                {item.tenant_name}
              </Link>
              <span className="block text-xs text-slate-500 font-mono">{item.slug}</span>
            </span>,
            <span key="a" className="min-w-0">
              <strong className="text-slate-900">{item.app_name}</strong>
              <span className="block text-xs text-slate-500 font-mono">{item.app_id}</span>
            </span>,
            <span key="v" className="font-mono text-xs">{item.installed_version}</span>,
            <Badge key="s" tone={!item.enabled ? "slate" : item.status === "installed" ? "emerald" : "amber"}>
              {item.enabled ? item.status : t("cp.state.off")}
            </Badge>,
            formatMoment(item.updated_at, locale),
          ])}
          empty={t("cp.message.no_installations")}
        />
      </Card>
    </div>
  );
}
