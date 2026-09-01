"use client";

import React, { useCallback, useEffect, useState } from "react";
import { api, type ManifestReleaseNotes, type ReleaseKind } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Banner } from "@/components/ui";
import AppHistory from "@/components/AppHistory";
import {
  Search,
  Download,
  Power,
  PowerOff,
  Boxes,
  Users,
  Package,
  ArrowUpCircle,
  LayoutGrid,
  Rows3,
  Sparkles,
  History,
  Lock,
} from "lucide-react";
import { MenuIcon } from "@/lib/icons";

interface AppItem {
  id: string;
  slug: string;
  name: string;
  description: string;
  icon_url: string;
  category: string;
  // "public" or "private". A private app is one the registry offers to named
  // platforms only; seeing it here means this deployment is one of them.
  visibility?: string;
  version: string;
  installed: boolean;
  enabled: boolean;
  installed_version?: string;
  latest_version: string;
  update_available: boolean;
  manifest: {
    dependencies?: Array<{ id: string; version_constraint: string }>;
    // What the app registers in the sidebar. Read here for the first entry's
    // icon, which is the app's own answer to what it looks like.
    menus?: Array<{ icon?: string }>;
    // The chronicle entry for the version being offered. Its summary is the
    // one sentence that makes an update a decision rather than a badge.
    release_notes?: ManifestReleaseNotes;
  };
}

/**
 * How a release reads at a glance.
 *
 * Only breaking and security get a colour. An update that changes how the app
 * behaves, or that closes a hole, is one somebody has to plan around; a feature
 * or a fix is one they can take on a Tuesday. Colouring all five would say
 * nothing — everything urgent means nothing is.
 */
const releaseTone: Partial<Record<NonNullable<ReleaseKind>, string>> = {
  breaking: "bg-rose-50 text-rose-700 border-rose-200",
  security: "bg-amber-50 text-amber-700 border-amber-200",
};


/**
 * An app's icon in the store, from the app's own manifest.
 *
 * There used to be a table here mapping three slugs to three icons. All three —
 * contacts, products, inventory — had left for business-gerege-nexus, so every
 * app in this catalogue already rendered the fallback and the table drew
 * nothing at all. An app's icon is the app's to declare, and it already does:
 * the first menu entry it registers names one, and that is the icon the sidebar
 * draws it with too.
 */
const appIcon = (app: AppItem) => app.manifest?.menus?.find((m) => m.icon)?.icon || "boxes";

/**
 * How the catalogue is laid out.
 *
 * Cards are the right shape for browsing something you have not seen before —
 * an icon, a sentence, room for the description to breathe. They are the wrong
 * shape for an operator who knows exactly which of nine apps they came for, and
 * who has to scroll past three screens of whitespace to reach it. Rows are that
 * second reading, and which one somebody prefers is a habit rather than a
 * decision, so it is remembered.
 */
type ViewMode = "grid" | "list";

const VIEW_STORAGE_KEY = "gerege_apps_view";

export default function AppStorePage() {
  const { t, locale } = useI18n();
  const [apps, setApps] = useState<AppItem[]>([]);
  const [search, setSearch] = useState("");
  const [selectedCategory, setSelectedCategory] = useState("All");
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);
  // Server and first client render must agree, so the stored preference is
  // applied in an effect rather than during initial state — the same rule the
  // sidebar follows for its collapsed groups.
  const [view, setView] = useState<ViewMode>("grid");
  // Which app's timeline is open, by slug. Null is closed.
  const [historyFor, setHistoryFor] = useState<string | null>(null);

  useEffect(() => {
    if (window.localStorage.getItem(VIEW_STORAGE_KEY) === "list") setView("list");
  }, []);

  const chooseView = (next: ViewMode) => {
    window.localStorage.setItem(VIEW_STORAGE_KEY, next);
    setView(next);
  };

  const loadApps = useCallback(async () => {
    try {
      const data = await api.getStoreApps();
      setApps(data || []);
    } catch (err: any) {
      setMessage({ type: "error", text: err.message || t("app_store.message.load_failed") });
    } finally {
      setLoading(false);
    }
  }, [t]);

  // The store's copy is translated by the API, so a language change means
  // fetching the catalogue again rather than re-rendering what is held. `t`
  // changes identity with the locale and nothing else, so depending on loadApps
  // says exactly that — the effect used to name `locale` and quietly leave out
  // the function it calls.
  useEffect(() => {
    setLoading(true);
    setSelectedCategory("All");
    void loadApps();
  }, [loadApps]);

  const handleInstall = async (app: AppItem) => {
    setActionLoading(app.slug);
    setMessage(null);
    try {
      await api.installApp(app.slug);
      setMessage({ type: "success", text: t("app_store.message.install_succeeded", { app: app.name }) });
      await loadApps();
    } catch (err: any) {
      setMessage({ type: "error", text: err.message || t("app_store.message.install_failed", { app: app.name }) });
    } finally {
      setActionLoading(null);
    }
  };

  // Updating is its own button rather than a second meaning for Install: the
  // server refuses an upgrade that has nothing to move to (409), and a screen
  // that sent "install" again would have shown that refusal as a failure.
  const handleUpdate = async (app: AppItem) => {
    setActionLoading(app.slug);
    setMessage(null);
    try {
      await api.upgradeApp(app.slug);
      setMessage({
        type: "success",
        text: t("app_store.message.update_succeeded", { app: app.name, version: app.latest_version }),
      });
      await loadApps();
    } catch (err: any) {
      setMessage({ type: "error", text: err.message || t("app_store.message.update_failed", { app: app.name }) });
    } finally {
      setActionLoading(null);
    }
  };

  const handleToggleState = async (app: AppItem) => {
    setActionLoading(app.slug);
    setMessage(null);
    try {
      if (app.enabled) {
        await api.disableApp(app.slug);
        setMessage({ type: "success", text: t("app_store.message.disabled", { app: app.name }) });
      } else {
        await api.enableApp(app.slug);
        setMessage({ type: "success", text: t("app_store.message.enabled", { app: app.name }) });
      }
      await loadApps();
    } catch (err: any) {
      setMessage({ type: "error", text: err.message || t("app_store.message.action_failed") });
    } finally {
      setActionLoading(null);
    }
  };

  // Chips and buttons are written once and read in both layouts. Two copies
  // would have been shorter today and different by the second change.
  const renderChips = (app: AppItem) => (
    <div className="flex flex-wrap items-center justify-end gap-2">
      {/* Both versions, and only when they differ: an installation that is
          current has one version, and printing it twice would read as a
          pending change. */}
      <span
        className="text-xs bg-surface-2 text-muted font-semibold px-2 py-0.5 rounded"
        title={
          app.update_available
            ? `${t("app_store.field.installed_version")}: ${app.installed_version} · ${t("app_store.field.latest_version")}: ${app.latest_version}`
            : `${t("app_store.field.latest_version")}: ${app.latest_version}`
        }
      >
        {app.update_available ? `v${app.installed_version} → v${app.latest_version}` : `v${app.version}`}
      </span>
      {app.installed && app.update_available && (
        <span className="text-xs font-semibold px-2 py-0.5 rounded bg-indigo-100 text-indigo-700">
          {t("app_store.state.update_available")}
        </span>
      )}
      {app.installed && (
        <span
          className={`text-xs font-semibold px-2 py-0.5 rounded ${
            app.enabled ? "bg-emerald-100 text-emerald-700" : "bg-amber-100 text-amber-700"
          }`}
        >
          {app.enabled ? t("app_store.state.installed") : t("app_store.state.disabled")}
        </span>
      )}
    </div>
  );

  /**
   * What the version on offer changed.
   *
   * Shown only where it is actionable: on an installed app with an update
   * waiting. On an app nobody has installed, the latest release note is a
   * fact about a product the reader has never run, and it would crowd out the
   * description — which is the sentence that actually helps them decide.
   */
  const renderReleaseNote = (app: AppItem) => {
    const notes = app.manifest.release_notes;
    const summary = notes?.summary;
    if (!app.installed || !app.update_available || !summary) return null;
    // The server resolves nothing here — this is the raw manifest — so the
    // fallback is the platform's own: asked-for language, then the source, then
    // English. A note reaching this point always has mn and en.
    const line = summary[locale] || summary.mn || summary.en;
    if (!line) return null;
    const tone = notes?.kind ? releaseTone[notes.kind] : undefined;
    const kindLabel =
      notes?.kind === "breaking"
        ? t("app_store.release_kind.breaking")
        : notes?.kind === "security"
          ? t("app_store.release_kind.security")
          : "";
    return (
      <p className="text-xs text-muted mt-1 flex items-start gap-1.5">
        <Sparkles className="w-3.5 h-3.5 text-indigo-500 shrink-0 mt-px" />
        <span className="min-w-0">
          <span className="font-semibold text-foreground">{t("app_store.field.whats_new")}</span>{" "}
          {line}
          {tone && kindLabel && (
            <span className={`ml-1.5 border px-1 py-px rounded text-[10px] font-semibold ${tone}`}>
              {kindLabel}
            </span>
          )}
        </span>
      </p>
    );
  };

  // In a card the buttons fill the width and sit under a rule; in a row they
  // are as wide as their words and sit at the end of the line.
  const renderActions = (app: AppItem, mode: ViewMode) => {
    const width = mode === "grid" ? "w-full" : "";
    return (
      <div
        className={
          mode === "grid"
            ? "pt-3 border-t border-line flex items-center justify-end gap-2"
            : "flex items-center justify-end gap-2 shrink-0"
        }
      >
        {!app.installed ? (
          <button
            onClick={() => handleInstall(app)}
            disabled={actionLoading === app.slug}
            className={`${width} bg-indigo-600 hover:bg-indigo-700 text-white font-medium text-sm py-2 px-4 rounded-lg flex items-center justify-center space-x-2 transition disabled:opacity-50`}
          >
            <Download className="w-4 h-4" />
            <span>
              {actionLoading === app.slug ? t("app_store.message.installing") : t("app_store.action.install")}
            </span>
          </button>
        ) : (
          <>
            {/* Offered only on an installed app: a history is the publisher's
                releases interleaved with this organisation's dealings, and an
                app nobody has installed has only the first half — which the
                card's release note already shows. */}
            <button
              onClick={() => setHistoryFor(app.slug)}
              className={`${width} bg-surface hover:bg-surface-hover text-foreground border border-line font-medium text-sm py-2 px-4 rounded-lg flex items-center justify-center space-x-2 transition`}
            >
              <History className="w-4 h-4" />
              <span>{t("app_store.action.history")}</span>
            </button>
            {/* Update sits beside enable/disable rather than replacing it: a
                tenant that has deliberately switched an app off should still be
                able to bring it up to date. */}
            {app.update_available && (
              <button
                onClick={() => handleUpdate(app)}
                disabled={actionLoading === app.slug}
                className={`${width} bg-indigo-600 hover:bg-indigo-700 text-white font-medium text-sm py-2 px-4 rounded-lg flex items-center justify-center space-x-2 transition disabled:opacity-50`}
              >
                <ArrowUpCircle className="w-4 h-4" />
                <span>
                  {actionLoading === app.slug ? t("app_store.message.updating") : t("app_store.action.update")}
                </span>
              </button>
            )}
            <button
              onClick={() => handleToggleState(app)}
              disabled={actionLoading === app.slug}
              className={`${width} font-medium text-sm py-2 px-4 rounded-lg flex items-center justify-center space-x-2 transition border ${
                app.enabled
                  ? "bg-surface hover:bg-red-50 text-red-600 border-red-200"
                  : "bg-emerald-600 hover:bg-emerald-700 text-white border-emerald-600"
              }`}
            >
              {app.enabled ? (
                <>
                  <PowerOff className="w-4 h-4" />
                  <span>{t("app_store.action.disable")}</span>
                </>
              ) : (
                <>
                  <Power className="w-4 h-4" />
                  <span>{t("app_store.action.enable")}</span>
                </>
              )}
            </button>
          </>
        )}
      </div>
    );
  };

  const categories = ["All", ...Array.from(new Set(apps.map((a) => a.category)))];

  const filteredApps = apps.filter((app) => {
    const matchesSearch =
      app.name.toLowerCase().includes(search.toLowerCase()) ||
      app.description.toLowerCase().includes(search.toLowerCase());
    const matchesCategory = selectedCategory === "All" || app.category === selectedCategory;
    return matchesSearch && matchesCategory;
  });

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-line pb-4">
        <div>
          <h1 className="text-2xl font-semibold text-foreground">{t("app_store.view.title")}</h1>
          <p className="text-sm text-muted">{t("app_store.view.subtitle")}</p>
        </div>

        {/* Search & Category */}
        <div className="flex items-center space-x-3">
          <div className="relative">
            <Search className="w-4 h-4 absolute left-3 top-2.5 text-muted" />
            <input
              type="text"
              placeholder={t("app_store.view.search_placeholder")}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9 pr-3 py-1.5 text-sm border border-input rounded-lg w-64 bg-surface"
            />
          </div>

          <select
            value={selectedCategory}
            onChange={(e) => setSelectedCategory(e.target.value)}
            className="px-3 py-1.5 text-sm border border-input rounded-lg bg-surface"
          >
            {categories.map((c) => (
              <option key={c} value={c}>
                {c === "All" ? t("app_store.filter.all") : c}
              </option>
            ))}
          </select>

          {/* Two buttons rather than a third dropdown entry: it is one choice
              with two answers, and the icons say which is which without being
              read. aria-pressed rather than a label change, so a screen reader
              hears the state instead of a button that renames itself. */}
          <div className="flex items-center gap-1 p-1 bg-surface-2 rounded-lg border border-line">
            {([
              { mode: "grid" as const, icon: <LayoutGrid className="w-4 h-4" />, label: t("app_store.action.view_grid") },
              { mode: "list" as const, icon: <Rows3 className="w-4 h-4" />, label: t("app_store.action.view_list") },
            ]).map((option) => (
              <button
                key={option.mode}
                type="button"
                onClick={() => chooseView(option.mode)}
                aria-pressed={view === option.mode}
                aria-label={option.label}
                title={option.label}
                className={`p-1.5 rounded-md transition ${
                  view === option.mode
                    ? "bg-surface text-indigo-600 shadow-sm"
                    : "text-muted hover:text-foreground"
                }`}
              >
                {option.icon}
              </button>
            ))}
          </div>
        </div>
      </div>

      {historyFor && <AppHistory slug={historyFor} onClose={() => setHistoryFor(null)} />}

      {/* Notifications */}
      {message && (
        <Banner tone={message.type} message={message.text} onDismiss={() => setMessage(null)} />
      )}

      {/* App Cards Grid */}
      {loading ? (
        <div className="py-12 text-center text-muted text-sm">{t("app_store.message.loading")}</div>
      ) : filteredApps.length === 0 ? (
        <div className="py-12 text-center text-muted text-sm">{t("app_store.message.no_match")}</div>
      ) : view === "list" ? (
        <div className="bg-surface border border-line rounded-xl divide-y divide-line">
          {filteredApps.map((app) => (
            <div key={app.id} className="p-4 flex flex-wrap items-center gap-4">
              <div className="p-2 bg-surface-2 rounded-lg border border-line shrink-0">
                <MenuIcon name={appIcon(app)} className="w-8 h-8 text-indigo-500" />
              </div>
              {/* min-w-0 so the description truncates instead of pushing the
                  buttons off the end of the row. */}
              <div className="flex-1 min-w-56">
                <div className="flex items-baseline gap-2">
                  <h2 className="font-semibold text-foreground">{app.name}</h2>
                  <span className="text-xs font-medium text-indigo-600">{app.category}</span>
                  {/* Said out loud, because an app in this list looks exactly
                      like one every platform can get. Whoever is deciding to
                      install it should know it arrived by arrangement — and
                      that the platform next door does not see it. */}
                  {app.visibility === "private" && (
                    <span className="inline-flex items-center gap-1 rounded-full bg-amber-50 px-2 py-0.5 text-[11px] font-semibold text-amber-700 ring-1 ring-amber-200">
                      <Lock className="w-3 h-3" />
                      {t("app_store.label.private")}
                    </span>
                  )}
                </div>
                <p className="text-sm text-muted truncate">{app.description}</p>
                {renderReleaseNote(app)}
                {app.manifest.dependencies && app.manifest.dependencies.length > 0 && (
                  <p className="text-xs text-muted mt-0.5">
                    <span className="font-semibold text-foreground">{t("app_store.field.requires")}</span>
                    {app.manifest.dependencies.map((d) => d.id.replace("io.gerege.nexus.", "")).join(", ")}
                  </p>
                )}
              </div>
              {renderChips(app)}
              {renderActions(app, "list")}
            </div>
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filteredApps.map((app) => (
            <div
              key={app.id}
              className="bg-surface border border-line rounded-xl p-5 hover:shadow-md transition flex flex-col justify-between"
            >
              <div>
                <div className="flex items-start justify-between mb-3">
                  <div className="p-2.5 bg-surface-2 rounded-xl border border-line">
                    <MenuIcon name={appIcon(app)} className="w-8 h-8 text-indigo-500" />
                  </div>
                  {renderChips(app)}
                </div>

                <h2 className="text-lg font-semibold text-foreground">{app.name}</h2>
                <p className="text-xs font-medium text-indigo-600 mb-2">{app.category}</p>
                <p className="text-sm text-muted line-clamp-2">{app.description}</p>
                {renderReleaseNote(app)}
                <div className="mb-4" />

                {/* Dependencies info */}
                {app.manifest.dependencies && app.manifest.dependencies.length > 0 && (
                  <div className="mb-4 text-xs text-muted bg-surface-2 p-2.5 rounded-lg border border-line">
                    <span className="font-semibold text-foreground">{t("app_store.field.requires")}</span>
                    {app.manifest.dependencies.map((d) => d.id.replace("io.gerege.nexus.", "")).join(", ")}
                  </div>
                )}
              </div>

              {renderActions(app, "grid")}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
