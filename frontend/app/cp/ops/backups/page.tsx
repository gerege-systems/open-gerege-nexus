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
import { cp, type BackupEntry, type Overview } from "@/lib/cp";
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
import { formatMoment } from "@/lib/datetime";
import { useI18n } from "@/lib/i18n";

export default function Backups() {
  const { t } = useI18n();
  const action = useAction();
  const [history, setHistory] = useState<BackupEntry[]>([]);
  const [status, setStatus] = useState<Overview["backups"] | null>(null);
  const [failure, setFailure] = useState("");
  // Until the first answer, an empty list is not yet a claim that nothing exists.
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setBusy(true);
    try {
      const answer = await cp.backups(50);
      setHistory(answer.backups);
      setLoaded(true);
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
          <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
            <DatabaseBackup className="w-6 h-6 text-accent" />
            {t("cp.section.backups")}
          </h1>
          <p className="mt-1 text-sm text-muted">{t("cp.hint.backups")}</p>
        </div>
        <Button
          onClick={() =>
            action.run({
              title: t("cp.action.record_restore_test"),
              perform: (reason) => cp.recordRestoreTest(reason, reason),
              onDone: load,
            })
          }
          leadingIcon={<ShieldCheck />}
        >
          {t("cp.action.record_restore_test")}
        </Button>
        <Button variant="outline" onClick={() => void load()} loading={busy} leadingIcon={<RefreshCw />}>
          {t("cp.action.refresh")}
        </Button>
      </div>

      {failure && (
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
      )}

      {status && !status.configured && (
        <p className="text-sm rounded-lg bg-warning-soft border border-warning-border text-warning px-4 py-3">
          {t("cp.message.no_backups_configured")}
        </p>
      )}

      <div className="grid gap-4 sm:grid-cols-3">
        <Stat label={t("cp.field.last_backup")} value={formatMoment(status?.last_backup_at) || "—"} />
        <Stat label={t("cp.field.size")} value={status?.last_size_mb ? `${status.last_size_mb.toFixed(1)} MB` : "—"} />
        <Stat
          label={t("cp.field.last_restore_test")}
          value={formatMoment(status?.last_restore_test_at) || t("cp.state.never")}
          warn={!status?.last_restore_test_at}
        />
      </div>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.history")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.when")}</TableHead>
              <TableHead>{t("cp.field.kind")}</TableHead>
              <TableHead>{t("cp.field.size")}</TableHead>
              <TableHead>{t("cp.field.status")}</TableHead>
              <TableHead>{t("cp.field.detail")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {history.map((entry, index) => (
              <TableRow key={index}>
                <TableCell>
                  {formatMoment(entry.started_at)}
                </TableCell>
                <TableCell>
                  <Badge tone={entry.kind === "backup" ? "neutral" : "success"}>
                    {t(entry.kind === "backup" ? "cp.kind.backup" : "cp.kind.restore_test")}
                  </Badge>
                </TableCell>
                <TableCell>
                  {entry.size_mb ? `${entry.size_mb.toFixed(1)} MB` : "—"}
                </TableCell>
                <TableCell>
                  <Badge tone={entry.ok ? "success" : "danger"}>
                    {entry.ok ? t("cp.state.normal") : t("cp.state.failing")}
                  </Badge>
                </TableCell>
                <TableCell>
                  <span className="text-xs text-muted wrap-break-word">{entry.detail || "—"}</span>
                </TableCell>
              </TableRow>
            ))}
            {!loaded && !failure && (
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
            {loaded && history.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="py-8 text-center text-muted">
                  {t("cp.message.no_backups")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      {action.dialog}
    </div>
  );
}

function Stat({ label, value, warn }: { label: string; value: string; warn?: boolean }) {
  return (
    <div className="rounded-lg border border-line bg-surface p-4">
      <p className="text-xs font-semibold uppercase tracking-wider text-muted">{label}</p>
      <p className={`mt-1 text-lg font-semibold ${warn ? "text-warning" : "text-foreground"}`}>{value}</p>
    </div>
  );
}
