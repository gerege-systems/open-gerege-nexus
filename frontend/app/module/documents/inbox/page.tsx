"use client";

import React from "react";
import { useRouter } from "next/navigation";
import { contracts, InboxItem } from "@/lib/contracts";
import { useResource, useLoadOnMount } from "@/lib/useResource";
import { useI18n } from "@/lib/i18n";
import { PageHeader } from "@/components/ui";
import { ListEmpty, ListSkeleton } from "@/components/documents/shared";
import { PartyBadge, fmtDate } from "@/components/documents/contracts";
import {
  Button,
  Card,
  ErrorState,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@gerege-systems/ui";
import { Inbox } from "lucide-react";

/**
 * What has been sent TO this organisation. Addressed by party id throughout:
 * the recipient does not own the document, so the server never hands out its
 * id — the party row is the recipient's whole view of the contract.
 */
export default function ContractInboxPage() {
  const { t } = useI18n();
  const router = useRouter();
  const list = useResource<InboxItem[]>(async () => (await contracts.inbox(true)).items, { initial: [] });
  useLoadOnMount(list.reload);

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Inbox className="w-6 h-6 text-accent" />}
        title={t("contracts.view.inbox_title")}
        subtitle={t("contracts.view.inbox_subtitle")}
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
        <ListEmpty icon={<Inbox />} title={t("contracts.view.inbox_empty")} />
      ) : (
        <Card padding="none" className="overflow-hidden">
          <Table containerClassName="rounded-none border-0" className="text-xs" scrollLabel={t("contracts.view.inbox_title")}>
            <TableHeader>
              <TableRow>
                <TableHead>{t("contracts.col.contract")}</TableHead>
                <TableHead>{t("contracts.col.issuer")}</TableHead>
                <TableHead>{t("contracts.col.state")}</TableHead>
                <TableHead>{t("contracts.col.received")}</TableHead>
                <TableHead>{t("contracts.col.due")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {list.data.map((item) => (
                <TableRow
                  key={item.party_id}
                  onClick={() => router.push(`/module/documents/inbox/${item.party_id}`)}
                  className="cursor-pointer"
                >
                  <TableCell className="font-semibold text-foreground">{item.title}</TableCell>
                  <TableCell className="text-muted">{item.issuer_name || "—"}</TableCell>
                  <TableCell><PartyBadge state={item.state} /></TableCell>
                  <TableCell className="text-muted">{fmtDate(item.invited_at)}</TableCell>
                  <TableCell className="text-muted">{fmtDate(item.due_at)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}
    </div>
  );
}
