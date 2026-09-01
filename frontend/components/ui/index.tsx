"use client";

import React from "react";
import { AlertTriangle, CheckCircle2, Loader2, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";

/**
 * The chrome every module screen is built from.
 *
 * Documents, E-Sign and Gov Services each grew their own copy of these four:
 * three PageHeaders that were identical character for character, three Banners
 * that differed only in padding and the shade of red, and two EmptyStates that
 * disagreed about whether the text is centred. Three copies of a header is not
 * three decisions — it is one decision recorded three times, and the screens had
 * already started to drift apart on it.
 *
 * Anything domain-specific stays in the module's own shared.tsx. What lands here
 * is what a screen needs regardless of which app it belongs to.
 */

export function PageHeader({
  icon,
  title,
  subtitle,
  actions,
}: {
  icon: React.ReactNode;
  title: string;
  subtitle: string;
  actions?: React.ReactNode;
}) {
  return (
    <header className="flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
          {icon}
          {title}
        </h1>
        <p className="text-sm text-muted mt-1">{subtitle}</p>
      </div>
      {actions}
    </header>
  );
}

export type BannerTone = "error" | "success" | "info" | "warning";

const BANNER_STYLE: Record<BannerTone, string> = {
  error: "bg-red-50 border-red-200 text-red-700",
  success: "bg-emerald-50 border-emerald-200 text-emerald-700",
  info: "bg-blue-50 border-blue-200 text-blue-700",
  // Not an outcome but a standing condition the screen cannot fix: no service
  // key, no encryption key. The settings screens had this in amber already.
  warning: "bg-amber-50 border-amber-200 text-amber-800",
};

/**
 * What a screen says after an action, good or bad.
 *
 * onDismiss is optional because the two cases are genuinely different: a banner
 * reporting the outcome of something the operator pressed should be dismissable,
 * and one stating a standing condition of the screen — mock mode, a signature
 * placed off the page — should not be, because dismissing it would not make it
 * untrue.
 */
export function Banner({
  tone,
  message,
  onDismiss,
}: {
  tone: BannerTone;
  message: string;
  onDismiss?: () => void;
}) {
  const { t } = useI18n();
  return (
    // A role so a screen reader announces the outcome of an action the user
    // cannot see the result of. The settings screens already did this; the rest
    // did not, and the answer to that disagreement is the accessible one.
    //
    // Failures get "alert" rather than "status" because the two are not the
    // same urgency: "status" is polite, so it waits for whatever the region is
    // already saying to finish. A failed create on /documents renders its
    // banner just as the loading text changes, and queued behind that the
    // operator hears nothing and believes the document was created.
    <div
      role={tone === "error" ? "alert" : "status"}
      className={`p-3 border text-sm rounded-lg flex items-start gap-2 ${BANNER_STYLE[tone]}`}
    >
      {tone === "error" || tone === "warning" ? (
        <AlertTriangle className="w-4 h-4 mt-0.5 shrink-0" />
      ) : (
        <CheckCircle2 className="w-4 h-4 mt-0.5 shrink-0" />
      )}
      <span className="flex-1">{message}</span>
      {onDismiss && (
        // type="button" because the default is "submit": every call site today
        // happens to sit outside a <form>, and the first one that does not
        // would otherwise submit the form on its way to dismissing the banner.
        <button type="button" onClick={onDismiss} aria-label={t("base.action.close")}>
          <X className="w-4 h-4" />
        </button>
      )}
    </div>
  );
}

export function Loading({ label }: { label?: string }) {
  const { t } = useI18n();
  return (
    <div className="flex items-center gap-2 text-muted text-sm" role="status">
      <Loader2 className="w-4 h-4 animate-spin" aria-hidden="true" />
      {label || t("base.message.loading")}
    </div>
  );
}

/**
 * Whether a wait has gone on long enough to be worth drawing, and — once it
 * has been drawn — long enough not to flash.
 *
 * Two numbers, both about the same thing: a placeholder that appears and
 * vanishes inside a fifth of a second is visual noise, not feedback. So
 * nothing is shown for the first 300ms, and once something *is* shown it stays
 * for at least 500ms even if the data has already arrived. A fast response
 * therefore renders straight to content, and a slow one does not blink on its
 * way there.
 */
export function useSettledWait(waiting: boolean): boolean {
  const [showing, setShowing] = React.useState(false);
  const shownAt = React.useRef(0);

  React.useEffect(() => {
    if (waiting) {
      const timer = setTimeout(() => {
        shownAt.current = Date.now();
        setShowing(true);
      }, 300);
      return () => clearTimeout(timer);
    }
    if (!showing) return;
    const remaining = 500 - (Date.now() - shownAt.current);
    if (remaining <= 0) {
      setShowing(false);
      return;
    }
    const timer = setTimeout(() => setShowing(false), remaining);
    return () => clearTimeout(timer);
  }, [waiting, showing]);

  return showing;
}

/**
 * A grey bar standing in for a line of content that has not arrived.
 *
 * It takes the shape of what it replaces rather than being a spinner in the
 * middle of the page, so nothing moves when the real thing lands.
 */
export function Skeleton({ className }: { className?: string }) {
  return (
    <span
      aria-hidden="true"
      className={`block animate-pulse rounded bg-surface-2${className ? ` ${className}` : ""}`}
    />
  );
}

/**
 * What a screen has to say when there is nothing to show.
 *
 * The two cases are not one case. A list that has never had anything in it is
 * the first thing a new operator sees, and it has a job: say what would go
 * here and offer the button that puts something there. A list that is empty
 * because of a filter has the opposite job — the records exist, the reader
 * hid them, and what they need is the way back. Showing the first screen's
 * copy in the second situation tells somebody to create a record they already
 * have.
 *
 * `message` alone still works and reads as it did, so the screens that have
 * not been given the fuller treatment are unaffected.
 */
export function EmptyState({
  message,
  title,
  action,
  filtered,
}: {
  message: string;
  /** A heading above the sentence. Worth it for a first-run screen. */
  title?: string;
  /** The way forward: "New…" when first-run, "Clear filters" when filtered. */
  action?: React.ReactNode;
  /** Nothing matched a filter, as opposed to nothing existing at all. */
  filtered?: boolean;
}) {
  if (!title && !action) {
    // The original shape, minus the italics: italic Cyrillic is harder to read
    // than upright, and the sentence was already muted and centred.
    return <p className={`text-sm text-muted text-center ${filtered ? "py-10" : "p-6"}`}>{message}</p>;
  }
  return (
    <div className={`flex flex-col items-center gap-2 text-center ${filtered ? "py-10" : "py-12"}`}>
      {title && <p className="text-sm font-semibold text-foreground">{title}</p>}
      <p className="max-w-prose text-sm text-muted">{message}</p>
      {action && <div className="mt-2">{action}</div>}
    </div>
  );
}

/**
 * The shapes below are style, not structure, so they are exported as class
 * strings rather than wrapped in components. A form field is an <input> on one
 * screen, a <select> on the next and a <textarea> on a third, all of them
 * carrying their own props; a component that took all of those would be a
 * worse <input>. What was actually duplicated is the look, and a name for the
 * look is enough to stop it drifting.
 */

/** Text, number, select and textarea inputs. Repeated 33 times before this. */
export const fieldClass =
  "w-full px-3 py-2 text-sm border border-input rounded-lg";

/**
 * A <select> wearing fieldClass alone renders SHORTER than the input beside
 * it: the vertical padding that gives an input its height is not honoured by
 * the native control on every browser, macOS Safari most visibly. The height
 * an input reaches implicitly — text-sm line + py-2 + borders — is stated
 * explicitly here, so a select in a form row sits flush with its neighbours.
 */
export const selectClass = `${fieldClass} h-[38px] bg-surface`;

/** The white panel a section sits on. */
export const cardClass = "bg-surface rounded-xl border border-line";

/** The header row of a listing table. */
export const tableHeadClass =
  "bg-surface-2 text-foreground font-semibold border-b border-line uppercase";

/** The small bordered button in a table row: Save, Use template, and friends. */
export const rowActionClass =
  "inline-flex items-center space-x-1 px-2.5 py-1.5 rounded-lg text-[11px] font-semibold " +
  "border border-indigo-200 text-indigo-600 hover:bg-indigo-50 disabled:opacity-50";

const FOCUSABLE = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  '[tabindex]:not([tabindex="-1"])',
].join(",");

/** The tabbable elements of an open dialog, in tab order, skipping hidden ones. */
function focusableWithin(panel: HTMLElement | null): HTMLElement[] {
  if (!panel) return [];
  return Array.from(panel.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
    (el) => el.offsetParent !== null || el === document.activeElement,
  );
}

/**
 * A centred dialog over a dimmed page.
 *
 * Escape closes it, and so does a click on the backdrop — but the backdrop only
 * when nothing has been typed into it. The earlier version dismissed on
 * neither, on the reasoning that several of these dialogs hold typed input and
 * one of them is a signing conversation with a citizen's device, so losing
 * either to a stray click is worse than reaching for Cancel. That reasoning is
 * right about the click and wrong about the key: Escape is not a stray gesture,
 * it is the one every dialog on the web answers to, and a keyboard operator who
 * presses it and stays trapped has no reason to think the dialog is closable at
 * all. So Escape always closes, and the click is held back exactly in the case
 * the original was protecting — see `holdsTypedInput`.
 *
 * Both need somewhere to go: a dialog without `onClose` keeps the old
 * behaviour, because there is nothing to call.
 *
 * Holding focus is a different question, and the answer to that one is yes.
 * The twelve originals left focus on the button that opened them, so Tab
 * walked the page behind the dimmed backdrop: with the signing dialog open on
 * /documents a keyboard operator could tab to a *different* row and press its
 * Reject. Nothing about that was intended — it is what you get when a dialog
 * is a dialog only visually — so this one traps Tab, takes focus on open, and
 * hands it back to the opener on close.
 */
export function Modal({
  size = "md",
  scrollable = false,
  className,
  label,
  onClose,
  children,
}: {
  size?: "md" | "lg";
  /**
   * Let the backdrop scroll instead of the panel. The signing dialog needs it:
   * it grows with the number of steps, and a panel taller than the window
   * otherwise puts its own buttons past the bottom edge with no way to reach
   * them. Pair it with a vertical margin on the panel, or the top of a tall
   * dialog is clipped by the centring.
   */
  scrollable?: boolean;
  /** Extra panel classes. Height and scrolling genuinely differ per dialog. */
  className?: string;
  /**
   * What the dialog is called, announced on open. Pass the same string the
   * dialog's own heading shows — a role of "dialog" with no name announces as
   * just "dialog", which tells the operator less than the heading they cannot
   * see yet.
   */
  label?: string;
  /**
   * Dismiss the dialog. Without it Escape and the backdrop do nothing, which
   * is what every dialog here did before this prop existed.
   */
  onClose?: () => void;
  children: React.ReactNode;
}) {
  const panelRef = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    const panel = panelRef.current;
    // Test against the panel rather than focusing it unconditionally, because
    // by the time this runs something inside may already hold focus: React
    // does not render autoFocus as an attribute, it calls focus() during
    // commit, which is before any effect here. A caller that asks for a field
    // gets that field; the rest get the panel.
    if (panel?.contains(document.activeElement)) return;
    // The panel and not its first field: focusing an input would skip past the
    // heading a screen reader announces on entry, and these dialogs open on a
    // title, not on a cursor.
    const opener = document.activeElement as HTMLElement | null;
    panel?.focus();
    // Back to whatever opened the dialog, so closing one from a table row does
    // not drop the operator at the top of the page. A no-op if that row has
    // since been re-rendered away.
    return () => opener?.focus?.();
  }, []);

  function handleKeyDown(event: React.KeyboardEvent<HTMLDivElement>) {
    if (event.key === "Escape" && onClose) {
      // stopPropagation so a dialog opened from inside another overlay does not
      // close both on one press.
      event.stopPropagation();
      onClose();
      return;
    }
    if (event.key !== "Tab") return;
    const items = focusableWithin(panelRef.current);
    if (items.length === 0) {
      // Nothing to move to, so the only correct move is to stay put rather
      // than let the browser send focus out to the page behind.
      event.preventDefault();
      return;
    }
    const first = items[0];
    const last = items[items.length - 1];
    const active = document.activeElement;
    if (event.shiftKey && (active === first || active === panelRef.current)) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && active === last) {
      event.preventDefault();
      first.focus();
    }
  }

  /**
   * Whether anything inside has been typed into since the dialog opened.
   *
   * The comparison is against `defaultValue`, so a field the dialog prefilled
   * does not count and a field the operator edited does. Checkboxes and radios
   * are compared the same way through `defaultChecked`. It is what decides
   * whether a click on the backdrop throws work away or is ignored.
   */
  function holdsTypedInput() {
    const panel = panelRef.current;
    if (!panel) return false;
    const fields = panel.querySelectorAll<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>(
      "input, textarea, select",
    );
    for (const field of fields) {
      if (field instanceof HTMLInputElement && (field.type === "checkbox" || field.type === "radio")) {
        if (field.checked !== field.defaultChecked) return true;
      } else if (field instanceof HTMLSelectElement) {
        // A <select> has no defaultValue; the option marked selected in the
        // markup is the one it opened on.
        const opened = Array.from(field.options).find((option) => option.defaultSelected);
        if (field.value !== (opened?.value ?? field.options[0]?.value ?? "")) return true;
      } else if (field.value !== field.defaultValue) {
        return true;
      }
    }
    return false;
  }

  function handleBackdrop(event: React.MouseEvent<HTMLDivElement>) {
    if (!onClose) return;
    // Only the backdrop itself — a click that started inside the panel and
    // ended on it (a drag out of a text selection) must not close anything.
    if (event.target !== event.currentTarget) return;
    if (holdsTypedInput()) return;
    onClose();
  }

  return (
    <div
      onMouseDown={handleBackdrop}
      className={`fixed inset-0 bg-overlay flex items-center justify-center z-modal p-4${
        scrollable ? " overflow-y-auto" : ""
      }`}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-label={label}
        tabIndex={-1}
        onKeyDown={handleKeyDown}
        className={`bg-surface rounded-xl ${size === "lg" ? "max-w-lg" : "max-w-md"} w-full p-6 shadow-lg border border-line${
          className ? ` ${className}` : ""
        }`}
      >
        {children}
      </div>
    </div>
  );
}

/**
 * What a listing shows while its first load is outstanding.
 *
 * Rows of the right shape rather than the word "Loading…" centred in an empty
 * box: the height is the height the list will have, so the page does not jump
 * when the rows arrive. It is held back for 300ms and, once drawn, held for
 * 500 — see `useSettledWait` — so a fast reply renders straight to content.
 *
 * The label is still announced, because a skeleton says nothing to a screen
 * reader.
 */
export function LoadingBlock({ label, rows = 4 }: { label?: string; rows?: number }) {
  const { t } = useI18n();
  const showing = useSettledWait(true);
  if (!showing) return null;
  return (
    <div className="py-4" role="status" aria-live="polite" aria-busy="true">
      <span className="sr-only">{label || t("base.message.loading")}</span>
      <div className="space-y-3">
        {Array.from({ length: rows }, (_, row) => (
          <div key={row} className="flex items-center gap-3">
            <Skeleton className="h-4 w-4 shrink-0 rounded-full" />
            <Skeleton className="h-4 flex-1" />
            <Skeleton className="h-4 w-24 shrink-0" />
          </div>
        ))}
      </div>
    </div>
  );
}

/** A listing table on its panel, with the header row the caller supplies. */
export function TableCard({
  head,
  footer,
  children,
}: {
  head: React.ReactNode;
  /**
   * Rendered inside the panel, below the table. This is where the lists put
   * "these rows are stale" and their Load more button — both of them statements
   * about the table, so they belong on the same piece of paper as it.
   */
  footer?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className={`${cardClass} overflow-hidden`}>
      <table className="w-full text-left text-xs text-muted">
        <thead className={tableHeadClass}>{head}</thead>
        <tbody className="divide-y divide-line">{children}</tbody>
      </table>
      {footer}
    </div>
  );
}
