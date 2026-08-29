"use client";

import { useCallback, useEffect, useState } from "react";
import { Building2, Check, House } from "lucide-react";
import { api } from "@/lib/api";
import { resetAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { isHome } from "@/lib/workspaceKind.mjs";

export interface TenantOption {
  id: string;
  name: string;
  slug: string;
  /** "organisation" or "personal" — see lib/workspaceKind.mjs. */
  kind?: string;
}

/**
 * The answer is cached the way lib/access caches the identity: two controls now
 * offer this list — the brand mark in the header and the account menu, which is
 * the only one of the two the mobile shell shows — and opening one after the
 * other should not ask the server the same question twice.
 */
let pending: Promise<{ tenants: TenantOption[]; active: string[] }> | null = null;

/** Forget the cached list. Sign-out and a completed switch both invalidate it. */
export function forgetTenants() {
  pending = null;
}

/**
 * The tenants the signed-in person may act for, fetched the first time a
 * control that offers them is opened rather than with the shell: most people
 * hold one membership and will never open either.
 */
export function useTenants(active: boolean) {
  const [tenants, setTenants] = useState<TenantOption[] | null>(null);
  const [activeIDs, setActiveIDs] = useState<string[]>([]);
  const [switching, setSwitching] = useState(false);
  // What went wrong, not merely that something did. A switch can be refused
  // for a reason the person can act on — the organisation is in Maintenance,
  // they are no longer a member — and "try again" is the wrong advice for both.
  const [failed, setFailed] = useState("");

  useEffect(() => {
    if (!active) return;
    let alive = true;
    // Never cache the failure: a rejected promise left in place would answer
    // every later opening with the error that belonged to one moment.
    const request = (pending ??= api
      .getTenants()
      .then((answer) => ({ tenants: answer.tenants || [], active: answer.active || [] }))
      .catch((error) => {
        pending = null;
        throw error;
      }));
    request
      .then((value) => {
        if (!alive) return;
        setTenants(value.tenants);
        setActiveIDs(value.active);
      })
      .catch(() => alive && setTenants([]));
    return () => {
      alive = false;
    };
  }, [active]);

  const switchTo = useCallback(
    async (id: string) => {
      setSwitching(true);
      setFailed("");
      try {
        await api.switchTenant(id);
        // Everything on screen was fetched for the workspace being left — the
        // menus, the permissions, every list on the page behind this control. A
        // full load is the only honest way to drop all of it at once.
        //
        // Where it lands used to be /apps unconditionally, with a comment
        // saying it is "somewhere every tenant has". Migration 00085 made that
        // false: a home has no app store, so switching into one landed on a
        // screen its own sidebar no longer offers. The profile is what is
        // actually true of every workspace, and in a home it is the point.
        resetAccess();
        forgetTenants();
        window.location.assign(isHome(tenants?.find((option) => option.id === id)) ? "/profile" : "/apps");
      } catch (error) {
        setSwitching(false);
        setFailed(error instanceof Error && error.message ? error.message : "unknown");
      }
    },
    [tenants],
  );

  // Reading alongside, rather than switching to. The reload is the same
  // reasoning as a switch: every list on the screen behind this control was
  // fetched for the old set, and narrowing the set without dropping them would
  // leave rows on screen that the next request will not return.
  const toggleActive = useCallback(
    async (id: string, current: string) => {
      setSwitching(true);
      setFailed("");
      const next = activeIDs.includes(id) ? activeIDs.filter((x) => x !== id) : [...activeIDs, id];
      try {
        await api.setActiveTenants(next.includes(current) ? next : [...next, current]);
        resetAccess();
        forgetTenants();
        window.location.reload();
      } catch (error) {
        setSwitching(false);
        setFailed(error instanceof Error && error.message ? error.message : "unknown");
      }
    },
    [activeIDs],
  );

  return { tenants, activeIDs, switching, failed, switchTo, toggleActive };
}

/**
 * The rows themselves, so the header control and the account menu offer the
 * same list rather than two that drift. Each host supplies its own heading and
 * surrounding chrome.
 */
export function TenantChoices({
  current,
  tenants,
  activeIDs,
  switching,
  failed,
  onChoose,
  onStay,
  onToggleActive,
}: {
  current?: string;
  tenants: TenantOption[] | null;
  activeIDs?: string[];
  switching: boolean;
  failed: string;
  onChoose: (id: string) => void;
  /** Called when the current tenant is picked — nothing to switch, so the host
   *  just closes. */
  onStay: () => void;
  /** Read alongside the current one, rather than moving to it. */
  onToggleActive?: (id: string) => void;
}) {
  const { t } = useI18n();

  return (
    <>
      {tenants === null && <p className="px-4 py-2 text-sm text-slate-500">{t("base.message.loading")}</p>}
      {tenants?.map((option) => (
        <button
          key={option.id}
          type="button"
          role="menuitem"
          disabled={switching}
          onClick={() => (option.id === current ? onStay() : onChoose(option.id))}
          className={`w-full flex items-center gap-3 rounded-lg px-4 py-2.5 text-left hover:bg-[var(--gerege-surface-2)] disabled:opacity-60 ${
            option.id === current ? "bg-[var(--gerege-blue-soft)]" : ""
          }`}
        >
          {/* The home wears a different mark and says so in words. Its slug is
              derived from a user id and means nothing to the person reading it,
              so the second line says what the row is instead of repeating an
              identifier — this is the one row where "which of these am I in"
              cannot be answered by the name, because the name is their own. */}
          <span className={option.id === current ? "text-[var(--gerege-blue)]" : "text-slate-400"}>
            {isHome(option) ? <House className="w-4 h-4" /> : <Building2 className="w-4 h-4" />}
          </span>
          <span className="min-w-0 flex-1">
            <strong className="block text-sm font-medium truncate">{option.name}</strong>
            <small className="block text-xs text-slate-500 truncate">
              {isHome(option) ? t("web.label.my_home") : option.slug}
            </small>
          </span>
          {option.id === current && <Check className="w-4 h-4 shrink-0 text-[var(--gerege-blue)]" />}
        </button>
      ))}
      {/* Reading alongside is offered only to somebody who has somewhere to
          read from: with one membership the whole idea is empty. It is kept
          apart from the rows above because the two do different things —
          switching moves where new records are written, this does not. */}
      {onToggleActive && tenants && tenants.length > 1 && (
        <div className="mt-1 border-t border-slate-200 pt-2">
          <p className="px-4 pb-1 text-[11px] font-semibold uppercase tracking-wide text-slate-400">
            {t("web.label.read_alongside")}
          </p>
          {tenants
            .filter((option) => option.id !== current)
            .map((option) => (
              <label
                key={option.id}
                className="w-full flex items-center gap-3 rounded-lg px-4 py-2 text-left hover:bg-[var(--gerege-surface-2)] cursor-pointer"
              >
                <input
                  type="checkbox"
                  disabled={switching}
                  checked={(activeIDs || []).includes(option.id)}
                  onChange={() => onToggleActive(option.id)}
                  className="w-4 h-4 rounded border-slate-300 text-[var(--gerege-blue)]"
                />
                <span className="min-w-0 flex-1 text-sm truncate">{option.name}</span>
              </label>
            ))}
          <p className="px-4 pt-1 pb-1 text-[11px] text-slate-500">{t("web.message.read_alongside_hint")}</p>
        </div>
      )}
      {tenants?.length === 1 && (
        <p className="px-4 pb-2 pt-1 text-xs text-slate-500">{t("web.message.only_tenant")}</p>
      )}
      {failed !== "" && (
        <p role="alert" className="px-4 pb-2 pt-1 text-xs text-rose-600">
          {failed === "unknown" ? t("web.message.tenant_switch_failed") : failed}
        </p>
      )}
    </>
  );
}
