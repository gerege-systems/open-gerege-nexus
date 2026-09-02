"use client";

import React, { useCallback, useEffect, useRef, useState } from "react";
import {
  Building2,
  Clock,
  Download,
  FileText,
  PenLine,
  RotateCcw,
  ShieldCheck,
  Smartphone,
  Upload,
  UserRound,
} from "lucide-react";
import {
  esign,
  saveBlob,
  type Representation,
  type SignSession,
} from "@/lib/esign";
import { useI18n } from "@/lib/i18n";
import { fieldClass } from "@/components/ui";
import { errorCode, useErrorMessage } from "./shared";

/**
 * Citizen PDF signing with eID Mongolia (PIN2).
 *
 * Pick a file → optionally choose an organisation to sign for → a verification
 * code appears → confirm with PIN2 in the eID Mongolia app → poll → download
 * the signed PDF.
 *
 * Signing in an organisation's name still uses the citizen's personal PIN2
 * certificate; eID checks the representation right when the ceremony starts.
 * It is a delegation record, not a company seal.
 */
type Phase =
  | { kind: "idle" }
  | { kind: "uploading"; filename: string; orgName?: string }
  // The account is not linked to eID, so who signs has to be asked rather than
  // inferred. The chosen file is carried through so the citizen does not have
  // to pick it again after answering.
  | { kind: "identity"; file: File; orgName?: string; message: string }
  | { kind: "waiting"; session: SignSession; orgName?: string }
  | { kind: "completed"; session: SignSession; orgName?: string }
  | { kind: "error"; message: string };

/**
 * Guard the upload in the browser so a citizen on a slow connection is told
 * immediately instead of pushing 25MB up the wire only to meet a 413. It has
 * to stay in step with the backend's policy limit and the edge proxy's
 * client_max_body_size.
 */
const MAX_UPLOAD_BYTES = 25 * 1024 * 1024;

/**
 * How long to wait before asking again after the server answers "still
 * pending".
 *
 * The reference implementation polls on a fixed 1.5s interval because its
 * status endpoint returns immediately. Ours long-polls for up to 20 seconds,
 * so a fixed interval would stack a dozen concurrent requests against the same
 * ceremony. Each request is therefore issued only after the previous one has
 * answered, and this is the pause between them.
 */
const POLL_PAUSE_MS = 1500;

export default function EidSignView({ onSigned }: { onSigned?: () => void }) {
  const { t, locale } = useI18n();
  const describe = useErrorMessage();
  const [phase, setPhase] = useState<Phase>({ kind: "idle" });
  const [orgEtsi, setOrgEtsi] = useState("");
  // Kept for the session only, never persisted: it is a national identifier,
  // and it exists here purely so a citizen signing several documents does not
  // retype it each time.
  const [signerId, setSignerId] = useState("");
  const [orgs, setOrgs] = useState<Representation[]>([]);
  const [orgsLoading, setOrgsLoading] = useState(true);
  const fileRef = useRef<HTMLInputElement | null>(null);

  // Flipped on unmount and on reset so an in-flight long poll cannot resurrect
  // a ceremony the user has already walked away from.
  const cancelled = useRef(false);

  const orgLabel = useCallback(
    (org: Representation) => (locale === "en" && org.org_name_en ? org.org_name_en : org.org_name),
    [locale],
  );

  useEffect(() => {
    let active = true;
    esign
      .organizations()
      .then((list) => active && setOrgs(list || []))
      .catch(() => active && setOrgs([]))
      .finally(() => active && setOrgsLoading(false));
    return () => {
      active = false;
    };
  }, []);

  useEffect(
    () => () => {
      cancelled.current = true;
    },
    [],
  );

  /** Polls until the ceremony reaches a terminal state. */
  const watch = useCallback(
    async (session: SignSession, orgName?: string) => {
      while (!cancelled.current) {
        let current: SignSession;
        try {
          current = await esign.session(session.session_id);
        } catch {
          // A transient failure is not a verdict. The ceremony is still open
          // on the citizen's phone, so keep waiting rather than declaring it
          // failed under them.
          await sleep(POLL_PAUSE_MS);
          continue;
        }
        if (cancelled.current) return;

        if (current.state === "completed") {
          setPhase({ kind: "completed", session: current, orgName });
          onSigned?.();
          return;
        }
        if (current.state === "rejected" || current.state === "expired" || current.state === "failed") {
          setPhase({
            kind: "error",
            message:
              current.state === "expired"
                ? t("esign.message.sign_expired")
                : current.state === "rejected"
                  ? t("esign.message.sign_rejected")
                  : t("esign.message.sign_failed"),
          });
          return;
        }
        await sleep(POLL_PAUSE_MS);
      }
    },
    [onSigned, t],
  );

  async function onFile(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    // Cleared straight away so picking the same file twice still fires change.
    event.target.value = "";
    if (!file) return;

    if (file.type !== "application/pdf" && !file.name.toLowerCase().endsWith(".pdf")) {
      setPhase({ kind: "error", message: t("esign.message.not_a_pdf") });
      return;
    }
    if (file.size > MAX_UPLOAD_BYTES) {
      setPhase({
        kind: "error",
        message: t("esign.message.file_too_large", { size: (file.size / (1024 * 1024)).toFixed(1) }),
      });
      return;
    }

    const selected = orgs.find((org) => org.org_etsi === orgEtsi);
    const orgName = selected ? orgLabel(selected) : undefined;

    await start(file, orgName, signerId);
  }

  /** Starts a ceremony, asking who signs when the account cannot say. */
  async function start(file: File, orgName: string | undefined, signer: string) {
    cancelled.current = false;
    setPhase({ kind: "uploading", filename: file.name, orgName });
    try {
      const session = await esign.signFile(file, orgEtsi || undefined, signer.trim() || undefined);
      if (cancelled.current) return;
      setPhase({ kind: "waiting", session, orgName });
      void watch(session, orgName);
    } catch (err) {
      const code = errorCode(err);
      // Not a dead end: the API takes an explicit signer, so ask for one and
      // keep the file rather than making the citizen start over.
      if (code === "NO_SIGNER_IDENTITY" || code === "INVALID_SIGNER") {
        setPhase({ kind: "identity", file, orgName, message: describe(err) });
        return;
      }
      setPhase({ kind: "error", message: describe(err, t("esign.message.sign_failed")) });
    }
  }

  function reset() {
    cancelled.current = true;
    setPhase({ kind: "idle" });
  }

  async function cancel(session: SignSession) {
    cancelled.current = true;
    try {
      await esign.cancelSession(session.session_id);
    } catch {
      // The ceremony is abandoned from this side either way; a failed cancel
      // only leaves a row for the housekeeping sweep to close.
    }
    setPhase({ kind: "idle" });
  }

  async function downloadSigned(session: SignSession) {
    try {
      const blob = await esign.downloadSigned(session.session_id);
      saveBlob(blob, session.filename.replace(/\.pdf$/i, "") + "-signed.pdf");
    } catch (err) {
      setPhase({ kind: "error", message: describe(err, t("esign.message.error_download_short")) });
    }
  }

  return (
    <>
      <input
        ref={fileRef}
        type="file"
        accept="application/pdf,.pdf"
        className="hidden"
        onChange={onFile}
        aria-label={t("esign.action.pick_pdf")}
      />

      {phase.kind === "idle" && (
        <section className="bg-surface border border-line rounded-xl shadow-sm">
          <header className="px-4 py-3 border-b border-line flex items-center gap-2">
            <PenLine className="w-4 h-4 text-indigo-600" />
            <h2 className="text-sm font-semibold text-foreground">{t("esign.view.pick_document")}</h2>
          </header>

          <div className="px-4 py-8 text-center">
            <div className="max-w-sm mx-auto mb-6 text-left">
              <label
                htmlFor="esign-onbehalf"
                className="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wider text-muted"
              >
                <Building2 className="w-3.5 h-3.5" />
                {t("esign.field.sign_as")}
              </label>
              <select
                id="esign-onbehalf"
                value={orgEtsi}
                onChange={(event) => setOrgEtsi(event.target.value)}
                disabled={orgsLoading}
                className="mt-1.5 w-full h-[38px] px-3 py-2 text-sm border border-input rounded-lg bg-surface focus:ring-2 focus:ring-indigo-500 disabled:bg-surface-2"
              >
                <option value="">{t("esign.field.sign_as_self")}</option>
                {orgs.map((org) => (
                  <option key={org.org_etsi} value={org.org_etsi}>
                    {orgLabel(org)}
                    {org.right_type ? ` (${org.right_type})` : ""}
                  </option>
                ))}
              </select>
              <p className="text-xs text-muted mt-1.5">
                {orgEtsi
                  ? t("esign.message.on_behalf_hint")
                  : !orgsLoading && orgs.length === 0
                    ? t("esign.message.no_organizations")
                    : t("esign.message.sign_as_hint")}
              </p>
            </div>

            <button
              type="button"
              onClick={() => fileRef.current?.click()}
              className="bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-semibold px-5 py-2.5 rounded-lg inline-flex items-center gap-2 shadow-sm transition"
            >
              <Upload className="w-4 h-4" />
              {t("esign.action.pick_pdf")}
            </button>
            <p className="text-xs text-muted mt-3">{t("esign.message.pdf_only")}</p>
          </div>
        </section>
      )}

      {phase.kind === "identity" && (
        <section className="bg-surface border border-line rounded-xl shadow-sm">
          <header className="px-4 py-3 border-b border-line flex items-center gap-2">
            <UserRound className="w-4 h-4 text-indigo-600" />
            <h2 className="text-sm font-semibold text-foreground">{t("esign.view.signer_identity")}</h2>
          </header>

          <form
            className="px-4 py-6"
            onSubmit={(event) => {
              event.preventDefault();
              if (signerId.trim()) void start(phase.file, phase.orgName, signerId);
            }}
          >
            <p className="text-sm text-muted max-w-md mx-auto text-center">{phase.message}</p>

            <div className="max-w-sm mx-auto mt-5">
              <label htmlFor="esign-signer-id" className="block text-xs font-semibold text-foreground mb-1">
                {t("esign.field.signer_id")} *
              </label>
              <input
                id="esign-signer-id"
                value={signerId}
                onChange={(event) => setSignerId(event.target.value.toUpperCase())}
                placeholder={t("esign.field.signer_id_placeholder")}
                autoFocus
                className={fieldClass}
              />
              <p className="text-xs text-muted mt-2">{t("esign.message.signer_id_hint")}</p>
              <p className="text-xs text-muted mt-1">{t("esign.message.signer_id_link_hint")}</p>

              <p className="text-xs text-muted mt-4 inline-flex items-center gap-1.5">
                <FileText className="w-3.5 h-3.5" />
                {phase.file.name}
              </p>
            </div>

            <div className="mt-5 flex gap-2.5 justify-center flex-wrap">
              <button
                type="submit"
                disabled={!signerId.trim()}
                className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white text-sm font-semibold px-5 py-2.5 rounded-lg"
              >
                {t("esign.action.continue_signing")}
              </button>
              <button
                type="button"
                onClick={reset}
                className="bg-surface-2 hover:bg-slate-200 text-foreground text-sm font-medium px-4 py-2 rounded-lg inline-flex items-center gap-1.5"
              >
                <RotateCcw className="w-3.5 h-3.5" />
                {t("base.action.cancel")}
              </button>
            </div>
          </form>
        </section>
      )}

      {phase.kind === "uploading" && (
        <section className="bg-surface border border-line rounded-xl shadow-sm p-8 text-center">
          <Clock className="w-7 h-7 text-indigo-600 mx-auto animate-pulse" />
          <p className="font-semibold text-foreground mt-3">{t("esign.message.uploading")}</p>
          <p className="text-sm text-muted mt-1 inline-flex items-center gap-1.5">
            <FileText className="w-3.5 h-3.5" />
            {phase.filename}
          </p>
        </section>
      )}

      {phase.kind === "waiting" && (
        <section className="bg-surface border border-line rounded-xl shadow-sm p-8 text-center">
          <div className="w-14 h-14 rounded-2xl bg-indigo-50 text-indigo-600 inline-flex items-center justify-center">
            <Smartphone className="w-7 h-7" />
          </div>
          <h2 className="text-lg font-semibold text-foreground mt-3.5">{t("esign.view.confirm_on_phone")}</h2>
          <p className="text-sm text-muted mt-1 inline-flex items-center gap-1.5">
            <FileText className="w-3.5 h-3.5" />
            {phase.session.filename}
          </p>
          {phase.orgName && (
            <p className="text-sm text-indigo-600 mt-1 flex items-center justify-center gap-1.5">
              <Building2 className="w-3.5 h-3.5" />
              {t("esign.message.on_behalf_of", { org: phase.orgName })}
            </p>
          )}

          {phase.session.verification_code && (
            <>
              <p className="text-[11px] uppercase tracking-[0.18em] text-muted mt-5">
                {t("esign.field.verification_code")}
              </p>
              <div className="flex justify-center gap-2.5 mt-2">
                {phase.session.verification_code.split("").map((digit, index) => (
                  <span
                    key={index}
                    className="w-11 h-14 inline-flex items-center justify-center text-2xl font-semibold font-mono text-indigo-600 bg-surface-2 rounded-xl"
                  >
                    {digit}
                  </span>
                ))}
              </div>
            </>
          )}

          <p className="text-sm text-muted mt-5 max-w-sm mx-auto leading-relaxed">
            {t("esign.message.pin2_instruction")}
          </p>

          <button
            type="button"
            onClick={() => cancel(phase.session)}
            className="mt-5 bg-surface-2 hover:bg-slate-200 text-foreground text-sm font-medium px-4 py-2 rounded-lg inline-flex items-center gap-1.5"
          >
            <RotateCcw className="w-3.5 h-3.5" />
            {t("base.action.cancel")}
          </button>
        </section>
      )}

      {phase.kind === "completed" && (
        <section className="bg-surface border border-line rounded-xl shadow-sm p-8 text-center">
          <div className="w-14 h-14 rounded-2xl bg-emerald-50 text-emerald-600 inline-flex items-center justify-center">
            <ShieldCheck className="w-7 h-7" />
          </div>
          <h2 className="text-lg font-semibold text-foreground mt-3.5">{t("esign.message.sign_success")}</h2>
          <p className="text-sm text-muted mt-1">{phase.session.filename}</p>
          {phase.orgName && (
            <p className="text-sm text-indigo-600 mt-1 flex items-center justify-center gap-1.5">
              <Building2 className="w-3.5 h-3.5" />
              {t("esign.message.on_behalf_of", { org: phase.orgName })}
            </p>
          )}
          {phase.session.certificate_level && (
            <p className="text-xs text-muted mt-2 font-mono">{phase.session.certificate_level}</p>
          )}

          <div className="mt-5 flex gap-2.5 justify-center flex-wrap">
            <button
              type="button"
              onClick={() => downloadSigned(phase.session)}
              className="bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-semibold px-4 py-2 rounded-lg inline-flex items-center gap-2"
            >
              <Download className="w-4 h-4" />
              {t("base.action.download")}
            </button>
            <button
              type="button"
              onClick={reset}
              className="bg-surface-2 hover:bg-slate-200 text-foreground text-sm font-medium px-4 py-2 rounded-lg inline-flex items-center gap-1.5"
            >
              <RotateCcw className="w-3.5 h-3.5" />
              {t("esign.action.sign_another")}
            </button>
          </div>
        </section>
      )}

      {phase.kind === "error" && (
        <section className="bg-surface border border-line rounded-xl shadow-sm p-8 text-center">
          <p className="font-semibold text-red-600">{phase.message}</p>
          <button
            type="button"
            onClick={reset}
            className="mt-4 bg-surface-2 hover:bg-slate-200 text-foreground text-sm font-medium px-4 py-2 rounded-lg inline-flex items-center gap-1.5"
          >
            <RotateCcw className="w-3.5 h-3.5" />
            {t("base.action.retry")}
          </button>
        </section>
      )}
    </>
  );
}

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));
