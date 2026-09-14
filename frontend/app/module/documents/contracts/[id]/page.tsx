"use client";

import React, { useId, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useRouter } from "next/navigation";
import {
  contracts, CeremonySession, ContractRow, ContractShape, Invitation, Party,
} from "@/lib/contracts";
import { useResource, useLoadOnMount } from "@/lib/useResource";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { ListSkeleton, SelectField } from "@/components/documents/shared";
import {
  CeremonyButton, ContractBadge, PartyBadge, fmtWhen, useContractLabels,
} from "@/components/documents/contracts";
import {
  Alert,
  Badge,
  type BadgeProps,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  ErrorState,
  Input,
  Progress,
  Textarea,
} from "@gerege-systems/ui";
import { ArrowLeft, Copy, FileUp, Link2, Plus, Send, Undo2, Upload, X } from "lucide-react";

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

  if (shape.loading) return <ListSkeleton />;
  if (shape.failed || !shape.data) {
    return (
      <ErrorState
        title={t("contracts.msg.load_failed")}
        description=""
        live
        action={
          <Button variant="outline" onClick={() => void shape.reload()}>
            {t("base.action.retry")}
          </Button>
        }
      />
    );
  }

  const contract = shape.data;
  const state = contract.contract_state;
  const editable = state === "DRAFT" || state === "NONE";
  const reload = async () => { await shape.reload(); };

  return (
    <div className="space-y-6">
      <Button variant="link" size="sm" asChild>
        <Link href="/module/documents/contracts">
          <ArrowLeft aria-hidden />
          {t("contracts.view.back")}
        </Link>
      </Button>

      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-semibold text-foreground">{contract.title}</h1>
        <ContractBadge state={state} />
      </div>

      {/* Талуудын явц — гэрээ хаана байгааг нэг харцаар. */}
      {contract.parties.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {contract.parties.map((party) => (
            <Badge key={party.id} variant="outline" tone={partyTone(party.state)} dot className="gap-2 py-1">
              <span className="font-medium text-foreground">{party.display_name}</span>
              <PartyBadge state={party.state} />
            </Badge>
          ))}
        </div>
      )}

      {message && (
        <Alert variant={message.tone === "error" ? "danger" : "success"} live dismissible onDismiss={() => setMessage(null)}>
          {message.text}
        </Alert>
      )}

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

/** The dot beside a party's name: where that party is on the contract. */
function partyTone(state: Party["state"]): BadgeProps["tone"] {
  if (state === "signed") return "success";
  if (state === "declined") return "danger";
  if (state === "invited" || state === "viewed") return "warning";
  return "neutral";
}

/** A section on its own card: a title row with its one action, then the body. */
function Section({ title, action, description, children }: {
  title: string;
  action?: React.ReactNode;
  description?: string;
  children: React.ReactNode;
}) {
  return (
    <Card>
      <CardHeader className="flex-row flex-wrap items-center justify-between gap-2 pb-4">
        <div className="flex flex-col gap-1">
          <h2 className="text-sm leading-tight font-semibold text-foreground">{title}</h2>
          {description && <CardDescription className="text-xs">{description}</CardDescription>}
        </div>
        {action}
      </CardHeader>
      <CardContent className="space-y-4">{children}</CardContent>
    </Card>
  );
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

  return (
    <Section
      title={t("contracts.section.facts")}
      action={
        editable ? (
          <Button size="sm" variant="outline" onClick={() => void save()} loading={busy}>
            {t("contracts.action.save")}
          </Button>
        ) : undefined
      }
    >
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        <Input label={t("contracts.field.number")} value={facts.contract_number} onChange={set("contract_number")} disabled={!editable} />
        <Input type="date" label={t("contracts.field.due")} value={facts.due_at} onChange={set("due_at")} disabled={!editable} />
        <Input type="number" step="0.01" label={t("contracts.field.amount")} value={facts.amount} onChange={set("amount")} disabled={!editable} />
        <Input maxLength={3} label={t("contracts.field.currency")} value={facts.currency} onChange={set("currency")} disabled={!editable} />
        <Input type="date" label={t("contracts.field.effective_from")} value={facts.effective_from} onChange={set("effective_from")} disabled={!editable} />
        <Input type="date" label={t("contracts.field.effective_to")} value={facts.effective_to} onChange={set("effective_to")} disabled={!editable} />
      </div>
    </Section>
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
    <Section
      title={t("contracts.section.body")}
      action={
        editable && !frozen ? (
          <Button size="sm" variant="outline" onClick={() => void save()} loading={busy}>
            {t("contracts.action.save")}
          </Button>
        ) : undefined
      }
    >
      {frozen && <Alert variant="info">{t("contracts.body.frozen_note")}</Alert>}
      <Textarea
        label={t("contracts.section.body")}
        hideLabel
        rows={12}
        className="[&_textarea]:font-mono [&_textarea]:leading-relaxed"
        placeholder={t("contracts.body.placeholder")}
        value={text}
        onChange={(event) => setText(event.target.value)}
        readOnly={frozen || !editable}
        helperText={t("contracts.body.hint")}
      />
      <details className="text-xs text-muted">
        <summary className="cursor-pointer hover:text-foreground">…</summary>
        {t("contracts.body.advanced", { tokens: "{{тал}} {{регистр}} {{төлөөлөгч}} {{хаяг}} {{гэрээ}} {{дугаар}} {{огноо}}" })}
      </details>
    </Section>
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
    <Section title={t("contracts.section.parties")} description={t("contracts.parties.note")}>
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
    </Section>
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
    <Card padding="sm" className="bg-surface-2 space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-semibold text-foreground">{party.display_name}</span>
        <Badge tone="neutral">{partyRole(party.party_role)}</Badge>
        <Badge tone="neutral">{partyKind(party.party_kind)}</Badge>
        {party.sign_order != null && (
          <Badge tone="neutral">{t("contracts.msg.signs_at", { n: party.sign_order })}</Badge>
        )}
        <span className="ms-auto"><PartyBadge state={party.state} /></span>
      </div>
      {contact && <div className="text-xs text-muted">{contact}</div>}
      {party.decline_reason && (
        <Alert variant="danger">{t("contracts.msg.decline_reason_of", { reason: party.decline_reason })}</Alert>
      )}
      {(party.signatories?.length ?? 0) > 0 && (
        <div className="text-xs text-muted">
          {party.signatories!.map((signatory) => (
            <span key={signatory.id} className="me-3">
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
          <Button size="sm" variant="outline" onClick={onAddSignatory}>
            {t("contracts.action.add_signatory")}
          </Button>
        )}
        {party.has_copy && (
          <Button size="sm" variant="outline" asChild>
            <a href={contracts.copyUrl(id, party.id)} target="_blank" rel="noopener noreferrer">
              {t("contracts.action.frozen_pdf")}
            </a>
          </Button>
        )}
        {party.has_signed_copy && (
          <Button size="sm" variant="outline" className="text-success" asChild>
            <a href={contracts.signedUrl(id, party.id)} target="_blank" rel="noopener noreferrer">
              {t("contracts.action.signed_pdf")}
            </a>
          </Button>
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
          <Button size="sm" variant="outline" onClick={() => void onInvite()} leadingIcon={<Link2 />}>
            {t("contracts.action.invite")}
          </Button>
        )}
        {mayParties && draft && party.state === "draft" && (
          <Button
            size="sm"
            variant="outline"
            className="text-danger"
            onClick={() => { void contracts.removeParty(id, party.id).then(onChanged).catch(onError); }}
          >
            {t("contracts.action.remove")}
          </Button>
        )}
      </div>
    </Card>
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
  const set = (key: keyof typeof form) => (event: React.ChangeEvent<HTMLInputElement>) =>
    setForm((current) => ({ ...current, [key]: event.target.value }));
  const pick = (key: keyof typeof form) => (value: string) =>
    setForm((current) => ({ ...current, [key]: value }));
  const needsHome = form.party_kind === "tenant" || form.party_kind === "member";
  const base = useId();

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

  return (
    <div className="border-t border-line pt-4 space-y-3">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-muted">{t("contracts.section.add_party")}</h3>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
        <Input label={t("contracts.field.name")} value={form.display_name} onChange={set("display_name")} />
        <Input label={t("contracts.field.reg")} value={form.registration_number} onChange={set("registration_number")} />
        <SelectField
          id={`${base}party_role`}
          label={t("contracts.field.role")}
          value={form.party_role}
          onValueChange={pick("party_role")}
          options={[
            { value: "counterparty", label: t("contracts.role.counterparty") },
            { value: "issuer", label: t("contracts.role.issuer") },
            { value: "witness", label: t("contracts.role.witness") },
            { value: "guarantor", label: t("contracts.role.guarantor") },
          ]}
        />
        <SelectField
          id={`${base}party_kind`}
          label={t("contracts.field.kind")}
          value={form.party_kind}
          onValueChange={pick("party_kind")}
          options={[
            { value: "organisation", label: t("contracts.kind.organisation") },
            { value: "person", label: t("contracts.kind.person") },
            { value: "tenant", label: t("contracts.kind.tenant") },
            { value: "member", label: t("contracts.kind.member") },
          ]}
        />
        <Input label={t("contracts.field.email")} value={form.contact_email} onChange={set("contact_email")} />
        <Input label={t("contracts.field.phone")} value={form.contact_phone} onChange={set("contact_phone")} />
        <Input label={t("contracts.field.address")} value={form.address_line} onChange={set("address_line")} />
        <Input type="number" min={1} label={t("contracts.field.sign_order")} value={form.sign_order} onChange={set("sign_order")} />
        {needsHome && (
          <Input
            label={form.party_kind === "member" ? t("contracts.field.home_user") : t("contracts.field.home_tenant")}
            placeholder="UUID"
            value={form.home}
            onChange={set("home")}
          />
        )}
      </div>
      {needsHome && <p className="text-xs text-muted">{t("contracts.field.home_tenant_hint")}</p>}
      <p className="text-xs text-muted">{t("contracts.field.sign_order_hint")}</p>
      <Button onClick={() => void add()} loading={busy} disabled={!form.display_name.trim()} leadingIcon={<Plus />}>
        {t("contracts.action.add")}
      </Button>
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
  const typed = Object.values(form).some((value) => value.trim() !== "");

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
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent
        showClose={false}
        // A stray click outside must not throw away a half-filled form.
        onInteractOutside={(event) => { if (typed) event.preventDefault(); }}
      >
        <div className="space-y-4">
          <DialogHeader>
            <DialogTitle>{party.display_name} — {t("contracts.section.signatory")}</DialogTitle>
            <DialogDescription>{t("contracts.msg.nominate_hint")}</DialogDescription>
          </DialogHeader>
          <Input autoFocus label={t("contracts.field.full_name")} value={form.full_name} onChange={set("full_name")} />
          <Input label={t("contracts.field.position")} value={form.position} onChange={set("position")} />
          <Input label={t("contracts.field.reg")} value={form.reg_number} onChange={set("reg_number")} />
          <DialogFooter>
            <Button variant="outline" onClick={onClose}>
              {t("contracts.action.cancel")}
            </Button>
            <Button onClick={() => void add()} loading={busy} disabled={!form.full_name.trim()}>
              {t("contracts.action.add")}
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
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
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent showClose={false}>
        <div className="space-y-4">
          <DialogHeader>
            <DialogTitle>{t("contracts.action.invite")}</DialogTitle>
            <DialogDescription>{t("contracts.msg.invite_for", { name: party.display_name })}</DialogDescription>
          </DialogHeader>
          <div className="font-mono text-xs bg-surface-2 rounded-md p-3 break-all">{url}</div>
          <Alert variant="warning">{t("contracts.msg.invite_once")}</Alert>
          <p className="text-xs text-muted">{t("contracts.msg.invite_expires", { when: fmtWhen(invitation.expires_at) })}</p>
          <DialogFooter>
            <Button variant="outline" onClick={onClose}>
              {t("contracts.action.cancel")}
            </Button>
            <Button onClick={() => { void navigator.clipboard?.writeText(url); }} leadingIcon={<Copy />}>
              {t("contracts.action.copy_link")}
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
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
  const modeId = useId();

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
    <Section title={t("contracts.section.send")} description={t("contracts.send.note")}>
      {state === "WITHDRAWN" && <Alert variant="warning">{t("contracts.send.withdrawn_note")}</Alert>}
      <SelectField
        id={modeId}
        label={t("contracts.field.mode")}
        value={signingMode}
        onValueChange={(value) => setSigningMode(value as "counterpart" | "joint")}
        options={[
          { value: "counterpart", label: t("contracts.mode.counterpart") },
          { value: "joint", label: t("contracts.mode.joint") },
        ]}
        className="max-w-xs"
      />
      {skips.map((skip, index) => (
        <Alert key={index} variant="warning">{`${skip.name}: ${skip.reason}`}</Alert>
      ))}
      <div className="flex flex-wrap gap-2">
        {state !== "WITHDRAWN" && (
          <Button
            onClick={() => void act(async () => {
              const result = await contracts.send(id, signingMode);
              setSkips(result.skipped.map((skip) => ({ name: skip.name, reason: skip.reason })));
              onInfo(t("contracts.msg.sent", { count: result.sent }));
            })}
            loading={busy}
            leadingIcon={<Send />}
          >
            {state === "DRAFT" || state === "NONE" ? t("contracts.action.send") : t("contracts.action.resend")}
          </Button>
        )}
        {(state === "SENT" || state === "PARTIALLY_SIGNED" || state === "DECLINED") && (
          <Button
            variant="outline"
            className="text-danger"
            onClick={() => {
              const reason = window.prompt(t("contracts.field.reason")) ?? "";
              void act(() => contracts.withdraw(id, reason));
            }}
            disabled={busy}
          >
            {t("contracts.action.withdraw")}
          </Button>
        )}
        {state === "WITHDRAWN" && (
          <Button variant="outline" onClick={() => void act(() => contracts.reopen(id))} loading={busy} leadingIcon={<Undo2 />}>
            {t("contracts.action.reopen")}
          </Button>
        )}
      </div>
    </Section>
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
    <Section
      title={t("contracts.section.pdf")}
      description={t("contracts.pdf.note")}
      action={
        mayManage ? (
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
            <Button size="sm" variant="outline" onClick={() => fileInput.current?.click()} loading={busy} leadingIcon={<Upload />}>
              {attachment ? t("contracts.action.replace_pdf") : t("contracts.action.attach_pdf")}
            </Button>
          </>
        ) : undefined
      }
    >
      <Button variant="link" size="sm" asChild>
        <a href={contracts.wordTemplateUrl()}>{t("contracts.action.word_template")}</a>
      </Button>
      {attachment ? (
        <Card padding="sm" className="bg-surface-2 flex flex-wrap items-center gap-3">
          <FileUp className="w-4 h-4 text-muted" aria-hidden />
          <Button variant="link" asChild>
            <a href={contracts.fileUrl(id)} target="_blank" rel="noopener noreferrer">
              {attachment.file_name}
            </a>
          </Button>
          <span className="text-xs text-muted">{(attachment.size_bytes / (1024 * 1024)).toFixed(1)} MB</span>
          {attachment.file_name.toLowerCase().endsWith(".docx") ? (
            <Badge tone="accent">{t("contracts.pdf.word_badge")}</Badge>
          ) : attachment.master_signed ? (
            <Badge tone="success">{t("contracts.pdf.master_signed")}</Badge>
          ) : maySign ? (
            <span className="ms-auto flex items-center gap-2">
              <Input
                size="sm"
                label={t("contracts.field.reg")}
                hideLabel
                placeholder={t("contracts.field.reg")}
                value={reg}
                onChange={(event) => setReg(event.target.value)}
                className="w-44"
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
        </Card>
      ) : (
        <p className="text-xs text-muted">{t("contracts.pdf.none")}</p>
      )}
    </Section>
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
    <Section
      title={t("contracts.section.issue")}
      description={t("contracts.issue.note")}
      action={
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="link" size="sm" asChild>
            <a href={contracts.importTemplateUrl()}>{t("contracts.action.import_template")}</a>
          </Button>
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
          <Button size="sm" variant="outline" onClick={() => fileInput.current?.click()} loading={busy} leadingIcon={<FileUp />}>
            {t("contracts.action.import_excel")}
          </Button>
        </div>
      }
    >
      {/* Гараар: нэр + регистр, хэдийг ч. */}
      <div className="flex flex-wrap items-end gap-2">
        <div className="grow min-w-48">
          <Input label={t("contracts.field.recipient_name")} value={name} onChange={(event) => setName(event.target.value)} />
        </div>
        <div className="min-w-44">
          <Input label={t("contracts.field.recipient_reg")} value={reg} onChange={(event) => setReg(event.target.value)} />
        </div>
        <Button
          variant="outline"
          onClick={() => {
            if (!name.trim() || !reg.trim()) return;
            setPending((current) => [...current, { name: name.trim(), signer_reg: reg.trim() }]);
            setName("");
            setReg("");
          }}
          disabled={!name.trim() || !reg.trim()}
          leadingIcon={<Plus />}
        >
          {t("contracts.action.add")}
        </Button>
      </div>
      {pending.length > 0 && (
        <div className="space-y-1">
          {pending.map((row, index) => (
            <div key={index} className="flex items-center gap-2 text-sm text-foreground">
              <span className="grow">{row.name} · {row.signer_reg}</span>
              <Button
                variant="ghost"
                size="sm"
                className="text-danger"
                onClick={() => setPending((current) => current.filter((_, i) => i !== index))}
                leadingIcon={<X />}
              >
                {t("contracts.action.remove")}
              </Button>
            </div>
          ))}
          <Button className="mt-1" onClick={() => void runManual()} loading={busy} leadingIcon={<Send />}>
            {t("contracts.action.issue", { count: pending.length })}
          </Button>
        </div>
      )}

      {progress && progress.total > 10 && (
        <div className="space-y-1">
          <Progress value={(progress.done / progress.total) * 100} size="sm" aria-label={t("contracts.section.issue")} />
          <p className="text-xs font-medium text-accent">
            {t("contracts.issue.progress", { done: progress.done, total: progress.total })}
          </p>
        </div>
      )}
      {outcome && (
        <Alert variant={outcome.skipped.length ? "warning" : "success"} live>
          {t("contracts.import.result", { added: outcome.issued, skipped: outcome.skipped.length })}
        </Alert>
      )}
      {outcome?.skipped.slice(0, 6).map((skip, index) => (
        <p key={index} className="text-xs text-warning">
          {skip.name ? `${skip.name} — ` : ""}{skip.reason}
        </p>
      ))}
      <IssuedChildren id={id} refreshKey={outcome?.issued ?? 0} />
    </Section>
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
        <Button
          key={child.id}
          variant="ghost"
          size="sm"
          onClick={() => router.push(`/module/documents/contracts/${child.id}`)}
          className="w-full justify-start gap-2 text-start font-normal h-auto py-1.5"
        >
          <span className="grow text-foreground">{child.counterparties || child.title}</span>
          <span className="font-mono text-xs text-muted">{child.signed_count}/{child.required_count}</span>
          <ContractBadge state={child.contract_state} />
        </Button>
      ))}
    </div>
  );
}
