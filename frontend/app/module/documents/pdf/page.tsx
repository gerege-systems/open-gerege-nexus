"use client";

import React, { useCallback, useEffect, useState } from "react";
import {
  CheckCircle,
  Clock,
  Download,
  FileText,
  PenTool,
  Plus,
  ShieldCheck,
  Smartphone,
  Trash2,
  Upload,
} from "lucide-react";
import {
  esign,
  saveBlob,
  type EsignDocument,
  type Settings,
} from "@/lib/esign";
import { useI18n } from "@/lib/i18n";
import EidSignView from "@/components/esign/EidSignView";
import SignaturePad from "@/components/esign/SignaturePad";
import { PageHeader } from "@/components/ui";
import { ListEmpty, ListSkeleton, pinnedDialogProps } from "@/components/documents/shared";
import { useErrorMessage, formatBytes } from "@/components/esign/shared";
import {
  Alert,
  Badge,
  Button,
  Card,
  ConfirmationDialog,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  FileUpload,
  IconButton,
  Input,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@gerege-systems/ui";

/**
 * The documents screen. Two things live here because they are two answers to
 * the same question — "sign this PDF":
 *
 *   Sign now — pick a file and sign it with eID Mongolia in one pass. Nothing
 *              is stored beyond the ceremony unless the citizen keeps it.
 *   Documents — the register of PDFs held for signing, their status and the
 *              signed copies.
 */
type Tab = "sign" | "documents";

export default function EsignPage() {
  const { t } = useI18n();
  const describe = useErrorMessage();
  const [tab, setTab] = useState<Tab>("sign");
  const [documents, setDocuments] = useState<EsignDocument[]>([]);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [showUpload, setShowUpload] = useState(false);
  const [signDoc, setSignDoc] = useState<EsignDocument | null>(null);
  const [archiving, setArchiving] = useState<EsignDocument | null>(null);

  const report = useCallback((err: unknown) => setError(describe(err, t("base.message.error"))), [describe, t]);

  const load = useCallback(async () => {
    try {
      const [page, config] = await Promise.all([esign.documents({ limit: 100 }), esign.settings()]);
      setDocuments(page.items || []);
      setSettings(config);
    } catch (err) {
      report(err);
    }
  }, [report]);

  useEffect(() => {
    (async () => {
      setLoading(true);
      await load();
      setLoading(false);
    })();
  }, [load]);

  const download = async (doc: EsignDocument, variant: "original" | "signed") => {
    try {
      const blob = await esign.downloadDocument(doc.id, variant);
      const name =
        variant === "signed" ? doc.file_name.replace(/\.pdf$/i, "") + "-signed.pdf" : doc.file_name;
      saveBlob(blob, name);
    } catch (err) {
      report(err);
    }
  };

  // Signed PDFs are evidence, so this archives rather than destroys — the
  // confirmation dialog says so before asking, because "delete" reads as
  // irreversible.
  const archive = async (doc: EsignDocument) => {
    try {
      await esign.remove(doc.id);
      setNotice(t("esign.message.archived"));
      await load();
    } catch (err) {
      report(err);
    }
  };

  const hsmAvailable = settings ? !settings.policy.require_eid : true;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<PenTool className="w-7 h-7 text-accent" />}
        title={t("esign.view.title")}
        subtitle={t("esign.view.subtitle")}
        actions={
          tab === "documents" ? (
            <Button onClick={() => setShowUpload(true)} leadingIcon={<Plus />}>
              {t("esign.action.upload")}
            </Button>
          ) : undefined
        }
      />

      {error && <Alert variant="danger" live dismissible onDismiss={() => setError(null)}>{error}</Alert>}
      {notice && <Alert variant="success" live dismissible onDismiss={() => setNotice(null)}>{notice}</Alert>}

      <Tabs value={tab} onValueChange={(value) => setTab(value as Tab)}>
        <TabsList>
          <TabsTrigger value="sign">
            <Smartphone className="w-4 h-4" aria-hidden />
            {t("esign.view.tab_sign")}
          </TabsTrigger>
          <TabsTrigger value="documents">
            <FileText className="w-4 h-4" aria-hidden />
            {t("esign.view.tab_documents")}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="sign">
          <EidSignView onSigned={load} />
        </TabsContent>

        <TabsContent value="documents">
          {loading ? (
            <ListSkeleton label={t("esign.message.loading")} />
          ) : documents.length === 0 ? (
            <ListEmpty icon={<FileText />} title={t("esign.message.empty")} />
          ) : (
            <Card padding="none" className="overflow-hidden">
              <Table containerClassName="rounded-none border-0" className="text-xs" scrollLabel={t("esign.view.tab_documents")}>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("esign.field.document")}</TableHead>
                    <TableHead>{t("esign.field.pages")}</TableHead>
                    <TableHead>{t("base.field.status")}</TableHead>
                    <TableHead>{t("esign.field.signer")}</TableHead>
                    <TableHead>{t("base.field.date")}</TableHead>
                    <TableHead align="right">{t("base.field.actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {documents.map((doc) => (
                    <TableRow key={doc.id}>
                      <TableCell>
                        <div className="font-semibold text-foreground flex items-center gap-1.5">
                          <FileText className="w-3.5 h-3.5 text-muted" aria-hidden />
                          {doc.title}
                        </div>
                        <div className="text-muted font-mono mt-0.5">
                          {doc.file_name} · {formatBytes(doc.byte_size)}
                        </div>
                      </TableCell>
                      <TableCell className="font-mono">{doc.page_count}</TableCell>
                      <TableCell>
                        {doc.status === "SIGNED" ? (
                          <Badge tone="success" icon={<CheckCircle />}>{t("esign.state.signed")}</Badge>
                        ) : (
                          <Badge tone="warning" icon={<Clock />}>{t("esign.state.pending")}</Badge>
                        )}
                      </TableCell>
                      <TableCell>
                        {doc.signer_name || doc.signer_reg_no ? (
                          <div className="space-y-0.5">
                            <Badge tone="info" icon={<ShieldCheck />}>{doc.signer_name || doc.signer_reg_no}</Badge>
                            {doc.on_behalf_of_name && (
                              <div className="text-xs text-muted">{doc.on_behalf_of_name}</div>
                            )}
                            <div className="text-xs text-muted font-mono">
                              {doc.provider}
                              {doc.certificate_level ? ` · ${doc.certificate_level}` : ""}
                            </div>
                          </div>
                        ) : (
                          <span className="text-muted">—</span>
                        )}
                      </TableCell>
                      <TableCell className="text-muted">
                        {new Date(doc.created_at).toLocaleDateString()}
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center justify-end gap-2 flex-wrap">
                          {doc.status !== "SIGNED" && (
                            <>
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => setSignDoc(doc)}
                                disabled={!hsmAvailable}
                                title={hsmAvailable ? undefined : t("esign.message.hsm_disabled")}
                                leadingIcon={<PenTool />}
                              >
                                {t("esign.action.sign_hsm")}
                              </Button>
                              <SignWithEidButton document={doc} onDone={load} onError={report} />
                            </>
                          )}
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => download(doc, "original")}
                            title={t("esign.action.download_original")}
                            leadingIcon={<Download />}
                          >
                            {t("esign.action.original_short")}
                          </Button>
                          {doc.status === "SIGNED" && (
                            <Button
                              size="sm"
                              variant="outline"
                              className="text-success"
                              onClick={() => download(doc, "signed")}
                              title={t("esign.action.download_signed")}
                              leadingIcon={<Download />}
                            >
                              {t("esign.action.signed_short")}
                            </Button>
                          )}
                          <IconButton
                            size="sm"
                            onClick={() => setArchiving(doc)}
                            title={t("esign.action.archive")}
                            aria-label={t("esign.action.archive")}
                            icon={<Trash2 />}
                          />
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </Card>
          )}
        </TabsContent>
      </Tabs>

      {showUpload && (
        <UploadModal
          maxMB={settings?.policy.max_upload_mb ?? 25}
          onClose={() => setShowUpload(false)}
          onUploaded={async () => {
            setShowUpload(false);
            setNotice(t("esign.message.uploaded"));
            await load();
          }}
        />
      )}

      {archiving && (
        <ConfirmationDialog
          open
          onOpenChange={(open) => {
            if (!open) setArchiving(null);
          }}
          title={archiving.title}
          description={t("esign.message.confirm_archive", { title: archiving.title })}
          confirmLabel={t("esign.action.archive")}
          cancelLabel={t("base.action.cancel")}
          onConfirm={() => {
            const doc = archiving;
            setArchiving(null);
            void archive(doc);
          }}
        />
      )}

      {signDoc && (
        <HSMSignModal
          doc={signDoc}
          onClose={() => setSignDoc(null)}
          onSigned={async () => {
            setSignDoc(null);
            setNotice(t("esign.message.signed"));
            await load();
          }}
        />
      )}
    </div>
  );
}

/**
 * Signs a stored document with eID. It reuses the same ceremony as the sign
 * tab but keeps the citizen on the register, which is what an operator working
 * through a queue of uploads wants.
 */
function SignWithEidButton({
  document: doc,
  onDone,
  onError,
}: {
  document: EsignDocument;
  onDone: () => Promise<void>;
  onError: (err: unknown) => void;
}) {
  const { t } = useI18n();
  const [code, setCode] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const cancelled = React.useRef(false);

  useEffect(
    () => () => {
      cancelled.current = true;
    },
    [],
  );

  const start = async () => {
    setBusy(true);
    try {
      const session = await esign.signDocument(doc.id);
      setCode(session.verification_code ?? "····");
      while (!cancelled.current) {
        await new Promise((resolve) => setTimeout(resolve, 1500));
        let current;
        try {
          current = await esign.session(session.session_id);
        } catch {
          continue; // transient — the ceremony is still open on the phone
        }
        if (current.state === "completed") {
          await onDone();
          break;
        }
        if (current.state !== "pending") {
          onError(new Error(t("esign.message.sign_failed")));
          break;
        }
      }
    } catch (err) {
      onError(err);
    } finally {
      if (!cancelled.current) {
        setBusy(false);
        setCode(null);
      }
    }
  };

  if (busy) {
    return (
      <Badge tone="accent" variant="outline" icon={<Smartphone className="animate-pulse" />} className="font-mono py-1.5">
        {code}
      </Badge>
    );
  }

  return (
    <Button size="sm" onClick={start} leadingIcon={<Smartphone />}>
      {t("esign.action.sign_eid")}
    </Button>
  );
}

function UploadModal({
  maxMB,
  onClose,
  onUploaded,
}: {
  maxMB: number;
  onClose: () => void;
  onUploaded: () => Promise<void>;
}) {
  const { t } = useI18n();
  const describe = useErrorMessage();
  const [title, setTitle] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!file) return;
    if (file.size > maxMB * 1024 * 1024) {
      setError(t("esign.message.file_too_large", { size: (file.size / (1024 * 1024)).toFixed(1) }));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await esign.upload(file, title);
      await onUploaded();
    } catch (err) {
      setError(describe(err, t("base.message.error")));
    } finally {
      setBusy(false);
    }
  };

  // Escape and the backdrop do not dismiss — Cancel does, as before.
  return (
    <Dialog open>
      <DialogContent {...pinnedDialogProps} aria-describedby={undefined}
      >
        <DialogHeader>
          <DialogTitle>{t("esign.view.upload_title")}</DialogTitle>
        </DialogHeader>
        {error && <Alert variant="danger" live dismissible onDismiss={() => setError(null)}>{error}</Alert>}
        <form onSubmit={submit} className="space-y-4">
          <Input
            id="esign-title"
            type="text"
            label={t("esign.field.title")}
            helperText={t("esign.message.title_optional")}
            placeholder={t("esign.field.title_placeholder")}
            value={title}
            onChange={(event) => setTitle(event.target.value)}
          />

          <div className="flex flex-col gap-1.5">
            <span className="text-sm font-medium text-foreground">{t("esign.field.file", { max: maxMB })} *</span>
            <FileUpload
              accept="application/pdf,.pdf"
              value={file ? [file] : []}
              onChange={(files) => setFile(files[0] ?? null)}
            />
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              {t("base.action.cancel")}
            </Button>
            <Button type="submit" loading={busy} disabled={!file} leadingIcon={<Upload />}>
              {busy ? t("esign.message.uploading") : t("esign.action.submit_upload")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

/** The HSM rail: prove a certificate, draw a signature, the service stamps it. */
function HSMSignModal({
  doc,
  onClose,
  onSigned,
}: {
  doc: EsignDocument;
  onClose: () => void;
  onSigned: () => Promise<void>;
}) {
  const { t } = useI18n();
  const describe = useErrorMessage();
  const [phone, setPhone] = useState("");
  const [regNo, setRegNo] = useState("");
  const [cert, setCert] = useState<{ given_name: string; surname: string } | null>(null);
  const [signature, setSignature] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const check = async () => {
    setBusy(true);
    setError(null);
    try {
      setCert(await esign.checkCertificate({ phone_no: phone, civil_id: regNo }));
    } catch (err) {
      setError(describe(err, t("base.message.error")));
    } finally {
      setBusy(false);
    }
  };

  const sign = async () => {
    if (!cert || !signature) return;
    setBusy(true);
    setError(null);
    try {
      await esign.signWithHSM(doc.id, {
        phone_no: phone,
        signer_name: `${cert.surname} ${cert.given_name}`.trim(),
        signer_reg_no: regNo,
        // toDataURL yields "data:image/png;base64,…"; the API wants only the payload.
        signature_image64: signature.split(",")[1],
      });
      await onSigned();
    } catch (err) {
      setError(describe(err, t("base.message.error")));
    } finally {
      setBusy(false);
    }
  };

  // Escape and the backdrop do not dismiss a signing conversation — Cancel does.
  return (
    <Dialog open>
      <DialogContent
        {...pinnedDialogProps}
        size="lg"
        className="max-h-[90dvh] overflow-y-auto"
      >
        <DialogHeader>
          <DialogTitle>{t("esign.view.sign_title")}</DialogTitle>
          <DialogDescription>
            {t("esign.view.sign_placement", { title: doc.title, page: doc.page_count })}
          </DialogDescription>
        </DialogHeader>

        {error && <Alert variant="danger" live dismissible onDismiss={() => setError(null)}>{error}</Alert>}

        <div className="space-y-4">
          <Card padding="sm" className="space-y-3">
            <div className="text-xs font-semibold text-foreground uppercase tracking-wide">
              {t("esign.view.step_certificate")}
            </div>
            <div className="grid grid-cols-2 gap-3">
              <Input
                id="hsm-phone"
                type="tel"
                label={`${t("esign.field.phone")} *`}
                placeholder="88001234"
                value={phone}
                onChange={(event) => setPhone(event.target.value)}
                disabled={!!cert}
              />
              <Input
                id="hsm-reg"
                type="text"
                label={`${t("esign.field.civil_id")} *`}
                placeholder="УА00112233"
                value={regNo}
                onChange={(event) => setRegNo(event.target.value)}
                disabled={!!cert}
              />
            </div>
            {cert ? (
              <Alert variant="success" icon={<ShieldCheck className="mt-0.5 size-4 shrink-0" aria-hidden />}>
                {t("esign.message.certificate_valid", { name: `${cert.surname} ${cert.given_name}` })}
              </Alert>
            ) : (
              <Button onClick={check} loading={busy} disabled={!phone || !regNo}>
                {busy ? t("esign.message.checking") : t("esign.action.check_certificate")}
              </Button>
            )}
          </Card>

          <Card padding="sm">
            <SignaturePad onChange={setSignature} disabled={!cert} />
          </Card>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              {t("base.action.cancel")}
            </Button>
            <Button onClick={sign} loading={busy && !!cert} disabled={!cert || !signature} leadingIcon={<PenTool />}>
              {busy && cert ? t("esign.message.signing") : t("esign.view.sign_title")}
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  );
}
