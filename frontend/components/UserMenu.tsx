"use client";

import { UserRound } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { ChevronDown, LogOut, Monitor, Moon, Settings, Sun } from "lucide-react";
import { LOCALES, TranslationKey, useI18n } from "@/lib/i18n";
import { ColorMode, useTheme } from "@/lib/theme";
import { TenantChoices, useTenants } from "@/components/TenantChoices";

/**
 * Two letters, not one.
 *
 * A single initial is the same letter for most of a Mongolian directory — one
 * mark that says nothing about whose account is open. Two words give a letter
 * each; one word gives its first two.
 */
function initialsOf(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  if (words.length >= 2) return (words[0].slice(0, 1) + words[1].slice(0, 1)).toUpperCase();
  return (words[0] || "?").slice(0, 2).toUpperCase();
}

const MODES: { value: ColorMode; icon: typeof Sun; labelKey: TranslationKey }[] = [
  { value: "light", icon: Sun, labelKey: "appearance.mode.light" },
  { value: "dark", icon: Moon, labelKey: "appearance.mode.dark" },
  { value: "system", icon: Monitor, labelKey: "appearance.mode.system" },
];

export interface UserMenuLink { href: string; label: string; icon: React.ReactNode }

/**
 * Account menu in the header. Language and colour mode live here rather than as
 * separate header controls, so the toolbar carries one affordance instead of
 * three.
 *
 * The operator console wears the same menu with the two parts it has no
 * session for turned off: no organisations to switch between, and no /profile
 * or /settings pages to reach. `showTenants={false}` also skips the fetch — an
 * unauthenticated call to the tenant API on every opening would be a 401 in
 * the console's network log and nothing else.
 */
export default function UserMenu({
  user,
  onLogout,
  showTenants = true,
  links,
  subtitle,
}: {
  user: { name?: string; email?: string; tenant_id?: string } | null;
  onLogout: () => void;
  showTenants?: boolean;
  /** Rows above the preferences; defaults to the workspace's own pair. */
  links?: UserMenuLink[];
  /** A line under the address — the console names the operator's role there. */
  subtitle?: string;
}) {
  const { t, locale, setLocale, availableLocales } = useI18n();
  const offeredLocales = LOCALES.filter((option) => availableLocales.includes(option.code));
  const theme = useTheme();
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  // The brand mark in the header offers the same list, but the mobile shell
  // hides the brand — this is the only way to change organisation on a phone.
  const { tenants, activeIDs, switching, failed, switchTo, toggleActive } = useTenants(open && showTenants);

  // Close on an outside click or Escape, the way a menu is expected to behave.
  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: MouseEvent) => {
      if (!containerRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  const initials = initialsOf(user?.name || user?.email || "G");
  const rows = links ?? [
    { href: "/profile", label: t("profile.title"), icon: <UserRound className="w-4 h-4" /> },
    { href: "/settings/appearance", label: t("web.menu.settings"), icon: <Settings className="w-4 h-4" /> },
  ];

  return (
    <div className="relative" ref={containerRef}>
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        aria-haspopup="menu"
        aria-expanded={open}
        className={`flex items-center gap-2.5 rounded-full border py-1 pl-1 pr-2.5 transition ${
          open
            ? "gerege-topbar-onlight border-[var(--gerege-blue)] bg-[var(--gerege-blue-soft)]"
            : "border-slate-200 hover:bg-slate-50"
        }`}
      >
        <span className="w-8 h-8 rounded-full bg-[var(--gerege-blue-soft)] text-[var(--gerege-blue)] grid place-items-center text-xs font-bold">
          {initials}
        </span>
        {/* The whole name, not the first word of it: "Цэнддорж Эрдэнэбат" is
            one name in two parts and cutting it at 9rem said the wrong one. */}
        <span className="hidden md:block text-sm font-medium text-slate-800 truncate max-w-60">{user?.name}</span>
        <ChevronDown className={`w-3.5 h-3.5 text-slate-400 transition-transform ${open ? "rotate-180" : ""}`} />
      </button>

      {open && (
        <div
          role="menu"
          className="gerege-topbar-onlight absolute right-0 mt-2 w-[320px] rounded-2xl border border-slate-200 bg-white shadow-xl overflow-hidden z-50"
        >
          <div className="px-4 py-3.5 border-b border-slate-100">
            <p className="text-sm font-semibold text-slate-900 truncate">{user?.name}</p>
            <p className="text-xs text-slate-500 truncate">{user?.email}</p>
            {subtitle && <p className="mt-0.5 text-xs text-[var(--gerege-blue)] truncate">{subtitle}</p>}
          </div>

          {/* Only when there is a choice to make. A list of one would be a
              third line of chrome in a menu that already carries two, saying
              nothing the header does not already show. */}
          {showTenants && tenants && tenants.length > 1 && (
            <div className="py-1.5 border-b border-slate-100">
              <p className="px-4 py-1.5 text-[11px] font-semibold uppercase tracking-wider text-slate-400">
                {t("web.view.tenants")}
              </p>
              <TenantChoices
          activeIDs={activeIDs}
          onToggleActive={(id) => void toggleActive(id, user?.tenant_id || "")}
                current={user?.tenant_id}
                tenants={tenants}
                switching={switching}
                failed={failed}
                onChoose={(id) => void switchTo(id)}
                onStay={() => setOpen(false)}
              />
            </div>
          )}

          {rows.length > 0 && (
            <div className="py-1.5 border-b border-slate-100">
              {rows.map((row) => (
                <Link
                  key={row.href}
                  href={row.href}
                  onClick={() => setOpen(false)}
                  className="flex items-center gap-3 px-4 py-2.5 text-sm text-slate-700 hover:bg-slate-50"
                  role="menuitem"
                >
                  <span className="text-slate-400">{row.icon}</span>
                  {row.label}
                </Link>
              ))}
            </div>
          )}

          <div className="px-4 py-3 space-y-3 border-b border-slate-100">
            <p className="text-[11px] font-semibold uppercase tracking-wider text-slate-400">
              {t("web.menu.preferences")}
            </p>

            <div className="flex items-center justify-between gap-3">
              <span className="text-sm text-slate-600">{t("base.field.language")}</span>
              <div className="inline-flex rounded-lg border border-slate-200 p-0.5 bg-slate-50">
                {offeredLocales.map((option) => (
                  <button
                    key={option.code}
                    type="button"
                    onClick={() => setLocale(option.code)}
                    aria-pressed={locale === option.code}
                    title={option.label}
                    className={`px-2.5 py-1 rounded-md text-xs font-semibold uppercase transition ${
                      locale === option.code ? "bg-white text-[var(--gerege-blue)] shadow-sm" : "text-slate-500 hover:text-slate-700"
                    }`}
                  >
                    {option.code}
                  </button>
                ))}
              </div>
            </div>

            <div className="flex items-center justify-between gap-3">
              <span className="text-sm text-slate-600">{t("web.field.theme")}</span>
              <div className="inline-flex rounded-lg border border-slate-200 p-0.5 bg-slate-50">
                {MODES.map(({ value, icon: Icon, labelKey }) => (
                  <button
                    key={value}
                    type="button"
                    onClick={() => theme.updateTheme({ mode: value })}
                    aria-pressed={theme.mode === value}
                    title={t(labelKey)}
                    className={`grid place-items-center w-8 h-7 rounded-md transition ${
                      theme.mode === value ? "bg-white text-[var(--gerege-blue)] shadow-sm" : "text-slate-500 hover:text-slate-700"
                    }`}
                  >
                    <Icon className="w-4 h-4" />
                  </button>
                ))}
              </div>
            </div>
          </div>

          <button
            type="button"
            onClick={() => {
              setOpen(false);
              onLogout();
            }}
            className="w-full flex items-center gap-3 px-4 py-3 text-sm font-medium text-red-600 hover:bg-red-50"
            role="menuitem"
          >
            <LogOut className="w-4 h-4" />
            {t("web.action.logout")}
          </button>
        </div>
      )}
    </div>
  );
}
