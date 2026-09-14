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
import { Badge, Button, type BadgeProps, type ButtonProps } from "@gerege-systems/ui";
import type { CeremonyProgress, CeremonySession, ContractState, PartyState } from "@/lib/contracts";
import { formatDay, formatMoment, formatMoney } from "@/lib/datetime";

// ─────────────────────────────────────────────────────────────────── labels

type Tone = NonNullable<BadgeProps["tone"]>;

const CONTRACT_TONE: Record<ContractState, Tone> = {
  NONE: "neutral",
  DRAFT: "neutral",
  SENT: "warning",
  PARTIALLY_SIGNED: "accent",
  EXECUTED: "success",
  DECLINED: "danger",
  WITHDRAWN: "neutral",
  EXPIRED: "neutral",
  TERMINATED: "neutral",
};

const PARTY_TONE: Record<PartyState, Tone> = {
  draft: "neutral",
  invited: "warning",
  viewed: "accent",
  signed: "success",
  declined: "danger",
  withdrawn: "neutral",
  expired: "neutral",
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
    <Badge tone={CONTRACT_TONE[state] ?? "neutral"} className="whitespace-nowrap">
      {contractState(state)}
    </Badge>
  );
}

export function PartyBadge({ state }: { state: PartyState }) {
  const { partyState } = useContractLabels();
  return (
    <Badge tone={PARTY_TONE[state] ?? "neutral"} className="whitespace-nowrap">
      {partyState(state)}
    </Badge>
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
  variant = "primary",
  size = "sm",
  className,
}: {
  label: string;
  start: () => Promise<CeremonySession>;
  /** Receives the session start returned — the master-sign poll needs its id. */
  poll: (session: CeremonySession) => Promise<CeremonyProgress>;
  onDone: () => void | Promise<void>;
  onError: (message: string) => void;
  variant?: ButtonProps["variant"];
  size?: ButtonProps["size"];
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
      let failures = 0;
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
          // Сервер огт хариулахаа больсон бол мөнхөд эргэлдэхгүй: 10 удаа
          // дараалан унавал (~30 сек) зогсоож хэлнэ — утсан дээрх ёслол
          // нээлттэй хэвээр, дахин дарахад асуулт үргэлжилнэ.
          failures += 1;
          if (failures >= 10) {
            onError(t("contracts.msg.poll_lost"));
            break;
          }
          continue;
        }
        failures = 0;
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
      <Badge tone="accent" variant="outline" icon={<Smartphone className="animate-pulse" />} className="font-mono py-1.5">
        {code}
        <span className="font-sans font-medium">{t("contracts.msg.check_phone")}</span>
      </Badge>
    );
  }
  return (
    <Button variant={variant} size={size} onClick={() => void run()} leadingIcon={<Smartphone />} className={className}>
      {label}
    </Button>
  );
}
