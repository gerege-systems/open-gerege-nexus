"use client";

import React, { useCallback } from "react";
import { EsignApiError, type BatchItemStatus, type BatchStatus, type LogOutcome } from "@/lib/esign";
import { useI18n } from "@/lib/i18n";

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

export function Card({ title, children, actions }: { title?: string; children: React.ReactNode; actions?: React.ReactNode }) {
  return (
    <section className="bg-surface border border-line rounded-xl shadow-sm overflow-hidden">
      {(title || actions) && (
        <header className="px-4 py-3 border-b border-line flex items-center justify-between gap-3">
          {title && <h2 className="text-sm font-semibold text-foreground">{title}</h2>}
          {actions}
        </header>
      )}
      {children}
    </section>
  );
}


const BATCH_STYLE: Record<BatchStatus, string> = {
  DRAFT: "bg-surface-2 text-muted border-line",
  RUNNING: "bg-blue-50 text-blue-700 border-blue-200",
  COMPLETED: "bg-emerald-50 text-emerald-700 border-emerald-200",
  FAILED: "bg-red-50 text-red-700 border-red-200",
  CANCELLED: "bg-surface-2 text-muted border-line",
};

const ITEM_STYLE: Record<BatchItemStatus, string> = {
  PENDING: "bg-surface-2 text-muted border-line",
  RUNNING: "bg-blue-50 text-blue-700 border-blue-200",
  SIGNED: "bg-emerald-50 text-emerald-700 border-emerald-200",
  FAILED: "bg-red-50 text-red-700 border-red-200",
  SKIPPED: "bg-surface-2 text-muted border-line",
};

const OUTCOME_STYLE: Record<LogOutcome, string> = {
  OK: "bg-emerald-50 text-emerald-700 border-emerald-200",
  FAILED: "bg-red-50 text-red-700 border-red-200",
  REJECTED: "bg-rose-50 text-rose-700 border-rose-200",
  EXPIRED: "bg-surface-2 text-muted border-line",
  CANCELLED: "bg-surface-2 text-muted border-line",
  UNVERIFIED: "bg-amber-50 text-amber-700 border-amber-200",
};

export function Badge({ tone, children }: { tone: string; children: React.ReactNode }) {
  return (
    <span className={`inline-flex items-center gap-1 text-[11px] font-semibold px-2.5 py-0.5 rounded-full border ${tone}`}>
      {children}
    </span>
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
  return <Badge tone={BATCH_STYLE[status] ?? BATCH_STYLE.DRAFT}>{label(status)}</Badge>;
}

export function ItemBadge({ status }: { status: BatchItemStatus }) {
  const label = useEnumLabel("esign.item");
  return <Badge tone={ITEM_STYLE[status] ?? ITEM_STYLE.PENDING}>{label(status)}</Badge>;
}

export function OutcomeBadge({ outcome }: { outcome: LogOutcome }) {
  const label = useEnumLabel("esign.outcome");
  return <Badge tone={OUTCOME_STYLE[outcome] ?? OUTCOME_STYLE.OK}>{label(outcome)}</Badge>;
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
  const { t } = useI18n();
  if (total <= pageSize) return null;

  const page = Math.floor(offset / pageSize) + 1;
  const pages = Math.ceil(total / pageSize);

  return (
    <nav className="flex items-center justify-between text-xs text-muted">
      <span>{t("base.message.page_summary", { page, pages, total })}</span>
      <div className="flex gap-2">
        <button
          onClick={() => onPage(Math.max(0, offset - pageSize))}
          disabled={offset === 0}
          className="px-3 py-1.5 rounded-lg bg-surface-2 hover:bg-slate-200 disabled:opacity-40 font-medium"
        >
          {t("base.action.previous")}
        </button>
        <button
          onClick={() => onPage(offset + pageSize)}
          disabled={offset + pageSize >= total}
          className="px-3 py-1.5 rounded-lg bg-surface-2 hover:bg-slate-200 disabled:opacity-40 font-medium"
        >
          {t("base.action.next")}
        </button>
      </div>
    </nav>
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
