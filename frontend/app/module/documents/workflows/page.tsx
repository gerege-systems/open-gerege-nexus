"use client";

import React, { useState } from "react";
import { api } from "@/lib/api";
import { useResource } from "@/lib/useResource";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { PageHeader } from "@/components/ui";
import { Alert, Button, Card, CardContent, CardHeader, CardTitle, ErrorState, IconButton, Input } from "@gerege-systems/ui";
import { ActionMessage, ListSkeleton } from "@/components/documents/shared";
import { Plus, Save, Trash2, Workflow as WorkflowIcon } from "lucide-react";

interface Step {
  order?: number;
  name: string;
  signer_reg_number: string;
}

interface Chain {
  doc_type: string;
  steps: Step[];
}

/**
 * Document workflows: the ordered approval chain each document type needs. One
 * step per signature; a type with no steps is approved by a single signature,
 * which is how the app behaved before chains existed.
 */
export default function DocumentWorkflowsPage() {
  const { t } = useI18n();
  const { can } = useAccess();
  const mayManage = can("documents.manage");

  const [busy, setBusy] = useState<string | null>(null);
  const [message, setMessage] = useState<ActionMessage | null>(null);

  // The server answers with a row for every document type, so an empty list can only
  // mean the load failed — hence `failed` below rather than an empty table. Rendering
  // the table anyway left a blank page that reads as "this tenant has no document
  // types" the moment the error banner is dismissed.
  const {
    data: chains,
    loading,
    failed: loadFailed,
    reload,
    setData: setChains,
  } = useResource(async () => (await api.getDocumentWorkflows()) || [], {
    initial: [] as Chain[],
    onError: (err: any) =>
      setMessage({ type: "error", text: err?.message || t("documents.message.workflows_failed") }),
  });

  const editChain = (docType: string, steps: Step[]) =>
    setChains((current) => current.map((c) => (c.doc_type === docType ? { ...c, steps } : c)));

  const addStep = (chain: Chain) => editChain(chain.doc_type, [...chain.steps, { name: "", signer_reg_number: "" }]);

  const removeStep = (chain: Chain, index: number) =>
    editChain(
      chain.doc_type,
      chain.steps.filter((_, i) => i !== index)
    );

  const editStep = (chain: Chain, index: number, patch: Partial<Step>) =>
    editChain(
      chain.doc_type,
      chain.steps.map((step, i) => (i === index ? { ...step, ...patch } : step))
    );

  const save = async (chain: Chain) => {
    setBusy(chain.doc_type);
    setMessage(null);
    try {
      const saved = await api.saveDocumentWorkflow(
        chain.doc_type,
        chain.steps.map((step) => ({ name: step.name, signer_reg_number: step.signer_reg_number }))
      );
      setMessage({ type: "success", text: t("documents.message.workflow_saved", { type: chain.doc_type }) });
      // Only the chain that was saved is replaced, with what the server stored.
      // Reloading everything discarded steps the operator had typed into the other
      // document types under a banner saying only this one was saved.
      //
      // And only if this row still holds what was sent. The inputs stay editable while
      // the request is in flight, so a step typed after clicking Save would otherwise be
      // replaced by the server's answer to the older chain — work discarded under a
      // banner saying the save succeeded.
      if (saved && Array.isArray((saved as Chain).steps)) {
        const sent = JSON.stringify(chain.steps);
        setChains((current) =>
          current.map((row) =>
            row.doc_type === chain.doc_type && JSON.stringify(row.steps) === sent
              ? { ...row, steps: (saved as Chain).steps }
              : row
          )
        );
      }
    } catch (err: any) {
      // The draft stays on screen: a refused save is usually one blank step name,
      // and reloading here threw away everything the operator had typed.
      setMessage({ type: "error", text: err?.message || t("documents.message.workflows_failed") });
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<WorkflowIcon className="w-7 h-7 text-accent" />}
        title={t("documents.menu.workflows")}
        subtitle={t("documents.view.workflows_hint")}
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
            title={t("documents.message.workflows_failed")}
            description=""
            action={
              <Button variant="outline" onClick={() => void reload()}>
                {t("base.action.retry")}
              </Button>
            }
          />
        </Card>
      ) : (
        <div className="space-y-4">
          {chains.map((chain) => {
            const unnamed = chain.steps.some((s) => !s.name.trim());
            return (
              <Card key={chain.doc_type} padding="none" className="overflow-hidden">
                <CardHeader className="flex-row flex-wrap items-center justify-between gap-3 px-4 py-3 bg-surface-2 border-b border-line">
                  <div className="flex items-center gap-3">
                    <CardTitle className="font-mono text-sm">{chain.doc_type}</CardTitle>
                    <span className="text-xs text-muted">
                      {chain.steps.length === 0
                        ? t("documents.message.chain_single_signature")
                        : t("documents.message.chain_signature_count", { count: chain.steps.length })}
                    </span>
                  </div>
                  {mayManage && (
                    <div className="flex items-center gap-2">
                      <Button size="sm" variant="outline" onClick={() => addStep(chain)} leadingIcon={<Plus />}>
                        {t("documents.action.add_step")}
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => save(chain)}
                        loading={busy === chain.doc_type}
                        disabled={unnamed}
                        title={unnamed ? t("documents.message.step_needs_name") : undefined}
                        leadingIcon={<Save />}
                      >
                        {t("base.action.save")}
                      </Button>
                    </div>
                  )}
                </CardHeader>

                <CardContent>
                  {chain.steps.length === 0 ? (
                    <p className="px-4 py-5 text-xs text-muted">{t("documents.message.no_steps")}</p>
                  ) : (
                    <ul className="divide-y divide-line">
                      {chain.steps.map((step, index) => (
                        <li key={index} className="px-4 py-3 flex flex-wrap items-end gap-3">
                          <span className="w-6 h-6 mb-1 shrink-0 rounded-full bg-accent-soft text-accent text-xs font-semibold grid place-items-center">
                            {index + 1}
                          </span>
                          <Input
                            type="text"
                            size="sm"
                            label={t("documents.field.step_name")}
                            value={step.name}
                            disabled={!mayManage}
                            onChange={(e) => editStep(chain, index, { name: e.target.value })}
                            className="flex-1 min-w-44"
                          />
                          {/* The placeholder is the Cyrillic form, because that is what
                              eID vouches for and what the step is compared against: an
                              example in Latin letters guides an operator into a step no
                              live signature can ever fill. Same example as the dialog. */}
                          <Input
                            type="text"
                            size="sm"
                            label={t("documents.field.step_signer")}
                            placeholder="УБ99010111"
                            value={step.signer_reg_number}
                            disabled={!mayManage}
                            onChange={(e) => editStep(chain, index, { signer_reg_number: e.target.value })}
                            className="flex-1 min-w-44 [&_input]:font-mono"
                          />
                          {mayManage && (
                            <IconButton
                              size="sm"
                              variant="outline"
                              className="text-danger"
                              onClick={() => removeStep(chain, index)}
                              aria-label={t("base.action.delete")}
                              icon={<Trash2 />}
                            />
                          )}
                        </li>
                      ))}
                    </ul>
                  )}
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}

      <p className="text-xs text-muted">{t("documents.message.step_signer_hint")}</p>
    </div>
  );
}
