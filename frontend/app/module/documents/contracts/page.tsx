"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation";
import { contracts, ContractRow } from "@/lib/contracts";
import { useResource, useLoadOnMount } from "@/lib/useResource";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { PageHeader } from "@/components/ui";
import { ListEmpty, ListSkeleton } from "@/components/documents/shared";
import { ContractBadge, fmtDate, fmtMoney } from "@/components/documents/contracts";
import {
  Alert,
  Button,
  Card,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  ErrorState,
  Input,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@gerege-systems/ui";
import { FileSignature, Plus } from "lucide-react";

/**
 * The issuer's register: every contract this organisation has drawn up, one
 * row each, newest first. A row opens the contract's own page; the button
 * starts a new one with nothing but a title — everything else (facts, text,
 * parties) belongs to the contract page, because that is where it is edited.
 */
/**
 * Тараагдсан гэрээнүүд эцгийнхээ АРД жагсана — 800 хүүхэд гэрээ жагсаалтыг
 * живүүлэлгүй, аль тараалтынх нь тодорхой харагдана. Эцэггүй мөрүүд өөрийн
 * дарааллаараа (шинэ нь эхэндээ) үлдэнэ.
 */
function orderByFamily(rows: ContractRow[]): ContractRow[] {
  const children = new Map<string, ContractRow[]>();
  for (const row of rows) {
    if (row.parent_document_id) {
      children.set(row.parent_document_id, [...(children.get(row.parent_document_id) ?? []), row]);
    }
  }
  const ordered: ContractRow[] = [];
  for (const row of rows) {
    if (row.parent_document_id && children.has(row.parent_document_id)) continue;
    ordered.push(row, ...(children.get(row.id) ?? []));
  }
  // Эцэг нь жагсаалтад байхгүй (хуучин өгөгдөл) хүүхэд орхигдох ёсгүй.
  for (const row of rows) {
    if (row.parent_document_id && !rows.some((candidate) => candidate.id === row.parent_document_id)) {
      ordered.push(row);
    }
  }
  return ordered;
}

export default function ContractsPage() {
  const { t } = useI18n();
  const { can } = useAccess();
  const router = useRouter();
  const mayManage = can("documents.manage");

  const list = useResource<ContractRow[]>(
    async () => (await contracts.list()).contracts,
    { initial: [] },
  );
  useLoadOnMount(list.reload);

  const [creating, setCreating] = useState(false);
  const [title, setTitle] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const create = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!title.trim()) return;
    setBusy(true);
    setError(null);
    try {
      const doc = await contracts.create(title.trim());
      router.push(`/module/documents/contracts/${doc.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setBusy(false);
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<FileSignature className="w-6 h-6 text-accent" />}
        title={t("contracts.view.title")}
        subtitle={t("contracts.view.subtitle")}
        actions={
          mayManage ? (
            <Button onClick={() => setCreating(true)} leadingIcon={<Plus />}>
              {t("contracts.view.new")}
            </Button>
          ) : undefined
        }
      />

      {list.loading ? (
        <ListSkeleton />
      ) : list.failed ? (
        <ErrorState
          title={t("contracts.msg.load_failed")}
          description=""
          live
          action={
            <Button variant="outline" onClick={() => void list.reload()}>
              {t("base.action.retry")}
            </Button>
          }
        />
      ) : list.data.length === 0 ? (
        <ListEmpty icon={<FileSignature />} title={t("contracts.view.empty")} />
      ) : (
        <Card padding="none" className="overflow-hidden">
          <Table containerClassName="rounded-none border-0" className="text-xs" scrollLabel={t("contracts.view.title")}>
            <TableHeader>
              <TableRow>
                <TableHead>{t("contracts.col.contract")}</TableHead>
                <TableHead>{t("contracts.col.parties")}</TableHead>
                <TableHead>{t("contracts.col.state")}</TableHead>
                <TableHead>{t("contracts.col.signatures")}</TableHead>
                <TableHead>{t("contracts.col.amount")}</TableHead>
                <TableHead>{t("contracts.col.date")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {orderByFamily(list.data).map((row) => (
                <TableRow
                  key={row.id}
                  onClick={() => router.push(`/module/documents/contracts/${row.id}`)}
                  className="cursor-pointer"
                >
                  <TableCell>
                    <div className={`font-semibold text-foreground ${row.parent_document_id ? "ps-5 relative" : ""}`}>
                      {row.parent_document_id && <span className="absolute inset-s-0 text-subtle">↳</span>}
                      {row.parent_document_id ? row.counterparties || row.title : row.title}
                    </div>
                    {!row.parent_document_id && row.contract_number && (
                      <div className="text-xs text-muted">№ {row.contract_number}</div>
                    )}
                    {!row.parent_document_id && (row.issued_count ?? 0) > 0 && (
                      <div className="text-xs text-accent font-semibold">
                        {t("contracts.list.issued", { total: row.issued_count ?? 0, executed: row.issued_executed ?? 0 })}
                      </div>
                    )}
                  </TableCell>
                  <TableCell className="text-muted">{row.parent_document_id ? "" : row.counterparties || "—"}</TableCell>
                  <TableCell><ContractBadge state={row.contract_state} /></TableCell>
                  <TableCell className="font-mono text-muted">{row.signed_count} / {row.required_count}</TableCell>
                  <TableCell className="text-muted">{fmtMoney(row.amount, row.currency)}</TableCell>
                  <TableCell className="text-muted">{fmtDate(row.sent_at || row.created_at)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}

      {creating && (
        <Dialog open onOpenChange={(open) => { if (!open) setCreating(false); }}>
          <DialogContent
            showClose={false}
            aria-describedby={undefined}
            // A stray click outside must not throw away a typed title.
            onInteractOutside={(event) => { if (title.trim()) event.preventDefault(); }}
          >
            <form onSubmit={create} className="space-y-4">
              <DialogHeader>
                <DialogTitle>{t("contracts.view.new")}</DialogTitle>
              </DialogHeader>
              <Input
                autoFocus
                label={t("contracts.field.title")}
                helperText={t("contracts.field.title_hint")}
                value={title}
                onChange={(event) => setTitle(event.target.value)}
              />
              {error && <Alert variant="danger" live>{error}</Alert>}
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setCreating(false)}>
                  {t("contracts.action.cancel")}
                </Button>
                <Button type="submit" loading={busy} disabled={!title.trim()}>
                  {t("contracts.action.create")}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}
