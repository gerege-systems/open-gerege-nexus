"use client";

import { useState } from "react";
import Link from "next/link";
import { ChevronDown, LogOut, Monitor, Moon, Settings, Sun, UserRound } from "lucide-react";
import {
  Avatar,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  cn,
} from "@gerege-systems/ui";
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
 *
 * A design-system DropdownMenu: outside click, Escape, focus return and arrow
 * keys come from it. So does the height — the panel hangs off the button, and
 * the room it has is what is left *below* that point, not the height of the
 * screen. Capping it at `100dvh` minus a guess, which this once did, put the
 * last rows (sign out among them) past the bottom edge on a phone. Radix
 * measures the actual room from the visual viewport — the one iOS shrinks
 * while its toolbars are up — and hands it over as a CSS variable.
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
  // The brand mark in the header offers the same list, but the mobile shell
  // hides the rail — this is the other way to change organisation on a phone.
  const { tenants, activeIDs, switching, failed, switchTo, toggleActive } = useTenants(open && showTenants);

  const initials = initialsOf(user?.name || user?.email || "G");
  const rows = links ?? [
    { href: "/profile", label: t("profile.title"), icon: <UserRound aria-hidden /> },
    { href: "/settings/appearance", label: t("web.menu.settings"), icon: <Settings aria-hidden /> },
  ];
  // A preference is something people set twice in a row — try one, compare
  // the other — and a menu that shuts on the first pick makes the second a
  // whole reopening.
  const keepOpen = (event: Event) => event.preventDefault();

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className={cn(
            "group flex h-9 items-center gap-2 rounded-full border border-line py-0.5 ps-0.5 pe-2 outline-none",
            "transition-colors hover:bg-surface-2",
            "focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
            "data-[state=open]:border-accent data-[state=open]:bg-accent-soft",
          )}
        >
          <Avatar size="md" fallback={initials} alt="" />
          {/* The whole name, not the first word of it: "Цэнддорж Эрдэнэбат" is
              one name in two parts and cutting it at 9rem said the wrong one. */}
          <span className="hidden max-w-60 truncate text-sm font-medium text-foreground md:block">{user?.name}</span>
          <ChevronDown className="size-3.5 text-subtle transition-transform group-data-[state=open]:rotate-180" aria-hidden />
        </button>
      </DropdownMenuTrigger>

      <DropdownMenuContent
        align="end"
        className="w-80 max-h-[var(--radix-dropdown-menu-content-available-height)] overflow-y-auto overscroll-contain"
      >
        <div className="flex items-start gap-3 px-2 py-2">
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-semibold text-foreground">{user?.name}</p>
            <p className="truncate text-xs text-muted">{user?.email}</p>
            {subtitle && <p className="mt-0.5 truncate text-xs text-accent">{subtitle}</p>}
          </div>
          {/* Signing out is also the last row of this menu, and on a phone
              with several organisations that row is a whole scroll away.
              The header is the one part of the menu that never moves, and
              the space beside a truncated name was empty. */}
          <DropdownMenuItem
            onSelect={onLogout}
            aria-label={t("web.action.logout")}
            title={t("web.action.logout")}
            className="size-8 shrink-0 justify-center p-0 text-muted data-[highlighted]:bg-danger-soft data-[highlighted]:text-danger"
          >
            <LogOut aria-hidden />
          </DropdownMenuItem>
        </div>

        {/* Only when there is a choice to make. A list of one would be a
            third line of chrome in a menu that already carries two, saying
            nothing the header does not already show. */}
        {showTenants && tenants && tenants.length > 1 && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuLabel>{t("web.view.tenants")}</DropdownMenuLabel>
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
          </>
        )}

        {rows.length > 0 && (
          <>
            <DropdownMenuSeparator />
            {rows.map((row) => (
              <DropdownMenuItem key={row.href} asChild>
                <Link href={row.href}>
                  <span className="text-muted">{row.icon}</span>
                  {row.label}
                </Link>
              </DropdownMenuItem>
            ))}
          </>
        )}

        <DropdownMenuSeparator />
        <DropdownMenuLabel>{t("base.field.language")}</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={locale} onValueChange={(code) => setLocale(code as typeof locale)}>
          {offeredLocales.map((option) => (
            <DropdownMenuRadioItem key={option.code} value={option.code} onSelect={keepOpen}>
              {option.label}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>

        <DropdownMenuSeparator />
        <DropdownMenuLabel>{t("web.field.theme")}</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={theme.mode} onValueChange={(mode) => theme.updateTheme({ mode: mode as ColorMode })}>
          {MODES.map(({ value, icon: Icon, labelKey }) => (
            <DropdownMenuRadioItem key={value} value={value} onSelect={keepOpen}>
              <Icon aria-hidden />
              {t(labelKey)}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>

        <DropdownMenuSeparator />
        <DropdownMenuItem destructive onSelect={onLogout}>
          <LogOut aria-hidden />
          {t("web.action.logout")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
