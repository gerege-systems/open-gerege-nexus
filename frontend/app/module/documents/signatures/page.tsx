"use client";

import React, { useState } from "react";
import { api } from "@/lib/api";
import { useResource } from "@/lib/useResource";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { PageHeader } from "@/components/ui";
import {
  Alert,
  Badge,
  Button,
  Card,
  Checkbox,
  ErrorState,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@gerege-systems/ui";
import { ActionMessage, ListSkeleton } from "@/components/documents/shared";
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
    reload,
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
        icon={<PenTool className="w-7 h-7 text-accent" />}
        title={t("documents.menu.signature_policies")}
        subtitle={t("documents.view.signature_policies_hint")}
      />

      {message && (
        <Alert variant={message.type === "error" ? "danger" : message.type} live dismissible onDismiss={() => setMessage(null)}>
          {message.text}
        </Alert>
      )}

      {loading ? (
        <ListSkeleton label={t("documents.message.loading")} />
      ) : loadFailed ? (
        <Card>
          <ErrorState
            title={t("documents.message.policies_failed")}
            description=""
            action={
              <Button variant="outline" onClick={() => void reload()}>
                {t("base.action.retry")}
              </Button>
            }
          />
        </Card>
      ) : (
        <Card padding="none" className="overflow-hidden">
          <Table containerClassName="rounded-none border-0" className="text-xs" scrollLabel={t("documents.menu.signature_policies")}>
            <TableHeader>
              <TableRow>
                <TableHead>{t("base.field.type")}</TableHead>
                <TableHead>E-ID</TableHead>
                <TableHead>DAN</TableHead>
                <TableHead>{t("documents.field.require_named_signer")}</TableHead>
                <TableHead>{t("base.field.status")}</TableHead>
                <TableHead align="right">{t("base.field.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {policies.map((policy) => (
                <TableRow key={policy.doc_type}>
                  <TableCell className="font-mono font-semibold">{policy.doc_type}</TableCell>
                  <TableCell>
                    <Checkbox
                      checked={policy.allow_eid}
                      aria-label={`E-ID — ${policy.doc_type}`}
                      disabled={!mayManage}
                      onCheckedChange={(checked) => edit(policy.doc_type, { allow_eid: checked === true })}
                    />
                  </TableCell>
                  <TableCell>
                    <Checkbox
                      checked={policy.allow_dan}
                      aria-label={`DAN — ${policy.doc_type}`}
                      disabled={!mayManage}
                      onCheckedChange={(checked) => edit(policy.doc_type, { allow_dan: checked === true })}
                    />
                  </TableCell>
                  <TableCell>
                    <Checkbox
                      checked={policy.require_named_signer}
                      aria-label={`${t("documents.field.require_named_signer")} — ${policy.doc_type}`}
                      disabled={!mayManage}
                      onCheckedChange={(checked) => edit(policy.doc_type, { require_named_signer: checked === true })}
                    />
                  </TableCell>
                  <TableCell>
                    {policy.configured ? (
                      <Badge tone="accent">{t("documents.state.policy_configured")}</Badge>
                    ) : (
                      <Badge tone="neutral">{t("documents.state.policy_default")}</Badge>
                    )}
                  </TableCell>
                  <TableCell align="right">
                    {mayManage ? (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => save(policy)}
                        loading={busy === policy.doc_type}
                        disabled={!policy.allow_eid && !policy.allow_dan}
                        leadingIcon={<Save />}
                      >
                        {t("base.action.save")}
                      </Button>
                    ) : (
                      <span className="text-subtle text-xs">—</span>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}

      <p className="text-xs text-muted">{t("documents.message.policy_named_signer_hint")}</p>
    </div>
  );
}
