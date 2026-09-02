"use client";

import React, { useCallback, useEffect, useState } from "react";
import { ChevronLeft, FileText, Layers, Play, Plus, Smartphone, XCircle } from "lucide-react";
import { esign, type Batch, type EsignDocument } from "@/lib/esign";
import { useI18n } from "@/lib/i18n";
import { Banner, EmptyState, Loading, Modal, PageHeader, cardClass, fieldClass, tableHeadClass } from "@/components/ui";
import { BatchBadge, Card, ItemBadge, useErrorMessage } from "@/components/esign/shared";

/**
 * Batch signing.
 *
 * A run is a queue with progress, not a shortcut around consent. eID signs one
 * digest per approval, so the citizen still confirms each document with PIN2 —
 * what the batch removes is the clicking between them, and what it adds is a
 * record of how far the run got when somebody walked away halfway.
 */
export default function EsignBatchPage() {
  const { t } = useI18n();
  const describe = useErrorMessage();
  const [batches, setBatches] = useState<Batch[]>([]);
  const [selected, setSelected] = useState<Batch | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const load = useCallback(async () => {
    try {
      const page = await esign.batches({ limit: 50 });
      setBatches(page.items || []);
    } catch (err) {
      setError(describe(err, t("base.message.error")));
    }
  }, [describe, t]);

  useEffect(() => {
    (async () => {
      setLoading(true);
      await load();
      setLoading(false);
    })();
  }, [load]);

  const open = async (batch: Batch) => {
    try {
      setSelected(await esign.batch(batch.id));
    } catch (err) {
      setError(describe(err, t("base.message.error")));
    }
  };

  if (selected) {
    return (
      <BatchDetail
        batch={selected}
        onBack={async () => {
          setSelected(null);
          await load();
        }}
        onRefresh={async (id) => setSelected(await esign.batch(id))}
      />
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Layers className="w-7 h-7 text-indigo-600" />}
        title={t("esign.view.batch_title")}
        subtitle={t("esign.view.batch_subtitle")}
        actions={
          <button
            onClick={() => setCreating(true)}
            className="bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-semibold px-4 py-2 rounded-lg flex items-center gap-2 shadow-sm"
          >
            <Plus className="w-4 h-4" />
            {t("esign.action.new_batch")}
          </button>
        }
      />

      {error && <Banner tone="error" message={error} onDismiss={() => setError(null)} />}

      <div className={`${cardClass} overflow-x-auto`}>
        {loading ? (
          <div className="p-6">
            <Loading />
          </div>
        ) : (
          <table className="w-full text-left text-xs text-muted">
            <thead className={tableHeadClass}>
              <tr>
                <th className="px-4 py-3">{t("esign.field.batch_name")}</th>
                <th className="px-4 py-3">{t("base.field.status")}</th>
                <th className="px-4 py-3">{t("esign.field.progress")}</th>
                <th className="px-4 py-3">{t("esign.field.provider")}</th>
                <th className="px-4 py-3">{t("base.field.date")}</th>
                <th className="px-4 py-3 text-right">{t("base.field.actions")}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {batches.length === 0 && (
                <tr>
                  <td colSpan={6}>
                    <EmptyState message={t("esign.message.batch_empty")} />
                  </td>
                </tr>
              )}
              {batches.map((batch) => (
                <tr key={batch.id} className="hover:bg-surface-hover">
                  <td className="px-4 py-3 font-semibold text-foreground">{batch.name}</td>
                  <td className="px-4 py-3">
                    <BatchBadge status={batch.status} />
                  </td>
                  <td className="px-4 py-3">
                    <Progress signed={batch.signed} failed={batch.failed} total={batch.total} />
                  </td>
                  <td className="px-4 py-3 font-mono text-[11px]">{batch.provider}</td>
                  <td className="px-4 py-3 text-muted">{new Date(batch.created_at).toLocaleDateString()}</td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => open(batch)}
                      className="bg-surface-2 hover:bg-slate-200 text-foreground text-[11px] font-semibold px-3 py-1.5 rounded-lg"
                    >
                      {t("base.action.open")}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {creating && (
        <CreateBatchModal
          onClose={() => setCreating(false)}
          onCreated={async (batch) => {
            setCreating(false);
            await load();
            setSelected(batch);
          }}
        />
      )}
    </div>
  );
}

function Progress({ signed, failed, total }: { signed: number; failed: number; total: number }) {
  const done = total ? Math.round(((signed + failed) / total) * 100) : 0;
  return (
    <div className="min-w-[120px]">
      <div className="h-1.5 bg-surface-2 rounded-full overflow-hidden flex">
        <div className="bg-emerald-500 h-full" style={{ width: `${total ? (signed / total) * 100 : 0}%` }} />
        <div className="bg-red-400 h-full" style={{ width: `${total ? (failed / total) * 100 : 0}%` }} />
      </div>
      <div className="text-[10px] text-muted mt-1 font-mono">
        {signed}/{total} · {done}%
      </div>
    </div>
  );
}

function BatchDetail({
  batch,
  onBack,
  onRefresh,
}: {
  batch: Batch;
  onBack: () => Promise<void>;
  onRefresh: (id: string) => Promise<void>;
}) {
  const { t } = useI18n();
  const describe = useErrorMessage();
  const [running, setRunning] = useState(false);
  const [code, setCode] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const cancelled = React.useRef(false);

  useEffect(
    () => () => {
      cancelled.current = true;
    },
    [],
  );

  /**
   * Drives the run: ask the server for the next document, show its
   * verification code, wait for the citizen's PIN2, repeat. The server advances
   * one document per call rather than looping, so the screen can always say
   * which document it is currently asking about.
   */
  const run = async () => {
    setRunning(true);
    setError(null);
    cancelled.current = false;
    try {
      while (!cancelled.current) {
        const step = await esign.runBatch(batch.id);
        await onRefresh(batch.id);

        if (step.error) {
          setError(step.error);
        }
        if (!step.session) break; // nothing left to sign

        setCode(step.session.verification_code ?? "····");

        let settled = false;
        while (!cancelled.current && !settled) {
          await new Promise((resolve) => setTimeout(resolve, 1500));
          try {
            const current = await esign.session(step.session.session_id);
            if (current.state !== "pending") settled = true;
          } catch {
            // transient — the ceremony is still open on the citizen's phone
          }
        }
        setCode(null);
        await onRefresh(batch.id);
      }
    } catch (err) {
      setError(describe(err, t("base.message.error")));
    } finally {
      setRunning(false);
      setCode(null);
    }
  };

  const cancelBatch = async () => {
    cancelled.current = true;
    try {
      await esign.cancelBatch(batch.id);
      await onRefresh(batch.id);
    } catch (err) {
      setError(describe(err, t("base.message.error")));
    }
  };

  const pending = batch.items?.some((item) => item.status === "PENDING") ?? false;

  return (
    <div className="space-y-6">
      <button onClick={onBack} className="text-xs text-muted hover:text-foreground flex items-center gap-1">
        <ChevronLeft className="w-4 h-4" />
        {t("esign.action.back_to_batches")}
      </button>

      <PageHeader
        icon={<Layers className="w-7 h-7 text-indigo-600" />}
        title={batch.name}
        subtitle={t("esign.view.batch_detail_subtitle", {
          signed: batch.signed,
          total: batch.total,
        })}
        actions={
          <div className="flex gap-2">
            {pending && batch.status !== "CANCELLED" && (
              <button
                onClick={run}
                disabled={running}
                className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white text-xs font-semibold px-4 py-2 rounded-lg flex items-center gap-2"
              >
                <Play className="w-4 h-4" />
                {running ? t("esign.message.batch_running") : t("esign.action.run_batch")}
              </button>
            )}
            {batch.status !== "CANCELLED" && batch.status !== "COMPLETED" && (
              <button
                onClick={cancelBatch}
                className="bg-surface-2 hover:bg-slate-200 text-foreground text-xs font-semibold px-4 py-2 rounded-lg flex items-center gap-2"
              >
                <XCircle className="w-4 h-4" />
                {t("base.action.cancel")}
              </button>
            )}
          </div>
        }
      />

      {error && <Banner tone="error" message={error} onDismiss={() => setError(null)} />}

      {code && (
        <div className="bg-indigo-50 border border-indigo-200 rounded-xl p-5 text-center">
          <div className="inline-flex items-center gap-2 text-indigo-700 font-semibold text-sm">
            <Smartphone className="w-4 h-4 animate-pulse" />
            {t("esign.message.pin2_instruction")}
          </div>
          <div className="flex justify-center gap-2 mt-3">
            {code.split("").map((digit, index) => (
              <span
                key={index}
                className="w-10 h-12 inline-flex items-center justify-center text-xl font-semibold font-mono text-indigo-700 bg-surface rounded-lg border border-indigo-200"
              >
                {digit}
              </span>
            ))}
          </div>
        </div>
      )}

      <Card title={t("esign.view.batch_documents")}>
        <table className="w-full text-left text-xs text-muted">
          <thead className={tableHeadClass}>
            <tr>
              <th className="px-4 py-3 w-10">#</th>
              <th className="px-4 py-3">{t("esign.field.document")}</th>
              <th className="px-4 py-3">{t("base.field.status")}</th>
              <th className="px-4 py-3">{t("esign.field.detail")}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {(batch.items ?? []).map((item, index) => (
              <tr key={item.id} className="hover:bg-surface-hover">
                <td className="px-4 py-3 text-muted font-mono">{index + 1}</td>
                <td className="px-4 py-3">
                  <div className="font-semibold text-foreground flex items-center gap-1.5">
                    <FileText className="w-3.5 h-3.5 text-muted" />
                    {item.document_title}
                  </div>
                  <div className="text-muted font-mono mt-0.5">{item.file_name}</div>
                </td>
                <td className="px-4 py-3">
                  <ItemBadge status={item.status} />
                </td>
                <td className="px-4 py-3 text-muted">{item.error || "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}

function CreateBatchModal({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: (batch: Batch) => Promise<void>;
}) {
  const { t } = useI18n();
  const describe = useErrorMessage();
  const [name, setName] = useState("");
  const [documents, setDocuments] = useState<EsignDocument[]>([]);
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    esign
      // Only unsigned documents can join a batch; offering signed ones would
      // just produce a run that fails on every item.
      .documents({ status: "PENDING", limit: 100 })
      .then((page) => setDocuments(page.items || []))
      .catch((err) => setError(describe(err, t("base.message.error"))))
      .finally(() => setLoading(false));
  }, [describe, t]);

  const toggle = (id: string) => {
    const next = new Set(picked);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setPicked(next);
  };

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const batch = await esign.createBatch({
        name: name.trim(),
        provider: "EID",
        document_ids: [...picked],
      });
      await onCreated(batch);
    } catch (err) {
      setError(describe(err, t("base.message.error")));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal size="lg" className="max-h-[90vh] flex flex-col" label={t("esign.view.new_batch_title")}>
      <h2 className="text-xl font-semibold text-foreground mb-4">{t("esign.view.new_batch_title")}</h2>
      {error && <div className="mb-3"><Banner tone="error" message={error} onDismiss={() => setError(null)} /></div>}

      <form onSubmit={submit} className="flex-1 flex flex-col min-h-0 space-y-4">
        <div>
          <label htmlFor="batch-name" className="block text-xs font-semibold text-foreground mb-1">
            {t("esign.field.batch_name")} *
          </label>
          <input
            id="batch-name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder={t("esign.field.batch_name_placeholder")}
            className={fieldClass}
            required
          />
        </div>

        <div className="flex-1 min-h-0 flex flex-col">
          <span className="block text-xs font-semibold text-foreground mb-1">
            {t("esign.field.batch_documents", { count: picked.size })}
          </span>
          <div className="flex-1 overflow-y-auto border border-line rounded-lg divide-y divide-line">
            {loading ? (
              <div className="p-4">
                <Loading />
              </div>
            ) : documents.length === 0 ? (
              <EmptyState message={t("esign.message.no_pending_documents")} />
            ) : (
              documents.map((doc) => (
                <label key={doc.id} className="flex items-center gap-3 px-3 py-2.5 hover:bg-surface-hover cursor-pointer">
                  <input
                    type="checkbox"
                    checked={picked.has(doc.id)}
                    onChange={() => toggle(doc.id)}
                    className="rounded border-input text-indigo-600 focus:ring-indigo-500"
                  />
                  <span className="min-w-0">
                    <span className="block text-sm font-medium text-foreground truncate">{doc.title}</span>
                    <span className="block text-[11px] text-muted font-mono truncate">{doc.file_name}</span>
                  </span>
                </label>
              ))
            )}
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={onClose}
            className="w-1/2 bg-surface-2 hover:bg-slate-200 text-foreground font-medium py-2 rounded-lg text-xs"
          >
            {t("base.action.cancel")}
          </button>
          <button
            type="submit"
            disabled={busy || picked.size === 0 || !name.trim()}
            className="w-1/2 bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white font-medium py-2 rounded-lg text-xs"
          >
            {t("esign.action.create_batch")}
          </button>
        </div>
      </form>
    </Modal>
  );
}
