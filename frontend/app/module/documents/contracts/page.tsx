"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation";
import { contracts, ContractRow } from "@/lib/contracts";
import { useResource, useLoadOnMount } from "@/lib/useResource";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { Banner, LoadingBlock, Modal, PageHeader, TableCard, EmptyState, fieldClass } from "@/components/ui";
import { ContractBadge, fmtDate, fmtMoney } from "@/components/documents/contracts";
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
        icon={<FileSignature className="w-6 h-6 text-indigo-600" />}
        title={t("contracts.view.title")}
        subtitle={t("contracts.view.subtitle")}
        actions={
          mayManage ? (
            <button
              onClick={() => setCreating(true)}
              className="bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-semibold px-4 py-2 rounded-lg flex items-center gap-2"
            >
              <Plus className="w-4 h-4" />
              {t("contracts.view.new")}
            </button>
          ) : undefined
        }
      />

      {list.failed && <Banner tone="error" message={t("contracts.msg.load_failed")} />}
      {list.loading ? (
        <LoadingBlock />
      ) : list.data.length === 0 ? (
        <EmptyState message={t("contracts.view.empty")} />
      ) : (
        <TableCard
          head={
            <tr>
              <th className="px-4 py-3">{t("contracts.col.contract")}</th>
              <th className="px-4 py-3">{t("contracts.col.parties")}</th>
              <th className="px-4 py-3">{t("contracts.col.state")}</th>
              <th className="px-4 py-3">{t("contracts.col.signatures")}</th>
              <th className="px-4 py-3">{t("contracts.col.amount")}</th>
              <th className="px-4 py-3">{t("contracts.col.date")}</th>
            </tr>
          }
        >
          {orderByFamily(list.data).map((row) => (
            <tr
              key={row.id}
              onClick={() => router.push(`/module/documents/contracts/${row.id}`)}
              className="cursor-pointer hover:bg-surface-hover"
            >
              <td className="px-4 py-3">
                <div className={`font-semibold text-foreground ${row.parent_document_id ? "pl-5 relative" : ""}`}>
                  {row.parent_document_id && <span className="absolute left-0 text-subtle">↳</span>}
                  {row.parent_document_id ? row.counterparties || row.title : row.title}
                </div>
                {!row.parent_document_id && row.contract_number && (
                  <div className="text-[11px] text-muted">№ {row.contract_number}</div>
                )}
                {!row.parent_document_id && (row.issued_count ?? 0) > 0 && (
                  <div className="text-[11px] text-indigo-600 font-semibold">
                    {t("contracts.list.issued", { total: row.issued_count ?? 0, executed: row.issued_executed ?? 0 })}
                  </div>
                )}
              </td>
              <td className="px-4 py-3">{row.parent_document_id ? "" : row.counterparties || "—"}</td>
              <td className="px-4 py-3"><ContractBadge state={row.contract_state} /></td>
              <td className="px-4 py-3 font-mono">{row.signed_count} / {row.required_count}</td>
              <td className="px-4 py-3">{fmtMoney(row.amount, row.currency)}</td>
              <td className="px-4 py-3 text-muted">{fmtDate(row.sent_at || row.created_at)}</td>
            </tr>
          ))}
        </TableCard>
      )}

      {creating && (
        <Modal onClose={() => setCreating(false)} label={t("contracts.view.new")}>
          <form onSubmit={create} className="space-y-4">
            <h2 className="text-lg font-semibold text-foreground">{t("contracts.view.new")}</h2>
            <div>
              <label className="block text-xs font-medium text-muted mb-1">{t("contracts.field.title")}</label>
              <input
                autoFocus
                className={fieldClass}
                value={title}
                onChange={(event) => setTitle(event.target.value)}
              />
              <p className="text-[11px] text-muted mt-1">{t("contracts.field.title_hint")}</p>
            </div>
            {error && <Banner tone="error" message={error} />}
            <div className="flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setCreating(false)}
                className="text-sm font-medium text-muted px-4 py-2 rounded-lg hover:bg-surface-hover"
              >
                {t("contracts.action.cancel")}
              </button>
              <button
                type="submit"
                disabled={busy || !title.trim()}
                className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white text-sm font-semibold px-4 py-2 rounded-lg"
              >
                {t("contracts.action.create")}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </div>
  );
}
