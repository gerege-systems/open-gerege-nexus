"use client";

/**
 * Who the platform has been asked to write to, across every organisation.
 *
 * The service behind this is the deployment's — one credential, one provider,
 * one quota — so both of the questions worth asking are platform questions: is
 * it working, and who has it written to. An organisation's own administrator
 * could only ever see a quarter of the answer, which is why the screen moved
 * here.
 *
 * Read-only on purpose. These rows are people's addresses and what they were
 * asked to prove; nothing on this screen deletes one.
 */

import React, { useCallback, useEffect, useState } from "react";
import { CheckCircle2, Clock, ExternalLink, MailCheck, RefreshCw, XCircle } from "lucide-react";

import { Badge, Card, formatMoment, Table } from "@/components/cp/ui";
import { cp, type VerificationLedger } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

export default function Verifications() {
  const { t, locale } = useI18n();
  const [ledger, setLedger] = useState<VerificationLedger | null>(null);
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setBusy(true);
    try {
      setLedger(await cp.verifications(50));
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

  const stats = ledger?.stats;
  const service = ledger?.service;

  return (
    <div className="space-y-6">
      <div className="flex items-start gap-3">
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-slate-900 flex items-center gap-2">
            <MailCheck className="w-6 h-6 text-[var(--gerege-blue)]" />
            {t("cp.section.verifications")}
          </h1>
          <p className="mt-1 text-sm text-slate-500">{t("cp.hint.verifications")}</p>
        </div>
        <button
          type="button"
          onClick={() => void load()}
          disabled={busy}
          className="inline-flex items-center gap-2 rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 hover:bg-slate-50 disabled:opacity-50"
        >
          <RefreshCw className={`w-4 h-4 ${busy ? "animate-spin" : ""}`} />
          {t("cp.action.refresh")}
        </button>
      </div>

      {failure && (
        <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
      )}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Stat label={t("emailverify.stat.total")} value={stats?.total} />
        <Stat label={t("emailverify.stat.verified")} value={stats?.verified} hint={stats ? `${stats.verified_pct.toFixed(0)}%` : ""} />
        <Stat label={t("emailverify.stat.pending")} value={stats?.pending} />
        <Stat label={t("emailverify.stat.last_24h")} value={stats?.last_24h} hint={stats ? t("cp.hint.tenants_touched", { count: String(stats.tenants) }) : ""} />
      </div>

      <Card title={t("emailverify.view.service_title")}>
        <div className="p-4 space-y-2 text-sm">
          <p className="flex items-center gap-2">
            {service?.reachable ? (
              <CheckCircle2 className="w-4 h-4 text-emerald-600" />
            ) : (
              <XCircle className="w-4 h-4 text-red-600" />
            )}
            <span className="text-slate-700">
              {!service?.configured
                ? t("emailverify.message.not_configured")
                : service.reachable
                  ? t("emailverify.message.reachable")
                  : service.health || t("emailverify.message.unreachable", { reason: "" })}
            </span>
          </p>
          {service?.provider_url && (
            <p className="text-xs text-slate-500 font-mono break-all">{service.provider_url}</p>
          )}
          {service?.admin_url && (
            <a
              href={service.admin_url}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 text-sm text-[var(--gerege-blue)] hover:underline"
            >
              {t("emailverify.action.open_admin")}
              <ExternalLink className="w-3.5 h-3.5" />
            </a>
          )}
        </div>
      </Card>

      <Card title={t("emailverify.view.recent_title")}>
        <Table
          head={[
            t("cp.field.organisation"),
            t("emailverify.field.email"),
            t("emailverify.field.purpose"),
            t("cp.field.status"),
            t("cp.field.when"),
          ]}
          rows={(ledger?.recent ?? []).map((row) => [
            <span key="t" className="text-slate-700">
              {row.tenant_name || <span className="text-slate-400">{t("cp.state.deleted")}</span>}
            </span>,
            <span key="e" className="font-mono text-xs">{row.email}</span>,
            <span key="p" className="text-xs text-slate-500">{row.purpose || row.source || "—"}</span>,
            <Badge key="s" tone={row.status === "VERIFIED" ? "emerald" : row.status === "PENDING" ? "amber" : "slate"}>
              <span className="inline-flex items-center gap-1">
                {row.status === "PENDING" && <Clock className="w-3 h-3" />}
                {t(`emailverify.state.${row.status.toLowerCase()}` as "emailverify.state.pending")}
              </span>
            </Badge>,
            formatMoment(row.verified_at || row.created_at, locale),
          ])}
          empty={t("emailverify.message.no_verifications")}
        />
      </Card>
    </div>
  );
}

function Stat({ label, value, hint }: { label: string; value?: number; hint?: string }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-4">
      <p className="text-xs font-semibold uppercase tracking-wider text-slate-400">{label}</p>
      <p className="mt-1 text-2xl font-semibold text-slate-900">{value ?? "—"}</p>
      {hint && <p className="text-xs text-slate-500">{hint}</p>}
    </div>
  );
}
