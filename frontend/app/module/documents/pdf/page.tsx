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
import { Banner, EmptyState, Loading, Modal, PageHeader, cardClass, fieldClass, tableHeadClass } from "@/components/ui";
import { Badge, useErrorMessage, formatBytes } from "@/components/esign/shared";

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

  const archive = async (doc: EsignDocument) => {
    // Signed PDFs are evidence, so this archives rather than destroys — say so
    // before asking, because "delete" reads as irreversible.
    if (!window.confirm(t("esign.message.confirm_archive", { title: doc.title }))) return;
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
        icon={<PenTool className="w-7 h-7 text-indigo-600" />}
        title={t("esign.view.title")}
        subtitle={t("esign.view.subtitle")}
        actions={
          tab === "documents" ? (
            <button
              onClick={() => setShowUpload(true)}
              className="bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-semibold px-4 py-2 rounded-lg flex items-center gap-2 shadow-sm transition"
            >
              <Plus className="w-4 h-4" />
              {t("esign.action.upload")}
            </button>
          ) : undefined
        }
      />

      {error && <Banner tone="error" message={error} onDismiss={() => setError(null)} />}
      {notice && <Banner tone="success" message={notice} onDismiss={() => setNotice(null)} />}

      <nav className="flex gap-1 border-b border-line" role="tablist">
        <TabButton active={tab === "sign"} onClick={() => setTab("sign")} icon={<Smartphone className="w-4 h-4" />}>
          {t("esign.view.tab_sign")}
        </TabButton>
        <TabButton active={tab === "documents"} onClick={() => setTab("documents")} icon={<FileText className="w-4 h-4" />}>
          {t("esign.view.tab_documents")}
        </TabButton>
      </nav>

      {tab === "sign" && <EidSignView onSigned={load} />}

      {tab === "documents" && (
        <>
          {loading ? (
            <Loading label={t("esign.message.loading")} />
          ) : (
            <div className={`${cardClass} overflow-x-auto`}>
              <table className="w-full text-left text-xs text-muted">
                <thead className={tableHeadClass}>
                  <tr>
                    <th className="px-4 py-3">{t("esign.field.document")}</th>
                    <th className="px-4 py-3">{t("esign.field.pages")}</th>
                    <th className="px-4 py-3">{t("base.field.status")}</th>
                    <th className="px-4 py-3">{t("esign.field.signer")}</th>
                    <th className="px-4 py-3">{t("base.field.date")}</th>
                    <th className="px-4 py-3 text-right">{t("base.field.actions")}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-line">
                  {documents.length === 0 && (
                    <tr>
                      <td colSpan={6}>
                        <EmptyState message={t("esign.message.empty")} />
                      </td>
                    </tr>
                  )}
                  {documents.map((doc) => (
                    <tr key={doc.id} className="hover:bg-surface-hover">
                      <td className="px-4 py-3">
                        <div className="font-semibold text-foreground flex items-center gap-1.5">
                          <FileText className="w-3.5 h-3.5 text-muted" />
                          {doc.title}
                        </div>
                        <div className="text-muted font-mono mt-0.5">
                          {doc.file_name} · {formatBytes(doc.byte_size)}
                        </div>
                      </td>
                      <td className="px-4 py-3 font-mono">{doc.page_count}</td>
                      <td className="px-4 py-3">
                        {doc.status === "SIGNED" ? (
                          <Badge tone="bg-emerald-50 text-emerald-700 border-emerald-200">
                            <CheckCircle className="w-3 h-3" />
                            {t("esign.state.signed")}
                          </Badge>
                        ) : (
                          <Badge tone="bg-amber-50 text-amber-700 border-amber-200">
                            <Clock className="w-3 h-3" />
                            {t("esign.state.pending")}
                          </Badge>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        {doc.signer_name || doc.signer_reg_no ? (
                          <div className="space-y-0.5">
                            <Badge tone="bg-blue-50 text-blue-700 border-blue-200">
                              <ShieldCheck className="w-3 h-3" />
                              {doc.signer_name || doc.signer_reg_no}
                            </Badge>
                            {doc.on_behalf_of_name && (
                              <div className="text-[11px] text-muted">{doc.on_behalf_of_name}</div>
                            )}
                            <div className="text-[10px] text-muted font-mono">
                              {doc.provider}
                              {doc.certificate_level ? ` · ${doc.certificate_level}` : ""}
                            </div>
                          </div>
                        ) : (
                          <span className="text-muted italic">—</span>
                        )}
                      </td>
                      <td className="px-4 py-3 text-muted">
                        {new Date(doc.created_at).toLocaleDateString()}
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center justify-end gap-2 flex-wrap">
                          {doc.status !== "SIGNED" && (
                            <>
                              <button
                                onClick={() => setSignDoc(doc)}
                                disabled={!hsmAvailable}
                                title={hsmAvailable ? undefined : t("esign.message.hsm_disabled")}
                                className="bg-surface-2 hover:bg-slate-200 disabled:opacity-40 disabled:cursor-not-allowed text-foreground text-[11px] font-semibold px-3 py-1.5 rounded-lg flex items-center gap-1"
                              >
                                <PenTool className="w-3 h-3" />
                                {t("esign.action.sign_hsm")}
                              </button>
                              <SignWithEidButton document={doc} onDone={load} onError={report} />
                            </>
                          )}
                          <button
                            onClick={() => download(doc, "original")}
                            className="bg-surface-2 hover:bg-slate-200 text-foreground text-[11px] font-semibold px-3 py-1.5 rounded-lg flex items-center gap-1"
                            title={t("esign.action.download_original")}
                          >
                            <Download className="w-3 h-3" />
                            {t("esign.action.original_short")}
                          </button>
                          {doc.status === "SIGNED" && (
                            <button
                              onClick={() => download(doc, "signed")}
                              className="bg-emerald-50 hover:bg-emerald-100 text-emerald-700 border border-emerald-200 text-[11px] font-semibold px-3 py-1.5 rounded-lg flex items-center gap-1"
                              title={t("esign.action.download_signed")}
                            >
                              <Download className="w-3 h-3" />
                              {t("esign.action.signed_short")}
                            </button>
                          )}
                          <button
                            onClick={() => archive(doc)}
                            className="text-muted hover:text-red-600 p-1.5 rounded-lg"
                            title={t("esign.action.archive")}
                            aria-label={t("esign.action.archive")}
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

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

function TabButton({
  active,
  onClick,
  icon,
  children,
}: {
  active: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <button
      role="tab"
      aria-selected={active}
      onClick={onClick}
      className={`px-4 py-2.5 text-sm font-semibold border-b-2 -mb-px flex items-center gap-2 transition ${
        active
          ? "border-indigo-600 text-indigo-700"
          : "border-transparent text-muted hover:text-foreground"
      }`}
    >
      {icon}
      {children}
    </button>
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
      <span className="bg-indigo-50 text-indigo-700 border border-indigo-200 text-[11px] font-semibold px-3 py-1.5 rounded-lg inline-flex items-center gap-1.5 font-mono">
        <Smartphone className="w-3 h-3 animate-pulse" />
        {code}
      </span>
    );
  }

  return (
    <button
      onClick={start}
      className="bg-indigo-600 hover:bg-indigo-700 text-white text-[11px] font-semibold px-3 py-1.5 rounded-lg flex items-center gap-1"
    >
      <Smartphone className="w-3 h-3" />
      {t("esign.action.sign_eid")}
    </button>
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

  return (
    <Modal label={t("esign.view.upload_title")}>
      <h2 className="text-xl font-semibold text-foreground mb-4">{t("esign.view.upload_title")}</h2>
      {error && <div className="mb-3"><Banner tone="error" message={error} onDismiss={() => setError(null)} /></div>}
      <form onSubmit={submit} className="space-y-4">
        <div>
          <label htmlFor="esign-title" className="block text-xs font-semibold text-foreground mb-1">
            {t("esign.field.title")}
          </label>
          <input
            id="esign-title"
            type="text"
            placeholder={t("esign.field.title_placeholder")}
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            className={fieldClass}
          />
          <p className="text-[11px] text-muted mt-1">{t("esign.message.title_optional")}</p>
        </div>

        <div>
          <label htmlFor="esign-file" className="block text-xs font-semibold text-foreground mb-1">
            {t("esign.field.file", { max: maxMB })} *
          </label>
          <input
            id="esign-file"
            type="file"
            accept="application/pdf,.pdf"
            onChange={(event) => setFile(event.target.files?.[0] || null)}
            className="w-full text-xs text-muted file:mr-3 file:px-3 file:py-2 file:rounded-lg file:border-0 file:bg-indigo-50 file:text-indigo-700 file:text-xs file:font-semibold hover:file:bg-indigo-100"
            required
          />
        </div>

        <div className="flex items-center gap-2 pt-2">
          <button
            type="button"
            onClick={onClose}
            className="w-1/2 bg-surface-2 hover:bg-slate-200 text-foreground font-medium py-2 rounded-lg text-xs"
          >
            {t("base.action.cancel")}
          </button>
          <button
            type="submit"
            disabled={busy || !file}
            className="w-1/2 bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white font-medium py-2 rounded-lg text-xs inline-flex items-center justify-center gap-1.5"
          >
            <Upload className="w-3.5 h-3.5" />
            {busy ? t("esign.message.uploading") : t("esign.action.submit_upload")}
          </button>
        </div>
      </form>
    </Modal>
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

  return (
    <Modal size="lg" scrollable className="my-8" label={t("esign.view.sign_title")}>
      <h2 className="text-xl font-semibold text-foreground mb-1">{t("esign.view.sign_title")}</h2>
      <p className="text-xs text-muted mb-4">
        {t("esign.view.sign_placement", { title: doc.title, page: doc.page_count })}
      </p>

      {error && <div className="mb-3"><Banner tone="error" message={error} onDismiss={() => setError(null)} /></div>}

      <div className="space-y-4">
        <div className="border border-line rounded-lg p-4 space-y-3">
          <div className="text-xs font-semibold text-foreground uppercase tracking-wide">
            {t("esign.view.step_certificate")}
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label htmlFor="hsm-phone" className="block text-xs font-semibold text-foreground mb-1">
                {t("esign.field.phone")} *
              </label>
              <input
                id="hsm-phone"
                type="tel"
                placeholder="88001234"
                value={phone}
                onChange={(event) => setPhone(event.target.value)}
                disabled={!!cert}
                className={`${fieldClass} disabled:bg-surface-2`}
              />
            </div>
            <div>
              <label htmlFor="hsm-reg" className="block text-xs font-semibold text-foreground mb-1">
                {t("esign.field.civil_id")} *
              </label>
              <input
                id="hsm-reg"
                type="text"
                placeholder="УА00112233"
                value={regNo}
                onChange={(event) => setRegNo(event.target.value)}
                disabled={!!cert}
                className={`${fieldClass} disabled:bg-surface-2`}
              />
            </div>
          </div>
          {cert ? (
            <div className="bg-emerald-50 border border-emerald-200 text-emerald-700 text-xs font-semibold px-3 py-2 rounded-lg flex items-center gap-1.5">
              <ShieldCheck className="w-4 h-4" />
              {t("esign.message.certificate_valid", { name: `${cert.surname} ${cert.given_name}` })}
            </div>
          ) : (
            <button
              onClick={check}
              disabled={busy || !phone || !regNo}
              className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white text-xs font-semibold px-4 py-2 rounded-lg"
            >
              {busy ? t("esign.message.checking") : t("esign.action.check_certificate")}
            </button>
          )}
        </div>

        <div className="border border-line rounded-lg p-4">
          <SignaturePad onChange={setSignature} disabled={!cert} />
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={onClose}
            className="w-1/2 bg-surface-2 hover:bg-slate-200 text-foreground font-medium py-2 rounded-lg text-xs"
          >
            {t("base.action.cancel")}
          </button>
          <button
            onClick={sign}
            disabled={busy || !cert || !signature}
            className="w-1/2 bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 text-white font-medium py-2 rounded-lg text-xs flex items-center justify-center gap-1.5"
          >
            <PenTool className="w-3.5 h-3.5" />
            {busy ? t("esign.message.signing") : t("esign.view.sign_title")}
          </button>
        </div>
      </div>
    </Modal>
  );
}
