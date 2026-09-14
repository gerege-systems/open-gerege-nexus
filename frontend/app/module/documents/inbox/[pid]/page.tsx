"use client";

import React, { useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { contracts, InboxDetail, PartyState } from "@/lib/contracts";
import { useResource, useLoadOnMount } from "@/lib/useResource";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { ListSkeleton } from "@/components/documents/shared";
import { CeremonyButton, PartyBadge, fmtWhen, useContractLabels } from "@/components/documents/contracts";
import {
  Alert,
  Badge,
  type BadgeProps,
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  ErrorState,
  Input,
  Textarea,
} from "@gerege-systems/ui";
import { ArrowLeft, FileText } from "lucide-react";

/** The dot beside a party's name: where that party is on the contract. */
function partyTone(state: PartyState): BadgeProps["tone"] {
  if (state === "signed") return "success";
  if (state === "declined") return "danger";
  if (state === "invited" || state === "viewed") return "warning";
  return "neutral";
}

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
  const { partyState } = useContractLabels();
  const mayNominate = can("documents.parties");
  const maySign = can("documents.sign");

  const detail = useResource<InboxDetail | null>(() => contracts.inboxShow(pid), { initial: null });
  useLoadOnMount(detail.reload);

  const [message, setMessage] = useState<{ tone: "error" | "success"; text: string } | null>(null);
  const fail = (err: unknown) => setMessage({ tone: "error", text: err instanceof Error ? err.message : String(err) });
  const [declining, setDeclining] = useState(false);

  if (detail.loading) return <ListSkeleton />;
  if (detail.failed || !detail.data) {
    return (
      <ErrorState
        title={t("contracts.msg.load_failed")}
        description=""
        live
        action={
          <Button variant="outline" onClick={() => void detail.reload()}>
            {t("base.action.retry")}
          </Button>
        }
      />
    );
  }
  const item = detail.data;
  const open = item.state === "invited" || item.state === "viewed";
  const named = item.my_signatories.length > 0;

  return (
    <div className="space-y-6">
      <Button asChild variant="link" size="sm">
        <Link href="/module/documents/inbox">
          <ArrowLeft aria-hidden />
          {t("contracts.view.inbox_back")}
        </Link>
      </Button>

      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-semibold text-foreground">{item.title}</h1>
        <PartyBadge state={item.state} />
      </div>

      {item.parties.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {item.parties.map((party, index) => (
            <Badge key={index} variant="outline" tone={partyTone(party.state)} dot className="py-1">
              {party.display_name}{party.mine ? ` ${t("contracts.msg.you")}` : ""} · {partyState(party.state)}
            </Badge>
          ))}
        </div>
      )}

      {message && (
        <Alert variant={message.tone === "error" ? "danger" : "success"} live dismissible onDismiss={() => setMessage(null)}>
          {message.text}
        </Alert>
      )}

      {item.has_copy ? (
        <Card padding="sm">
          <CardHeader className="flex-row items-center justify-between pb-3">
            <CardTitle className="text-sm">{t("contracts.section.body")}</CardTitle>
            <Button asChild variant="outline" size="sm">
              <a href={contracts.inboxCopyUrl(pid)} target="_blank" rel="noopener noreferrer">
                <FileText aria-hidden />
                {t("contracts.action.view_pdf")}
              </a>
            </Button>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="whitespace-pre-wrap text-sm leading-relaxed text-foreground bg-surface-2 border border-line rounded-lg p-4 max-h-112 overflow-auto">
              {item.body_text}
            </div>
            <p className="text-xs text-muted font-mono">{t("contracts.msg.sha", { sha: item.sha256 || "—" })}</p>
          </CardContent>
        </Card>
      ) : (
        <Alert variant="warning">{t("contracts.msg.not_delivered")}</Alert>
      )}

      <Card padding="sm">
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">{t("contracts.section.signatory")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
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
        </CardContent>
      </Card>

      {open && item.has_copy && maySign && (
        <Card padding="sm">
          <CardHeader className="pb-3">
            <CardTitle className="text-sm">{t("contracts.section.decision")}</CardTitle>
          </CardHeader>
          <CardContent>
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
                size="md"
              />
              <Button variant="outline" className="text-danger" onClick={() => setDeclining(true)}>
                {t("contracts.action.decline")}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {item.state === "signed" && (
        <Card padding="sm">
          <CardContent className="space-y-3">
            <Alert variant="success">{t("contracts.msg.signed_done")}</Alert>
            <Button asChild variant="outline" size="sm" className="text-success">
              <a href={contracts.inboxSignedUrl(pid)} target="_blank" rel="noopener noreferrer">
                {t("contracts.action.signed_pdf")}
              </a>
            </Button>
          </CardContent>
        </Card>
      )}
      {item.state === "declined" && <Alert variant="danger">{t("contracts.msg.declined_done")}</Alert>}

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
        <Input label={t("contracts.field.full_name")} hideLabel placeholder={t("contracts.field.full_name")} value={form.full_name} onChange={set("full_name")} />
        <Input label={t("contracts.field.reg")} hideLabel placeholder={t("contracts.field.reg")} value={form.reg_number} onChange={set("reg_number")} />
        <Input label={t("contracts.field.position")} hideLabel placeholder={t("contracts.field.position")} value={form.position} onChange={set("position")} />
      </div>
      <Button size="sm" onClick={() => void nominate()} loading={busy} disabled={!form.full_name.trim() || !form.reg_number.trim()}>
        {t("contracts.action.nominate")}
      </Button>
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
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent
        showClose={false}
        // A stray click outside must not throw away a typed reason.
        onInteractOutside={(event) => { if (reason.trim()) event.preventDefault(); }}
      >
        <div className="space-y-4">
          <DialogHeader>
            <DialogTitle>{t("contracts.action.decline")}</DialogTitle>
            <DialogDescription>{t("contracts.msg.decline_hint")}</DialogDescription>
          </DialogHeader>
          <Textarea
            autoFocus
            label={t("contracts.field.reason")}
            hideLabel
            rows={4}
            value={reason}
            onChange={(event) => setReason(event.target.value)}
          />
          <DialogFooter>
            <Button variant="outline" onClick={onClose}>
              {t("contracts.action.cancel")}
            </Button>
            <Button variant="destructive" onClick={() => void decline()} loading={busy} disabled={!reason.trim()}>
              {t("contracts.action.decline")}
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  );
}
