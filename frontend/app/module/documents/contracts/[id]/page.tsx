"use client";

import React, { useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useRouter } from "next/navigation";
import {
  contracts, CeremonySession, ContractRow, ContractShape, Invitation, Party,
} from "@/lib/contracts";
import { useResource, useLoadOnMount } from "@/lib/useResource";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { Banner, LoadingBlock, Modal, cardClass, fieldClass, selectClass } from "@/components/ui";
import {
  CeremonyButton, ContractBadge, PartyBadge, fmtWhen, useContractLabels,
} from "@/components/documents/contracts";
import { ArrowLeft, Copy, FileUp, Link2, Plus, Send, Undo2, Upload } from "lucide-react";

/**
 * One contract, everything the issuer does to it: the facts (number, amount,
 * term), the text, the parties with their signatories, and the acts — send,
 * sign for a party, create an invitation link, withdraw, reopen.
 *
 * One load feeds the whole page: the parties answer carries the contract's
 * facts, so the screen cannot show a number the register disagrees with.
 */
export default function ContractPage() {
  const { id } = useParams<{ id: string }>();
  const { t } = useI18n();
  const { can } = useAccess();
  const mayManage = can("documents.manage");
  const mayParties = can("documents.parties");
  const maySend = can("documents.send");
  const maySign = can("documents.sign");

  const shape = useResource<ContractShape | null>(() => contracts.parties(id), { initial: null });
  const text = useResource<string>(async () => (await contracts.body(id)).body ?? "", { initial: "" });
  useLoadOnMount(shape.reload);
  useLoadOnMount(text.reload);

  const [message, setMessage] = useState<{ tone: "error" | "success"; text: string } | null>(null);
  const say = (tone: "error" | "success", value: string) => setMessage({ tone, text: value });
  const fail = (err: unknown) => say("error", err instanceof Error ? err.message : String(err));

  if (shape.loading) return <LoadingBlock />;
  if (shape.failed || !shape.data) return <Banner tone="error" message={t("contracts.msg.load_failed")} />;

  const contract = shape.data;
  const state = contract.contract_state;
  const editable = state === "DRAFT" || state === "NONE";
  const reload = async () => { await shape.reload(); };

  return (
    <div className="space-y-6">
      <Link href="/module/documents/contracts" className="inline-flex items-center gap-1 text-sm text-indigo-700 hover:underline">
        <ArrowLeft className="w-4 h-4" />
        {t("contracts.view.back")}
      </Link>

      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-semibold text-foreground">{contract.title}</h1>
        <ContractBadge state={state} />
      </div>

      {/* Талуудын явц — гэрээ хаана байгааг нэг харцаар. */}
      {contract.parties.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {contract.parties.map((party) => (
            <span key={party.id} className="inline-flex items-center gap-2 bg-surface border border-line rounded-full px-3 py-1 text-xs">
              <PartyDot state={party.state} />
              <span className="font-medium text-foreground">{party.display_name}</span>
              <PartyBadge state={party.state} />
            </span>
          ))}
        </div>
      )}

      {message && <Banner tone={message.tone} message={message.text} />}

      <FactsCard id={id} contract={contract} editable={mayManage && editable} onSaved={reload} onError={fail} />
      <MasterPdfCard
        id={id}
        contract={contract}
        mayManage={mayManage}
        maySign={maySign}
        onChanged={reload}
        onError={fail}
        onInfo={(value) => say("success", value)}
      />
      <BodyCard
        id={id}
        text={text.data}
        setText={text.setData}
        editable={mayManage}
        frozen={!editable}
        onSaved={() => say("success", t("contracts.msg.saved"))}
        onError={fail}
      />
      <PartiesCard
        id={id}
        contract={contract}
        mayParties={mayParties}
        maySend={maySend}
        maySign={maySign}
        onChanged={reload}
        onError={fail}
        onInfo={(value) => say("success", value)}
      />
      {maySend && (
        <IssueCard id={id} onChanged={reload} onError={fail} onInfo={(value) => say("success", value)} />
      )}
      {maySend && contract.parties.some((party) => party.party_role !== "issuer") && (
        <SendCard id={id} state={state} mode={contract.mode} onChanged={reload} onError={fail} onInfo={(value) => say("success", value)} />
      )}
    </div>
  );
}

function PartyDot({ state }: { state: Party["state"] }) {
  const color =
    state === "signed" ? "bg-emerald-500" :
    state === "declined" ? "bg-red-500" :
    state === "invited" || state === "viewed" ? "bg-amber-400" : "bg-slate-300";
  return <span className={`w-2 h-2 rounded-full ${color}`} />;
}

// ─────────────────────────────────────────────────────────── гэрээний мэдээлэл

function FactsCard({
  id, contract, editable, onSaved, onError,
}: {
  id: string;
  contract: ContractShape;
  editable: boolean;
  onSaved: () => Promise<void>;
  onError: (err: unknown) => void;
}) {
  const { t } = useI18n();
  const day = (value?: string) => (value ? value.slice(0, 10) : "");
  const [facts, setFacts] = useState({
    contract_number: contract.contract_number || "",
    amount: contract.amount != null ? String(contract.amount) : "",
    currency: contract.currency || "MNT",
    effective_from: day(contract.effective_from),
    effective_to: day(contract.effective_to),
    due_at: day(contract.due_at),
  });
  const [busy, setBusy] = useState(false);
  const set = (key: keyof typeof facts) => (event: React.ChangeEvent<HTMLInputElement>) =>
    setFacts((current) => ({ ...current, [key]: event.target.value }));

  const save = async () => {
    setBusy(true);
    try {
      await contracts.saveFacts(id, {
        contract_number: facts.contract_number.trim(),
        amount: facts.amount === "" ? null : Number(facts.amount),
        currency: facts.currency.trim().toUpperCase(),
        effective_from: facts.effective_from,
        effective_to: facts.effective_to,
        due_at: facts.due_at ? `${facts.due_at}T23:59:59Z` : "",
      });
      await onSaved();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  const field = (label: string, node: React.ReactNode) => (
    <div>
      <label className="block text-xs font-medium text-muted mb-1">{label}</label>
      {node}
    </div>
  );

  return (
    <section className={`${cardClass} p-5 space-y-4`}>
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-foreground">{t("contracts.section.facts")}</h2>
        {editable && (
          <button onClick={() => void save()} disabled={busy}
            className="text-xs font-semibold text-indigo-700 bg-indigo-50 hover:bg-indigo-100 px-3 py-1.5 rounded-lg disabled:opacity-50">
            {t("contracts.action.save")}
          </button>
        )}
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {field(t("contracts.field.number"), <input className={fieldClass} value={facts.contract_number} onChange={set("contract_number")} disabled={!editable} />)}
        {field(t("contracts.field.due"), <input type="date" className={fieldClass} value={facts.due_at} onChange={set("due_at")} disabled={!editable} />)}
        {field(t("contracts.field.amount"), <input type="number" step="0.01" className={fieldClass} value={facts.amount} onChange={set("amount")} disabled={!editable} />)}
        {field(t("contracts.field.currency"), <input maxLength={3} className={fieldClass} value={facts.currency} onChange={set("currency")} disabled={!editable} />)}
        {field(t("contracts.field.effective_from"), <input type="date" className={fieldClass} value={facts.effective_from} onChange={set("effective_from")} disabled={!editable} />)}
        {field(t("contracts.field.effective_to"), <input type="date" className={fieldClass} value={facts.effective_to} onChange={set("effective_to")} disabled={!editable} />)}
      </div>
    </section>
  );
}

// ─────────────────────────────────────────────────────────────────── бичвэр

function BodyCard({
  id, text, setText, editable, frozen, onSaved, onError,
}: {
  id: string;
  text: string;
  setText: (value: string) => void;
  editable: boolean;
  frozen: boolean;
  onSaved: () => void;
  onError: (err: unknown) => void;
}) {
  const { t } = useI18n();
  const [busy, setBusy] = useState(false);
  const save = async () => {
    setBusy(true);
    try {
      await contracts.saveBody(id, text);
      onSaved();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };
  return (
    <section className={`${cardClass} p-5 space-y-3`}>
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-foreground">{t("contracts.section.body")}</h2>
        {editable && !frozen && (
          <button onClick={() => void save()} disabled={busy}
            className="text-xs font-semibold text-indigo-700 bg-indigo-50 hover:bg-indigo-100 px-3 py-1.5 rounded-lg disabled:opacity-50">
            {t("contracts.action.save")}
          </button>
        )}
      </div>
      {frozen && <Banner tone="info" message={t("contracts.body.frozen_note")} />}
      <textarea
        className={`${fieldClass} min-h-[260px] font-mono text-[13px] leading-relaxed`}
        placeholder={t("contracts.body.placeholder")}
        value={text}
        onChange={(event) => setText(event.target.value)}
        readOnly={frozen || !editable}
      />
      <p className="text-[11px] text-muted">{t("contracts.body.hint")}</p>
      <details className="text-[11px] text-muted">
        <summary className="cursor-pointer hover:text-foreground">…</summary>
        {t("contracts.body.advanced", { tokens: "{{тал}} {{регистр}} {{төлөөлөгч}} {{хаяг}} {{гэрээ}} {{дугаар}} {{огноо}}" })}
      </details>
    </section>
  );
}

// ─────────────────────────────────────────────────────────────────── талууд

function PartiesCard({
  id, contract, mayParties, maySend, maySign, onChanged, onError, onInfo,
}: {
  id: string;
  contract: ContractShape;
  mayParties: boolean;
  maySend: boolean;
  maySign: boolean;
  onChanged: () => Promise<void>;
  onError: (err: unknown) => void;
  onInfo: (value: string) => void;
}) {
  const { t } = useI18n();
  const [invite, setInvite] = useState<{ party: Party; invitation: Invitation } | null>(null);
  const [signatoryFor, setSignatoryFor] = useState<Party | null>(null);
  const draft = contract.contract_state === "DRAFT" || contract.contract_state === "NONE";

  return (
    <section className={`${cardClass} p-5 space-y-4`}>
      <h2 className="text-sm font-semibold text-foreground">{t("contracts.section.parties")}</h2>
      <p className="text-xs text-muted">{t("contracts.parties.note")}</p>
      {contract.parties.length === 0 && <p className="text-sm text-muted">{t("contracts.msg.no_parties")}</p>}

      {contract.parties.map((party) => (
        <PartyRow
          key={party.id}
          id={id}
          party={party}
          draft={draft}
          mayParties={mayParties}
          maySend={maySend}
          maySign={maySign}
          onChanged={onChanged}
          onError={onError}
          onAddSignatory={() => setSignatoryFor(party)}
          onInvite={async () => {
            try {
              setInvite({ party, invitation: await contracts.invite(id, party.id) });
            } catch (err) {
              onError(err);
            }
          }}
          onSigned={() => {
            onInfo(t("contracts.msg.signed"));
            void onChanged();
          }}
        />
      ))}

      {mayParties && <AddPartyForm id={id} onAdded={onChanged} onError={onError} />}

      {signatoryFor && (
        <SignatoryModal
          id={id}
          party={signatoryFor}
          onClose={() => setSignatoryFor(null)}
          onAdded={async () => { setSignatoryFor(null); await onChanged(); }}
          onError={onError}
        />
      )}
      {invite && (
        <InviteModal party={invite.party} invitation={invite.invitation} onClose={() => setInvite(null)} />
      )}
    </section>
  );
}

function PartyRow({
  id, party, draft, mayParties, maySend, maySign, onChanged, onError, onAddSignatory, onInvite, onSigned,
}: {
  id: string;
  party: Party;
  draft: boolean;
  mayParties: boolean;
  maySend: boolean;
  maySign: boolean;
  onChanged: () => Promise<void>;
  onError: (err: unknown) => void;
  onAddSignatory: () => void;
  onInvite: () => Promise<void>;
  onSigned: () => void;
}) {
  const { t } = useI18n();
  const { partyRole, partyKind } = useContractLabels();
  const open = party.state === "invited" || party.state === "viewed";
  const contact = [party.registration_number, party.contact_email, party.contact_phone].filter(Boolean).join(" · ");

  return (
    <div className="border border-line rounded-xl p-4 bg-slate-50/60 space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-semibold text-foreground">{party.display_name}</span>
        <span className="text-[11px] bg-slate-200/70 text-muted rounded-full px-2 py-0.5">{partyRole(party.party_role)}</span>
        <span className="text-[11px] bg-slate-200/70 text-muted rounded-full px-2 py-0.5">{partyKind(party.party_kind)}</span>
        {party.sign_order != null && (
          <span className="text-[11px] bg-slate-200/70 text-muted rounded-full px-2 py-0.5">
            {t("contracts.msg.signs_at", { n: party.sign_order })}
          </span>
        )}
        <span className="ml-auto"><PartyBadge state={party.state} /></span>
      </div>
      {contact && <div className="text-xs text-muted">{contact}</div>}
      {party.decline_reason && (
        <Banner tone="error" message={t("contracts.msg.decline_reason_of", { reason: party.decline_reason })} />
      )}
      {(party.signatories?.length ?? 0) > 0 && (
        <div className="text-xs text-muted">
          {party.signatories!.map((signatory) => (
            <span key={signatory.id} className="mr-3">
              {signatory.full_name}
              {signatory.position ? ` (${signatory.position})` : ""}
              {signatory.reg_number ? ` · ${signatory.reg_number}` : ""}
              {signatory.signed_at ? ` ✓ ${fmtWhen(signatory.signed_at)}` : ""}
            </span>
          ))}
        </div>
      )}
      <div className="flex flex-wrap gap-2 pt-1">
        {mayParties && party.party_role !== "issuer" && !party.counterparty_tenant_id && (
          <button onClick={onAddSignatory} className="text-xs font-semibold text-foreground bg-surface border border-line hover:bg-surface-hover px-3 py-1.5 rounded-lg">
            {t("contracts.action.add_signatory")}
          </button>
        )}
        {party.has_copy && (
          <a href={contracts.copyUrl(id, party.id)} target="_blank" rel="noopener noreferrer"
            className="text-xs font-semibold text-foreground bg-surface border border-line hover:bg-surface-hover px-3 py-1.5 rounded-lg">
            {t("contracts.action.frozen_pdf")}
          </a>
        )}
        {party.has_signed_copy && (
          <a href={contracts.signedUrl(id, party.id)} target="_blank" rel="noopener noreferrer"
            className="text-xs font-semibold text-emerald-700 bg-surface border border-emerald-200 hover:bg-emerald-50 px-3 py-1.5 rounded-lg">
            {t("contracts.action.signed_pdf")}
          </a>
        )}
        {maySign && party.party_role !== "issuer" && open && (
          <CeremonyButton
            label={t("contracts.action.sign_for_party")}
            start={() => contracts.signStart(id, party.id)}
            poll={() => contracts.signPoll(id, party.id)}
            onDone={onSigned}
            onError={(value) => onError(new Error(value))}
          />
        )}
        {maySend && party.party_role !== "issuer" && open && (
          <button onClick={() => void onInvite()} className="text-xs font-semibold text-foreground bg-surface border border-line hover:bg-surface-hover px-3 py-1.5 rounded-lg inline-flex items-center gap-1">
            <Link2 className="w-3.5 h-3.5" />
            {t("contracts.action.invite")}
          </button>
        )}
        {mayParties && draft && party.state === "draft" && (
          <button
            onClick={() => { void contracts.removeParty(id, party.id).then(onChanged).catch(onError); }}
            className="text-xs font-semibold text-red-700 bg-surface border border-red-200 hover:bg-red-50 px-3 py-1.5 rounded-lg"
          >
            {t("contracts.action.remove")}
          </button>
        )}
      </div>
    </div>
  );
}

function AddPartyForm({ id, onAdded, onError }: {
  id: string;
  onAdded: () => Promise<void>;
  onError: (err: unknown) => void;
}) {
  const { t } = useI18n();
  const [form, setForm] = useState({
    display_name: "", registration_number: "", party_role: "counterparty", party_kind: "organisation",
    contact_email: "", contact_phone: "", address_line: "", sign_order: "", home: "",
  });
  const [busy, setBusy] = useState(false);
  const set = (key: keyof typeof form) => (event: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) =>
    setForm((current) => ({ ...current, [key]: event.target.value }));
  const needsHome = form.party_kind === "tenant" || form.party_kind === "member";

  const add = async () => {
    setBusy(true);
    try {
      const payload: Record<string, unknown> = {
        party_role: form.party_role, party_kind: form.party_kind,
        display_name: form.display_name.trim(), registration_number: form.registration_number.trim(),
        contact_email: form.contact_email.trim(), contact_phone: form.contact_phone.trim(),
        address_line: form.address_line.trim(),
      };
      if (form.sign_order) payload.sign_order = Number(form.sign_order);
      if (form.party_kind === "tenant") payload.counterparty_tenant_id = form.home.trim() || null;
      if (form.party_kind === "member") payload.member_user_id = form.home.trim() || null;
      await contracts.addParty(id, payload);
      setForm({ display_name: "", registration_number: "", party_role: "counterparty", party_kind: "organisation", contact_email: "", contact_phone: "", address_line: "", sign_order: "", home: "" });
      await onAdded();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  const field = (label: string, node: React.ReactNode) => (
    <div>
      <label className="block text-xs font-medium text-muted mb-1">{label}</label>
      {node}
    </div>
  );

  return (
    <div className="border-t border-line pt-4 space-y-3">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-muted">{t("contracts.section.add_party")}</h3>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
        {field(t("contracts.field.name"), <input className={fieldClass} value={form.display_name} onChange={set("display_name")} />)}
        {field(t("contracts.field.reg"), <input className={fieldClass} value={form.registration_number} onChange={set("registration_number")} />)}
        {field(t("contracts.field.role"), (
          <select className={selectClass} value={form.party_role} onChange={set("party_role")}>
            <option value="counterparty">{t("contracts.role.counterparty")}</option>
            <option value="issuer">{t("contracts.role.issuer")}</option>
            <option value="witness">{t("contracts.role.witness")}</option>
            <option value="guarantor">{t("contracts.role.guarantor")}</option>
          </select>
        ))}
        {field(t("contracts.field.kind"), (
          <select className={selectClass} value={form.party_kind} onChange={set("party_kind")}>
            <option value="organisation">{t("contracts.kind.organisation")}</option>
            <option value="person">{t("contracts.kind.person")}</option>
            <option value="tenant">{t("contracts.kind.tenant")}</option>
            <option value="member">{t("contracts.kind.member")}</option>
          </select>
        ))}
        {field(t("contracts.field.email"), <input className={fieldClass} value={form.contact_email} onChange={set("contact_email")} />)}
        {field(t("contracts.field.phone"), <input className={fieldClass} value={form.contact_phone} onChange={set("contact_phone")} />)}
        {field(t("contracts.field.address"), <input className={fieldClass} value={form.address_line} onChange={set("address_line")} />)}
        {field(t("contracts.field.sign_order"), <input type="number" min={1} className={fieldClass} value={form.sign_order} onChange={set("sign_order")} />)}
        {needsHome && field(
          form.party_kind === "member" ? t("contracts.field.home_user") : t("contracts.field.home_tenant"),
          <input className={fieldClass} placeholder="UUID" value={form.home} onChange={set("home")} />,
        )}
      </div>
      {needsHome && <p className="text-[11px] text-muted">{t("contracts.field.home_tenant_hint")}</p>}
      <p className="text-[11px] text-muted">{t("contracts.field.sign_order_hint")}</p>
      <button onClick={() => void add()} disabled={busy || !form.display_name.trim()}
        className="bg-slate-800 hover:bg-slate-900 disabled:opacity-50 text-white text-xs font-semibold px-4 py-2 rounded-lg inline-flex items-center gap-1.5">
        <Plus className="w-3.5 h-3.5" />
        {t("contracts.action.add")}
      </button>
    </div>
  );
}

function SignatoryModal({ id, party, onClose, onAdded, onError }: {
  id: string;
  party: Party;
  onClose: () => void;
  onAdded: () => Promise<void>;
  onError: (err: unknown) => void;
}) {
  const { t } = useI18n();
  const [form, setForm] = useState({ full_name: "", position: "", reg_number: "" });
  const [busy, setBusy] = useState(false);
  const set = (key: keyof typeof form) => (event: React.ChangeEvent<HTMLInputElement>) =>
    setForm((current) => ({ ...current, [key]: event.target.value }));

  const add = async () => {
    setBusy(true);
    try {
      await contracts.addSignatory(id, party.id, {
        full_name: form.full_name.trim(), position: form.position.trim(), reg_number: form.reg_number.trim(),
      });
      await onAdded();
    } catch (err) {
      onError(err);
      setBusy(false);
    }
  };

  return (
    <Modal onClose={onClose} label={t("contracts.action.add_signatory")}>
      <div className="space-y-4">
        <h2 className="text-lg font-semibold text-foreground">{party.display_name} — {t("contracts.section.signatory")}</h2>
        <p className="text-xs text-muted">{t("contracts.msg.nominate_hint")}</p>
        <div>
          <label className="block text-xs font-medium text-muted mb-1">{t("contracts.field.full_name")}</label>
          <input autoFocus className={fieldClass} value={form.full_name} onChange={set("full_name")} />
        </div>
        <div>
          <label className="block text-xs font-medium text-muted mb-1">{t("contracts.field.position")}</label>
          <input className={fieldClass} value={form.position} onChange={set("position")} />
        </div>
        <div>
          <label className="block text-xs font-medium text-muted mb-1">{t("contracts.field.reg")}</label>
          <input className={fieldClass} value={form.reg_number} onChange={set("reg_number")} />
        </div>
        <div className="flex justify-end gap-2">
          <button onClick={onClose} className="text-sm font-medium text-muted px-4 py-2 rounded-lg hover:bg-surface-hover">
            {t("contracts.action.cancel")}
          </button>
          <button onClick={() => void add()} disabled={busy || !form.full_name.trim()}
            className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white text-sm font-semibold px-4 py-2 rounded-lg">
            {t("contracts.action.add")}
          </button>
        </div>
      </div>
    </Modal>
  );
}

function InviteModal({ party, invitation, onClose }: {
  party: Party;
  invitation: Invitation;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const url = typeof window !== "undefined" ? `${window.location.origin}${invitation.path}` : invitation.path;
  return (
    <Modal onClose={onClose} label={t("contracts.action.invite")}>
      <div className="space-y-4">
        <h2 className="text-lg font-semibold text-foreground">{t("contracts.action.invite")}</h2>
        <p className="text-sm text-muted">{t("contracts.msg.invite_for", { name: party.display_name })}</p>
        <div className="font-mono text-xs bg-surface-2 rounded-lg p-3 break-all">{url}</div>
        <Banner tone="warning" message={t("contracts.msg.invite_once")} />
        <p className="text-xs text-muted">{t("contracts.msg.invite_expires", { when: fmtWhen(invitation.expires_at) })}</p>
        <div className="flex justify-end gap-2">
          <button
            onClick={() => { void navigator.clipboard?.writeText(url); }}
            className="bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-semibold px-4 py-2 rounded-lg inline-flex items-center gap-1.5"
          >
            <Copy className="w-4 h-4" />
            {t("contracts.action.copy_link")}
          </button>
          <button onClick={onClose} className="text-sm font-medium text-muted px-4 py-2 rounded-lg hover:bg-surface-hover">
            {t("contracts.action.cancel")}
          </button>
        </div>
      </div>
    </Modal>
  );
}

// ─────────────────────────────────────────────────────────────────── илгээх

function SendCard({ id, state, mode, onChanged, onError, onInfo }: {
  id: string;
  state: ContractShape["contract_state"];
  mode: string;
  onChanged: () => Promise<void>;
  onError: (err: unknown) => void;
  onInfo: (value: string) => void;
}) {
  const { t } = useI18n();
  const [signingMode, setSigningMode] = useState<"counterpart" | "joint">(mode === "joint" ? "joint" : "counterpart");
  const [busy, setBusy] = useState(false);
  const [skips, setSkips] = useState<Array<{ name: string; reason: string }>>([]);

  const act = async (run: () => Promise<unknown>) => {
    setBusy(true);
    try {
      await run();
      await onChanged();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className={`${cardClass} p-5 space-y-3`}>
      <h2 className="text-sm font-semibold text-foreground">{t("contracts.section.send")}</h2>
      <p className="text-xs text-muted">{t("contracts.send.note")}</p>
      {state === "WITHDRAWN" && <Banner tone="warning" message={t("contracts.send.withdrawn_note")} />}
      <div className="max-w-xs">
        <label className="block text-xs font-medium text-muted mb-1">{t("contracts.field.mode")}</label>
        <select className={selectClass} value={signingMode} onChange={(event) => setSigningMode(event.target.value as "counterpart" | "joint")}>
          <option value="counterpart">{t("contracts.mode.counterpart")}</option>
          <option value="joint">{t("contracts.mode.joint")}</option>
        </select>
      </div>
      {skips.map((skip, index) => (
        <Banner key={index} tone="warning" message={`${skip.name}: ${skip.reason}`} />
      ))}
      <div className="flex flex-wrap gap-2">
        {state !== "WITHDRAWN" && (
          <button
            onClick={() => void act(async () => {
              const result = await contracts.send(id, signingMode);
              setSkips(result.skipped.map((skip) => ({ name: skip.name, reason: skip.reason })));
              onInfo(t("contracts.msg.sent", { count: result.sent }));
            })}
            disabled={busy}
            className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white text-sm font-semibold px-4 py-2 rounded-lg inline-flex items-center gap-1.5"
          >
            <Send className="w-4 h-4" />
            {state === "DRAFT" || state === "NONE" ? t("contracts.action.send") : t("contracts.action.resend")}
          </button>
        )}
        {(state === "SENT" || state === "PARTIALLY_SIGNED" || state === "DECLINED") && (
          <button
            onClick={() => {
              const reason = window.prompt(t("contracts.field.reason")) ?? "";
              void act(() => contracts.withdraw(id, reason));
            }}
            disabled={busy}
            className="bg-red-50 hover:bg-red-100 disabled:opacity-50 text-red-700 text-sm font-semibold px-4 py-2 rounded-lg"
          >
            {t("contracts.action.withdraw")}
          </button>
        )}
        {state === "WITHDRAWN" && (
          <button
            onClick={() => void act(() => contracts.reopen(id))}
            disabled={busy}
            className="bg-surface-2 hover:bg-slate-200 disabled:opacity-50 text-foreground text-sm font-semibold px-4 py-2 rounded-lg inline-flex items-center gap-1.5"
          >
            <Undo2 className="w-4 h-4" />
            {t("contracts.action.reopen")}
          </button>
        )}
      </div>
    </section>
  );
}

// ─────────────────────────────────────────────────────────── мастер PDF

/**
 * Гэрээний PDF: гаргагч өөрийн бэлтгэсэн файлаа хавсаргаж, ӨӨРӨӨ PIN2-оор
 * зурна. Илгээх агшинд тал бүрийн хөлдсөн хувь нь энэ файл — гаргагч зурсан
 * бол ГАРЫН ҮСЭГТЭЙ хувь нь — болно: захирал бүрийн зурах байт гаргагчийн
 * гарын үсгийг хамарна.
 *
 * Дараалал нь чухал бөгөөд сервер өөрөө барьдаг: гарын үсэг зурагдмагц файл
 * ч, талууд ч, бичвэр ч өөрчлөгдөхгүй. Тиймээс UI зөв дарааллыг хэлж өгнө:
 * PDF → талууд → өөрийн гарын үсэг → илгээх.
 */
function MasterPdfCard({ id, contract, mayManage, maySign, onChanged, onError, onInfo }: {
  id: string;
  contract: ContractShape;
  mayManage: boolean;
  maySign: boolean;
  onChanged: () => Promise<void>;
  onError: (err: unknown) => void;
  onInfo: (value: string) => void;
}) {
  const { t } = useI18n();
  const fileInput = React.useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [reg, setReg] = useState("");
  const attachment = contract.attachment ?? null;

  // Гаргагчийн өөрийн eID регистр — PIN2 яг түүний утсанд очно.
  React.useEffect(() => {
    let alive = true;
    contracts.myEidReg().then((value) => { if (alive && value) setReg(value); }).catch(() => undefined);
    return () => { alive = false; };
  }, []);

  const upload = async (file: File) => {
    setBusy(true);
    try {
      await contracts.attach(id, file);
      onInfo(t("contracts.msg.pdf_attached"));
      await onChanged();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
      if (fileInput.current) fileInput.current.value = "";
    }
  };

  return (
    <section className={`${cardClass} p-5 space-y-3`}>
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-foreground">{t("contracts.section.pdf")}</h2>
        {mayManage && (
          <>
            <input
              ref={fileInput}
              type="file"
              accept=".pdf,.docx"
              className="hidden"
              onChange={(event) => {
                const file = event.target.files?.[0];
                if (file) void upload(file);
              }}
            />
            <button
              onClick={() => fileInput.current?.click()}
              disabled={busy}
              className="text-xs font-semibold text-indigo-700 bg-indigo-50 hover:bg-indigo-100 px-3 py-1.5 rounded-lg disabled:opacity-50 inline-flex items-center gap-1.5"
            >
              <Upload className="w-3.5 h-3.5" />
              {attachment ? t("contracts.action.replace_pdf") : t("contracts.action.attach_pdf")}
            </button>
          </>
        )}
      </div>
      <p className="text-xs text-muted">{t("contracts.pdf.note")}</p>
      <a href={contracts.wordTemplateUrl()} className="text-xs text-indigo-700 hover:underline">
        {t("contracts.action.word_template")}
      </a>
      {attachment ? (
        <div className="flex flex-wrap items-center gap-3 rounded-xl border border-line bg-slate-50/60 px-4 py-3">
          <FileUp className="w-4 h-4 text-muted" />
          <a href={contracts.fileUrl(id)} target="_blank" rel="noopener noreferrer"
            className="text-sm font-semibold text-indigo-700 hover:underline">
            {attachment.file_name}
          </a>
          <span className="text-xs text-muted">{(attachment.size_bytes / (1024 * 1024)).toFixed(1)} MB</span>
          {attachment.file_name.toLowerCase().endsWith(".docx") ? (
            <span className="text-xs font-semibold text-indigo-700 bg-indigo-50 rounded-full px-2.5 py-0.5">
              {t("contracts.pdf.word_badge")}
            </span>
          ) : attachment.master_signed ? (
            <span className="text-xs font-semibold text-emerald-700 bg-emerald-50 rounded-full px-2.5 py-0.5">
              {t("contracts.pdf.master_signed")}
            </span>
          ) : maySign ? (
            <span className="ml-auto flex items-center gap-2">
              <input
                className={`${fieldClass} !w-44`}
                placeholder={t("contracts.field.reg")}
                value={reg}
                onChange={(event) => setReg(event.target.value)}
              />
              <CeremonyButton
                label={t("contracts.action.master_sign")}
                start={() => contracts.masterSignStart(id, reg.trim())}
                poll={(session: CeremonySession) => contracts.masterSignPoll(id, session.session_id)}
                onDone={async () => {
                  onInfo(t("contracts.msg.master_signed"));
                  await onChanged();
                }}
                onError={(value) => onError(new Error(value))}
              />
            </span>
          ) : null}
        </div>
      ) : (
        <p className="text-xs text-muted">{t("contracts.pdf.none")}</p>
      )}
    </section>
  );
}

// ─────────────────────────────────────────────── тараалт

/**
 * Нэг загвар — хүн бүрд ТУСДАА гэрээ.
 *
 * Зээлийн гэрээг 500 хүнтэй байгуулахад 500 хүн НЭГ гэрээний хамтрагч тал
 * болдоггүй: хүн бүртэй тус тусдаа гэрээ байгуулагдана. Хүлээн авагчид
 * бие биеэ огт харахгүй, хүн бүрийн гэрээ өөрийнхөө гарын үсгээр хүчин
 * төгөлдөр болно. Жагсаалтыг Excel-ээс эсвэл энд нэг нэгээр нь нэмнэ.
 */
function IssueCard({ id, onChanged, onError, onInfo }: {
  id: string;
  onChanged: () => Promise<void>;
  onError: (err: unknown) => void;
  onInfo: (value: string) => void;
}) {
  const { t } = useI18n();
  const router = useRouter();
  const fileInput = React.useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [progress, setProgress] = useState<{ done: number; total: number } | null>(null);
  const [pending, setPending] = useState<Array<{ name: string; signer_reg: string }>>([]);
  const [name, setName] = useState("");
  const [reg, setReg] = useState("");
  const [outcome, setOutcome] = useState<{
    issued: number;
    skipped: Array<{ row?: number; name?: string; reason: string }>;
  } | null>(null);

  const finish = async (result: { issued: number; children: unknown[]; skipped: Array<{ row?: number; name?: string; reason: string }> }) => {
    setOutcome({ issued: result.issued, skipped: result.skipped });
    setPending([]);
    if (result.issued > 0) onInfo(t("contracts.issue.done", { count: result.issued }));
    await onChanged();
    router.refresh();
  };

  // Жагсаалт хэдий ч урт байг — 10-аар хэсэглэн ДАРААЛАН явуулна.
  // Word мастертай тараалтад хүн бүр LibreOffice хөрвүүлэлт «үнэтэй» тул
  // серверийн нэг хүсэлтийн дээд хязгаарт багтаж, явц нь чухам харагдана.
  const runChunked = async (rows: Array<{ name: string; org_reg?: string; signer_name?: string; signer_reg: string; position?: string }>) => {
    setProgress({ done: 0, total: rows.length });
    try {
      await finish(await contracts.issueChunked(id, rows, (done, total) => setProgress({ done, total })));
    } finally {
      setProgress(null);
    }
  };

  const runFile = async (file: File) => {
    setBusy(true);
    setOutcome(null);
    try {
      // Excel нэг удаа урьдчилан уншигдана — юу ч үүсгэхгүй. Ирсэн JSON-ыг
      // хэсэглэж явуулна: 500 мөр нэг хүсэлтэд багтах албагүй болно.
      const preview = await contracts.issuePreview(id, file);
      await runChunked(preview.recipients.map((row) => ({
        name: row.name, org_reg: row.org_reg, signer_name: row.signer_name,
        signer_reg: row.signer_reg, position: row.position,
      })));
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
      if (fileInput.current) fileInput.current.value = "";
    }
  };

  const runManual = async () => {
    setBusy(true);
    setOutcome(null);
    try {
      await runChunked(pending.map((row) => ({ name: row.name, signer_reg: row.signer_reg })));
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className={`${cardClass} p-5 space-y-3`}>
      <div className="flex flex-wrap items-center gap-2">
        <h2 className="text-sm font-semibold text-foreground grow">{t("contracts.section.issue")}</h2>
        <a href={contracts.importTemplateUrl()} className="text-xs text-indigo-700 hover:underline">
          {t("contracts.action.import_template")}
        </a>
        <input
          ref={fileInput}
          type="file"
          accept=".xlsx,.csv"
          className="hidden"
          onChange={(event) => {
            const file = event.target.files?.[0];
            if (file) void runFile(file);
          }}
        />
        <button
          onClick={() => fileInput.current?.click()}
          disabled={busy}
          className="text-xs font-semibold text-foreground bg-surface border border-line hover:bg-surface-hover px-3 py-1.5 rounded-lg disabled:opacity-50 inline-flex items-center gap-1.5"
        >
          <FileUp className="w-3.5 h-3.5" />
          {t("contracts.action.import_excel")}
        </button>
      </div>
      <p className="text-xs text-muted">{t("contracts.issue.note")}</p>

      {/* Гараар: нэр + регистр, хэдийг ч. */}
      <div className="flex flex-wrap items-end gap-2">
        <div className="grow min-w-[12rem]">
          <label className="block text-xs font-medium text-muted mb-1">{t("contracts.field.recipient_name")}</label>
          <input className={fieldClass} value={name} onChange={(event) => setName(event.target.value)} />
        </div>
        <div className="min-w-[11rem]">
          <label className="block text-xs font-medium text-muted mb-1">{t("contracts.field.recipient_reg")}</label>
          <input className={fieldClass} value={reg} onChange={(event) => setReg(event.target.value)} />
        </div>
        <button
          onClick={() => {
            if (!name.trim() || !reg.trim()) return;
            setPending((current) => [...current, { name: name.trim(), signer_reg: reg.trim() }]);
            setName("");
            setReg("");
          }}
          disabled={!name.trim() || !reg.trim()}
          className="h-[38px] text-xs font-semibold text-foreground bg-surface border border-line hover:bg-surface-hover px-3 rounded-lg disabled:opacity-50 inline-flex items-center gap-1"
        >
          <Plus className="w-3.5 h-3.5" />
          {t("contracts.action.add")}
        </button>
      </div>
      {pending.length > 0 && (
        <div className="space-y-1">
          {pending.map((row, index) => (
            <div key={index} className="flex items-center gap-2 text-sm text-foreground">
              <span className="grow">{row.name} · {row.signer_reg}</span>
              <button
                onClick={() => setPending((current) => current.filter((_, i) => i !== index))}
                className="text-xs text-red-600 hover:underline"
              >
                {t("contracts.action.remove")}
              </button>
            </div>
          ))}
          <button
            onClick={() => void runManual()}
            disabled={busy}
            className="mt-1 bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white text-sm font-semibold px-4 py-2 rounded-lg inline-flex items-center gap-1.5"
          >
            <Send className="w-4 h-4" />
            {t("contracts.action.issue", { count: pending.length })}
          </button>
        </div>
      )}

      {progress && progress.total > 10 && (
        <p className="text-xs font-medium text-indigo-700">
          {t("contracts.issue.progress", { done: progress.done, total: progress.total })}
        </p>
      )}
      {outcome && (
        <Banner
          tone={outcome.skipped.length ? "warning" : "success"}
          message={t("contracts.import.result", { added: outcome.issued, skipped: outcome.skipped.length })}
        />
      )}
      {outcome?.skipped.slice(0, 6).map((skip, index) => (
        <p key={index} className="text-[11px] text-amber-700">
          {skip.name ? `${skip.name} — ` : ""}{skip.reason}
        </p>
      ))}
      <IssuedChildren id={id} refreshKey={outcome?.issued ?? 0} />
    </section>
  );
}

/** Энэ мастераас тараагдсан гэрээнүүд, тус бүрийн төлөвтэйгөө. */
function IssuedChildren({ id, refreshKey }: { id: string; refreshKey: number }) {
  const { t } = useI18n();
  const router = useRouter();
  const [children, setChildren] = useState<ContractRow[]>([]);

  React.useEffect(() => {
    let alive = true;
    contracts.list()
      .then((res) => {
        if (alive) setChildren(res.contracts.filter((row) => row.parent_document_id === id));
      })
      .catch(() => undefined);
    return () => { alive = false; };
  }, [id, refreshKey]);

  if (children.length === 0) return null;
  return (
    <div className="border-t border-line pt-3 space-y-1">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-muted">
        {t("contracts.issue.children", { count: children.length })}
      </h3>
      {children.map((child) => (
        <button
          key={child.id}
          onClick={() => router.push(`/module/documents/contracts/${child.id}`)}
          className="w-full flex items-center gap-2 text-left text-sm rounded-lg px-2 py-1.5 hover:bg-surface-hover"
        >
          <span className="grow text-foreground">{child.counterparties || child.title}</span>
          <span className="font-mono text-xs text-muted">{child.signed_count}/{child.required_count}</span>
          <ContractBadge state={child.contract_state} />
        </button>
      ))}
    </div>
  );
}
