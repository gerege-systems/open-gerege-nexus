"use client";

import React, { useCallback, useEffect, useState } from "react";
import { Download, ScrollText } from "lucide-react";
import { esign, saveBlob, type LogFilter, type SignatureLogEntry } from "@/lib/esign";
import { useI18n } from "@/lib/i18n";
import { PageHeader } from "@/components/ui";
import { ListSkeleton, SelectField } from "@/components/documents/shared";
import {
  Alert,
  Button,
  Card,
  EmptyState,
  Input,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@gerege-systems/ui";
import { OutcomeBadge, Pager, useErrorMessage } from "@/components/esign/shared";

const PAGE_SIZE = 50;

/**
 * The signature log — every certificate check, ceremony and download with its
 * outcome.
 *
 * Failures are the point. The original log only ever recorded successes, so a
 * refused or expired signature left no trace, which is exactly the event an
 * auditor comes here looking for.
 */
export default function EsignLogsPage() {
  const { t } = useI18n();
  const describe = useErrorMessage();
  const [entries, setEntries] = useState<SignatureLogEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [filter, setFilter] = useState<LogFilter>({});
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(
    async (nextOffset: number, nextFilter: LogFilter) => {
      setLoading(true);
      try {
        const page = await esign.logs({ ...nextFilter, limit: PAGE_SIZE, offset: nextOffset });
        setEntries(page.items || []);
        setTotal(page.total);
        setOffset(nextOffset);
      } catch (err) {
        setError(describe(err, t("base.message.error")));
      } finally {
        setLoading(false);
      }
    },
    [describe, t],
  );

  useEffect(() => {
    void load(0, {});
  }, [load]);

  const applyFilter = (patch: Partial<LogFilter>) => {
    const next = { ...filter, ...patch };
    setFilter(next);
    // Any filter change resets to the first page; keeping the offset would
    // land on an empty page whenever the result set shrank.
    void load(0, next);
  };

  const submitSearch = (event: React.FormEvent) => {
    event.preventDefault();
    applyFilter({ q: search.trim() || undefined });
  };

  const exportCsv = async () => {
    try {
      saveBlob(await esign.exportLogs(filter), "esign-signature-log.csv");
    } catch (err) {
      setError(describe(err, t("base.message.error")));
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ScrollText className="w-7 h-7 text-accent" />}
        title={t("esign.view.logs_title")}
        subtitle={t("esign.view.logs_subtitle")}
        actions={
          <Button variant="outline" onClick={exportCsv} leadingIcon={<Download />}>
            {t("esign.action.export_csv")}
          </Button>
        }
      />

      {error && <Alert variant="danger" live dismissible onDismiss={() => setError(null)}>{error}</Alert>}

      <Card padding="sm" className="flex flex-wrap gap-3 items-end">
        <form onSubmit={submitSearch} className="flex-1 min-w-56">
          <Input
            id="log-search"
            type="search"
            label={t("base.action.search")}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t("esign.field.search_placeholder")}
            clearable
            onClear={() => setSearch("")}
          />
        </form>

        <SelectField
          id="log-action"
          label={t("esign.field.action")}
          value={filter.action ?? ""}
          onValueChange={(value) => applyFilter({ action: value || undefined })}
          options={[
            { value: "", label: t("base.label.all") },
            { value: "SIGN", label: t("esign.action_type.sign") },
            { value: "SIGN_START", label: t("esign.action_type.sign_start") },
            { value: "BATCH_SIGN", label: t("esign.action_type.batch_sign") },
            { value: "CERT_CHECK", label: t("esign.action_type.cert_check") },
            { value: "DOWNLOAD", label: t("esign.action_type.download") },
          ]}
        />

        <SelectField
          id="log-outcome"
          label={t("esign.field.outcome")}
          value={filter.outcome ?? ""}
          onValueChange={(value) => applyFilter({ outcome: value || undefined })}
          options={[
            { value: "", label: t("base.label.all") },
            { value: "OK", label: t("esign.outcome.ok") },
            { value: "FAILED", label: t("esign.outcome.failed") },
            { value: "REJECTED", label: t("esign.outcome.rejected") },
            { value: "EXPIRED", label: t("esign.outcome.expired") },
            { value: "CANCELLED", label: t("esign.outcome.cancelled") },
          ]}
        />

        <SelectField
          id="log-provider"
          label={t("esign.field.provider")}
          value={filter.provider ?? ""}
          onValueChange={(value) => applyFilter({ provider: value || undefined })}
          options={[
            { value: "", label: t("base.label.all") },
            { value: "EID", label: "eID Mongolia" },
            { value: "HSM", label: "Gerege eSign HSM" },
          ]}
        />

        <Input
          id="log-from"
          type="date"
          label={t("esign.field.from")}
          value={filter.from ?? ""}
          onChange={(event) => applyFilter({ from: event.target.value || undefined })}
          className="w-auto"
        />
        <Input
          id="log-to"
          type="date"
          label={t("esign.field.to")}
          value={filter.to ?? ""}
          onChange={(event) => applyFilter({ to: event.target.value || undefined })}
          className="w-auto"
        />
      </Card>

      {loading ? (
        <ListSkeleton label={t("base.message.loading")} rows={6} />
      ) : entries.length === 0 ? (
        <Card>
          <EmptyState icon={<ScrollText />} title={t("esign.message.logs_empty")} />
        </Card>
      ) : (
        <Card padding="none" className="overflow-hidden">
          <Table containerClassName="rounded-none border-0" className="text-xs" scrollLabel={t("esign.view.logs_title")}>
            <TableHeader>
              <TableRow>
                <TableHead>{t("base.field.date")}</TableHead>
                <TableHead>{t("esign.field.action")}</TableHead>
                <TableHead>{t("esign.field.outcome")}</TableHead>
                <TableHead>{t("esign.field.document")}</TableHead>
                <TableHead>{t("esign.field.signer")}</TableHead>
                <TableHead>{t("esign.field.provider")}</TableHead>
                <TableHead>{t("esign.field.detail")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {entries.map((entry) => (
                <TableRow key={entry.id}>
                  <TableCell className="whitespace-nowrap text-muted">
                    {new Date(entry.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell className="font-semibold">{entry.action}</TableCell>
                  <TableCell>
                    <OutcomeBadge outcome={entry.outcome} />
                  </TableCell>
                  <TableCell>{entry.document_title || <span className="text-muted">—</span>}</TableCell>
                  <TableCell>
                    {[entry.last_name, entry.first_name].filter(Boolean).join(" ") || entry.reg_no || (
                      <span className="text-muted">—</span>
                    )}
                    {entry.reg_no && (entry.first_name || entry.last_name) && (
                      <div className="text-xs text-muted font-mono">{entry.reg_no}</div>
                    )}
                  </TableCell>
                  <TableCell className="font-mono">{entry.provider}</TableCell>
                  <TableCell className="text-muted max-w-xs truncate" title={entry.detail}>
                    {entry.detail || <span className="text-muted">—</span>}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}

      <Pager
        total={total}
        offset={offset}
        pageSize={PAGE_SIZE}
        onPage={(next) => load(next, filter)}
      />
    </div>
  );
}
