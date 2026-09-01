"use client";

import React, { useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { contracts, InboxDetail } from "@/lib/contracts";
import { useResource, useLoadOnMount } from "@/lib/useResource";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { Banner, LoadingBlock, Modal, cardClass, fieldClass } from "@/components/ui";
import { CeremonyButton, PartyBadge, fmtWhen } from "@/components/documents/contracts";
import { ArrowLeft, FileText } from "lucide-react";

/**
 * One incoming contract, as the recipient sees it: the frozen text, who else
 * is on the contract (names and states only — no contact details, that is the
 * issuer's view), the signatories this organisation has named, and the two
 * possible answers — a PIN2 signature or a reasoned refusal.
 */
export default function ContractInboxDetailPage() {
  const { pid } = useParams<{ pid: string }>();
  const { t } = useI18n();
  const { can } = useAccess();
  const mayNominate = can("documents.parties");
  const maySign = can("documents.sign");

  const detail = useResource<InboxDetail | null>(() => contracts.inboxShow(pid), { initial: null });
  useLoadOnMount(detail.reload);

  const [message, setMessage] = useState<{ tone: "error" | "success"; text: string } | null>(null);
  const fail = (err: unknown) => setMessage({ tone: "error", text: err instanceof Error ? err.message : String(err) });
  const [declining, setDeclining] = useState(false);

  if (detail.loading) return <LoadingBlock />;
  if (detail.failed || !detail.data) return <Banner tone="error" message={t("contracts.msg.load_failed")} />;
  const item = detail.data;
  const open = item.state === "invited" || item.state === "viewed";
  const named = item.my_signatories.length > 0;

  return (
    <div className="space-y-6">
      <Link href="/module/documents/inbox" className="inline-flex items-center gap-1 text-sm text-indigo-700 hover:underline">
        <ArrowLeft className="w-4 h-4" />
        {t("contracts.view.inbox_back")}
      </Link>

      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-semibold text-foreground">{item.title}</h1>
        <PartyBadge state={item.state} />
      </div>

      {item.parties.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {item.parties.map((party, index) => (
            <span key={index} className="inline-flex items-center gap-2 bg-surface border border-line rounded-full px-3 py-1 text-xs">
              <span className={`w-2 h-2 rounded-full ${
                party.state === "signed" ? "bg-emerald-500" :
                party.state === "declined" ? "bg-red-500" :
                party.state === "invited" || party.state === "viewed" ? "bg-amber-400" : "bg-slate-300"}`} />
              <span className="font-medium text-foreground">
                {party.display_name}{party.mine ? ` ${t("contracts.msg.you")}` : ""}
              </span>
              <PartyBadge state={party.state} />
            </span>
          ))}
        </div>
      )}

      {message && <Banner tone={message.tone} message={message.text} />}

      {item.has_copy ? (
        <section className={`${cardClass} p-5 space-y-3`}>
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-semibold text-foreground">{t("contracts.section.body")}</h2>
            <a href={contracts.inboxCopyUrl(pid)} target="_blank" rel="noopener noreferrer"
              className="text-xs font-semibold text-foreground bg-surface-2 hover:bg-slate-200 px-3 py-1.5 rounded-lg inline-flex items-center gap-1.5">
              <FileText className="w-3.5 h-3.5" />
              {t("contracts.action.view_pdf")}
            </a>
          </div>
          <div className="whitespace-pre-wrap text-sm leading-relaxed text-foreground bg-surface-2 border border-line rounded-lg p-4 max-h-[28rem] overflow-auto">
            {item.body_text}
          </div>
          <p className="text-[11px] text-muted font-mono">{t("contracts.msg.sha", { sha: item.sha256 || "—" })}</p>
        </section>
      ) : (
        <Banner tone="warning" message={t("contracts.msg.not_delivered")} />
      )}

      <section className={`${cardClass} p-5 space-y-3`}>
        <h2 className="text-sm font-semibold text-foreground">{t("contracts.section.signatory")}</h2>
        {named ? (
          item.my_signatories.map((signatory) => (
            <p key={signatory.id} className="text-sm text-foreground">
              {signatory.full_name}
              {signatory.position ? ` (${signatory.position})` : ""}
              {signatory.reg_number ? ` · ${signatory.reg_number}` : ""}
              {signatory.signed_at ? ` ✓ ${fmtWhen(signatory.signed_at)}` : ""}
            </p>
          ))
        ) : (
          <p className="text-sm text-muted">{t("contracts.msg.no_signatory")}</p>
        )}
        {mayNominate && open && <NominateForm pid={pid} onAdded={detail.reload} onError={fail} />}
      </section>

      {open && item.has_copy && maySign && (
        <section className={`${cardClass} p-5 space-y-3`}>
          <h2 className="text-sm font-semibold text-foreground">{t("contracts.section.decision")}</h2>
          <div className="flex flex-wrap gap-2">
            <CeremonyButton
              label={t("contracts.action.sign")}
              start={() => contracts.inboxSignStart(pid)}
              poll={() => contracts.inboxSignPoll(pid)}
              onDone={async () => {
                setMessage({ tone: "success", text: t("contracts.msg.signed") });
                await detail.reload();
              }}
              onError={(value) => setMessage({ tone: "error", text: value })}
              className="bg-amber-400 hover:bg-amber-500 text-foreground text-sm font-semibold px-4 py-2 rounded-lg inline-flex items-center gap-1.5"
            />
            <button onClick={() => setDeclining(true)}
              className="bg-red-50 hover:bg-red-100 text-red-700 text-sm font-semibold px-4 py-2 rounded-lg">
              {t("contracts.action.decline")}
            </button>
          </div>
        </section>
      )}

      {item.state === "signed" && (
        <section className={`${cardClass} p-5 space-y-3`}>
          <Banner tone="success" message={t("contracts.msg.signed_done")} />
          <a href={contracts.inboxSignedUrl(pid)} target="_blank" rel="noopener noreferrer"
            className="text-xs font-semibold text-emerald-700 bg-surface border border-emerald-200 hover:bg-emerald-50 px-3 py-1.5 rounded-lg inline-block">
            {t("contracts.action.signed_pdf")}
          </a>
        </section>
      )}
      {item.state === "declined" && <Banner tone="error" message={t("contracts.msg.declined_done")} />}

      {declining && (
        <DeclineModal
          pid={pid}
          onClose={() => setDeclining(false)}
          onDeclined={async () => { setDeclining(false); await detail.reload(); }}
          onError={fail}
        />
      )}
    </div>
  );
}

function NominateForm({ pid, onAdded, onError }: {
  pid: string;
  onAdded: () => Promise<void>;
  onError: (err: unknown) => void;
}) {
  const { t } = useI18n();
  const [form, setForm] = useState({ full_name: "", position: "", reg_number: "" });
  const [busy, setBusy] = useState(false);
  const set = (key: keyof typeof form) => (event: React.ChangeEvent<HTMLInputElement>) =>
    setForm((current) => ({ ...current, [key]: event.target.value }));

  const nominate = async () => {
    setBusy(true);
    try {
      await contracts.inboxNominate(pid, {
        full_name: form.full_name.trim(), position: form.position.trim(), reg_number: form.reg_number.trim(),
      });
      setForm({ full_name: "", position: "", reg_number: "" });
      await onAdded();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="border-t border-line pt-3 space-y-3">
      <p className="text-xs text-muted">{t("contracts.msg.nominate_hint")}</p>
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <input className={fieldClass} placeholder={t("contracts.field.full_name")} value={form.full_name} onChange={set("full_name")} />
        <input className={fieldClass} placeholder={t("contracts.field.reg")} value={form.reg_number} onChange={set("reg_number")} />
        <input className={fieldClass} placeholder={t("contracts.field.position")} value={form.position} onChange={set("position")} />
      </div>
      <button onClick={() => void nominate()} disabled={busy || !form.full_name.trim() || !form.reg_number.trim()}
        className="bg-slate-800 hover:bg-slate-900 disabled:opacity-50 text-white text-xs font-semibold px-4 py-2 rounded-lg">
        {t("contracts.action.nominate")}
      </button>
    </div>
  );
}

function DeclineModal({ pid, onClose, onDeclined, onError }: {
  pid: string;
  onClose: () => void;
  onDeclined: () => Promise<void>;
  onError: (err: unknown) => void;
}) {
  const { t } = useI18n();
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const decline = async () => {
    setBusy(true);
    try {
      await contracts.inboxDecline(pid, reason.trim());
      await onDeclined();
    } catch (err) {
      onError(err);
      setBusy(false);
    }
  };
  return (
    <Modal onClose={onClose} label={t("contracts.action.decline")}>
      <div className="space-y-4">
        <h2 className="text-lg font-semibold text-foreground">{t("contracts.action.decline")}</h2>
        <p className="text-xs text-muted">{t("contracts.msg.decline_hint")}</p>
        <textarea autoFocus className={`${fieldClass} min-h-[100px]`} value={reason} onChange={(event) => setReason(event.target.value)} />
        <div className="flex justify-end gap-2">
          <button onClick={onClose} className="text-sm font-medium text-muted px-4 py-2 rounded-lg hover:bg-surface-hover">
            {t("contracts.action.cancel")}
          </button>
          <button onClick={() => void decline()} disabled={busy || !reason.trim()}
            className="bg-red-600 hover:bg-red-700 disabled:opacity-50 text-white text-sm font-semibold px-4 py-2 rounded-lg">
            {t("contracts.action.decline")}
          </button>
        </div>
      </div>
    </Modal>
  );
}
