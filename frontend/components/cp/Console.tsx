"use client";

/**
 * The console's frame: who is signed in, and the sign-in form when nobody is.
 *
 * Every /cp page renders inside it, so there is one place that decides whether
 * the operator is signed in and one place that draws the form. A page that made
 * that decision for itself would eventually make it differently.
 *
 * It wears the product's design, class for class. The console had a dark
 * chrome of its own for a phase, and the argument for it — an operator with
 * both windows open should know which is which — is answered by the shield and
 * the word "Консол" in the corner rather than by a second design system: two
 * visual languages in one repository is two things to maintain and one of them
 * always falls behind. This file was that one, which is how a sign-in screen
 * built from raw Tailwind outlived the `signin-card` every other door uses.
 */

import React, { createContext, useCallback, useContext, useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  Activity,
  BrainCircuit,
  BarChart3,
  BellRing,
  Building2,
  CalendarClock,
  CheckCheck,
  ChevronDown,
  ChevronsDownUp,
  ChevronsUpDown,
  DatabaseBackup,
  Gauge,
  LayoutGrid,
  LifeBuoy,
  PackageCheck,
  MailCheck,
  Megaphone,
  Menu as HamburgerIcon,
  Scale,
  ScrollText,
  Search,
  ServerCog,
  ShieldCheck,
  SlidersHorizontal,
  Timer,
} from "lucide-react";

import LanguageSwitcher from "@/components/LanguageSwitcher";
import UserMenu from "@/components/UserMenu";
import { cp, Unauthorized, type Operator } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";
import { useBrand } from "@/lib/brandContext";
import { useTheme } from "@/lib/theme";

// The console's apps, in the shape the workspace draws them: a tile in the
// rail, and under it the groups of destinations its panel shows. Ids are
// translation keys, which are also what the folded set is remembered by — a
// stable string that does not change when the operator changes language.
interface ConsoleDestination { href: string; label: string; icon: React.ReactNode; exact?: boolean }
interface ConsoleSection { id: string; items: ConsoleDestination[] }
interface ConsoleApp { id: string; label: string; icon: React.ReactNode; sections: ConsoleSection[] }

const APPS: ConsoleApp[] = [
  {
    id: "console",
    label: "cp.view.title",
    icon: <LayoutGrid className="w-5 h-5" />,
    sections: [
      { id: "cp.group.watch", items: [
        { href: "/cp", exact: true, label: "cp.section.health", icon: <Activity className="w-5 h-5" /> },
      ] },
      { id: "cp.group.platform", items: [
        { href: "/cp/config", label: "cp.section.config", icon: <SlidersHorizontal className="w-5 h-5" /> },
        { href: "/cp/announcements", label: "cp.section.announcements", icon: <Megaphone className="w-5 h-5" /> },
        { href: "/cp/assistant", label: "cp.section.assistant", icon: <BrainCircuit className="w-5 h-5" /> },
        { href: "/cp/email-verification", label: "cp.section.verifications", icon: <MailCheck className="w-5 h-5" /> },
      ] },
      { id: "cp.group.investigation", items: [
        { href: "/cp/audit", label: "cp.section.audit", icon: <ScrollText className="w-5 h-5" /> },
        { href: "/cp/operators", label: "cp.section.operators", icon: <ShieldCheck className="w-5 h-5" /> },
      ] },
    ],
  },
  {
    // The organisations on this deployment: who they are, what they may use,
    // and what is installed for them. Its own tile rather than a group in the
    // console's, because it is where an operator spends a working day and the
    // console's other groups are things they visit.
    id: "tenants",
    label: "cp.app.tenants",
    icon: <Building2 className="w-5 h-5" />,
    sections: [
      { id: "cp.group.organisations", items: [
        { href: "/cp/tenants", label: "cp.section.tenants", icon: <Building2 className="w-5 h-5" /> },
        { href: "/cp/support", label: "cp.section.support", icon: <LifeBuoy className="w-5 h-5" /> },
        { href: "/cp/approvals", label: "cp.section.approvals", icon: <CheckCheck className="w-5 h-5" /> },
      ] },
      { id: "cp.group.entitlements", items: [
        { href: "/cp/quotas", label: "cp.section.quotas", icon: <Scale className="w-5 h-5" /> },
        { href: "/cp/installations", label: "cp.section.installations", icon: <PackageCheck className="w-5 h-5" /> },
      ] },
    ],
  },
  {
    // Running the deployment rather than administering what is on it: the
    // three questions an operator asks at 3am — is it up, is anything being
    // produced, and is anything being kept.
    id: "ops",
    label: "cp.app.ops",
    icon: <ServerCog className="w-5 h-5" />,
    sections: [
      { id: "cp.group.monitor", items: [
        { href: "/cp/ops", exact: true, label: "cp.section.metrics", icon: <Gauge className="w-5 h-5" /> },
        { href: "/cp/ops/alerts", label: "cp.section.alerts", icon: <BellRing className="w-5 h-5" /> },
        { href: "/cp/ops/jobs", label: "cp.section.jobs", icon: <Timer className="w-5 h-5" /> },
      ] },
      { id: "cp.group.report", items: [
        { href: "/cp/ops/usage", label: "cp.section.usage", icon: <BarChart3 className="w-5 h-5" /> },
        { href: "/cp/ops/schedules", label: "cp.section.schedules", icon: <CalendarClock className="w-5 h-5" /> },
      ] },
      { id: "cp.group.backup", items: [
        { href: "/cp/ops/backups", label: "cp.section.backups", icon: <DatabaseBackup className="w-5 h-5" /> },
      ] },
    ],
  },
];

// Which app the operator is in.
//
// The longest matching destination wins, as the workspace's rail decides it:
// "/cp/ops" and "/cp" both prefix-match a route under ops, and a plain
// startsWith would light the console tile on every screen in the deployment.
function appFor(pathname: string): ConsoleApp {
  let best = APPS[0];
  let bestLength = -1;
  for (const app of APPS) {
    for (const section of app.sections) {
      for (const item of section.items) {
        const matches = item.exact
          ? pathname === item.href
          : pathname === item.href || pathname.startsWith(item.href + "/");
        if (matches && item.href.length > bestLength) {
          best = app;
          bestLength = item.href.length;
        }
      }
    }
  }
  return best;
}

// Its own keys, not the workspace's: an operator's folded groups and a tenant
// user's are different opinions that happen to share a browser.
const GROUPS_KEY = "gerege_cp_sidebar_groups";
const PANEL_KEY = "gerege_cp_sidebar_open";

interface ConsoleState {
  operator: Operator;
  signOut: () => Promise<void>;
}

const ConsoleContext = createContext<ConsoleState | null>(null);

/** useConsole is how a page reaches the signed-in operator. */
export function useConsole(): ConsoleState {
  const state = useContext(ConsoleContext);
  if (!state) throw new Error("useConsole outside the console frame");
  return state;
}

export default function Console({ children }: { children: React.ReactNode }) {
  const { t } = useI18n();
  const brand = useBrand();
  const theme = useTheme();
  const router = useRouter();
  const pathname = usePathname();
  const app = appFor(pathname);
  const [operator, setOperator] = useState<Operator | null>(null);
  const [loading, setLoading] = useState(true);
  const [panelOpen, setPanelOpen] = useState(true);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [closedGroups, setClosedGroups] = useState<string[]>([]);
  const [query, setQuery] = useState("");

  useEffect(() => setPanelOpen(localStorage.getItem(PANEL_KEY) !== "false"), []);
  useEffect(() => {
    try {
      const saved = JSON.parse(localStorage.getItem(GROUPS_KEY) || "[]");
      if (Array.isArray(saved)) setClosedGroups(saved.filter((id) => typeof id === "string"));
    } catch { /* hand-edited or half-written storage is not worth a crashed shell */ }
  }, []);

  // Below 901px the panel is a drawer, as in the workspace: one button that
  // means "fold the column" on a desktop and "slide the drawer" on a phone.
  function togglePanel() {
    if (window.matchMedia("(min-width:901px)").matches) {
      setPanelOpen((open) => { localStorage.setItem(PANEL_KEY, String(!open)); return !open; });
    } else setMobileOpen((open) => !open);
  }
  function persistGroups(next: string[]) { setClosedGroups(next); localStorage.setItem(GROUPS_KEY, JSON.stringify(next)); }
  function toggleGroup(id: string) { persistGroups(closedGroups.includes(id) ? closedGroups.filter((x) => x !== id) : [...closedGroups, id]); }
  const allGroupsOpen = app.sections.every((section) => !closedGroups.includes(section.id));
  function toggleAllGroups() { persistGroups(allGroupsOpen ? app.sections.map((section) => section.id) : []); }

  const needle = query.trim().toLocaleLowerCase();
  const results = needle
    ? APPS.flatMap((entry) => entry.sections.flatMap((section) => section.items.map((item) => ({ ...item, group: section.id }))))
        .filter((item) => t(item.label).toLocaleLowerCase().includes(needle))
        .slice(0, 8)
    : [];

  const load = useCallback(async () => {
    try {
      const me = await cp.me();
      setOperator(me.operator);
    } catch (error) {
      // Anything other than "not signed in" is still answered with the form:
      // there is nothing else the console can offer, and an error page that
      // cannot be signed in from is a dead end.
      if (!(error instanceof Unauthorized)) {
        console.error("control plane: could not read the session", error);
      }
      setOperator(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const signOut = useCallback(async () => {
    try {
      await cp.signOut();
    } finally {
      setOperator(null);
    }
  }, []);

  if (loading) {
    return (
      <div className="min-h-screen grid place-items-center text-slate-500">
        <div className="animate-pulse">…</div>
      </div>
    );
  }

  if (!operator) return <SignIn onSignedIn={setOperator} />;

  return (
    <ConsoleContext.Provider value={{ operator, signOut }}>
      {/*
        The workspace's shell, cell for cell: the same topbar row — brand mark,
        context, menu toggle, session, search, the person — and the same
        two-part left menu, a 4rem app rail beside a 14rem module panel that
        folds. The console is one app in that grammar, so the rail carries one
        tile and the panel carries its groups, exactly as the platform's own
        menus do for a workspace with nothing installed.

        Copied rather than shared because Layout.tsx is bound to the tenant
        session — it asks /api/v1/me on mount, which an operator does not have.
        What is shared is the stylesheet: every class here is the workspace's,
        so a change to the design reaches both.
      */}
      <div className="gerege-shell min-h-screen flex flex-col">
        <header className="gerege-topbar h-16 flex items-center border-b sticky top-0 z-50">
          <Link
            href="/cp"
            aria-label={t("cp.view.title")}
            className="gerege-header-brand relative w-16 h-full shrink-0 grid place-items-center border-r border-[var(--gerege-chrome-border)]"
          >
            {/* The mark the product uses, and for the same reason: on the
                original design the chrome is blue, and a slate-900 square with
                an amber shield in it was a second brand nobody chose. */}
            {theme.design === "gerege" ? (
              <img src={brand.logoUrl} width={36} height={36} alt="" className="w-9 h-9 rounded-lg shadow-sm" />
            ) : (
              <span className="original-brand-mark w-9 h-9 rounded-lg grid place-items-center">
                <ShieldCheck className="w-5 h-5" />
              </span>
            )}
          </Link>
          <div className={`gerege-header-context h-full flex items-center gap-3 overflow-hidden transition-all duration-200 ${panelOpen ? "is-open" : ""}`}>
            <span className="shrink-0 text-[var(--gerege-blue)]">{app.icon}</span>
            <span className="min-w-0">
              <small className="block text-[11px] leading-4 text-slate-500 truncate">{brand.name}</small>
              <strong className="block text-[15px] leading-5 text-slate-900 truncate">{t(app.label)}</strong>
            </span>
          </div>
          <div className="gerege-menu-toggle h-full shrink-0 flex items-center justify-center gap-1">
            <button type="button" onClick={togglePanel} aria-label={t("web.action.toggle_menu")} aria-expanded={mobileOpen} className="grid place-items-center w-10 h-10 rounded-lg text-slate-600 hover:bg-slate-50">
              <HamburgerIcon className="w-5 h-5" />
            </button>
            <button
              type="button"
              onClick={toggleAllGroups}
              aria-expanded={allGroupsOpen}
              aria-label={allGroupsOpen ? t("web.action.collapse_all") : t("web.action.expand_all")}
              title={allGroupsOpen ? t("web.action.collapse_all") : t("web.action.expand_all")}
              className="grid place-items-center w-10 h-10 rounded-lg text-slate-600 hover:bg-slate-50"
            >
              {allGroupsOpen ? <ChevronsDownUp className="w-5 h-5" /> : <ChevronsUpDown className="w-5 h-5" />}
            </button>
          </div>
          {/* Where the workspace names the organisation, the console names the
              operator: the same cell answers "whose session is this". */}
          <div className="hidden lg:flex items-center gap-2 px-4 min-w-0">
            <span className="gerege-session-dot w-2 h-2 rounded-full shrink-0" />
            <strong className="text-base text-slate-800 font-semibold truncate max-w-56">{operator.name}</strong>
          </div>
          <div className="gerege-header-search hidden md:flex flex-1 items-center justify-center min-w-0 px-5 relative">
            <div className="relative w-full max-w-md">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
              <input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                onKeyDown={(event) => { if (event.key === "Enter" && results[0]) { router.push(results[0].href); setQuery(""); } }}
                placeholder={t("web.view.search_placeholder")}
                className="w-full h-10 rounded-full border border-slate-200 bg-slate-100/80 pl-10 pr-4 text-sm outline-none focus:border-[var(--gerege-blue)] focus:ring-2 focus:ring-[color-mix(in_srgb,var(--gerege-blue)_15%,transparent)]"
              />
              {results.length > 0 && (
                <div className="gerege-topbar-onlight absolute top-12 inset-x-0 bg-white border border-slate-200 rounded-xl shadow-xl p-1.5 z-[70]">
                  {results.map((item) => (
                    <button key={item.href} type="button" onClick={() => { router.push(item.href); setQuery(""); }} className="w-full flex items-center gap-3 rounded-lg px-3 py-2.5 text-left hover:bg-[var(--gerege-surface-2)]">
                      <span className="text-[var(--gerege-blue)]">{item.icon}</span>
                      <span className="min-w-0">
                        <strong className="block text-sm truncate">{t(item.label)}</strong>
                        <small className="text-slate-500 truncate">{t(item.group)}</small>
                      </span>
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>
          {/* The product's own account menu, with the two parts a console has
              no session for turned off: no organisations to switch between,
              and no /profile or /settings pages to reach. What it brings is
              what the operator was missing — language, colour mode and
              sign-out where the rest of the platform keeps them, instead of a
              lone button on the bar. The role rides under the address, because
              on this side "who is signed in" is half of "what they may do". */}
          <div className="gerege-header-user flex items-center gap-2 pr-2 sm:pr-4 lg:pr-6">
            <UserMenu
              user={{ name: operator.name, email: operator.email }}
              onLogout={() => void signOut()}
              showTenants={false}
              links={[]}
              subtitle={t(`cp.role.${operator.role}`)}
            />
          </div>
        </header>

        <div className="flex flex-1 min-h-0">
          {mobileOpen && <button type="button" className="gerege-mobile-backdrop fixed inset-0 top-16 bg-slate-950/40 z-30" aria-label={t("web.action.close_menu")} onClick={() => setMobileOpen(false)} />}
          <div className={`gerege-sidebar top-16 bottom-0 left-0 z-40 flex overflow-hidden ${mobileOpen ? "is-mobile-open" : ""} ${panelOpen ? "is-desktop-open" : ""}`}>
            {/* Division one: the app rail, a tile per console app. */}
            <nav className="w-16 min-w-16 shrink-0 py-3 flex flex-col items-center gap-2 border-r border-[var(--gerege-border)]">
              {APPS.map((entry) => (
                <Link
                  key={entry.id}
                  href={entry.sections[0].items[0].href}
                  title={t(entry.label)}
                  aria-label={t(entry.label)}
                  aria-current={entry.id === app.id ? "page" : undefined}
                  className={`w-11 h-11 rounded-xl grid place-items-center transition ${
                    entry.id === app.id
                      ? "bg-[var(--gerege-blue-soft)] text-[var(--gerege-blue)] shadow-sm"
                      : "text-slate-500 hover:bg-[var(--gerege-surface-2)] hover:text-slate-800"
                  }`}
                >
                  {entry.icon}
                </Link>
              ))}
            </nav>
            {/* Division two: the current app's modules. */}
            <aside className="gerege-menu-panel overflow-hidden">
              <div className="w-56 py-4">
                <nav className="space-y-1 px-2">
                  {app.sections.map((section) => (
                    <MenuGroup key={section.id} id={section.id} title={t(section.id)} closed={closedGroups.includes(section.id)} onToggle={toggleGroup}>
                      {section.items.map((item) => (
                        <ConsoleLink key={item.href} href={item.href} exact={item.exact} icon={item.icon} label={t(item.label)} />
                      ))}
                    </MenuGroup>
                  ))}
                </nav>
              </div>
            </aside>
          </div>
          {/* No centring wrapper: the workspace's main fills its column, and
              a max-w-6xl here left a wide screen with a band of empty chrome
              down each side of a page whose tables want the width. */}
          <main className="gerege-main flex-1 p-4 sm:p-6 lg:p-8 overflow-y-auto min-w-0">{children}</main>
        </div>
      </div>
    </ConsoleContext.Provider>
  );
}

/**
 * A titled group of destinations, folding — the workspace's own MenuGroup,
 * markup for markup, `inert` body included: a link folded away is still a
 * link, and Tab must not walk into it.
 */
function MenuGroup({ id, title, closed, onToggle, children }: { id: string; title: string; closed: boolean; onToggle: (id: string) => void; children: React.ReactNode }) {
  const bodyId = `cp-menu-group-${id.replace(/\./g, "-")}`;
  return (
    <section className="gerege-menu-group mb-6">
      <h3 className="mb-2">
        <button type="button" onClick={() => onToggle(id)} aria-expanded={!closed} aria-controls={bodyId} className="w-full flex items-center gap-1.5 px-3 py-1 rounded-md text-[11px] font-bold uppercase tracking-wider text-slate-400 hover:text-slate-600 hover:bg-[var(--gerege-surface-2)] transition">
          <span className="min-w-0 truncate text-left">{title}</span>
          <ChevronDown className={`w-3.5 h-3.5 ml-auto shrink-0 transition-transform duration-200 ${closed ? "" : "rotate-180"}`} />
        </button>
      </h3>
      <div id={bodyId} data-collapsed={closed} inert={closed} className="gerege-menu-group-body">
        <div className="space-y-1">{children}</div>
      </div>
    </section>
  );
}

/**
 * One destination.
 *
 * `exact` exists for the front page: every other route begins with /cp, so a
 * prefix test would light the first entry on every screen in the console.
 */
function ConsoleLink({
  href,
  label,
  icon,
  exact,
}: {
  href: string;
  label: string;
  icon: React.ReactNode;
  exact?: boolean;
}) {
  const pathname = usePathname();
  const active = exact ? pathname === href : pathname === href || pathname.startsWith(href + "/");
  return (
    <Link
      href={href}
      title={label}
      aria-current={active ? "page" : undefined}
      className={`gerege-nav-link flex items-center gap-3 px-3 py-2.5 text-sm font-medium transition ${
        active ? "gerege-nav-link-active font-semibold" : ""
      }`}
    >
      <span className="gerege-nav-icon">{icon}</span>
      <span>{label}</span>
    </Link>
  );
}

function SignIn({ onSignedIn }: { onSignedIn: (operator: Operator) => void }) {
  const { t } = useI18n();
  const brand = useBrand();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [failed, setFailed] = useState(false);
  const [busy, setBusy] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setFailed(false);
    try {
      const result = await cp.signIn(email, password, code);
      onSignedIn(result.operator);
    } catch {
      // One message for every reason. Which of the three was wrong is
      // deliberately not said — the API does not distinguish them either.
      setFailed(true);
      setCode("");
    } finally {
      setBusy(false);
    }
  }

  /*
    The same door /login and /setup are: `signin-shell` outside, `signin-card`
    inside, `setup-form` for the fields. It was a hand-rolled Tailwind card
    until now, which is how the console ended up with its own input border, its
    own focus ring and its own idea of a primary button — three decisions that
    were already made once, in CSS, for every other screen where somebody is
    standing outside a deployment they cannot get into.
  */
  return (
    <main className="signin-shell">
      <header className="signin-shell__nav">
        <span className="gp-brand">
          <img src={brand.logoUrl} alt="" />
          <span>{brand.name}</span>
        </span>
        <LanguageSwitcher />
      </header>
      <section className="signin-shell__body">
        <div className="signin-card">
          <div>
            <h1 className="signin-card__title">{t("cp.login.title")}</h1>
            <p className="signin-card__lede">{t("cp.login.hint")}</p>
          </div>

          {failed && <p className="signin-alert">{t("cp.login.failed")}</p>}

          <form className="setup-form" onSubmit={submit}>
            <label>
              <span>{t("cp.field.email")}</span>
              <input
                type="email"
                autoComplete="username"
                required
                value={email}
                onChange={(event) => setEmail(event.target.value)}
              />
            </label>

            <label>
              <span>{t("cp.field.password")}</span>
              <input
                type="password"
                autoComplete="current-password"
                required
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
            </label>

            <label>
              <span>{t("cp.field.code")}</span>
              <input
                // A numeric keypad on a telephone, and no autofill: a one-time
                // code is not something a password manager should be filling
                // from a saved value.
                inputMode="numeric"
                autoComplete="one-time-code"
                pattern="[0-9]*"
                maxLength={6}
                required
                value={code}
                onChange={(event) => setCode(event.target.value.replace(/\D/g, ""))}
                className="font-mono tracking-[0.4em]"
              />
            </label>

            <button className="signin-btn signin-btn--primary" type="submit" disabled={busy}>
              {t("cp.action.sign_in")}
            </button>
          </form>
        </div>
      </section>
    </main>
  );
}
