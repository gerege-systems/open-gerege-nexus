"use client";

import React, { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import {
  Alert,
  Badge,
  Button,
  Card,
  CardContent,
  ConfirmationDialog,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  Skeleton,
  Spinner,
  type BadgeProps,
} from "@gerege-systems/ui";
import {
  Ban,
  CheckCircle,
  Clock,
  FileText,
  History,
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

  if (status === "APPROVED") {
    return <Badge tone="success" icon={<CheckCircle />}>{t("documents.state.approved")}</Badge>;
  }
  if (status === "REJECTED") {
    return <Badge tone="danger" icon={<XCircle />}>{t("documents.state.rejected")}</Badge>;
  }
  if (status === PENDING) {
    return <Badge tone="warning" icon={<Clock />}>{t("documents.state.pending")}</Badge>;
  }
  if (status === "DRAFT") {
    return <Badge tone="neutral" icon={<FileText />}>{t("documents.state.draft")}</Badge>;
  }
  return <Badge tone="neutral">{status}</Badge>;
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
    <Badge
      tone={complete ? "success" : "accent"}
      icon={<Users />}
      title={t("documents.message.signature_progress", {
        applied: doc.signature_count,
        required: doc.required_signatures,
      })}
    >
      {doc.signature_count}/{doc.required_signatures}
    </Badge>
  );
}

/** Who signed, plus the reg number and hash the identity provider returned. */
export function SignatureCell({ doc }: { doc: DocumentRecord }) {
  const { t } = useI18n();

  if (doc.signed_by) {
    return (
      <div className="space-y-1">
        <Badge tone="info" icon={<ShieldCheck />}>{doc.signed_by}</Badge>
        {doc.signature_hash && (
          <div className="font-mono text-xs text-muted truncate max-w-[220px]" title={doc.signature_hash}>
            {doc.signer_reg_number ? `${doc.signer_reg_number} · ` : ""}
            {doc.signature_hash}
          </div>
        )}
      </div>
    );
  }
  if (doc.status === "REJECTED") {
    return <span className="text-danger text-xs">{t("documents.message.rejected_not_signed")}</span>;
  }
  return <span className="text-muted text-xs">{t("documents.state.pending_signature")}</span>;
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

  // Confirmed in RowActions, the only place that offers the button.
  const reject = async (doc: DocumentRecord) => {
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
  const [rejecting, setRejecting] = useState(false);

  // A draft is not awaiting anybody: it has to be sent for approval first, and
  // that is a routing decision rather than a signing one.
  if (doc.status === "DRAFT") {
    if (!canManage || !onRoute) return <span className="text-subtle text-xs">—</span>;
    return (
      <Button variant="outline" size="sm" onClick={() => onRoute(doc)} disabled={busy} leadingIcon={<Send />}>
        {t("documents.action.route")}
      </Button>
    );
  }

  if (doc.status !== PENDING) return <span className="text-subtle text-xs">—</span>;
  if (!canSign) return <span className="text-subtle text-xs">—</span>;

  return (
    <div className="flex items-center justify-end gap-2">
      <Button variant="outline" size="sm" onClick={() => onSign(doc)} disabled={busy} leadingIcon={<PenLine />} className="text-accent">
        {t("documents.action.sign")}
      </Button>
      <Button variant="outline" size="sm" onClick={() => setRejecting(true)} disabled={busy} leadingIcon={<Ban />} className="text-danger">
        {t("documents.action.reject")}
      </Button>
      {rejecting && (
        <ConfirmationDialog
          open
          onOpenChange={(open) => { if (!open) setRejecting(false); }}
          title={doc.title}
          description={t("documents.message.reject_confirm", { title: doc.title })}
          confirmLabel={t("documents.action.reject")}
          cancelLabel={t("base.action.cancel")}
          confirmVariant="destructive"
          onConfirm={() => {
            setRejecting(false);
            onReject(doc);
          }}
        />
      )}
    </div>
  );
}

/** A number a screen states about itself: the queue's size, what is past its term. */
export function StatTile({ value, label, tone = "foreground", mono }: {
  value: React.ReactNode;
  label: string;
  tone?: "foreground" | "accent" | "warning";
  mono?: boolean;
}) {
  const colour = tone === "accent" ? "text-accent" : tone === "warning" ? "text-warning" : "text-foreground";
  return (
    <Card padding="sm">
      <CardContent>
        <div className={`text-2xl font-semibold ${colour}`}>{value}</div>
        <div className={`text-xs text-muted leading-snug mt-1 ${mono ? "font-mono" : ""}`}>{label}</div>
      </CardContent>
    </Card>
  );
}

/**
 * What a listing shows while its first load is outstanding: rows of the right
 * shape, so the page does not jump when the real ones land. The library's
 * Skeleton holds itself back for 300ms and, once drawn, stays for 500, so a
 * fast reply renders straight to content. The label is still announced,
 * because a skeleton says nothing to a screen reader.
 */
export function ListSkeleton({ label, rows = 4 }: { label?: string; rows?: number }) {
  const { t } = useI18n();
  return (
    <div className="py-4" role="status" aria-live="polite" aria-busy="true">
      <span className="sr-only">{label || t("base.message.loading")}</span>
      <div className="space-y-3">
        {Array.from({ length: rows }, (_, row) => (
          <div key={row} className="flex items-center gap-3">
            <Skeleton variant="circle" className="size-4 shrink-0" />
            <Skeleton className="h-4 flex-1" />
            <Skeleton className="h-4 w-24 shrink-0" />
          </div>
        ))}
      </div>
    </div>
  );
}

/**
 * The news that the rows on screen are old, with the way to try again. Shown
 * under a table for as long as a refresh keeps failing — the banner can be
 * dismissed, and a refresh that failed after an action must not be the only
 * thing that says the rows are stale.
 */
export function StaleNotice({ busy, onRetry, inset }: { busy: boolean; onRetry: () => void; inset?: boolean }) {
  const { t } = useI18n();
  return (
    <div className={inset ? "border-t border-line" : ""}>
      <Alert variant="warning" className={inset ? "rounded-none border-0" : undefined}>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <span>{t("documents.message.stale_rows")}</span>
          <Button variant="outline" size="sm" disabled={busy} onClick={onRetry}>
            {t("documents.action.retry")}
          </Button>
        </div>
      </Alert>
    </div>
  );
}

/** A partial list says so, and can be read further. */
export function LoadMoreFooter({ text, busy, onMore }: { text: string; busy: boolean; onMore: () => void }) {
  const { t } = useI18n();
  return (
    <div className="flex items-center justify-between gap-3 px-4 py-3 border-t border-line bg-surface-2">
      <p className="text-xs text-muted">{text}</p>
      <Button variant="outline" size="sm" disabled={busy} onClick={onMore}>
        {t("documents.action.load_more")}
      </Button>
    </div>
  );
}

/**
 * A labelled single-choice field over the design system's Select. Radix cannot
 * carry an empty string as a value, so a "not chosen" option travels as
 * `SELECT_NONE` and comes back to the caller as "".
 */
export const SELECT_NONE = "__none__";

export function SelectField({
  id,
  label,
  hideLabel,
  value,
  onValueChange,
  options,
  disabled,
  helperText,
  size,
  className,
  placeholder,
}: {
  id?: string;
  label: string;
  hideLabel?: boolean;
  value: string;
  onValueChange: (value: string) => void;
  options: { value: string; label: string }[];
  disabled?: boolean;
  helperText?: string;
  size?: "sm" | "md" | "lg";
  className?: string;
  placeholder?: string;
}) {
  const autoId = React.useId();
  const fieldId = id ?? autoId;
  return (
    <div className={`flex flex-col gap-1.5 ${className ?? ""}`}>
      <label htmlFor={fieldId} className={`text-sm font-medium text-foreground ${hideLabel ? "sr-only" : ""}`}>
        {label}
      </label>
      <Select
        value={value === "" ? SELECT_NONE : value}
        onValueChange={(next) => onValueChange(next === SELECT_NONE ? "" : next)}
        disabled={disabled}
      >
        <SelectTrigger id={fieldId} size={size} placeholder={placeholder} />
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={option.value || SELECT_NONE} value={option.value === "" ? SELECT_NONE : option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {helperText && <p className="text-xs text-subtle">{helperText}</p>}
    </div>
  );
}

/** A listing that has nothing to show. */
export function ListEmpty({ icon, title, description }: { icon: React.ReactNode; title: string; description?: string }) {
  return <EmptyState icon={icon} title={title} description={description} />;
}

/**
 * The props that make a library DialogContent behave as these dialogs always
 * have: no Escape, no backdrop dismissal — a stray click must not end a signing
 * conversation or throw away a half-filled form. The dialog's own buttons close it.
 */
export const pinnedDialogProps = {
  showClose: false,
  onEscapeKeyDown: (event: Event) => event.preventDefault(),
  onPointerDownOutside: (event: Event) => event.preventDefault(),
  onInteractOutside: (event: Event) => event.preventDefault(),
} as const;

export type Tone = NonNullable<BadgeProps["tone"]>;

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
    <Dialog open>
      <DialogContent {...pinnedDialogProps}>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <PenLine className="size-5 text-accent" aria-hidden />
            {t("documents.view.sign_title")}
          </DialogTitle>
          <DialogDescription className="truncate">{doc.title}</DialogDescription>
        </DialogHeader>

        {failure && (
          <div className="mb-4">
            <Alert variant="danger" live>{failure}</Alert>
          </div>
        )}

        {session ? (
          <div className="space-y-4">
            <div className="rounded-lg border border-accent bg-accent-soft p-4 text-center">
              <p className="text-xs font-semibold text-accent uppercase tracking-wide">
                {t("documents.field.verification_code")}
              </p>
              <p className="text-3xl font-semibold tracking-[0.2em] text-accent mt-1">
                {session.verification_code}
              </p>
              <p className="text-xs text-accent mt-2">{t("documents.message.verification_code_hint")}</p>
            </div>

            <div className="flex items-center gap-2 text-sm text-muted" role="status">
              <Spinner size="md" decorative className="shrink-0" />
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

            <p className="text-xs text-muted">
              {t("documents.message.approval_display_text")}: <span className="italic">{session.display_text}</span>
            </p>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={onClose}>
                {t("base.action.cancel")}
              </Button>
            </DialogFooter>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <span className="block text-sm font-medium text-foreground mb-1.5">
                {t("documents.field.signature_method")}
              </span>
              <div className="grid grid-cols-2 gap-2">
                {(["EID", "DAN"] as SignMethod[]).map((m) => (
                  <Button
                    key={m}
                    type="button"
                    size="sm"
                    variant={method === m ? "primary" : "outline"}
                    aria-pressed={method === m}
                    onClick={() => setMethod(m)}
                  >
                    {m === "EID" ? "E-ID (eidmongolia.mn)" : "DAN (dan.gerege.mn)"}
                  </Button>
                ))}
              </div>
              <p className="text-xs text-muted mt-1.5">
                {method === "EID"
                  ? t("documents.message.eid_method_hint")
                  : t("documents.message.dan_method_hint")}
              </p>
            </div>

            {/* The same bounds the server holds (RegNumberLimit..RegNumberMax), so a
                number that could never be a registration number is caught in the
                field rather than as a 400 after the operator has committed to it. */}
            <Input
              type="text"
              label={`${t("documents.field.reg_number")} *`}
              placeholder="УБ99010111"
              value={regNumber}
              onChange={(e) => setRegNumber(e.target.value)}
              className="[&_input]:font-mono"
              minLength={8}
              maxLength={64}
              required
            />

            {method === "DAN" && (
              <Input
                type="text"
                label={t("documents.field.otp_code")}
                placeholder="123456"
                value={otpCode}
                onChange={(e) => setOtpCode(e.target.value)}
                className="[&_input]:font-mono"
              />
            )}

            <DialogFooter className="pt-2">
              <Button type="button" variant="outline" onClick={onClose}>
                {t("base.action.cancel")}
              </Button>
              <Button type="submit" loading={busy}>
                {busy
                  ? t("documents.message.signing")
                  : method === "EID"
                    ? t("documents.action.request_approval")
                    : t("documents.action.sign")}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
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
    <Dialog open>
      <DialogContent {...pinnedDialogProps} size="lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <ShieldCheck className="size-5 text-accent" aria-hidden />
            {t("documents.view.history_title")}
          </DialogTitle>
          {/* The trail covers the list, so the status has to travel with it: the same
              unfilled steps mean "still to come" on a pending document and "never
              given" on one that has been decided, and the dialog says nothing else
              about which this is. */}
          <div className="flex items-center gap-2 min-w-0">
            <DialogDescription className="truncate">{doc.title}</DialogDescription>
            <StatusBadge status={doc.status} />
          </div>
        </DialogHeader>

        {error ? (
          <Alert variant="danger" live>{error}</Alert>
        ) : signatures === null ? (
          <div className="flex items-center gap-2 text-muted text-sm py-6" role="status">
            <Spinner size="md" decorative />
            {t("documents.message.loading")}
          </div>
        ) : rows.length === 0 ? (
          <EmptyState icon={<History />} title={t("documents.message.no_signatures")} />
        ) : (
          <ol className="space-y-3 max-h-[60dvh] overflow-y-auto">
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
                        ? "border-accent bg-accent-soft"
                        : "border-dashed border-line"
                  }`}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span
                          className={`w-5 h-5 shrink-0 rounded-full text-xs font-semibold grid place-items-center ${
                            filled ? "bg-success-soft text-success" : "bg-surface-2 text-muted"
                          }`}
                        >
                          {step ? step.order : index + 1}
                        </span>
                        <p className="text-sm font-semibold text-foreground truncate">
                          {signature ? signature.signer_name : step?.name || "—"}
                        </p>
                        {!step && signature && steps.length > 0 && (
                          <span className="shrink-0 text-xs text-muted italic">
                            {t("documents.message.signature_outside_chain")}
                          </span>
                        )}
                      </div>
                      <p className="font-mono text-xs text-muted mt-0.5 ps-7">
                        {signature
                          ? signature.signer_reg_number
                          : step?.signer_reg_number || t("documents.message.step_open_to_anyone")}
                      </p>
                    </div>
                    {filled && signature ? (
                      <Badge tone="info" className="shrink-0">
                        {signature.signer_method === "EID" ? "E-ID" : signature.signer_method}
                      </Badge>
                    ) : (
                      <Badge tone={isNext ? "accent" : "neutral"} className="shrink-0">
                        {isNext
                          ? t("documents.state.awaiting_now")
                          : open
                            ? t("documents.state.awaiting_later")
                            : t("documents.state.never_given")}
                      </Badge>
                    )}
                  </div>

                  {signature && (
                    <dl className="mt-2 space-y-1 text-xs ps-7">
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

        <DialogFooter className="mt-5">
          <Button type="button" variant="outline" onClick={onClose}>
            {t("base.action.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
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
    <Button
      variant="outline"
      size="sm"
      onClick={() => onOpen(doc)}
      title={t("documents.action.view_history")}
      aria-label={t("documents.action.view_history")}
      leadingIcon={<History />}
    >
      {doc.signature_count}/{doc.required_signatures}
    </Button>
  );
}
