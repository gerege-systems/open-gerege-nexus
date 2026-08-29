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

import { Badge, Card, formatMoment, Table } from "@/components/cp/ui";
import { cp, type QuotaLine } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

export default function Quotas() {
  const { t, locale } = useI18n();
  const [quotas, setQuotas] = useState<QuotaLine[]>([]);
  const [failure, setFailure] = useState("");

  const load = useCallback(async () => {
    try {
      setQuotas((await cp.quotas()).quotas);
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
        <h1 className="text-2xl font-semibold text-slate-900 flex items-center gap-2">
          <Scale className="w-6 h-6 text-[var(--gerege-blue)]" />
          {t("cp.section.quotas")}
        </h1>
        <p className="mt-1 text-sm text-slate-500">{t("cp.hint.quotas")}</p>
      </div>

      {failure && (
        <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
      )}

      {unlimited > 0 && (
        <p className="text-sm rounded-xl bg-amber-50 border border-amber-200 text-amber-900 px-4 py-3">
          {t("cp.message.unlimited_count", { count: String(unlimited) })}
        </p>
      )}

      <Card title={t("cp.section.quotas")}>
        <Table
          head={[
            t("cp.field.organisation"),
            t("cp.metric.active_users"),
            t("cp.metric.storage_mb"),
            t("cp.metric.ai_calls"),
            t("cp.field.enforcement"),
            t("cp.field.updated"),
          ]}
          rows={quotas.map((line) => [
            <span key="n" className="min-w-0">
              <Link href={`/cp/tenants/${line.tenant_id}`} className="font-medium text-[var(--gerege-blue)] hover:underline">
                {line.tenant_name}
              </Link>
              <span className="block text-xs text-slate-500 font-mono">{line.slug}</span>
              {line.suspended && <Badge tone="red">{t("cp.state.suspended")}</Badge>}
            </span>,
            <span key="u" className="tabular-nums">
              {line.users}
              {line.max_users === null ? (
                <span className="text-slate-400"> / {t("cp.state.no_limit")}</span>
              ) : (
                <span className={line.users > line.max_users ? "text-red-600 font-semibold" : "text-slate-500"}>
                  {" "}/ {line.max_users}
                </span>
              )}
            </span>,
            line.max_storage_mb === null ? <span key="s" className="text-slate-400">{t("cp.state.no_limit")}</span> : `${line.max_storage_mb} MB`,
            line.max_ai_calls_monthly === null ? <span key="a" className="text-slate-400">{t("cp.state.no_limit")}</span> : String(line.max_ai_calls_monthly),
            <Badge key="e" tone={line.enforcement === "hard" ? "red" : "slate"}>
              {t(line.enforcement === "hard" ? "cp.state.hard" : "cp.state.soft")}
            </Badge>,
            formatMoment(line.updated_at, locale),
          ])}
          empty={t("cp.message.no_tenants")}
        />
      </Card>
    </div>
  );
}
