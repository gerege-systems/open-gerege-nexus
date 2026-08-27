"use client";

import React, { useCallback, useEffect, useState } from "react";
import { Integration, IntegrationProvider, api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Banner, LoadingBlock, Modal, fieldClass } from "@/components/ui";
import { AdminOnly, useAccess } from "@/lib/permissions";
import {
  Activity, AlertTriangle, CheckCircle2, Cloud, Globe, HardDrive, Link2, Plus,
  RefreshCw, Share2, ShieldAlert, Trash2, Unlink, Video,
} from "lucide-react";

/**
 * Two kinds of connector share this screen.
 *
 * A webhook or REST endpoint is a URL and a signing secret an operator types
 * in. A Google Drive, Dropbox or Meet connector is an *account*, reached by
 * sending the administrator through the provider's consent screen — so it has
 * no URL field, and the row is not usable until it says Connected.
 */

const PROVIDER_ICONS: Record<IntegrationProvider, React.ReactNode> = {
  webhook: <Globe className="w-5 h-5" />,
  government: <Globe className="w-5 h-5" />,
  payment: <Globe className="w-5 h-5" />,
  custom_rest: <Globe className="w-5 h-5" />,
  google_drive: <HardDrive className="w-5 h-5" />,
  dropbox: <Cloud className="w-5 h-5" />,
  google_meet: <Video className="w-5 h-5" />,
};

const PROVIDER_LABEL_KEYS = {
  webhook: "integrations.type.webhook",
  government: "integrations.type.government_gateway",
  payment: "integrations.type.payment_gateway",
  custom_rest: "integrations.type.custom_rest",
  google_drive: "integrations.type.google_drive",
  dropbox: "integrations.type.dropbox",
  google_meet: "integrations.type.google_meet",
} as const;

const OAUTH_PROVIDERS: IntegrationProvider[] = ["google_drive", "dropbox", "google_meet"];

type ProviderInfo = {
  provider: IntegrationProvider;
  oauth: boolean;
  capabilities: string[];
  available: boolean;
  reason?: string;
};

const emptyForm = {
  provider: "webhook" as IntegrationProvider,
  name: "",
  target_url: "",
  secret_key: "",
  folder: "",
  auto_export: true,
  calendar_id: "",
};

export default function IntegrationsPage() {
  const { t } = useI18n();
  const [integrations, setIntegrations] = useState<Integration[]>([]);
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [encryptionReady, setEncryptionReady] = useState(true);
  const [loading, setLoading] = useState(true);
  const { loading: checking, isAdmin } = useAccess();
  const [showModal, setShowModal] = useState(false);
  const [banner, setBanner] = useState<{ kind: "ok" | "error"; text: string } | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [form, setForm] = useState(emptyForm);

  const report = useCallback((err: unknown, fallback: string) => {
    const message = err instanceof Error && err.message ? err.message : fallback;
    setBanner({ kind: "error", text: message });
  }, []);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [list, catalog] = await Promise.all([
        api.getIntegrations(),
        api.getIntegrationProviders(),
      ]);
      setIntegrations(list || []);
      setProviders(catalog.providers || []);
      setEncryptionReady(catalog.encryption_configured);
    } catch (err) {
      // Reported rather than swallowed: an empty list reads as "you have no
      // integrations" to a member who simply is not allowed to see them.
      report(err, t("integrations.message.load_failed"));
    } finally {
      setLoading(false);
    }
  }, [report, t]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  // The OAuth callback returns the browser here with the outcome in the query.
  // Reading it once and clearing it keeps a reload from repeating the banner.
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const connected = params.get("connected");
    if (!connected) return;
    if (connected === "1") {
      setBanner({ kind: "ok", text: t("integrations.message.connected", { name: params.get("name") || "" }) });
    } else {
      setBanner({ kind: "error", text: params.get("reason") || t("integrations.message.connect_failed") });
    }
    window.history.replaceState({}, "", window.location.pathname);
  }, [t]);

  const providerInfo = (provider: IntegrationProvider) => providers.find((p) => p.provider === provider);
  const isOAuth = OAUTH_PROVIDERS.includes(form.provider);
  const selected = providerInfo(form.provider);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setBanner(null);
    try {
      const config: Record<string, string> = {};
      if (form.provider === "google_drive" && form.folder) config.folder_id = form.folder;
      if (form.provider === "dropbox" && form.folder) config.folder_path = form.folder;
      if (form.provider === "google_meet" && form.calendar_id) config.calendar_id = form.calendar_id;
      if (form.provider === "google_drive" || form.provider === "dropbox") {
        config.auto_export = String(form.auto_export);
      }

      await api.registerIntegration({
        provider: form.provider,
        name: form.name,
        target_url: isOAuth ? undefined : form.target_url,
        secret_key: isOAuth ? undefined : form.secret_key || undefined,
        config,
      });
      setShowModal(false);
      setForm(emptyForm);
      await loadData();
    } catch (err) {
      report(err, t("integrations.message.register_failed"));
    }
  }

  async function handleConnect(item: Integration) {
    setBusy(item.id);
    try {
      const { authorization_url } = await api.connectIntegration(item.id);
      // A full navigation, not a popup: the consent screen is the provider's
      // own page and some of them refuse to render inside a frame.
      window.location.assign(authorization_url);
    } catch (err) {
      report(err, t("integrations.message.connect_failed"));
      setBusy(null);
    }
  }

  async function handleDisconnect(item: Integration) {
    setBusy(item.id);
    try {
      await api.disconnectIntegration(item.id);
      await loadData();
    } catch (err) {
      report(err, t("integrations.message.disconnect_failed"));
    } finally {
      setBusy(null);
    }
  }

  async function handleDelete(item: Integration) {
    if (!window.confirm(t("integrations.message.confirm_delete", { name: item.name }))) return;
    setBusy(item.id);
    try {
      await api.deleteIntegration(item.id);
      await loadData();
    } catch (err) {
      report(err, t("integrations.message.delete_failed"));
    } finally {
      setBusy(null);
    }
  }

  // The endpoints behind this screen are administrator-only, so a member
  // without those rights is told as much rather than shown an empty list.
  if (!checking && !isAdmin) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold text-slate-900 flex items-center space-x-2">
          <Share2 className="w-7 h-7 text-indigo-600" />
          <span>{t("integrations.view.title")}</span>
        </h1>
        <AdminOnly />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900 flex items-center space-x-2">
            <Share2 className="w-7 h-7 text-indigo-600" />
            <span>{t("integrations.view.title")}</span>
          </h1>
          <p className="text-sm text-slate-500 mt-1">{t("integrations.view.subtitle")}</p>
        </div>
        <div className="flex items-center space-x-2">
          <button
            onClick={() => void loadData()}
            aria-label={t("base.action.retry")}
            className="p-2 text-slate-600 hover:bg-slate-100 rounded-lg border border-slate-200 transition"
          >
            <RefreshCw className="w-4 h-4" />
          </button>
          <button
            onClick={() => setShowModal(true)}
            className="bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-semibold px-4 py-2 rounded-lg flex items-center space-x-2 shadow-sm transition"
          >
            <Plus className="w-4 h-4" />
            <span>{t("integrations.action.create")}</span>
          </button>
        </div>
      </div>

      {banner && (
        <Banner tone={banner.kind === "ok" ? "success" : "error"} message={banner.text} />
      )}

      {/* Without a key the server refuses to store a credential, so say that
          here rather than letting the save fail with the same message. */}
      {!encryptionReady && (
        <Banner tone="warning" message={t("integrations.message.encryption_missing")} />
      )}

      {loading ? (
        <LoadingBlock label={t("integrations.message.loading")} />
      ) : integrations.length === 0 ? (
        <div className="bg-white border border-slate-200 rounded-xl p-12 text-center text-slate-500 text-sm">
          {t("integrations.message.empty")}
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {integrations.map((item) => {
            const oauth = OAUTH_PROVIDERS.includes(item.provider);
            // A connector that is switched on but whose last attempt failed is
            // shown as failing without being shown as off: the server keeps
            // trying it, and the two states have different remedies. status is
            // the administrator's switch; last_error is how it went.
            const health = item.status !== "ACTIVE" ? "off" : item.last_error ? "failing" : "ok";
            return (
              <div
                key={item.id}
                className="bg-white p-5 rounded-xl border border-slate-200 shadow-sm flex flex-col justify-between gap-3"
              >
                <div>
                  <div className="flex items-start justify-between mb-3 gap-3">
                    <div className="flex items-center space-x-3 min-w-0">
                      <div className="p-2.5 bg-indigo-50 text-indigo-600 rounded-lg shrink-0">
                        {PROVIDER_ICONS[item.provider]}
                      </div>
                      <div className="min-w-0">
                        <h3 className="font-bold text-slate-900 text-base truncate">{item.name}</h3>
                        <span className="text-[11px] bg-slate-100 text-slate-600 px-2 py-0.5 rounded">
                          {t(PROVIDER_LABEL_KEYS[item.provider])}
                        </span>
                      </div>
                    </div>
                    <span
                      className={`inline-flex items-center space-x-1 text-xs font-bold px-2.5 py-1 rounded-full shrink-0 ${
                        health === "ok"
                          ? "bg-emerald-50 text-emerald-700 border border-emerald-200"
                          : health === "failing"
                            ? "bg-red-50 text-red-700 border border-red-200"
                            : "bg-amber-50 text-amber-700 border border-amber-200"
                      }`}
                    >
                      {health === "ok" ? (
                        <CheckCircle2 className="w-3.5 h-3.5" />
                      ) : health === "failing" ? (
                        <AlertTriangle className="w-3.5 h-3.5" />
                      ) : (
                        <ShieldAlert className="w-3.5 h-3.5" />
                      )}
                      <span>{health === "failing" ? "ERROR" : item.status}</span>
                    </span>
                  </div>

                  <div className="text-xs text-slate-500 space-y-1 bg-slate-50 p-2.5 rounded-lg border border-slate-100">
                    {oauth ? (
                      <div className="truncate">
                        {item.connected
                          ? t("integrations.field.connected_account") + ": " + (item.account_label || "—")
                          : t("integrations.message.not_connected")}
                      </div>
                    ) : (
                      <div className="truncate font-mono">{item.target_url}</div>
                    )}
                    {item.config?.auto_export === "true" && (
                      <div className="text-emerald-700">{t("integrations.message.auto_export_on")}</div>
                    )}
                    {item.last_ping_at && (
                      <div className="flex items-center gap-1">
                        <Activity className="w-3 h-3" />
                        {new Date(item.last_ping_at).toLocaleString()}
                      </div>
                    )}
                    {item.last_error && <div className="text-red-600 break-words">{item.last_error}</div>}
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  {oauth &&
                    (item.connected ? (
                      <button
                        onClick={() => void handleDisconnect(item)}
                        disabled={busy === item.id}
                        className="flex-1 flex items-center justify-center gap-1.5 border border-slate-300 text-slate-700 hover:bg-slate-50 text-xs font-semibold py-2 rounded-lg disabled:opacity-50"
                      >
                        <Unlink className="w-3.5 h-3.5" />
                        {t("integrations.action.disconnect")}
                      </button>
                    ) : (
                      <button
                        onClick={() => void handleConnect(item)}
                        disabled={busy === item.id}
                        className="flex-1 flex items-center justify-center gap-1.5 bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-semibold py-2 rounded-lg disabled:opacity-50"
                      >
                        <Link2 className="w-3.5 h-3.5" />
                        {t("integrations.action.connect")}
                      </button>
                    ))}
                  <button
                    onClick={() => void handleDelete(item)}
                    disabled={busy === item.id}
                    aria-label={t("base.action.delete")}
                    className="p-2 border border-slate-300 text-red-600 hover:bg-red-50 rounded-lg disabled:opacity-50"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {showModal && (
        <Modal className="max-h-[90vh] overflow-y-auto" label={t("integrations.view.create_title")}>
          <h2 className="text-xl font-bold text-slate-900 mb-4">{t("integrations.view.create_title")}</h2>
          <form onSubmit={handleCreate} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">
                {t("integrations.field.type")}
              </label>
              <select
                value={form.provider}
                onChange={(e) => setForm({ ...form, provider: e.target.value as IntegrationProvider })}
                className={fieldClass}
              >
                {(Object.keys(PROVIDER_LABEL_KEYS) as IntegrationProvider[]).map((provider) => {
                  const info = providerInfo(provider);
                  return (
                    <option key={provider} value={provider} disabled={info ? !info.available : false}>
                      {t(PROVIDER_LABEL_KEYS[provider])}
                      {info && !info.available ? " — " + t("integrations.state.unavailable") : ""}
                    </option>
                  );
                })}
              </select>
              {selected && !selected.available && (
                <p className="mt-1 text-[11px] text-amber-700">{selected.reason}</p>
              )}
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">
                {t("integrations.field.name")} *
              </label>
              <input
                type="text"
                placeholder={t("integrations.field.name_placeholder")}
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                className={fieldClass}
                required
              />
            </div>

            {!isOAuth && (
              <>
                <div>
                  <label className="block text-xs font-semibold text-slate-700 mb-1">
                    {t("integrations.field.target_url")} *
                  </label>
                  <input
                    type="url"
                    placeholder="https://api.example.com/webhooks"
                    value={form.target_url}
                    onChange={(e) => setForm({ ...form, target_url: e.target.value })}
                    className={fieldClass}
                    required
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-700 mb-1">
                    {t("integrations.field.secret")}
                  </label>
                  <input
                    type="password"
                    placeholder={t("integrations.field.secret_placeholder")}
                    value={form.secret_key}
                    onChange={(e) => setForm({ ...form, secret_key: e.target.value })}
                    className={fieldClass}
                  />
                </div>
              </>
            )}

            {(form.provider === "google_drive" || form.provider === "dropbox") && (
              <>
                <div>
                  <label className="block text-xs font-semibold text-slate-700 mb-1">
                    {form.provider === "google_drive"
                      ? t("integrations.field.drive_folder")
                      : t("integrations.field.dropbox_folder")}
                  </label>
                  <input
                    type="text"
                    placeholder={
                      form.provider === "google_drive"
                        ? t("integrations.field.drive_folder_placeholder")
                        : t("integrations.field.dropbox_folder_placeholder")
                    }
                    value={form.folder}
                    onChange={(e) => setForm({ ...form, folder: e.target.value })}
                    className={fieldClass}
                  />
                </div>
                <label className="flex items-start gap-2 text-xs text-slate-700">
                  <input
                    type="checkbox"
                    checked={form.auto_export}
                    onChange={(e) => setForm({ ...form, auto_export: e.target.checked })}
                    className="mt-0.5"
                  />
                  <span>{t("integrations.field.auto_export")}</span>
                </label>
              </>
            )}

            {form.provider === "google_meet" && (
              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">
                  {t("integrations.field.calendar_id")}
                </label>
                <input
                  type="text"
                  placeholder="primary"
                  value={form.calendar_id}
                  onChange={(e) => setForm({ ...form, calendar_id: e.target.value })}
                  className={fieldClass}
                />
                <p className="mt-1 text-[11px] text-slate-500">{t("integrations.message.meet_via_calendar")}</p>
              </div>
            )}

            {isOAuth && <p className="text-[11px] text-slate-500">{t("integrations.message.connect_after_save")}</p>}

            <div className="flex items-center space-x-2 pt-2">
              <button
                type="button"
                onClick={() => setShowModal(false)}
                className="w-1/2 bg-slate-100 hover:bg-slate-200 text-slate-700 font-medium py-2 rounded-lg text-xs"
              >
                {t("base.action.cancel")}
              </button>
              <button
                type="submit"
                className="w-1/2 bg-indigo-600 hover:bg-indigo-700 text-white font-medium py-2 rounded-lg text-xs"
              >
                {t("integrations.action.register")}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </div>
  );
}
