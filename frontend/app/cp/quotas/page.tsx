"use client";

/**
 * Which limits are set where.
 *
 * The organisation's own page carries its limits, which answers "is this one
 * near its ceiling". It cannot answer "who has no ceiling at all" — and a
 * platform where nobody has looked at that is one where the first limit is
 * discovered during a billing dispute.
 *
 * Editing stays on the organisation's page: setting a limit is a change to one
 * organisation, with a reason, and a grid of inputs is how somebody sets the
 * wrong organisation's.
 */

import React, { useCallback, useEffect, useState } from "react";
import { Scale } from "lucide-react";
import Link from "next/link";

import { cp, type QuotaLine } from "@/lib/cp";
import {
  Badge,
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

export default function Quotas() {
  const { t } = useI18n();
  const [quotas, setQuotas] = useState<QuotaLine[]>([]);
  const [failure, setFailure] = useState("");
  // Until the first answer, an empty list is not yet a claim that nothing exists.
  const [loaded, setLoaded] = useState(false);

  const load = useCallback(async () => {
    try {
      setQuotas((await cp.quotas()).quotas);
      setLoaded(true);
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const unlimited = quotas.filter((line) => line.max_users === null).length;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
          <Scale className="w-6 h-6 text-accent" />
          {t("cp.section.quotas")}
        </h1>
        <p className="mt-1 text-sm text-muted">{t("cp.hint.quotas")}</p>
      </div>

      {failure && (
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
      )}

      {unlimited > 0 && (
        <p className="text-sm rounded-lg bg-warning-soft border border-warning-border text-warning px-4 py-3">
          {t("cp.message.unlimited_count", { count: String(unlimited) })}
        </p>
      )}

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.quotas")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.organisation")}</TableHead>
              <TableHead>{t("cp.metric.active_users")}</TableHead>
              <TableHead>{t("cp.metric.storage_mb")}</TableHead>
              <TableHead>{t("cp.metric.ai_calls")}</TableHead>
              <TableHead>{t("cp.field.enforcement")}</TableHead>
              <TableHead>{t("cp.field.updated")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {quotas.map((line, index) => (
              <TableRow key={index}>
                <TableCell>
                  <span className="min-w-0">
                    <Link href={`/cp/tenants/${line.tenant_id}`} className="font-medium text-accent hover:underline">
                      {line.tenant_name}
                    </Link>
                    <span className="block text-xs text-muted font-mono">{line.slug}</span>
                    {line.suspended && <Badge tone="danger">{t("cp.state.suspended")}</Badge>}
                  </span>
                </TableCell>
                <TableCell>
                  <span className="tabular-nums">
                    {line.users}
                    {line.max_users === null ? (
                      <span className="text-muted"> / {t("cp.state.no_limit")}</span>
                    ) : (
                      <span className={line.users > line.max_users ? "text-danger font-semibold" : "text-muted"}>
                        {" "}/ {line.max_users}
                      </span>
                    )}
                  </span>
                </TableCell>
                <TableCell>
                  {line.max_storage_mb === null ? <span className="text-muted">{t("cp.state.no_limit")}</span> : `${line.max_storage_mb} MB`}
                </TableCell>
                <TableCell>
                  {line.max_ai_calls_monthly === null ? <span className="text-muted">{t("cp.state.no_limit")}</span> : String(line.max_ai_calls_monthly)}
                </TableCell>
                <TableCell>
                  <Badge tone={line.enforcement === "hard" ? "danger" : "neutral"}>
                    {t(line.enforcement === "hard" ? "cp.state.hard" : "cp.state.soft")}
                  </Badge>
                </TableCell>
                <TableCell>
                  {formatMoment(line.updated_at)}
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
            {loaded && quotas.length === 0 && (
              <TableRow>
                <TableCell colSpan={6} className="py-8 text-center text-muted">
                  {t("cp.message.no_tenants")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>
    </div>
  );
}
