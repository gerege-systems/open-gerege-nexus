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

import { cp, type Installation } from "@/lib/cp";
import {
  Badge,
  Card,
  CardHeader,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
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

const EVERY_APP = "__none__";

export default function Installations() {
  const { t } = useI18n();
  const [installations, setInstallations] = useState<Installation[]>([]);
  const [app, setApp] = useState("");
  const [failure, setFailure] = useState("");
  const [loaded, setLoaded] = useState(false);

  const load = useCallback(async () => {
    try {
      setInstallations((await cp.installations()).installations);
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setLoaded(true);
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
          <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
            <PackageCheck className="w-6 h-6 text-accent" />
            {t("cp.section.installations")}
          </h1>
          <p className="mt-1 text-sm text-muted">{t("cp.hint.installations")}</p>
        </div>
        {/* Radix refuses an empty value, so "every app" is a marker. */}
        <Select value={app || EVERY_APP} onValueChange={(value) => setApp(value === EVERY_APP ? "" : value)}>
          <SelectTrigger aria-label={t("cp.field.app")} className="w-auto min-w-48" />
          <SelectContent>
            <SelectItem value={EVERY_APP}>{t("cp.state.every_app")}</SelectItem>
            {apps.map(([id, name]) => (
              <SelectItem key={id} value={id}>{name}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {failure && (
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
      )}

      {versions.length > 1 && (
        <p className="text-sm rounded-lg bg-warning-soft border border-warning-border text-warning px-4 py-3">
          {t("cp.message.versions_in_the_field", {
            versions: versions.map(([version, count]) => `${version} × ${count}`).join(", "),
          })}
        </p>
      )}

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.installations")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.organisation")}</TableHead>
              <TableHead>{t("cp.field.app")}</TableHead>
              <TableHead>{t("cp.field.version")}</TableHead>
              <TableHead>{t("cp.field.status")}</TableHead>
              <TableHead>{t("cp.field.when")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {shown.map((item, index) => (
              <TableRow key={index}>
                <TableCell>
                  <span className="min-w-0">
                    <Link href={`/cp/tenants/${item.tenant_id}`} className="font-medium text-accent hover:underline">
                      {item.tenant_name}
                    </Link>
                    <span className="block text-xs text-muted font-mono">{item.slug}</span>
                  </span>
                </TableCell>
                <TableCell>
                  <span className="min-w-0">
                    <strong className="text-foreground">{item.app_name}</strong>
                    <span className="block text-xs text-muted font-mono">{item.app_id}</span>
                  </span>
                </TableCell>
                <TableCell>
                  <span className="font-mono text-xs">{item.installed_version}</span>
                </TableCell>
                <TableCell>
                  <Badge tone={!item.enabled ? "neutral" : item.status === "installed" ? "success" : "warning"}>
                    {item.enabled ? item.status : t("cp.state.off")}
                  </Badge>
                </TableCell>
                <TableCell>
                  {formatMoment(item.updated_at)}
                </TableCell>
              </TableRow>
            ))}
            {!loaded && (
              <TableRow>
                <TableCell colSpan={5}>
                  <div role="status" aria-busy="true" className="space-y-3 py-2">
                    <span className="sr-only">{t("base.message.loading")}</span>
                    <Skeleton className="h-4" />
                    <Skeleton className="h-4" />
                    <Skeleton className="h-4" />
                  </div>
                </TableCell>
              </TableRow>
            )}
            {loaded && shown.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="py-8 text-center text-muted">
                  {t("cp.message.no_installations")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>
    </div>
  );
}
