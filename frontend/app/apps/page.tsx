"use client";

import React, { useCallback, useEffect, useState } from "react";
import { api, type ManifestReleaseNotes, type ReleaseKind } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import {
  Alert,
  Badge,
  Button,
  Card,
  EmptyState,
  IconButton,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  Spinner,
} from "@gerege-systems/ui";
import AppHistory from "@/components/AppHistory";
import {
  Download,
  Power,
  PowerOff,
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
const releaseTone: Partial<Record<NonNullable<ReleaseKind>, "danger" | "warning">> = {
  breaking: "danger",
  security: "warning",
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
      <Badge
        variant="subtle"
        tone="neutral"
        title={
          app.update_available
            ? `${t("app_store.field.installed_version")}: ${app.installed_version} · ${t("app_store.field.latest_version")}: ${app.latest_version}`
            : `${t("app_store.field.latest_version")}: ${app.latest_version}`
        }
      >
        {app.update_available ? `v${app.installed_version} → v${app.latest_version}` : `v${app.version}`}
      </Badge>
      {app.installed && app.update_available && (
        <Badge variant="subtle" tone="accent">
          {t("app_store.state.update_available")}
        </Badge>
      )}
      {app.installed && (
        <Badge variant="subtle" tone={app.enabled ? "success" : "warning"} dot>
          {app.enabled ? t("app_store.state.installed") : t("app_store.state.disabled")}
        </Badge>
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
        <Sparkles className="w-3.5 h-3.5 text-accent shrink-0 mt-px" aria-hidden="true" />
        <span className="min-w-0">
          <span className="font-semibold text-foreground">{t("app_store.field.whats_new")}</span>{" "}
          {line}
          {tone && kindLabel && (
            <Badge variant="outline" tone={tone} className="ms-1.5">
              {kindLabel}
            </Badge>
          )}
        </span>
      </p>
    );
  };

  // In a card the buttons fill the width and sit under a rule; in a row they
  // are as wide as their words and sit at the end of the line.
  const renderActions = (app: AppItem, mode: ViewMode) => {
    const width = mode === "grid" ? "w-full" : "";
    const busy = actionLoading === app.slug;
    return (
      <div
        className={
          mode === "grid"
            ? "pt-3 border-t border-line flex items-center justify-end gap-2"
            : "flex items-center justify-end gap-2 shrink-0"
        }
      >
        {!app.installed ? (
          <Button className={width} loading={busy} leadingIcon={<Download />} onClick={() => handleInstall(app)}>
            {busy ? t("app_store.message.installing") : t("app_store.action.install")}
          </Button>
        ) : (
          <>
            {/* Offered only on an installed app: a history is the publisher's
                releases interleaved with this organisation's dealings, and an
                app nobody has installed has only the first half — which the
                card's release note already shows. */}
            <Button variant="outline" className={width} leadingIcon={<History />} onClick={() => setHistoryFor(app.slug)}>
              {t("app_store.action.history")}
            </Button>
            {/* Update sits beside enable/disable rather than replacing it: a
                tenant that has deliberately switched an app off should still be
                able to bring it up to date. */}
            {app.update_available && (
              <Button className={width} loading={busy} leadingIcon={<ArrowUpCircle />} onClick={() => handleUpdate(app)}>
                {busy ? t("app_store.message.updating") : t("app_store.action.update")}
              </Button>
            )}
            <Button
              variant="outline"
              className={`${width} ${app.enabled ? "border-danger-border text-danger hover:bg-danger-soft" : "border-success-border text-success hover:bg-success-soft"}`}
              disabled={busy}
              leadingIcon={app.enabled ? <PowerOff /> : <Power />}
              onClick={() => handleToggleState(app)}
            >
              {app.enabled ? t("app_store.action.disable") : t("app_store.action.enable")}
            </Button>
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
        <div className="flex flex-wrap items-center gap-3">
          <Input
            type="search"
            label={t("app_store.view.search_placeholder")}
            hideLabel
            placeholder={t("app_store.view.search_placeholder")}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onClear={() => setSearch("")}
            className="w-64"
          />

          <Select value={selectedCategory} onValueChange={setSelectedCategory}>
            {/* Named after what it currently says, the way a native select is
                announced — there is no separate caption for this control. */}
            <SelectTrigger
              aria-label={selectedCategory === "All" ? t("app_store.filter.all") : selectedCategory}
              className="w-44"
            />
            <SelectContent>
              {/* An app with no category still shows under "All"; an empty
                  value cannot be a Select item. */}
              {categories.filter(Boolean).map((c) => (
                <SelectItem key={c} value={c}>
                  {c === "All" ? t("app_store.filter.all") : c}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          {/* Two buttons rather than a third dropdown entry: it is one choice
              with two answers, and the icons say which is which without being
              read. aria-pressed rather than a label change, so a screen reader
              hears the state instead of a button that renames itself. */}
          <div className="flex items-center gap-1 p-1 bg-surface-2 rounded-lg border border-line">
            {([
              { mode: "grid" as const, icon: <LayoutGrid />, label: t("app_store.action.view_grid") },
              { mode: "list" as const, icon: <Rows3 />, label: t("app_store.action.view_list") },
            ]).map((option) => (
              <IconButton
                key={option.mode}
                size="sm"
                variant={view === option.mode ? "secondary" : "ghost"}
                onClick={() => chooseView(option.mode)}
                aria-pressed={view === option.mode}
                aria-label={option.label}
                title={option.label}
                icon={option.icon}
                className={view === option.mode ? "text-accent" : "text-muted"}
              />
            ))}
          </div>
        </div>
      </div>

      {historyFor && <AppHistory slug={historyFor} onClose={() => setHistoryFor(null)} />}

      {/* Notifications */}
      {message && (
        <Alert variant={message.type === "error" ? "danger" : "success"} live dismissible onDismiss={() => setMessage(null)}>
          {message.text}
        </Alert>
      )}

      {/* App Cards Grid */}
      {loading ? (
        <div className="py-12 flex items-center justify-center gap-2 text-muted text-sm" role="status">
          <Spinner size="md" decorative /> {t("app_store.message.loading")}
        </div>
      ) : filteredApps.length === 0 ? (
        <EmptyState title={t("app_store.message.no_match")} className="py-12" />
      ) : view === "list" ? (
        <Card padding="none" className="divide-y divide-line">
          {filteredApps.map((app) => (
            <div key={app.id} className="p-4 flex flex-wrap items-center gap-4">
              <div className="p-2 bg-surface-2 rounded-lg border border-line shrink-0">
                <MenuIcon name={appIcon(app)} className="w-8 h-8 text-accent" />
              </div>
              {/* min-w-0 so the description truncates instead of pushing the
                  buttons off the end of the row. */}
              <div className="flex-1 min-w-56">
                <div className="flex items-baseline gap-2">
                  <h2 className="font-semibold text-foreground">{app.name}</h2>
                  <span className="text-xs font-medium text-accent">{app.category}</span>
                  {/* Said out loud, because an app in this list looks exactly
                      like one every platform can get. Whoever is deciding to
                      install it should know it arrived by arrangement — and
                      that the platform next door does not see it. */}
                  {app.visibility === "private" && (
                    <Badge variant="outline" tone="warning" icon={<Lock />}>
                      {t("app_store.label.private")}
                    </Badge>
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
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filteredApps.map((app) => (
            <Card key={app.id} padding="none" className="p-5 flex flex-col justify-between hover:border-line-strong">
              <div>
                <div className="flex items-start justify-between mb-3">
                  <div className="p-2.5 bg-surface-2 rounded-lg border border-line">
                    <MenuIcon name={appIcon(app)} className="w-8 h-8 text-accent" />
                  </div>
                  {renderChips(app)}
                </div>

                <h2 className="text-lg font-semibold text-foreground">{app.name}</h2>
                <p className="text-xs font-medium text-accent mb-2">{app.category}</p>
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
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
