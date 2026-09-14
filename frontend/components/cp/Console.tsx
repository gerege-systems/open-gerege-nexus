"use client";

/**
 * The console's frame: who is signed in, and the sign-in form when nobody is.
 *
 * Every /cp page renders inside it, so there is one place that decides whether
 * the operator is signed in and one place that draws the form. A page that made
 * that decision for itself would eventually make it differently.
 *
 * It is built from the design system's shell parts — TopNav, Sidebar, Sheet —
 * in the library's "rail + panel" arrangement the workspace also uses: a
 * column of app tiles, beside it the panel of the current app's groups, above
 * the work area the bar with the search and the session. The console had a
 * chrome of its own for a phase, and the argument for it — an operator with
 * both windows open should know which is which — is answered by the word
 * "Консол" in the corner rather than by a second design system.
 */

import React, { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
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
  CheckCircle2,
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
  Users,
} from "lucide-react";
import {
  Alert,
  Button,
  Card,
  CardDescription,
  CardHeader,
  IconButton,
  Input,
  Sheet,
  SheetContent,
  SheetTitle,
  Sidebar,
  SidebarGroup,
  SidebarItem,
  SidebarSection,
  Spinner,
  Tooltip,
  TopNav,
  cn,
} from "@gerege-systems/ui";

import LanguageSwitcher from "@/components/LanguageSwitcher";
import UserMenu from "@/components/UserMenu";
import { cp, Unauthorized, type Operator } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";
import { useBrand } from "@/lib/brandContext";

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
    icon: <LayoutGrid />,
    sections: [
      { id: "cp.group.watch", items: [
        { href: "/cp", exact: true, label: "cp.section.health", icon: <Activity /> },
      ] },
      { id: "cp.group.platform", items: [
        { href: "/cp/config", label: "cp.section.config", icon: <SlidersHorizontal /> },
        { href: "/cp/announcements", label: "cp.section.announcements", icon: <Megaphone /> },
        { href: "/cp/assistant", label: "cp.section.assistant", icon: <BrainCircuit /> },
        { href: "/cp/email-verification", label: "cp.section.verifications", icon: <MailCheck /> },
      ] },
      { id: "cp.group.people", items: [
        { href: "/cp/people", label: "cp.section.people", icon: <Users /> },
      ] },
      { id: "cp.group.investigation", items: [
        { href: "/cp/audit", label: "cp.section.audit", icon: <ScrollText /> },
        { href: "/cp/operators", label: "cp.section.operators", icon: <ShieldCheck /> },
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
    icon: <Building2 />,
    sections: [
      { id: "cp.group.organisations", items: [
        { href: "/cp/tenants", label: "cp.section.tenants", icon: <Building2 /> },
        { href: "/cp/support", label: "cp.section.support", icon: <LifeBuoy /> },
        { href: "/cp/approvals", label: "cp.section.approvals", icon: <CheckCheck /> },
      ] },
      { id: "cp.group.entitlements", items: [
        { href: "/cp/quotas", label: "cp.section.quotas", icon: <Scale /> },
        { href: "/cp/installations", label: "cp.section.installations", icon: <PackageCheck /> },
      ] },
    ],
  },
  {
    // Running the deployment rather than administering what is on it: the
    // three questions an operator asks at 3am — is it up, is anything being
    // produced, and is anything being kept.
    id: "ops",
    label: "cp.app.ops",
    icon: <ServerCog />,
    sections: [
      { id: "cp.group.monitor", items: [
        { href: "/cp/ops", exact: true, label: "cp.section.metrics", icon: <Gauge /> },
        { href: "/cp/ops/alerts", label: "cp.section.alerts", icon: <BellRing /> },
        { href: "/cp/ops/jobs", label: "cp.section.jobs", icon: <Timer /> },
      ] },
      { id: "cp.group.report", items: [
        { href: "/cp/ops/usage", label: "cp.section.usage", icon: <BarChart3 /> },
        { href: "/cp/ops/schedules", label: "cp.section.schedules", icon: <CalendarClock /> },
      ] },
      { id: "cp.group.backup", items: [
        { href: "/cp/ops/backups", label: "cp.section.backups", icon: <DatabaseBackup /> },
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
  const router = useRouter();
  const pathname = usePathname();
  const app = appFor(pathname);
  const [operator, setOperator] = useState<Operator | null>(null);
  const [loading, setLoading] = useState(true);
  const [panelOpen, setPanelOpen] = useState(true);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [closedGroups, setClosedGroups] = useState<string[]>([]);
  const [query, setQuery] = useState("");
  // The button that opens the drawer, so closing it hands focus back there.
  const drawerTrigger = useRef<HTMLButtonElement>(null);

  useEffect(() => setPanelOpen(localStorage.getItem(PANEL_KEY) !== "false"), []);
  useEffect(() => {
    try {
      const saved = JSON.parse(localStorage.getItem(GROUPS_KEY) || "[]");
      if (Array.isArray(saved)) setClosedGroups(saved.filter((id) => typeof id === "string"));
    } catch { /* hand-edited or half-written storage is not worth a crashed shell */ }
  }, []);

  // Below 1024px the panel is a drawer, as in the workspace: one button that
  // means "fold the panel" on a desktop and "slide the drawer" on a phone.
  function togglePanel() {
    if (window.matchMedia("(min-width:1024px)").matches) {
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
  function go(href: string) { router.push(href); setQuery(""); }

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
      <div className="grid min-h-dvh place-items-center bg-background">
        <Spinner size="lg" tone="neutral" label={t("base.message.loading")} />
      </div>
    );
  }

  if (!operator) return <SignIn onSignedIn={setOperator} />;

  const appLabel = t(app.label);
  // Which module the panel shows, at the top of the panel it names, with the
  // control that folds or opens its groups.
  const moduleHeader = (
    <div className="flex w-full min-w-0 items-center gap-2 px-1">
      <span className="shrink-0 text-accent [&_svg]:size-5">{app.icon}</span>
      <h2 className="min-w-0 flex-1 truncate text-sm font-semibold text-foreground">{appLabel}</h2>
      <IconButton
        aria-label={allGroupsOpen ? t("web.action.collapse_all") : t("web.action.expand_all")}
        title={allGroupsOpen ? t("web.action.collapse_all") : t("web.action.expand_all")}
        aria-expanded={allGroupsOpen}
        icon={allGroupsOpen ? <ChevronsDownUp /> : <ChevronsUpDown />}
        size="sm"
        onClick={toggleAllGroups}
      />
    </div>
  );
  const panel = (onNavigate?: () => void) => (
    <ModulePanel app={app} closedGroups={closedGroups} onToggleGroup={toggleGroup} onNavigate={onNavigate} />
  );

  return (
    <ConsoleContext.Provider value={{ operator, signOut }}>
      {/* One screen tall, only <main> scrolls. The same arrangement as the
          workspace's Layout, drawn from the same library parts: Layout is
          bound to the tenant session — it asks /api/v1/me on mount, which an
          operator does not have — so the frame is its own, the parts are not. */}
      <div className="flex h-dvh overflow-hidden bg-background text-foreground print:h-auto print:overflow-visible">
        {/* Division one: the app rail, a tile per console app. */}
        <nav
          aria-label={t("cp.field.apps")}
          className="hidden w-14 shrink-0 flex-col items-center gap-1 border-r border-line bg-chrome py-2 lg:flex print:hidden"
        >
          <div className="mb-2 flex h-10 shrink-0 items-center">
            <BrandLink />
          </div>
          <div className="flex min-h-0 w-full flex-1 flex-col items-center gap-1 overflow-y-auto py-0.5 [scrollbar-width:none]">
            {APPS.map((entry) => (
              <RailTile key={entry.id} href={entry.sections[0].items[0].href} icon={entry.icon} current={entry.id === app.id} label={t(entry.label)} />
            ))}
          </div>
        </nav>

        {/* Division two, from lg: the current app's groups. The library's
            panel is `hidden md:flex`; the console promotes the breakpoint to
            lg and serves a drawer below it. Beside a rail the panel is
            fixed-width and its own collapse control is hidden. */}
        <Sidebar
          aria-label={appLabel}
          header={moduleHeader}
          className={cn("bg-chrome md:hidden print:hidden [&>button:last-child]:hidden", panelOpen && "lg:flex")}
        >
          {panel()}
        </Sidebar>

        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          <TopNav
            className="bg-chrome print:hidden"
            logo={
              <div className="flex min-w-0 items-center gap-2">
                {/* Two controls for one function: aria-expanded reports the
                    state of the thing each one actually acts on. */}
                <IconButton
                  ref={drawerTrigger}
                  aria-label={t("web.action.toggle_menu")}
                  aria-expanded={mobileOpen}
                  icon={<HamburgerIcon />}
                  size="sm"
                  className="lg:hidden"
                  onClick={togglePanel}
                />
                <IconButton
                  aria-label={t("web.action.toggle_menu")}
                  aria-expanded={panelOpen}
                  icon={<HamburgerIcon />}
                  size="sm"
                  className="hidden lg:inline-flex"
                  onClick={togglePanel}
                />
                {/* The module is named at the top of the panel from lg; below
                    that the panel is off screen, so the bar names it. */}
                <span className="ms-1 shrink-0 text-accent lg:hidden [&_svg]:size-5">{app.icon}</span>
                <span className="min-w-0 lg:hidden">
                  <small className="block truncate text-xs leading-4 text-muted">{brand.name}</small>
                  <strong className="block truncate text-sm leading-5 text-foreground">{appLabel}</strong>
                </span>
                {/* Where the workspace names the organisation, the console
                    names the operator: "whose session is this". */}
                <span className="ms-4 hidden min-w-0 items-center gap-2 lg:flex">
                  <span aria-hidden className="size-2 shrink-0 rounded-full bg-success-solid" />
                  <strong className="max-w-56 truncate text-sm font-semibold text-foreground">{operator.name}</strong>
                </span>
              </div>
            }
            search={
              <div className="relative hidden md:block">
                <Input
                  type="search"
                  size="sm"
                  label={t("base.action.search")}
                  hideLabel
                  placeholder={t("web.view.search_placeholder")}
                  prefix={<Search className="size-4" aria-hidden />}
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  onKeyDown={(event) => { if (event.key === "Enter" && results[0]) go(results[0].href); }}
                />
                {results.length > 0 && (
                  <div className="absolute inset-x-0 top-full z-dropdown mt-1 rounded-lg border border-line bg-surface p-1 shadow-md">
                    {results.map((item) => (
                      <button
                        key={item.href}
                        type="button"
                        onClick={() => go(item.href)}
                        className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-start outline-none hover:bg-surface-2 focus-visible:ring-2 focus-visible:ring-ring"
                      >
                        <span className="shrink-0 text-accent [&_svg]:size-4">{item.icon}</span>
                        <span className="min-w-0">
                          <strong className="block truncate text-sm font-medium text-foreground">{t(item.label)}</strong>
                          <small className="block truncate text-xs text-muted">{t(item.group)}</small>
                        </span>
                      </button>
                    ))}
                  </div>
                )}
              </div>
            }
            actions={
              // The product's own account menu, with the two parts a console
              // has no session for turned off: no organisations to switch
              // between, and no /profile or /settings pages to reach. What it
              // brings is language, colour mode and sign-out where the rest of
              // the platform keeps them. The role rides under the address,
              // because on this side "who is signed in" is half of "what they
              // may do".
              <UserMenu
                user={{ name: operator.name, email: operator.email }}
                onLogout={() => void signOut()}
                showTenants={false}
                links={[]}
                subtitle={t(`cp.role.${operator.role}`)}
              />
            }
          />
          {/* No centring wrapper: the main fills its column, and a max-w here
              left a wide screen with a band of empty chrome down each side of
              a page whose tables want the width. */}
          <main className="relative min-h-0 min-w-0 flex-1 overflow-y-auto p-4 sm:p-6 lg:p-8 print:overflow-visible print:p-0">{children}</main>
        </div>

        {/* Below lg the rail and the panel slide in together as a drawer.
            Escape, the backdrop and the focus trap are the Sheet's; a
            destination chosen inside it closes it, a tile only switches the
            panel. */}
        <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
          <SheetContent side="left" showClose={false} aria-describedby={undefined} returnFocusTo={drawerTrigger} className="w-72 p-0">
            <SheetTitle className="sr-only">{t("cp.view.title")}</SheetTitle>
            <Sidebar
              aria-label={appLabel}
              header={
                <nav aria-label={t("cp.field.apps")} className="flex w-full items-center gap-1 overflow-x-auto [scrollbar-width:none]">
                  {APPS.map((entry) => (
                    <RailTile
                      key={entry.id}
                      href={entry.sections[0].items[0].href}
                      icon={entry.icon}
                      current={entry.id === app.id}
                      label={t(entry.label)}
                      className="before:inset-x-2 before:top-auto before:-bottom-1 before:h-0.5 before:w-auto before:rounded-t-full"
                    />
                  ))}
                </nav>
              }
              className="flex h-full w-full border-r-0 [&>button:last-child]:hidden"
            >
              <div className="border-b border-line px-3 pb-3">{moduleHeader}</div>
              {panel(() => setMobileOpen(false))}
            </Sidebar>
          </SheetContent>
        </Sheet>
      </div>
    </ConsoleContext.Provider>
  );
}

/**
 * The brand mark, and the way home. The mark the product uses: a second,
 * console-only mark would be a second brand nobody chose.
 */
function BrandLink() {
  const { t } = useI18n();
  const brand = useBrand();
  return (
    <Link
      href="/cp"
      aria-label={t("cp.view.title")}
      className="flex min-w-0 items-center gap-2 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
    >
      <img src={brand.logoUrl} width={32} height={32} alt="" className="size-8 shrink-0 rounded-md" />
    </Link>
  );
}

/** One tile of the rail: an icon, named by its tooltip and for assistive tech. */
function RailTile({ href, icon, current, label, className }: { href: string; icon: React.ReactNode; current: boolean; label: string; className?: string }) {
  return (
    <Tooltip label={label} side="right">
      <Link
        href={href}
        aria-label={label}
        aria-current={current ? "page" : undefined}
        className={cn(
          "relative inline-flex size-10 shrink-0 items-center justify-center rounded-md outline-none [&_svg]:size-5",
          "transition-colors",
          "focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
          // Current = accent bar on the rail's edge + background + accent
          // icon, never colour alone.
          "before:absolute before:top-2 before:bottom-2 before:-left-2 before:w-0.5 before:rounded-r-full before:bg-accent before:opacity-0",
          current ? "bg-accent-soft text-accent before:opacity-100" : "text-muted hover:bg-surface-2 hover:text-foreground",
          className,
        )}
      >
        {icon}
      </Link>
    </Tooltip>
  );
}

/**
 * The current app's groups of destinations, each a folding SidebarGroup whose
 * open state is the operator's — remembered by the group's id, which is a
 * translation key and so survives a change of language.
 */
function ModulePanel({ app, closedGroups, onToggleGroup, onNavigate }: {
  app: ConsoleApp;
  closedGroups: string[];
  onToggleGroup: (id: string) => void;
  onNavigate?: () => void;
}) {
  const { t } = useI18n();
  return (
    <>
      {app.sections.map((section) => (
        <SidebarSection key={section.id}>
          <SidebarGroup
            label={t(section.id)}
            open={!closedGroups.includes(section.id)}
            onOpenChange={() => onToggleGroup(section.id)}
          >
            {section.items.map((item) => (
              <ConsoleLink key={item.href} href={item.href} exact={item.exact} icon={item.icon} label={t(item.label)} onNavigate={onNavigate} />
            ))}
          </SidebarGroup>
        </SidebarSection>
      ))}
    </>
  );
}

/**
 * One destination.
 *
 * `exact` exists for the front page: every other route begins with /cp, so a
 * prefix test would light the first entry on every screen in the console.
 * The library item wraps the router's Link so a click stays a client-side
 * navigation — the frame is a layout and must survive it.
 */
function ConsoleLink({ href, label, icon, exact, onNavigate }: {
  href: string;
  label: string;
  icon: React.ReactNode;
  exact?: boolean;
  onNavigate?: () => void;
}) {
  const pathname = usePathname();
  const active = exact ? pathname === href : pathname === href || pathname.startsWith(href + "/");
  // The icon goes through the item's prop: with asChild the Link's own
  // children become the label, wrapped in an inline truncating span, so an
  // icon passed as a child breaks onto its own line.
  return (
    <SidebarItem asChild active={active} tooltip={label} icon={icon}>
      <Link
        href={href}
        onClick={onNavigate}
        className={cn(
          // Active = accent bar + background, never colour alone.
          "relative before:absolute before:top-1.5 before:bottom-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-accent before:opacity-0",
          active && "bg-accent-soft text-accent before:opacity-100",
        )}
      >
        {label}
      </Link>
    </SidebarItem>
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
    The console's front door is a landing page, not a lonely card.

    Somebody who arrives at this hostname without a session gets the
    platform's own landing language — navy hero, grid pattern, 1180px column —
    with the sign-in card in the hero beside the copy, in the slot the public
    page keeps for the eID card. Somebody who came here to sign in reaches the
    form without scrolling; somebody who came to find out what this hostname is
    reads down the page.

    Everything the page claims below the fold is true of the code it is
    describing — the four roles, the one capability table, the two-superadmin
    deletion, the five conditions on impersonation. A front door that oversells
    the room behind it is the one kind of copy an operator will notice.
  */
  return (
    <div className="gp-landing" id="top">
      <header className="gp-nav gp-nav--plain">
        <span className="gp-brand">
          <img src={brand.logoUrl} alt="" />
          <span>{brand.name}</span>
          <small className="gp-brand__chip">{t("cp.landing.chip")}</small>
        </span>
        <LanguageSwitcher />
      </header>

      <section className="gp-hero">
        <div className="gp-pattern" />
        <div className="gp-hero__inner">
          <div className="gp-copy">
            <span className="gp-eyebrow">
              <i /> {t("cp.landing.eyebrow")}
            </span>
            <h1>
              {t("cp.landing.title_lead")} <em>{t("cp.landing.title_highlight")}</em>{" "}
              {t("cp.landing.title_tail")}
            </h1>
            <p>{t("cp.landing.lede")}</p>
            <div className="gp-stats">
              <span><b>{t("cp.landing.stat_roles")}</b>{t("cp.landing.stat_roles_label")}</span>
              <span><b>{t("cp.landing.stat_caps")}</b>{t("cp.landing.stat_caps_label")}</span>
              <span><b>{t("cp.landing.stat_session")}</b>{t("cp.landing.stat_session_label")}</span>
              <span><b>{t("cp.landing.stat_stepup")}</b>{t("cp.landing.stat_stepup_label")}</span>
            </div>
          </div>

          <div className="gp-login-slot">
            <Card padding="lg" className="relative w-full max-w-md text-foreground">
              <CardHeader>
                {/* An h2 under the hero's h1; the library's CardTitle is an h3. */}
                <h2 className="text-xl leading-tight font-semibold text-foreground">{t("cp.login.title")}</h2>
                <CardDescription>{t("cp.login.hint")}</CardDescription>
              </CardHeader>

              <form onSubmit={submit} className="flex flex-col gap-4">
                {failed && (
                  <Alert variant="danger" live>
                    {t("cp.login.failed")}
                  </Alert>
                )}
                <Input
                  type="email"
                  label={t("cp.field.email")}
                  autoComplete="username"
                  required
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                />
                <Input
                  type="password"
                  label={t("cp.field.password")}
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                />
                <Input
                  label={t("cp.field.code")}
                  // A numeric keypad on a telephone, and no autofill: a
                  // one-time code is not something a password manager should
                  // be filling from a saved value.
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  pattern="[0-9]*"
                  maxLength={6}
                  required
                  value={code}
                  onChange={(event) => setCode(event.target.value.replace(/\D/g, ""))}
                  className="[&_input]:font-mono [&_input]:tracking-[0.4em]"
                />
                <Button type="submit" size="lg" loading={busy} className="mt-2 w-full">
                  {t("cp.action.sign_in")}
                </Button>
              </form>
            </Card>
          </div>
        </div>
      </section>

      <section className="gp-section">
        <div className="gp-heading">
          <span>{t("cp.landing.model_eyebrow")}</span>
          <h2>{t("cp.landing.model_title")}</h2>
          <p>{t("cp.landing.model_lede")}</p>
        </div>

        <div className="gp-services">
          <div className="gp-feature">
            <span className="tag">{t("cp.landing.card1_tag")}</span>
            <h3>{t("cp.landing.card1_title")}</h3>
            <p>{t("cp.landing.card1_body")}</p>
          </div>
          <div className="gp-feature">
            <span className="tag">{t("cp.landing.card2_tag")}</span>
            <h3>{t("cp.landing.card2_title")}</h3>
            <p>{t("cp.landing.card2_body")}</p>
          </div>
          <div className="gp-feature gp-feature--dark">
            <span className="tag">{t("cp.landing.card3_tag")}</span>
            <h3>{t("cp.landing.card3_title")}</h3>
            <p>{t("cp.landing.card3_body")}</p>
          </div>
        </div>

        <p className="cp-landing__note">{t("cp.landing.auditor")}</p>
      </section>

      <section className="gp-trust">
        <div>
          <span className="gp-eyebrow gp-eyebrow--blue">
            <i /> {t("cp.landing.imp_eyebrow")}
          </span>
          <h2>{t("cp.landing.imp_title")}</h2>
          <p>{t("cp.landing.imp_lede")}</p>
        </div>
        <ul>
          <li><CheckCircle2 /> {t("cp.landing.imp_1")}</li>
          <li><CheckCircle2 /> {t("cp.landing.imp_2")}</li>
          <li><CheckCircle2 /> {t("cp.landing.imp_3")}</li>
          <li><CheckCircle2 /> {t("cp.landing.imp_4")}</li>
          <li><CheckCircle2 /> {t("cp.landing.imp_5")}</li>
        </ul>
      </section>

      <footer className="gp-footer">
        <span>{t("cp.landing.imp_note")}</span>
        <span>{t("cp.landing.footer")}</span>
      </footer>
    </div>
  );
}
