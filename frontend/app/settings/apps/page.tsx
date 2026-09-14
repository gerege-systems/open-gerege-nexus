"use client";

import React, { useEffect, useState } from "react";
import { api, type StoreOverviewApp } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Settings, Clock, ArrowUpCircle, ShieldAlert, Pin, GitCompareArrows } from "lucide-react";
import {
  Alert,
  Badge,
  Button,
  Card,
  EmptyState,
  Spinner,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@gerege-systems/ui";
import { formatDay, formatMoment } from "@/lib/datetime";

interface InstalledApp {
  id: string;
  app_id: string;
  slug: string;
  name: string;
  installed_version: string;
  status: string;
  enabled: boolean;
  installed_at: string;
  auto_update: boolean;
  pinned_version?: string;
  latest_version?: string;
  update_available: boolean;
  held_for?: string[];
  held_reason?: string;
  core: boolean;
}

interface CatalogStatus {
  source: "file" | "registry";
  apps: number;
  sync_interval: string;
  last_sync_at?: string;
  last_sync_ok?: boolean;
  last_sync_error?: string;
}

export default function InstalledAppsSettingsPage() {
  const { t, locale } = useI18n();
  const [apps, setApps] = useState<InstalledApp[]>([]);
  const [status, setStatus] = useState<CatalogStatus | null>(null);
  // Rows where the compiled module and the catalogue disagree. Unlike an update
  // waiting or an app held back, this is nobody's decision — it means this
  // instance is serving a catalogue that does not describe the code it runs,
  // which from every other screen looks exactly like a healthy one.
  const [drifted, setDrifted] = useState<StoreOverviewApp[]>([]);
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  const loadInstalled = async () => {
    try {
      const data = await api.getInstalledApps();
      setApps(data || []);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
    // Administrator-only, and this screen is reachable by anyone who types the
    // address, so a refusal here is expected rather than a fault.
    try {
      setStatus(await api.getCatalogStatus());
      // Administrator-only, and this page already is. A failure here must not
      // cost the page its list of installed apps, so it is asked for on its own.
      try {
        const overview = await api.getStoreOverview();
        setDrifted((overview.apps || []).filter((row) => row.drifted));
      } catch {
        setDrifted([]);
      }
    } catch {
      setStatus(null);
    }
  };

  useEffect(() => {
    setLoading(true);
    loadInstalled();
  }, [locale]);

  // An app whose new version asks for more is held rather than applied, so
  // approving it is the same action as updating it: the server moves the pin
  // forward with the version.
  const handleUpdate = async (app: InstalledApp) => {
    setActionLoading(app.slug);
    try {
      await api.upgradeApp(app.slug);
      await loadInstalled();
    } catch (err) {
      console.error(err);
    } finally {
      setActionLoading(null);
    }
  };

  const handleAutoUpdate = async (app: InstalledApp) => {
    setActionLoading(app.slug);
    try {
      await api.setAutoUpdate(app.slug, !app.auto_update);
      await loadInstalled();
    } catch (err) {
      console.error(err);
    } finally {
      setActionLoading(null);
    }
  };

  const handleToggle = async (app: InstalledApp) => {
    setActionLoading(app.slug);
    try {
      if (app.enabled) {
        await api.disableApp(app.slug);
      } else {
        await api.enableApp(app.slug);
      }
      await loadInstalled();
    } catch (err) {
      console.error(err);
    } finally {
      setActionLoading(null);
    }
  };

  return (
    <div className="space-y-6">
      <div className="border-b border-line pb-4">
        <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
          <Settings className="w-6 h-6 text-muted" aria-hidden="true" />
          <span>{t("app_store.view.installed_title")}</span>
        </h1>
        <p className="text-sm text-muted">
          {t("app_store.view.installed_subtitle")}
        </p>
      </div>

      {status && (
        <Card padding="none" className="px-4 py-3 text-sm flex flex-wrap items-center gap-x-4 gap-y-1">
          <span className="font-semibold text-foreground">
            {status.source === "registry"
              ? t("app_store.state.source_registry")
              : t("app_store.state.source_file")}
          </span>
          <span className="text-muted">{t("app_store.field.app_count", { count: status.apps })}</span>
          {status.last_sync_at && (
            <span className="text-muted">
              {t("app_store.field.last_sync")}: {formatMoment(status.last_sync_at)}
            </span>
          )}
          {/* A failing registry is the thing nobody notices: the store keeps
              serving the catalogue it already has, so nothing looks wrong. */}
          {status.last_sync_error && (
            <span className="text-danger">
              {t("app_store.message.sync_failed")}: {status.last_sync_error}
            </span>
          )}
        </Card>
      )}

      {drifted.length > 0 && (
        <Alert variant="danger" icon={<GitCompareArrows />} title={t("app_store.overview.drifted")}>
          <p>{t("app_store.overview.drift_note")}</p>
          <ul className="mt-2 space-y-0.5">
            {drifted.map((row) => (
              <li key={row.app_id} className="font-mono text-xs">
                {row.name}: {t("app_store.overview.binary")} v{row.binary_version} ≠{" "}
                {t("app_store.overview.catalog")} v{row.catalog_version}
              </li>
            ))}
          </ul>
        </Alert>
      )}

      {loading ? (
        <div className="py-8 flex items-center gap-2 text-muted text-sm" role="status">
          <Spinner size="md" decorative /> {t("app_store.message.loading_installed")}
        </div>
      ) : apps.length === 0 ? (
        <Card padding="none">
          <EmptyState
            title={t("app_store.message.none_installed")}
            action={
              <Button asChild variant="outline">
                <a href="/apps">{t("app_store.action.browse_store")}</a>
              </Button>
            }
            className="py-10"
          />
        </Card>
      ) : (
        <Card padding="none" className="overflow-hidden">
          <Table containerClassName="rounded-none border-0">
            <TableHeader>
              <TableRow>
                <TableHead>{t("app_store.field.application_name")}</TableHead>
                <TableHead>{t("app_store.field.module_id")}</TableHead>
                <TableHead>{t("app_store.field.installed_version")}</TableHead>
                <TableHead>{t("app_store.field.updates")}</TableHead>
                <TableHead>{t("base.field.status")}</TableHead>
                <TableHead>{t("app_store.field.installed_date")}</TableHead>
                <TableHead align="right">{t("base.field.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {apps.map((app) => (
                <TableRow key={app.id}>
                  <TableCell className="font-semibold text-foreground">{app.name}</TableCell>
                  <TableCell className="font-mono text-xs text-muted">{app.app_id}</TableCell>
                  <TableCell className="font-semibold text-foreground whitespace-nowrap">
                    v{app.installed_version}
                    {app.update_available && app.latest_version && (
                      <span className="ms-1.5 text-xs font-normal text-accent">→ v{app.latest_version}</span>
                    )}
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-col gap-1.5 items-start">
                      {/* The switch, said in words rather than as a bare toggle:
                          this decides whether somebody else's release reaches
                          this organisation without anybody looking at it. */}
                      <Switch
                        size="sm"
                        checked={app.auto_update}
                        disabled={actionLoading === app.slug}
                        onCheckedChange={() => handleAutoUpdate(app)}
                        label={
                          <span className="text-xs font-semibold">
                            {app.auto_update ? t("app_store.state.auto_update_on") : t("app_store.state.auto_update_off")}
                          </span>
                        }
                      />
                      {app.pinned_version && (
                        <span className="inline-flex items-center gap-1 text-xs text-warning">
                          <Pin className="w-3 h-3" aria-hidden="true" />
                          {t("app_store.state.pinned", { version: app.pinned_version })}
                        </span>
                      )}
                      {(app.held_reason || (app.held_for && app.held_for.length > 0)) && (
                        <span className="inline-flex items-start gap-1 text-xs text-warning max-w-56">
                          <ShieldAlert className="w-3.5 h-3.5 shrink-0 mt-0.5" aria-hidden="true" />
                          <span>
                            {app.held_for && app.held_for.length > 0 ? (
                              <>
                                {t("app_store.message.held_for_approval")}{" "}
                                <span className="font-mono">{app.held_for.join(", ")}</span>
                              </>
                            ) : (
                              /* Held with nothing to itemise — the installed
                                 version's manifest predates the history, so what
                                 the new one adds cannot be established. Saying so
                                 is better than an app that silently stops moving. */
                              app.held_reason
                            )}
                          </span>
                        </span>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="subtle" tone={app.enabled ? "success" : "neutral"} dot>
                      {app.enabled ? t("base.state.active") : t("app_store.state.disabled")}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-xs text-muted">
                    <span className="inline-flex items-center gap-1 whitespace-nowrap">
                      <Clock className="w-3.5 h-3.5" aria-hidden="true" />
                      <span>{formatDay(app.installed_at)}</span>
                    </span>
                  </TableCell>
                  <TableCell align="right">
                    <div className="inline-flex items-center justify-end gap-2">
                      {app.update_available && (
                        <Button
                          size="sm"
                          disabled={actionLoading === app.slug}
                          leadingIcon={<ArrowUpCircle />}
                          onClick={() => handleUpdate(app)}
                        >
                          {/* Approving a held version and updating an ordinary
                              one are the same request; only the wording differs,
                              because only one of them is a decision. */}
                          {app.held_for && app.held_for.length > 0
                            ? t("app_store.action.approve_update")
                            : t("app_store.action.update")}
                        </Button>
                      )}
                      {/* Every app can be turned off, including the ones a new
                          organisation starts with: the platform underneath —
                          sign-in, the organisation's own profile, settings —
                          is not an app and does not appear in this list. */}
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={actionLoading === app.slug}
                        className={app.enabled ? "border-danger-border text-danger hover:bg-danger-soft" : "border-success-border text-success hover:bg-success-soft"}
                        onClick={() => handleToggle(app)}
                      >
                        {app.enabled ? t("app_store.action.disable") : t("app_store.action.enable")}
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}
    </div>
  );
}
