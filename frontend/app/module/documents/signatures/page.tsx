"use client";

import React, { useState } from "react";
import { api } from "@/lib/api";
import { useResource } from "@/lib/useResource";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { Banner, LoadingBlock, PageHeader, TableCard, rowActionClass } from "@/components/ui";
import { ActionMessage } from "@/components/documents/shared";
import { PenTool, Save } from "lucide-react";

interface Policy {
  doc_type: string;
  allow_eid: boolean;
  allow_dan: boolean;
  require_named_signer: boolean;
  configured: boolean;
  updated_at?: string;
}

/**
 * Signature policies: which national channel may sign a document type, and
 * whether the signer has to be one the approval chain names. A type nobody has
 * configured accepts both channels and names no signer, which is how the app
 * behaved before this screen existed.
 */
export default function SignaturePoliciesPage() {
  const { t } = useI18n();
  const { can } = useAccess();
  const mayManage = can("documents.manage");

  const [busy, setBusy] = useState<string | null>(null);
  const [message, setMessage] = useState<ActionMessage | null>(null);

  // The server answers with a row for every document type, so an empty list can only
  // mean the load failed — which is why `failed` is rendered rather than the table.
  // Rendering the table anyway left a blank page that reads as "this tenant has no
  // document types" the moment the error banner is dismissed.
  const {
    data: policies,
    loading,
    failed: loadFailed,
    setData: setPolicies,
  } = useResource(async () => (await api.getSignaturePolicies()) || [], {
    initial: [] as Policy[],
    onError: (err: any) =>
      setMessage({ type: "error", text: err?.message || t("documents.message.policies_failed") }),
  });

  const edit = (docType: string, patch: Partial<Policy>) =>
    setPolicies((current) => current.map((p) => (p.doc_type === docType ? { ...p, ...patch } : p)));

  const save = async (policy: Policy) => {
    setBusy(policy.doc_type);
    setMessage(null);
    try {
      const saved = (await api.saveSignaturePolicy(policy.doc_type, {
        allow_eid: policy.allow_eid,
        allow_dan: policy.allow_dan,
        require_named_signer: policy.require_named_signer,
      })) as Policy | undefined;
      setMessage({ type: "success", text: t("documents.message.policy_saved", { type: policy.doc_type }) });
      // Only the row that was saved is replaced. Reloading the table reverted the
      // edits an operator had made to the other document types, under a banner
      // naming only this one.
      if (saved && saved.doc_type) edit(policy.doc_type, saved);
    } catch (err: any) {
      setMessage({ type: "error", text: err?.message || t("documents.message.policies_failed") });
      // This refusal cannot be fixed on this screen — the chain beneath the policy is
      // what has to change — so this row goes back to what is stored. The others keep
      // whatever the operator has typed.
      try {
        const stored = await api.getSignaturePolicies();
        const mine = (stored || []).find((row) => row.doc_type === policy.doc_type);
        if (mine) edit(policy.doc_type, mine);
      } catch {
        // Leave the row as typed rather than blank the screen over a second failure.
      }
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<PenTool className="w-7 h-7 text-indigo-600" />}
        title={t("documents.menu.signature_policies")}
        subtitle={t("documents.view.signature_policies_hint")}
      />

      {message && <Banner tone={message.type} message={message.text} onDismiss={() => setMessage(null)} />}

      {loading ? (
        <LoadingBlock label={t("documents.message.loading")} />
      ) : loadFailed ? (
        <div className="bg-surface border border-line rounded-xl p-8 text-center text-muted text-sm">
          {t("documents.message.policies_failed")}
        </div>
      ) : (
        <TableCard
          head={
            <tr>
              <th className="px-4 py-3">{t("base.field.type")}</th>
              <th className="px-4 py-3">E-ID</th>
              <th className="px-4 py-3">DAN</th>
              <th className="px-4 py-3">{t("documents.field.require_named_signer")}</th>
              <th className="px-4 py-3">{t("base.field.status")}</th>
              <th className="px-4 py-3 text-right">{t("base.field.actions")}</th>
            </tr>
          }
        >
          {policies.map((policy) => (
            <tr key={policy.doc_type} className="hover:bg-surface-hover">
              <td className="px-4 py-3 font-mono font-semibold text-foreground">{policy.doc_type}</td>
              <td className="px-4 py-3">
                <input
                  type="checkbox"
                  checked={policy.allow_eid}
                  disabled={!mayManage}
                  onChange={(e) => edit(policy.doc_type, { allow_eid: e.target.checked })}
                  className="w-4 h-4 accent-indigo-600"
                />
              </td>
              <td className="px-4 py-3">
                <input
                  type="checkbox"
                  checked={policy.allow_dan}
                  disabled={!mayManage}
                  onChange={(e) => edit(policy.doc_type, { allow_dan: e.target.checked })}
                  className="w-4 h-4 accent-indigo-600"
                />
              </td>
              <td className="px-4 py-3">
                <input
                  type="checkbox"
                  checked={policy.require_named_signer}
                  disabled={!mayManage}
                  onChange={(e) => edit(policy.doc_type, { require_named_signer: e.target.checked })}
                  className="w-4 h-4 accent-indigo-600"
                />
              </td>
              <td className="px-4 py-3">
                {policy.configured ? (
                  <span className="text-[11px] font-semibold px-2 py-0.5 rounded-full bg-indigo-50 text-indigo-700 border border-indigo-200">
                    {t("documents.state.policy_configured")}
                  </span>
                ) : (
                  <span className="text-[11px] font-semibold px-2 py-0.5 rounded-full bg-surface-2 text-muted border border-line">
                    {t("documents.state.policy_default")}
                  </span>
                )}
              </td>
              <td className="px-4 py-3 text-right">
                {mayManage ? (
                  <button
                    onClick={() => save(policy)}
                    disabled={busy === policy.doc_type || (!policy.allow_eid && !policy.allow_dan)}
                    className={rowActionClass}
                  >
                    <Save className="w-3.5 h-3.5" />
                    <span>{t("base.action.save")}</span>
                  </button>
                ) : (
                  <span className="text-subtle text-[11px]">—</span>
                )}
              </td>
            </tr>
          ))}
        </TableCard>
      )}

      <p className="text-xs text-muted">{t("documents.message.policy_named_signer_hint")}</p>
    </div>
  );
}
