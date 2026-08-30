"use client";

/**
 * What has been kept, and whether anybody has checked that it restores.
 *
 * The front page shows the latest of each, which answers "was there a backup
 * last night". This is the history, which answers "has this been failing" —
 * the question actually asked on the morning somebody needs one. An untested
 * backup is not a backup, so the restore test is recorded here too: by hand,
 * because only a person can say that a restore worked.
 */

import React, { useCallback, useEffect, useState } from "react";
import { DatabaseBackup, RefreshCw, ShieldCheck } from "lucide-react";

import { useAction } from "@/components/cp/Action";
import { Badge, Card, formatMoment, Table } from "@/components/cp/ui";
import { cp, type BackupEntry, type Overview } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

export default function Backups() {
  const { t, locale } = useI18n();
  const action = useAction();
  const [history, setHistory] = useState<BackupEntry[]>([]);
  const [status, setStatus] = useState<Overview["backups"] | null>(null);
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setBusy(true);
    try {
      const answer = await cp.backups(50);
      setHistory(answer.backups);
      setStatus(answer.status);
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
          <h1 className="text-2xl font-semibold text-slate-900 flex items-center gap-2">
            <DatabaseBackup className="w-6 h-6 text-[var(--gerege-blue)]" />
            {t("cp.section.backups")}
          </h1>
          <p className="mt-1 text-sm text-slate-500">{t("cp.hint.backups")}</p>
        </div>
        <button
          type="button"
          onClick={() =>
            action.run({
              title: t("cp.action.record_restore_test"),
              perform: (reason) => cp.recordRestoreTest(reason, reason),
              onDone: load,
            })
          }
          className="inline-flex items-center gap-2 rounded-lg bg-[var(--gerege-blue)] px-3 py-2 text-sm font-medium text-[var(--gerege-on-blue)] hover:brightness-105"
        >
          <ShieldCheck className="w-4 h-4" />
          {t("cp.action.record_restore_test")}
        </button>
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

      {status && !status.configured && (
        <p className="text-sm rounded-xl bg-amber-50 border border-amber-200 text-amber-900 px-4 py-3">
          {t("cp.message.no_backups_configured")}
        </p>
      )}

      <div className="grid gap-4 sm:grid-cols-3">
        <Stat label={t("cp.field.last_backup")} value={formatMoment(status?.last_backup_at, locale) || "—"} />
        <Stat label={t("cp.field.size")} value={status?.last_size_mb ? `${status.last_size_mb.toFixed(1)} MB` : "—"} />
        <Stat
          label={t("cp.field.last_restore_test")}
          value={formatMoment(status?.last_restore_test_at, locale) || t("cp.state.never")}
          warn={!status?.last_restore_test_at}
        />
      </div>

      <Card title={t("cp.section.history")}>
        <Table
          head={[t("cp.field.when"), t("cp.field.kind"), t("cp.field.size"), t("cp.field.status"), t("cp.field.detail")]}
          rows={history.map((entry) => [
            formatMoment(entry.started_at, locale),
            <Badge key="k" tone={entry.kind === "backup" ? "slate" : "emerald"}>
              {t(entry.kind === "backup" ? "cp.kind.backup" : "cp.kind.restore_test")}
            </Badge>,
            entry.size_mb ? `${entry.size_mb.toFixed(1)} MB` : "—",
            <Badge key="s" tone={entry.ok ? "emerald" : "red"}>
              {entry.ok ? t("cp.state.normal") : t("cp.state.failing")}
            </Badge>,
            <span key="d" className="text-xs text-slate-500 break-words">{entry.detail || "—"}</span>,
          ])}
          empty={t("cp.message.no_backups")}
        />
      </Card>

      {action.dialog}
    </div>
  );
}

function Stat({ label, value, warn }: { label: string; value: string; warn?: boolean }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-4">
      <p className="text-xs font-semibold uppercase tracking-wider text-slate-400">{label}</p>
      <p className={`mt-1 text-lg font-semibold ${warn ? "text-amber-700" : "text-slate-900"}`}>{value}</p>
    </div>
  );
}
