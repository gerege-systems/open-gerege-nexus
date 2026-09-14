"use client";

import React, { useCallback } from "react";
import { EsignApiError, type BatchItemStatus, type BatchStatus, type LogOutcome } from "@/lib/esign";
import { useI18n } from "@/lib/i18n";
import {
  Badge as UIBadge,
  Card as UICard,
  CardHeader,
  CardTitle,
  Pagination,
  type BadgeProps,
} from "@gerege-systems/ui";

/**
 * Turns any thrown value into a message, without the machine code.
 *
 * Only useErrorMessage calls this: the API answers in English, so this raw form
 * puts an English sentence in a Mongolian interface at exactly the moment
 * something has gone wrong. It is the fallback for a code with no translation
 * yet, not something a screen should reach for.
 */
function describeError(err: unknown, fallback: string): string {
  if (err instanceof EsignApiError) return err.message;
  if (err instanceof Error && err.message) return err.message;
  return fallback;
}

/** The backend's machine code for a thrown value, if it carried one. */
export function errorCode(err: unknown): string | null {
  return err instanceof EsignApiError && err.code !== "UNKNOWN" ? err.code : null;
}

/**
 * Translates a backend failure through the dictionary, keyed by its machine
 * code. An untranslated code falls back to the server's own message, so a new
 * code added on the server still says something useful.
 *
 * Memoised against the locale, and that is not a micro-optimisation: every
 * screen puts this function in the dependency list of the useCallback that
 * loads it. A new identity per render would make that load re-run on every
 * render, so the screens had been leaving it out of the list instead — which is
 * the same bug held one step further away, because a language switched
 * mid-session would then leave the old locale's message on screen.
 */
export function useErrorMessage() {
  const { t } = useI18n();
  return useCallback(
    (err: unknown, fallback?: string): string => {
      const code = errorCode(err);
      if (code) {
        const key = `esign.error.${code}`;
        const translated = t(key as never);
        if (translated !== key) return translated;
      }
      return describeError(err, fallback ?? t("base.message.error"));
    },
    [t],
  );
}

/**
 * A titled section on its own card. The body is the caller's — a table wants
 * no padding, a form brings its own — so the card itself has none.
 */
export function Card({ title, children, actions }: { title?: string; children: React.ReactNode; actions?: React.ReactNode }) {
  return (
    <UICard padding="none" className="overflow-hidden">
      {(title || actions) && (
        <CardHeader className="flex-row items-center justify-between gap-3 px-4 py-3 border-b border-line">
          {title && <CardTitle className="text-sm">{title}</CardTitle>}
          {actions}
        </CardHeader>
      )}
      {children}
    </UICard>
  );
}

export type Tone = NonNullable<BadgeProps["tone"]>;

const BATCH_TONE: Record<BatchStatus, Tone> = {
  DRAFT: "neutral",
  RUNNING: "info",
  COMPLETED: "success",
  FAILED: "danger",
  CANCELLED: "neutral",
};

const ITEM_TONE: Record<BatchItemStatus, Tone> = {
  PENDING: "neutral",
  RUNNING: "info",
  SIGNED: "success",
  FAILED: "danger",
  SKIPPED: "neutral",
};

const OUTCOME_TONE: Record<LogOutcome, Tone> = {
  OK: "success",
  FAILED: "danger",
  REJECTED: "danger",
  EXPIRED: "neutral",
  CANCELLED: "neutral",
  UNVERIFIED: "warning",
};

export function Badge({ tone, icon, children }: { tone: Tone; icon?: React.ReactNode; children: React.ReactNode }) {
  return (
    <UIBadge tone={tone} icon={icon}>
      {children}
    </UIBadge>
  );
}

/**
 * Translates a server enum through the dictionary, falling back to the raw
 * value. A rail added on the server before the dictionary catches up should
 * still render as something readable rather than a missing-key placeholder.
 */
function useEnumLabel(prefix: string) {
  const { t } = useI18n();
  return (value: string) => {
    const key = `${prefix}.${value.toLowerCase()}`;
    const label = t(key as never);
    return label === key ? value : label;
  };
}

export function BatchBadge({ status }: { status: BatchStatus }) {
  const label = useEnumLabel("esign.batch");
  return <Badge tone={BATCH_TONE[status] ?? BATCH_TONE.DRAFT}>{label(status)}</Badge>;
}

export function ItemBadge({ status }: { status: BatchItemStatus }) {
  const label = useEnumLabel("esign.item");
  return <Badge tone={ITEM_TONE[status] ?? ITEM_TONE.PENDING}>{label(status)}</Badge>;
}

export function OutcomeBadge({ outcome }: { outcome: LogOutcome }) {
  const label = useEnumLabel("esign.outcome");
  return <Badge tone={OUTCOME_TONE[outcome] ?? OUTCOME_TONE.OK}>{label(outcome)}</Badge>;
}

/**
 * Page controls for a listing. It renders nothing when everything fits on one
 * page, so a short log does not carry dead chrome.
 */
export function Pager({
  total,
  offset,
  pageSize,
  onPage,
}: {
  total: number;
  offset: number;
  pageSize: number;
  onPage: (offset: number) => void;
}) {
  if (total <= pageSize) return null;

  const page = Math.floor(offset / pageSize) + 1;
  const pages = Math.ceil(total / pageSize);

  return (
    <Pagination
      page={page}
      pageCount={pages}
      totalItems={total}
      pageSize={pageSize}
      onPageChange={(next) => onPage((next - 1) * pageSize)}
    />
  );
}

/** Renders a byte count the way a person reads a file size. */
export function formatBytes(bytes: number): string {
  if (!bytes) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/** A short, monospaced fingerprint. The full SHA-256 is unreadable in a table. */
