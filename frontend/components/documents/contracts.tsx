"use client";

/**
 * The pieces the two contract screens share: state labels, badges, and the
 * PIN2 ceremony button.
 *
 * The ceremony has one rule: KEEP ASKING. The signature is recorded inside the
 * poll handler on the server, so if nobody polls, a citizen's approved
 * signature is never written. Poll every 3 seconds until a terminal state.
 */

import React, { useEffect, useRef, useState } from "react";
import { useI18n } from "@/lib/i18n";
import { Smartphone } from "lucide-react";
import type { CeremonyProgress, CeremonySession, ContractState, PartyState } from "@/lib/contracts";
import { formatDay, formatMoment, formatMoney } from "@/lib/datetime";

// ─────────────────────────────────────────────────────────────────── labels

const CONTRACT_BADGE: Record<ContractState, string> = {
  NONE: "bg-surface-2 text-muted",
  DRAFT: "bg-surface-2 text-muted",
  SENT: "bg-amber-50 text-amber-700",
  PARTIALLY_SIGNED: "bg-indigo-50 text-indigo-700",
  EXECUTED: "bg-emerald-50 text-emerald-700",
  DECLINED: "bg-red-50 text-red-700",
  WITHDRAWN: "bg-surface-2 text-muted",
  EXPIRED: "bg-surface-2 text-muted",
  TERMINATED: "bg-surface-2 text-muted",
};

const PARTY_BADGE: Record<PartyState, string> = {
  draft: "bg-surface-2 text-muted",
  invited: "bg-amber-50 text-amber-700",
  viewed: "bg-indigo-50 text-indigo-700",
  signed: "bg-emerald-50 text-emerald-700",
  declined: "bg-red-50 text-red-700",
  withdrawn: "bg-surface-2 text-muted",
  expired: "bg-surface-2 text-muted",
};

export function useContractLabels() {
  const { t } = useI18n();
  const contractState = (state: ContractState): string => {
    switch (state) {
      case "DRAFT": return t("contracts.state.draft");
      case "SENT": return t("contracts.state.sent");
      case "PARTIALLY_SIGNED": return t("contracts.state.partial");
      case "EXECUTED": return t("contracts.state.executed");
      case "DECLINED": return t("contracts.state.declined");
      case "WITHDRAWN": return t("contracts.state.withdrawn");
      case "EXPIRED": return t("contracts.state.expired");
      case "TERMINATED": return t("contracts.state.terminated");
      default: return "—";
    }
  };
  const partyState = (state: PartyState): string => {
    switch (state) {
      case "draft": return t("contracts.party_state.draft");
      case "invited": return t("contracts.party_state.invited");
      case "viewed": return t("contracts.party_state.viewed");
      case "signed": return t("contracts.party_state.signed");
      case "declined": return t("contracts.party_state.declined");
      case "withdrawn": return t("contracts.state.withdrawn");
      case "expired": return t("contracts.state.expired");
      default: return state;
    }
  };
  const partyRole = (role: string): string => {
    switch (role) {
      case "issuer": return t("contracts.role.issuer");
      case "counterparty": return t("contracts.role.counterparty");
      case "witness": return t("contracts.role.witness");
      case "guarantor": return t("contracts.role.guarantor");
      default: return role;
    }
  };
  const partyKind = (kind: string): string => {
    switch (kind) {
      case "member": return t("contracts.kind.member");
      case "tenant": return t("contracts.kind.tenant");
      case "person": return t("contracts.kind.person");
      case "organisation": return t("contracts.kind.organisation");
      default: return kind;
    }
  };
  return { contractState, partyState, partyRole, partyKind };
}

export function ContractBadge({ state }: { state: ContractState }) {
  const { contractState } = useContractLabels();
  return (
    <span className={`inline-block px-2.5 py-0.5 rounded-full text-[11px] font-semibold whitespace-nowrap ${CONTRACT_BADGE[state] ?? "bg-surface-2 text-muted"}`}>
      {contractState(state)}
    </span>
  );
}

export function PartyBadge({ state }: { state: PartyState }) {
  const { partyState } = useContractLabels();
  return (
    <span className={`inline-block px-2.5 py-0.5 rounded-full text-[11px] font-semibold whitespace-nowrap ${PARTY_BADGE[state] ?? "bg-surface-2 text-muted"}`}>
      {partyState(state)}
    </span>
  );
}

export function fmtDate(value?: string | null): string {
  if (!value) return "";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? value : formatDay(d);
}

export function fmtWhen(value?: string | null): string {
  if (!value) return "";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? value : formatMoment(d);
}

export function fmtMoney(amount?: number | null, currency?: string): string {
  if (amount === null || amount === undefined) return "";
  return formatMoney(amount, currency || "MNT");
}

// ───────────────────────────────────────────────────────────────── ceremony

export function CeremonyButton({
  label,
  start,
  poll,
  onDone,
  onError,
  className,
}: {
  label: string;
  start: () => Promise<CeremonySession>;
  /** Receives the session start returned — the master-sign poll needs its id. */
  poll: (session: CeremonySession) => Promise<CeremonyProgress>;
  onDone: () => void | Promise<void>;
  onError: (message: string) => void;
  className?: string;
}) {
  const { t } = useI18n();
  const [code, setCode] = useState<string | null>(null);
  const cancelled = useRef(false);
  useEffect(() => () => { cancelled.current = true; }, []);

  const run = async () => {
    try {
      const session = await start();
      setCode(session.verification_code || "····");
      while (!cancelled.current) {
        await new Promise((resolve) => setTimeout(resolve, 3000));
        let progress: CeremonyProgress;
        try {
          progress = await poll(session);
        } catch (err) {
          // A 409 is an answer (settled elsewhere, bytes changed); a dropped
          // connection is not — the ceremony is still open on the phone.
          const status = (err as { status?: number }).status;
          if (status && status >= 400 && status < 500) {
            onError(err instanceof Error ? err.message : String(err));
            break;
          }
          continue;
        }
        if (progress.state === "COMPLETE") { await onDone(); break; }
        // Хоёр рельс хоёр өөр үгээр «хүлээж байна» гэдэг: талын зам PENDING,
        // мастерын зам RUNNING. Аль аль нь — асуусаар байх.
        if (progress.state === "PENDING" || progress.state === "RUNNING") continue;
        onError(progress.state === "REFUSED" ? t("contracts.msg.refused") : t("contracts.msg.ceremony_ended", { state: progress.state }));
        break;
      }
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      if (!cancelled.current) setCode(null);
    }
  };

  if (code) {
    return (
      <span className="bg-indigo-50 text-indigo-700 border border-indigo-200 text-xs font-semibold px-3 py-1.5 rounded-lg inline-flex items-center gap-1.5 font-mono">
        <Smartphone className="w-3.5 h-3.5 animate-pulse" />
        {code}
        <span className="font-sans font-medium text-indigo-500">{t("contracts.msg.check_phone")}</span>
      </span>
    );
  }
  return (
    <button
      onClick={() => void run()}
      className={className ?? "bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-semibold px-3 py-1.5 rounded-lg inline-flex items-center gap-1.5"}
    >
      <Smartphone className="w-3.5 h-3.5" />
      {label}
    </button>
  );
}
