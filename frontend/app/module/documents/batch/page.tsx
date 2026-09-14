"use client";

import React, { useCallback, useEffect, useState } from "react";
import { ChevronLeft, FileText, Layers, Play, Plus, Smartphone, XCircle } from "lucide-react";
import { esign, type Batch, type EsignDocument } from "@/lib/esign";
import { useI18n } from "@/lib/i18n";
import { PageHeader } from "@/components/ui";
import { ListEmpty, ListSkeleton, pinnedDialogProps } from "@/components/documents/shared";
import { BatchBadge, Card, ItemBadge, useErrorMessage } from "@/components/esign/shared";
import {
  Alert,
  Button,
  Card as UICard,
  Checkbox,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  Input,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@gerege-systems/ui";

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
        icon={<Layers className="w-7 h-7 text-accent" />}
        title={t("esign.view.batch_title")}
        subtitle={t("esign.view.batch_subtitle")}
        actions={
          <Button onClick={() => setCreating(true)} leadingIcon={<Plus />}>
            {t("esign.action.new_batch")}
          </Button>
        }
      />

      {error && <Alert variant="danger" live dismissible onDismiss={() => setError(null)}>{error}</Alert>}

      {loading ? (
        <ListSkeleton />
      ) : batches.length === 0 ? (
        <ListEmpty icon={<Layers />} title={t("esign.message.batch_empty")} />
      ) : (
        <UICard padding="none" className="overflow-hidden">
          <Table containerClassName="rounded-none border-0" className="text-xs" scrollLabel={t("esign.view.batch_title")}>
            <TableHeader>
              <TableRow>
                <TableHead>{t("esign.field.batch_name")}</TableHead>
                <TableHead>{t("base.field.status")}</TableHead>
                <TableHead>{t("esign.field.progress")}</TableHead>
                <TableHead>{t("esign.field.provider")}</TableHead>
                <TableHead>{t("base.field.date")}</TableHead>
                <TableHead align="right">{t("base.field.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {batches.map((batch) => (
                <TableRow key={batch.id}>
                  <TableCell className="font-semibold text-foreground">{batch.name}</TableCell>
                  <TableCell>
                    <BatchBadge status={batch.status} />
                  </TableCell>
                  <TableCell>
                    <Progress signed={batch.signed} failed={batch.failed} total={batch.total} />
                  </TableCell>
                  <TableCell className="font-mono">{batch.provider}</TableCell>
                  <TableCell className="text-muted">{new Date(batch.created_at).toLocaleDateString()}</TableCell>
                  <TableCell align="right">
                    <Button size="sm" variant="outline" onClick={() => open(batch)}>
                      {t("base.action.open")}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </UICard>
      )}

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

/**
 * Two colours in one bar — what signed and what failed — which the library's
 * single-tone Progress cannot draw, so the track is drawn here from the same
 * tokens.
 */
function Progress({ signed, failed, total }: { signed: number; failed: number; total: number }) {
  const done = total ? Math.round(((signed + failed) / total) * 100) : 0;
  return (
    <div className="min-w-[120px]">
      <div
        className="h-1.5 bg-surface-2 rounded-full overflow-hidden flex"
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={done}
      >
        <div className="bg-success-solid h-full" style={{ width: `${total ? (signed / total) * 100 : 0}%` }} />
        <div className="bg-danger-solid h-full" style={{ width: `${total ? (failed / total) * 100 : 0}%` }} />
      </div>
      <div className="text-xs text-muted mt-1 font-mono">
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
      <Button variant="ghost" size="sm" onClick={onBack} leadingIcon={<ChevronLeft />} className="-ms-2">
        {t("esign.action.back_to_batches")}
      </Button>

      <PageHeader
        icon={<Layers className="w-7 h-7 text-accent" />}
        title={batch.name}
        subtitle={t("esign.view.batch_detail_subtitle", {
          signed: batch.signed,
          total: batch.total,
        })}
        actions={
          <div className="flex gap-2">
            {pending && batch.status !== "CANCELLED" && (
              <Button onClick={run} loading={running} leadingIcon={<Play />}>
                {running ? t("esign.message.batch_running") : t("esign.action.run_batch")}
              </Button>
            )}
            {batch.status !== "CANCELLED" && batch.status !== "COMPLETED" && (
              <Button variant="outline" onClick={cancelBatch} leadingIcon={<XCircle />}>
                {t("base.action.cancel")}
              </Button>
            )}
          </div>
        }
      />

      {error && <Alert variant="danger" live dismissible onDismiss={() => setError(null)}>{error}</Alert>}

      {code && (
        <Alert variant="info" icon={<Smartphone className="mt-0.5 size-4 shrink-0 animate-pulse" aria-hidden />} live>
          <div className="text-center">
            <p className="font-semibold text-sm">{t("esign.message.pin2_instruction")}</p>
            <div className="flex justify-center gap-2 mt-3">
              {code.split("").map((digit, index) => (
                <span
                  key={index}
                  className="w-10 h-12 inline-flex items-center justify-center text-xl font-semibold font-mono text-accent bg-surface rounded-lg border border-accent-border"
                >
                  {digit}
                </span>
              ))}
            </div>
          </div>
        </Alert>
      )}

      <Card title={t("esign.view.batch_documents")}>
        <Table containerClassName="rounded-none border-0" className="text-xs" scrollLabel={t("esign.view.batch_documents")}>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">#</TableHead>
              <TableHead>{t("esign.field.document")}</TableHead>
              <TableHead>{t("base.field.status")}</TableHead>
              <TableHead>{t("esign.field.detail")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(batch.items ?? []).map((item, index) => (
              <TableRow key={item.id}>
                <TableCell className="text-muted font-mono">{index + 1}</TableCell>
                <TableCell>
                  <div className="font-semibold text-foreground flex items-center gap-1.5">
                    <FileText className="w-3.5 h-3.5 text-muted" aria-hidden />
                    {item.document_title}
                  </div>
                  <div className="text-muted font-mono mt-0.5">{item.file_name}</div>
                </TableCell>
                <TableCell>
                  <ItemBadge status={item.status} />
                </TableCell>
                <TableCell className="text-muted">{item.error || "—"}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
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

  // Escape and the backdrop do not dismiss: the picked set and the name would
  // go with it. Cancel is the way out, as before.
  return (
    <Dialog open>
      <DialogContent
        {...pinnedDialogProps}
        size="lg"
        aria-describedby={undefined}
        className="max-h-[90dvh] flex flex-col"
      >
        <DialogHeader>
          <DialogTitle>{t("esign.view.new_batch_title")}</DialogTitle>
        </DialogHeader>
        {error && <Alert variant="danger" live dismissible onDismiss={() => setError(null)}>{error}</Alert>}

        <form onSubmit={submit} className="flex-1 flex flex-col min-h-0 space-y-4">
          <Input
            id="batch-name"
            label={`${t("esign.field.batch_name")} *`}
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder={t("esign.field.batch_name_placeholder")}
            required
          />

          <div className="flex-1 min-h-0 flex flex-col">
            <span className="block text-sm font-medium text-foreground mb-1.5">
              {t("esign.field.batch_documents", { count: picked.size })}
            </span>
            <div className="flex-1 overflow-y-auto border border-line rounded-lg divide-y divide-line">
              {loading ? (
                <div className="px-4">
                  <ListSkeleton rows={3} />
                </div>
              ) : documents.length === 0 ? (
                <EmptyState icon={<FileText />} title={t("esign.message.no_pending_documents")} className="border-0 rounded-none" />
              ) : (
                documents.map((doc) => (
                  <div key={doc.id} className="px-3 py-2.5 hover:bg-surface-hover">
                    <Checkbox
                      checked={picked.has(doc.id)}
                      onCheckedChange={() => toggle(doc.id)}
                      label={<span className="block font-medium truncate">{doc.title}</span>}
                      description={<span className="font-mono truncate">{doc.file_name}</span>}
                    />
                  </div>
                ))
              )}
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              {t("base.action.cancel")}
            </Button>
            <Button type="submit" loading={busy} disabled={picked.size === 0 || !name.trim()}>
              {t("esign.action.create_batch")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
