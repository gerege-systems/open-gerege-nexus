"use client";

import React from "react";
import { useRouter } from "next/navigation";
import { contracts, InboxItem } from "@/lib/contracts";
import { useResource, useLoadOnMount } from "@/lib/useResource";
import { useI18n } from "@/lib/i18n";
import { Banner, LoadingBlock, PageHeader, TableCard, EmptyState } from "@/components/ui";
import { PartyBadge, fmtDate } from "@/components/documents/contracts";
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
        icon={<Inbox className="w-6 h-6 text-indigo-600" />}
        title={t("contracts.view.inbox_title")}
        subtitle={t("contracts.view.inbox_subtitle")}
      />
      {list.failed && <Banner tone="error" message={t("contracts.msg.load_failed")} />}
      {list.loading ? (
        <LoadingBlock />
      ) : list.data.length === 0 ? (
        <EmptyState message={t("contracts.view.inbox_empty")} />
      ) : (
        <TableCard
          head={
            <tr>
              <th className="px-4 py-3">{t("contracts.col.contract")}</th>
              <th className="px-4 py-3">{t("contracts.col.issuer")}</th>
              <th className="px-4 py-3">{t("contracts.col.state")}</th>
              <th className="px-4 py-3">{t("contracts.col.received")}</th>
              <th className="px-4 py-3">{t("contracts.col.due")}</th>
            </tr>
          }
        >
          {list.data.map((item) => (
            <tr
              key={item.party_id}
              onClick={() => router.push(`/module/documents/inbox/${item.party_id}`)}
              className="cursor-pointer hover:bg-surface-hover"
            >
              <td className="px-4 py-3 font-semibold text-foreground">{item.title}</td>
              <td className="px-4 py-3">{item.issuer_name || "—"}</td>
              <td className="px-4 py-3"><PartyBadge state={item.state} /></td>
              <td className="px-4 py-3 text-muted">{fmtDate(item.invited_at)}</td>
              <td className="px-4 py-3 text-muted">{fmtDate(item.due_at)}</td>
            </tr>
          ))}
        </TableCard>
      )}
    </div>
  );
}
