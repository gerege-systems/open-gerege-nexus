"use client";

import React, { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Banner, Modal, fieldClass } from "@/components/ui";
import {
  Ban,
  CheckCircle,
  Clock,
  FileText,
  History,
  Loader2,
  PenLine,
  Send,
  ShieldCheck,
  Users,
  XCircle,
} from "lucide-react";

/** A document_records row as the documents API returns it. */
export interface DocumentRecord {
  id: string;
  title: string;
  doc_type: string;
  status: string;
  signed_by?: string;
  signature_hash?: string;
  signer_reg_number?: string;
  signer_method?: string;
  signed_at?: string;
  /** How many signatures the document carries, and how many it asked for. */
  signature_count: number;
  required_signatures: number;
  /** How many steps of its own chain no signature has filled. */
  outstanding_steps: number;
  created_at: string;
}

/** The national identity channel the signature is applied through. */
type SignMethod = "EID" | "DAN";

// The API holds a live poll open for up to eid.PollWindow (25s) and answers the
// moment the citizen approves, so this gap is the only stretch where an approval is
// not being watched — pure delay between the citizen approving and the signature
// being recorded. Kept short for that reason, and non-zero because a mock or a
// fast RP answers at once: without any breather the dialog stacked long-polls
// until the browser ran out of connections to the host. Same figure and same
// reasoning as the sign-in card's GAP.
const POLL_GAP = 400;
// A dropped long-poll is ordinary on a mobile network, and the citizen may be
// about to approve, so a session survives a few failures before it is given up.
const TOLERATED_POLL_FAILURES = 3;
// Used when eID states no deadline, which is the normal case for a push session:
// eID decides when one dies and says so with EXPIRED, so there is nothing to count
// down. This is a BACKSTOP against polling for ever, deliberately set well beyond
// any real ceremony — a push session has been measured still RUNNING nine minutes
// in, and treating a shorter figure as a deadline is what made the sign-in card
// walk away from sessions eID was still waiting on. Keep in step with
// signSessionBackstop in eidsign.go.
const APPROVAL_BACKSTOP = 15 * 60_000;
// The server accepts an approval for this long past eID's own deadline, so a
// small clock difference does not throw one away. Giving up earlier here would
// tell the operator the request expired while the server would still have taken
// it. Keep in step with signSessionGrace in eidsign.go.
const APPROVAL_GRACE = 120_000;

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// The deadline is a foreign absolute timestamp compared against this browser's
// clock, so a workstation running fast could otherwise declare the request expired
// before the first poll went out — while the citizen's phone was showing the
// prompt. The grace absorbs ordinary skew, and the floor keeps a badly wrong clock
// from cutting the ceremony off at the knees.
const MIN_APPROVAL_WINDOW = 30_000;

function statedDeadline(expiresAt?: string) {
  const at = Date.parse(expiresAt ?? "");
  return Number.isNaN(at) ? null : at;
}

function deadlineOf(expiresAt?: string) {
  const stated = statedDeadline(expiresAt);
  if (stated === null) return Date.now() + APPROVAL_BACKSTOP;
  return Math.max(stated + APPROVAL_GRACE, Date.now() + MIN_APPROVAL_WINDOW);
}

/** The only state a document can be signed or rejected in. */
export const PENDING = "PENDING_APPROVAL";

/** What the citizen's device needs to be found, and what the operator reads out. */
interface EIDSignSession {
  session_id: string;
  verification_code: string;
  // Absent when eID states no deadline, which is the normal case for a push
  // session. Absent is not "expired": it means nobody has said when this dies.
  expires_at?: string;
  device_link_url?: string;
  display_text: string;
}

/**
 * Only the four states the module defines get a translated badge. Anything
 * else — the DRAFT the table still defaults to, or a state a later workflow
 * introduces — is shown verbatim rather than dressed up as "Pending", so a
 * screen never claims a document is awaiting signature when it is not.
 */
export function StatusBadge({ status }: { status: string }) {
  const { t } = useI18n();
  const shell = "inline-flex items-center space-x-1 text-[11px] font-semibold px-2.5 py-0.5 rounded-full";

  if (status === "APPROVED") {
    return (
      <span className={`${shell} bg-emerald-50 text-emerald-700 border border-emerald-200`}>
        <CheckCircle className="w-3 h-3 text-emerald-500" />
        <span>{t("documents.state.approved")}</span>
      </span>
    );
  }
  if (status === "REJECTED") {
    return (
      <span className={`${shell} bg-red-50 text-red-700 border border-red-200`}>
        <XCircle className="w-3 h-3 text-red-500" />
        <span>{t("documents.state.rejected")}</span>
      </span>
    );
  }
  if (status === PENDING) {
    return (
      <span className={`${shell} bg-amber-50 text-amber-700 border border-amber-200`}>
        <Clock className="w-3 h-3 text-amber-500" />
        <span>{t("documents.state.pending")}</span>
      </span>
    );
  }
  if (status === "DRAFT") {
    return (
      <span className={`${shell} bg-surface-2 text-muted border border-line`}>
        <FileText className="w-3 h-3 text-muted" />
        <span>{t("documents.state.draft")}</span>
      </span>
    );
  }
  return <span className={`${shell} bg-surface-2 text-muted border border-line`}>{status}</span>;
}

/**
 * How far a document is through its approval chain. A type that needs one
 * signature says nothing: the status badge already carries that.
 */
export function SignatureProgress({ doc }: { doc: DocumentRecord }) {
  const { t } = useI18n();
  if (doc.required_signatures <= 1) return null;
  // A rejected document's progress is moot, and showing it invited the reader to
  // work out whether "2/2" beside a red badge meant the chain had been satisfied.
  // The status badge says everything there is to say about it.
  if (doc.status === "REJECTED") return null;

  // Meeting the count is not enough on two counts. A signature from somebody no step
  // names counts toward the total and satisfies none of the chain — so this used to
  // paint "complete" beside "Pending" on a document that still owed a named approval.
  // And a REJECTED document is over: whatever it collected, its chain was never
  // satisfied, and an emerald "2/2" beside a red badge says it was.
  const complete =
    doc.status !== "REJECTED" &&
    doc.outstanding_steps === 0 &&
    doc.signature_count >= doc.required_signatures;
  return (
    <span
      className={`inline-flex items-center space-x-1 text-[11px] font-semibold px-2 py-0.5 rounded-full border ${
        complete
          ? "bg-emerald-50 text-emerald-700 border-emerald-200"
          : "bg-indigo-50 text-indigo-700 border-indigo-200"
      }`}
      title={t("documents.message.signature_progress", {
        applied: doc.signature_count,
        required: doc.required_signatures,
      })}
    >
      <Users className="w-3 h-3" />
      <span>
        {doc.signature_count}/{doc.required_signatures}
      </span>
    </span>
  );
}

/** Who signed, plus the reg number and hash the identity provider returned. */
export function SignatureCell({ doc }: { doc: DocumentRecord }) {
  const { t } = useI18n();

  if (doc.signed_by) {
    return (
      <div className="space-y-1">
        <span className="bg-blue-50 text-blue-700 text-[11px] font-semibold px-2.5 py-0.5 rounded-full border border-blue-200 inline-flex items-center w-max space-x-1">
          <ShieldCheck className="w-3 h-3 text-blue-500" />
          <span>{doc.signed_by}</span>
        </span>
        {doc.signature_hash && (
          <div className="font-mono text-[10px] text-muted truncate max-w-[220px]" title={doc.signature_hash}>
            {doc.signer_reg_number ? `${doc.signer_reg_number} · ` : ""}
            {doc.signature_hash}
          </div>
        )}
      </div>
    );
  }
  if (doc.status === "REJECTED") {
    return <span className="text-red-400 italic">{t("documents.message.rejected_not_signed")}</span>;
  }
  return <span className="text-muted italic">{t("documents.state.pending_signature")}</span>;
}

/**
 * The outcome of an action, as a screen holds it in state.
 *
 * `type` is the shared Banner's tone, so a screen renders one with
 * `<Banner tone={message.type} message={message.text} />`.
 */
export interface ActionMessage {
  type: "success" | "error";
  text: string;
}

/**
 * Reject and outcome reporting, shared by the documents list and the approval
 * queue so both screens say the same things. Signing itself lives in
 * SignatureDialog, because E-ID signing is a conversation with the citizen's
 * device rather than one call. `onChanged` reloads the caller's rows.
 */
export function useDocumentActions(onChanged: () => void | Promise<void>) {
  const { t } = useI18n();
  // One id per row, not one shared id: two overlapping actions had whichever finished
  // first re-enable the other's buttons while its request was still in flight.
  const [busyIds, setBusyIds] = useState<Set<string>>(new Set());
  const isBusy = (id: string) => busyIds.has(id);
  const setBusyId = (id: string, working: boolean) =>
    setBusyIds((current) => {
      const next = new Set(current);
      if (working) next.add(id);
      else next.delete(id);
      return next;
    });
  const [message, setMessage] = useState<ActionMessage | null>(null);

  const succeed = async (text: string) => {
    setMessage({ type: "success", text });
    await onChanged();
  };

  const fail = (text: string) => setMessage({ type: "error", text });

  const route = async (doc: DocumentRecord) => {
    setBusyId(doc.id, true);
    setMessage(null);
    try {
      await api.routeDocument(doc.id);
      setMessage({ type: "success", text: t("documents.message.route_success", { title: doc.title }) });
      await onChanged();
    } catch (err: any) {
      setMessage({ type: "error", text: err?.message || t("documents.message.route_failed") });
    } finally {
      setBusyId(doc.id, false);
    }
  };

  const reject = async (doc: DocumentRecord) => {
    if (!confirm(t("documents.message.reject_confirm", { title: doc.title }))) return;
    setBusyId(doc.id, true);
    setMessage(null);
    try {
      await api.rejectDocument(doc.id);
      setMessage({ type: "success", text: t("documents.message.reject_success", { title: doc.title }) });
      await onChanged();
    } catch (err: any) {
      setMessage({ type: "error", text: err?.message || t("documents.message.reject_failed") });
    } finally {
      setBusyId(doc.id, false);
    }
  };

  return { isBusy, message, setMessage, succeed, fail, route, reject };
}

/** The Sign / Reject pair, shown only while a document is still pending. */
export function RowActions({
  doc,
  busy,
  canSign,
  canManage,
  onSign,
  onReject,
  onRoute,
}: {
  doc: DocumentRecord;
  busy: boolean;
  canSign: boolean;
  canManage?: boolean;
  onSign: (doc: DocumentRecord) => void;
  onReject: (doc: DocumentRecord) => void;
  onRoute?: (doc: DocumentRecord) => void;
}) {
  const { t } = useI18n();

  // A draft is not awaiting anybody: it has to be sent for approval first, and
  // that is a routing decision rather than a signing one.
  if (doc.status === "DRAFT") {
    if (!canManage || !onRoute) return <span className="text-subtle text-[11px]">—</span>;
    return (
      <button
        onClick={() => onRoute(doc)}
        disabled={busy}
        className="inline-flex items-center space-x-1 px-2.5 py-1.5 rounded-lg text-[11px] font-semibold border border-input text-foreground hover:bg-surface-hover transition disabled:opacity-50"
      >
        <Send className="w-3.5 h-3.5" />
        <span>{t("documents.action.route")}</span>
      </button>
    );
  }

  if (doc.status !== PENDING) return <span className="text-subtle text-[11px]">—</span>;
  if (!canSign) return <span className="text-subtle text-[11px]">—</span>;

  return (
    <div className="flex items-center justify-end space-x-2">
      <button
        onClick={() => onSign(doc)}
        disabled={busy}
        className="inline-flex items-center space-x-1 px-2.5 py-1.5 rounded-lg text-[11px] font-semibold border border-indigo-200 text-indigo-600 hover:bg-indigo-50 transition disabled:opacity-50"
      >
        <PenLine className="w-3.5 h-3.5" />
        <span>{t("documents.action.sign")}</span>
      </button>
      <button
        onClick={() => onReject(doc)}
        disabled={busy}
        className="inline-flex items-center space-x-1 px-2.5 py-1.5 rounded-lg text-[11px] font-semibold border border-red-200 text-red-600 hover:bg-red-50 transition disabled:opacity-50"
      >
        <Ban className="w-3.5 h-3.5" />
        <span>{t("documents.action.reject")}</span>
      </button>
    </div>
  );
}


/**
 * The signing ceremony, which is not the same shape for the two channels.
 *
 * E-ID has no document-signing endpoint. What it has is an approval the citizen
 * gives on their own registered device, against a display text that names the
 * document — and that approval *is* the signature. So the dialog pushes the
 * request, shows the operator the verification code to read out, and waits.
 *
 * DAN exposes no approval push, so it stays a registration number and a code.
 *
 * The dialog owns these calls rather than the caller: a conversation with
 * somebody's phone has states of its own, and they belong next to the screen
 * showing them.
 */
export function SignatureDialog({
  doc,
  onClose,
  onDone,
  onError,
}: {
  doc: DocumentRecord;
  onClose: () => void;
  onDone: (message: string) => void | Promise<void>;
  onError: (message: string) => void;
}) {
  const { t } = useI18n();
  const [method, setMethod] = useState<SignMethod>("EID");
  const [regNumber, setRegNumber] = useState("");
  const [otpCode, setOtpCode] = useState("");

  // Нэвтэрсэн хүний өөрийнх нь eID регистр — асуухгүй, БӨГЛӨЧИХНӨ.
  // «Энэ миний данс юм чинь миний регистрийг мэдэхгүй юм уу» гэдэг асуулт
  // нэвтрэлт eID-ээр явдаг платформ дээр зөв асуулт. Талбар нь засварлагдана:
  // нярав захирлынхаа өмнөөс эхлүүлэх нь хэвийн хэрэглээ хэвээр.
  useEffect(() => {
    let alive = true;
    void (async () => {
      try {
        const profile = await api.profile();
        const eid = profile.identities?.find((identity) => identity.kind === "eid");
        const reg = eid?.claims?.reg_number;
        if (alive && typeof reg === "string" && reg) setRegNumber((current) => current || reg);
      } catch {
        // Профайл уншигдахгүй бол талбар хоосон үлдэнэ — асуудал биш.
      }
    })();
    return () => { alive = false; };
  }, []);
  const [busy, setBusy] = useState(false);
  const [session, setSession] = useState<EIDSignSession | null>(null);
  // The page's banner sits behind this dialog, so a failure reported only there is a
  // failure the operator cannot read while the dialog is still open. It is shown
  // here as well, and the caller is told too so it survives the dialog closing.
  const [failure, setFailure] = useState<string | null>(null);
  const report = (text: string) => {
    setFailure(text);
    onError(text);
  };

  // One check at a time, with a breather between them, a tolerance for the
  // dropped long-polls a mobile network produces, and a deadline — a request that
  // visibly runs out beats one that spins for ever. The eID app can go unanswered
  // without the RP session ever reporting it.
  useEffect(() => {
    if (!session) return;

    const deadline = deadlineOf(session.expires_at);
    let cancelled = false;
    let inflight: AbortController | null = null;
    let failures = 0;

    const wait = async () => {
      while (!cancelled) {
        if (Date.now() >= deadline) {
          report(t("documents.message.approval_expired"));
          setSession(null);
          return;
        }

        const controller = new AbortController();
        inflight = controller;
        try {
          const progress = await api.pollEIDSignature(doc.id, session.session_id, controller.signal);
          if (cancelled) return;
          failures = 0;

          if (progress.state === "COMPLETE") {
            await onDone(t("documents.message.sign_success", { title: doc.title, method: "E-ID" }));
            return;
          }
          if (progress.state === "REFUSED") {
            report(t("documents.message.approval_refused"));
            setSession(null);
            return;
          }
          if (progress.state === "EXPIRED") {
            report(t("documents.message.approval_expired"));
            setSession(null);
            return;
          }
        } catch (err: any) {
          if (cancelled) return;
          // A 4xx is an answer, not a blip: the document is no longer pending, the
          // session is unknown, the signer is not the one this step names. Retrying
          // it three times only delays telling the operator.
          const status: number | undefined = err?.status;
          const answered = typeof status === "number" && status >= 400 && status < 500;
          if (answered || ++failures > TOLERATED_POLL_FAILURES) {
            report(err?.message || t("documents.message.sign_failed"));
            setSession(null);
            return;
          }
        }
        await sleep(POLL_GAP);
      }
    };
    void wait();

    return () => {
      cancelled = true;
      inflight?.abort();
    };
    // onDone/onError are rebuilt by the caller on every render; re-running this on
    // their identity would restart the wait — and push nothing, but drop the
    // in-flight poll — on every unrelated keystroke.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session, doc.id, doc.title]);

  // The dialog counts the request down itself, so an operator can see it die
  // rather than wonder whether it is still alive.
  const [secondsLeft, setSecondsLeft] = useState(0);
  useEffect(() => {
    if (!session) return;
    const deadline = deadlineOf(session.expires_at);
    const tick = () => setSecondsLeft(Math.max(0, Math.ceil((deadline - Date.now()) / 1000)));
    tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
  }, [session]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setFailure(null);
    try {
      if (method === "DAN") {
        await api.signDocumentWithDAN(doc.id, { reg_number: regNumber, otp_code: otpCode });
        await onDone(t("documents.message.sign_success", { title: doc.title, method: "DAN" }));
        return;
      }
      setSession(await api.startEIDSignature(doc.id, regNumber));
    } catch (err: any) {
      report(err?.message || t("documents.message.sign_failed"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal label={t("documents.view.sign_title")}>
      <h2 className="text-xl font-semibold text-foreground mb-1 flex items-center space-x-2">
        <PenLine className="w-5 h-5 text-indigo-600" />
        <span>{t("documents.view.sign_title")}</span>
      </h2>
      <p className="text-xs text-muted mb-4 truncate">{doc.title}</p>

      {failure && (
        <div className="mb-4">
          <Banner tone="error" message={failure} />
        </div>
      )}

      {session ? (
        <div className="space-y-4">
          <div className="rounded-lg border border-indigo-200 bg-indigo-50 p-4 text-center">
            <p className="text-[11px] font-semibold text-indigo-700 uppercase tracking-wide">
              {t("documents.field.verification_code")}
            </p>
            <p className="text-3xl font-semibold tracking-[0.2em] text-indigo-900 mt-1">
              {session.verification_code}
            </p>
            <p className="text-[11px] text-indigo-700 mt-2">{t("documents.message.verification_code_hint")}</p>
          </div>

          <div className="flex items-center gap-2 text-sm text-muted">
            <Loader2 className="w-4 h-4 animate-spin shrink-0" />
            <span className="flex-1">
              {t("documents.message.awaiting_approval", { reg: regNumber.toUpperCase() })}
            </span>
            {/* Only when eID gave a deadline to count down. A countdown we made up
                is worse than none: it hurries the citizen and then says the
                request expired while eID is still waiting for them. */}
            {statedDeadline(session.expires_at) !== null && (
              <span className="font-mono text-xs text-muted tabular-nums">
                {Math.floor(secondsLeft / 60)}:{String(secondsLeft % 60).padStart(2, "0")}
              </span>
            )}
          </div>

          <p className="text-[11px] text-muted">
            {t("documents.message.approval_display_text")}: <span className="italic">{session.display_text}</span>
          </p>

          <button
            type="button"
            onClick={onClose}
            className="w-full bg-surface-2 hover:bg-slate-200 text-foreground font-medium py-2 rounded-lg text-xs"
          >
            {t("base.action.cancel")}
          </button>
        </div>
      ) : (
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-foreground mb-1.5">
              {t("documents.field.signature_method")}
            </label>
            <div className="grid grid-cols-2 gap-2">
              {(["EID", "DAN"] as SignMethod[]).map((m) => (
                <button
                  key={m}
                  type="button"
                  onClick={() => setMethod(m)}
                  className={`py-2 rounded-lg text-xs font-semibold border transition ${
                    method === m
                      ? "bg-indigo-600 text-white border-indigo-600"
                      : "bg-surface text-foreground border-input hover:bg-surface-hover"
                  }`}
                >
                  {m === "EID" ? "E-ID (eidmongolia.mn)" : "DAN (dan.gerege.mn)"}
                </button>
              ))}
            </div>
            <p className="text-[11px] text-muted mt-1.5">
              {method === "EID"
                ? t("documents.message.eid_method_hint")
                : t("documents.message.dan_method_hint")}
            </p>
          </div>

          <div>
            <label className="block text-xs font-semibold text-foreground mb-1">
              {t("documents.field.reg_number")} *
            </label>
            {/* The same bounds the server holds (RegNumberLimit..RegNumberMax), so a
                number that could never be a registration number is caught in the
                field rather than as a 400 after the operator has committed to it. */}
            <input
              type="text"
              placeholder="УБ99010111"
              value={regNumber}
              onChange={(e) => setRegNumber(e.target.value)}
              className={`${fieldClass} font-mono`}
              minLength={8}
              maxLength={64}
              required
            />
          </div>

          {method === "DAN" && (
            <div>
              <label className="block text-xs font-semibold text-foreground mb-1">
                {t("documents.field.otp_code")}
              </label>
              <input
                type="text"
                placeholder="123456"
                value={otpCode}
                onChange={(e) => setOtpCode(e.target.value)}
                className={`${fieldClass} font-mono`}
              />
            </div>
          )}

          <div className="flex items-center space-x-2 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="w-1/2 bg-surface-2 hover:bg-slate-200 text-foreground font-medium py-2 rounded-lg text-xs"
            >
              {t("base.action.cancel")}
            </button>
            <button
              type="submit"
              disabled={busy}
              className="w-1/2 bg-indigo-600 hover:bg-indigo-700 text-white font-medium py-2 rounded-lg text-xs disabled:opacity-50"
            >
              {busy
                ? t("documents.message.signing")
                : method === "EID"
                  ? t("documents.action.request_approval")
                  : t("documents.action.sign")}
            </button>
          </div>
        </form>
      )}
    </Modal>
  );
}

/** One step of the document's own approval chain, as the API returns it. */
export interface DocumentStep {
  order: number;
  name: string;
  signer_reg_number: string;
}

/** One row of a document's signature ledger, as the API returns it. */
export interface AppliedSignature {
  signer_name: string;
  signer_reg_number: string;
  signer_method: string;
  signature_hash: string;
  signed_at: string;
  /** Which step of the document's chain this signature filled. */
  step_order: number;
  certificate_serial?: string;
  certificate_issuer?: string;
}

/**
 * The document's approval trail: every step of the chain it started under, which
 * of them are filled and by whom, and which one it is waiting on. The list can
 * only show a count; this is the part a dispute turns on — who gave each approval,
 * through which channel, on which certificate — and the part an approver needs:
 * whose signature comes next.
 */
export function SignatureHistoryDialog({ doc, onClose }: { doc: DocumentRecord; onClose: () => void }) {
  const { t } = useI18n();
  const [signatures, setSignatures] = useState<AppliedSignature[] | null>(null);
  const [steps, setSteps] = useState<DocumentStep[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    Promise.all([api.getDocumentSignatures(doc.id), api.getDocumentSteps(doc.id)])
      .then(([applied, chain]) => {
        if (!alive) return;
        setSignatures(applied || []);
        setSteps(chain || []);
      })
      .catch((err: any) => alive && setError(err?.message || t("documents.message.history_failed")));
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [doc.id]);

  // Each step takes the signature that filled it. Anything left over — a signature
  // past the end of the chain, or on a document whose type had no chain — is shown
  // below rather than dropped: a trail missing a real approval is worse than none.
  const placed = new Set<AppliedSignature>();
  const chainRows = steps.map((step) => {
    const signature = (signatures || []).find((sig) => sig.step_order === step.order && !placed.has(sig));
    if (signature) placed.add(signature);
    return { step, signature };
  });
  const extraRows = (signatures || [])
    .filter((sig) => !placed.has(sig))
    .map((signature) => ({ step: undefined as DocumentStep | undefined, signature }));
  const rows = [...chainRows, ...extraRows];

  return (
    <Modal size="lg" label={t("documents.view.history_title")}>
      <h2 className="text-xl font-semibold text-foreground mb-1 flex items-center space-x-2">
        <ShieldCheck className="w-5 h-5 text-indigo-600" />
        <span>{t("documents.view.history_title")}</span>
      </h2>
      <div className="flex items-center gap-2 mb-4 min-w-0">
        <p className="text-xs text-muted truncate">{doc.title}</p>
        {/* The trail covers the list, so the status has to travel with it: the same
            unfilled steps mean "still to come" on a pending document and "never
            given" on one that has been decided, and the dialog says nothing else
            about which this is. */}
        <StatusBadge status={doc.status} />
      </div>

      {error ? (
        <p className="text-sm text-red-700 bg-red-50 border border-red-200 rounded-lg p-3">{error}</p>
      ) : signatures === null ? (
        <div className="flex items-center gap-2 text-muted text-sm py-6">
          <Loader2 className="w-4 h-4 animate-spin" />
          {t("documents.message.loading")}
        </div>
      ) : rows.length === 0 ? (
        <p className="text-sm text-muted py-6">{t("documents.message.no_signatures")}</p>
      ) : (
        <ol className="space-y-3 max-h-[60vh] overflow-y-auto">
          {rows.map(({ step, signature }, index) => {
            const filled = Boolean(signature);
            // Only a pending document is still waiting for anything. On a decided
            // one — rejected, or approved before its type had a chain — an unfilled
            // step is an approval that was never given, and calling it "Later" told
            // an operator that a closed document was still moving.
            const open = doc.status === PENDING;
            const isNext = !filled && open && rows.findIndex((r) => !r.signature) === index;

            return (
              <li
                key={step ? `step-${step.order}` : `sig-${index}`}
                className={`border rounded-lg p-3 ${
                  filled
                    ? "border-line"
                    : isNext
                      ? "border-indigo-300 bg-indigo-50/40"
                      : "border-dashed border-line"
                }`}
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span
                        className={`w-5 h-5 shrink-0 rounded-full text-[10px] font-semibold grid place-items-center ${
                          filled ? "bg-emerald-100 text-emerald-700" : "bg-surface-2 text-muted"
                        }`}
                      >
                        {step ? step.order : index + 1}
                      </span>
                      <p className="text-sm font-semibold text-foreground truncate">
                        {signature ? signature.signer_name : step?.name || "—"}
                      </p>
                      {!step && signature && steps.length > 0 && (
                        <span className="shrink-0 text-[10px] text-muted italic">
                          {t("documents.message.signature_outside_chain")}
                        </span>
                      )}
                    </div>
                    <p className="font-mono text-[11px] text-muted mt-0.5 pl-7">
                      {signature
                        ? signature.signer_reg_number
                        : step?.signer_reg_number || t("documents.message.step_open_to_anyone")}
                    </p>
                  </div>
                  {filled ? (
                    <span className="shrink-0 text-[11px] font-semibold px-2 py-0.5 rounded-full bg-blue-50 text-blue-700 border border-blue-200">
                      {signature!.signer_method === "EID" ? "E-ID" : signature!.signer_method}
                    </span>
                  ) : (
                    <span
                      className={`shrink-0 text-[11px] font-semibold px-2 py-0.5 rounded-full border ${
                        isNext
                          ? "bg-indigo-100 text-indigo-700 border-indigo-200"
                          : "bg-surface-2 text-muted border-line"
                      }`}
                    >
                      {isNext
                        ? t("documents.state.awaiting_now")
                        : open
                          ? t("documents.state.awaiting_later")
                          : t("documents.state.never_given")}
                    </span>
                  )}
                </div>

                {signature && (
                  <dl className="mt-2 space-y-1 text-[11px] pl-7">
                    <div className="flex gap-2">
                      <dt className="text-muted shrink-0">{t("documents.field.signed_at")}:</dt>
                      <dd className="text-foreground">{new Date(signature.signed_at).toLocaleString()}</dd>
                    </div>
                    {signature.certificate_serial ? (
                      <>
                        <div className="flex gap-2">
                          <dt className="text-muted shrink-0">{t("documents.field.certificate_serial")}:</dt>
                          <dd className="font-mono text-foreground break-all">{signature.certificate_serial}</dd>
                        </div>
                        {signature.certificate_issuer && (
                          <div className="flex gap-2">
                            <dt className="text-muted shrink-0">{t("documents.field.certificate_issuer")}:</dt>
                            <dd className="text-foreground break-all">{signature.certificate_issuer}</dd>
                          </div>
                        )}
                      </>
                    ) : (
                      <div className="flex gap-2">
                        <dt className="text-muted shrink-0">{t("documents.field.approval_reference")}:</dt>
                        <dd className="font-mono text-muted break-all">{signature.signature_hash}</dd>
                      </div>
                    )}
                  </dl>
                )}
              </li>
            );
          })}
        </ol>
      )}

      <button
        type="button"
        onClick={onClose}
        className="mt-5 w-full bg-surface-2 hover:bg-slate-200 text-foreground font-medium py-2 rounded-lg text-xs"
      >
        {t("base.action.close")}
      </button>
    </Modal>
  );
}

/** Opens the signature history, shown only once there is something to show. */
export function SignatureHistoryButton({ doc, onOpen }: { doc: DocumentRecord; onOpen: (doc: DocumentRecord) => void }) {
  const { t } = useI18n();
  // Worth opening as soon as there is anything to see: a signature already given, or
  // a step still to be filled. Testing required_signatures <= 1 used to stand in for
  // "no chain", which stopped being true — a document whose own chain is exactly one
  // step pins 1 as well, and hid its trail.
  if (doc.signature_count === 0 && doc.outstanding_steps === 0) return null;

  return (
    <button
      onClick={() => onOpen(doc)}
      title={t("documents.action.view_history")}
      aria-label={t("documents.action.view_history")}
      className="inline-flex items-center space-x-1 px-2 py-1 rounded-lg text-[11px] font-semibold border border-input text-muted hover:bg-surface-hover transition"
    >
      <History className="w-3.5 h-3.5" />
      <span>
        {doc.signature_count}/{doc.required_signatures}
      </span>
    </button>
  );
}
