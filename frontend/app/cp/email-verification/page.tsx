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

import { cp, type VerificationLedger } from "@/lib/cp";
import { formatMoment } from "@/lib/datetime";
import { useI18n } from "@/lib/i18n";

export default function Verifications() {
  const { t } = useI18n();
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
          <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
            <MailCheck className="w-6 h-6 text-accent" />
            {t("cp.section.verifications")}
          </h1>
          <p className="mt-1 text-sm text-muted">{t("cp.hint.verifications")}</p>
        </div>
        <Button
          type="button"
          variant="outline"
          onClick={() => void load()}
          disabled={busy}
          leadingIcon={<RefreshCw className={`w-4 h-4 ${busy ? "animate-spin" : ""}`} />}
        >
          {t("cp.action.refresh")}
        </Button>
      </div>

      {failure && (
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
      )}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Stat label={t("emailverify.stat.total")} value={stats?.total} />
        <Stat label={t("emailverify.stat.verified")} value={stats?.verified} hint={stats ? `${stats.verified_pct.toFixed(0)}%` : ""} />
        <Stat label={t("emailverify.stat.pending")} value={stats?.pending} />
        <Stat label={t("emailverify.stat.last_24h")} value={stats?.last_24h} hint={stats ? t("cp.hint.tenants_touched", { count: String(stats.tenants) }) : ""} />
      </div>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("emailverify.view.service_title")}</h2>
        </CardHeader>
        <div className="p-4 space-y-2 text-sm">
          <p className="flex items-center gap-2">
            {service?.reachable ? (
              <CheckCircle2 className="w-4 h-4 text-success" />
            ) : (
              <XCircle className="w-4 h-4 text-danger" />
            )}
            <span className="text-foreground">
              {!service?.configured
                ? t("emailverify.message.not_configured")
                : service.reachable
                  ? t("emailverify.message.reachable")
                  : service.health || t("emailverify.message.unreachable", { reason: "" })}
            </span>
          </p>
          {service?.provider_url && (
            <p className="text-xs text-muted font-mono break-all">{service.provider_url}</p>
          )}
          {service?.admin_url && (
            <a
              href={service.admin_url}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 text-sm text-accent hover:underline"
            >
              {t("emailverify.action.open_admin")}
              <ExternalLink className="w-3.5 h-3.5" />
            </a>
          )}
        </div>
      </Card>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("emailverify.view.recent_title")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.organisation")}</TableHead>
              <TableHead>{t("emailverify.field.email")}</TableHead>
              <TableHead>{t("emailverify.field.purpose")}</TableHead>
              <TableHead>{t("cp.field.status")}</TableHead>
              <TableHead>{t("cp.field.when")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(ledger?.recent ?? []).map((row) => (
              <TableRow key={row.id}>
                <TableCell>
                  <span className="text-foreground">
                    {row.tenant_name || <span className="text-muted">{t("cp.state.deleted")}</span>}
                  </span>
                </TableCell>
                <TableCell>
                  <span className="font-mono text-xs">{row.email}</span>
                </TableCell>
                <TableCell>
                  <span className="text-xs text-muted">{row.purpose || row.source || "—"}</span>
                </TableCell>
                <TableCell>
                  <Badge
                    tone={row.status === "VERIFIED" ? "success" : row.status === "PENDING" ? "warning" : "neutral"}
                    icon={row.status === "PENDING" ? <Clock /> : undefined}
                  >
                    {t(`emailverify.state.${row.status.toLowerCase()}` as "emailverify.state.pending")}
                  </Badge>
                </TableCell>
                <TableCell>{formatMoment(row.verified_at || row.created_at)}</TableCell>
              </TableRow>
            ))}
            {!ledger && !failure && (
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
            {ledger && ledger.recent.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="py-8 text-center text-muted">
                  {t("emailverify.message.no_verifications")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>
    </div>
  );
}

function Stat({ label, value, hint }: { label: string; value?: number; hint?: string }) {
  return (
    <div className="rounded-lg border border-line bg-surface p-4">
      <p className="text-xs font-semibold uppercase tracking-wider text-muted">{label}</p>
      <p className="mt-1 text-2xl font-semibold text-foreground">{value ?? "—"}</p>
      {hint && <p className="text-xs text-muted">{hint}</p>}
    </div>
  );
}
