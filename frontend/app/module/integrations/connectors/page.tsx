"use client";

import React, { useCallback, useEffect, useState } from "react";
import { Integration, IntegrationProvider, api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import {
  Alert,
  Badge,
  Button,
  Card,
  Checkbox,
  ConfirmationDialog,
  DialogHeader,
  DialogTitle,
  EmptyState,
  IconButton,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  Skeleton,
} from "@gerege-systems/ui";
import { Modal } from "@/components/ui";
import { AdminOnly, useAccess } from "@/lib/permissions";
import {
  Activity, AlertTriangle, CheckCircle2, Cloud, Globe, HardDrive, Link2, Plus,
  RefreshCw, Share2, ShieldAlert, Trash2, Unlink, Video,
} from "lucide-react";
import { formatMoment } from "@/lib/datetime";

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
  const [deleting, setDeleting] = useState<Integration | null>(null);
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

  // Asked first in the dialog below; by the time this runs the answer was yes.
  async function handleDelete(item: Integration) {
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
        <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
          <Share2 className="w-7 h-7 text-accent" />
          <span>{t("integrations.view.title")}</span>
        </h1>
        <AdminOnly />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
            <Share2 className="w-7 h-7 text-accent" />
            <span>{t("integrations.view.title")}</span>
          </h1>
          <p className="text-sm text-muted mt-1">{t("integrations.view.subtitle")}</p>
        </div>
        <div className="flex items-center gap-2">
          <IconButton
            variant="outline"
            aria-label={t("base.action.retry")}
            icon={<RefreshCw />}
            onClick={() => void loadData()}
          />
          <Button leadingIcon={<Plus />} onClick={() => setShowModal(true)}>
            {t("integrations.action.create")}
          </Button>
        </div>
      </div>

      {banner && (
        <Alert variant={banner.kind === "ok" ? "success" : "danger"} live>{banner.text}</Alert>
      )}

      {/* Without a key the server refuses to store a credential, so say that
          here rather than letting the save fail with the same message. */}
      {!encryptionReady && (
        <Alert variant="warning">{t("integrations.message.encryption_missing")}</Alert>
      )}

      {loading ? (
        <div className="space-y-3 py-4" role="status" aria-live="polite" aria-busy="true">
          <span className="sr-only">{t("integrations.message.loading")}</span>
          {Array.from({ length: 4 }, (_, row) => (
            <div key={row} className="flex items-center gap-3">
              <Skeleton variant="circle" className="size-4 shrink-0" />
              <Skeleton className="h-4 flex-1" />
              <Skeleton className="h-4 w-24 shrink-0" />
            </div>
          ))}
        </div>
      ) : integrations.length === 0 ? (
        <Card padding="none">
          <EmptyState icon={<Share2 />} title={t("integrations.message.empty")} className="p-12" />
        </Card>
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
              <Card key={item.id} padding="none" className="p-5 flex flex-col justify-between gap-3">
                <div>
                  <div className="flex items-start justify-between mb-3 gap-3">
                    <div className="flex items-center gap-3 min-w-0">
                      <div className="p-2.5 bg-accent-soft text-accent rounded-lg shrink-0">
                        {PROVIDER_ICONS[item.provider]}
                      </div>
                      <div className="min-w-0">
                        <h3 className="font-semibold text-foreground text-base truncate">{item.name}</h3>
                        <Badge tone="neutral">{t(PROVIDER_LABEL_KEYS[item.provider])}</Badge>
                      </div>
                    </div>
                    <Badge
                      variant="outline"
                      className="shrink-0"
                      tone={health === "ok" ? "success" : health === "failing" ? "danger" : "warning"}
                      icon={health === "ok" ? <CheckCircle2 /> : health === "failing" ? <AlertTriangle /> : <ShieldAlert />}
                    >
                      {health === "failing" ? "ERROR" : item.status}
                    </Badge>
                  </div>

                  <div className="text-xs text-muted space-y-1 bg-surface-2 p-2.5 rounded-lg border border-line">
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
                      <div className="text-success">{t("integrations.message.auto_export_on")}</div>
                    )}
                    {item.last_ping_at && (
                      <div className="flex items-center gap-1">
                        <Activity className="w-3 h-3" />
                        {formatMoment(item.last_ping_at)}
                      </div>
                    )}
                    {item.last_error && <div className="text-danger wrap-break-word">{item.last_error}</div>}
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  {oauth &&
                    (item.connected ? (
                      <Button
                        variant="outline"
                        size="sm"
                        className="flex-1"
                        disabled={busy === item.id}
                        leadingIcon={<Unlink />}
                        onClick={() => void handleDisconnect(item)}
                      >
                        {t("integrations.action.disconnect")}
                      </Button>
                    ) : (
                      <Button
                        size="sm"
                        className="flex-1"
                        disabled={busy === item.id}
                        leadingIcon={<Link2 />}
                        onClick={() => void handleConnect(item)}
                      >
                        {t("integrations.action.connect")}
                      </Button>
                    ))}
                  <IconButton
                    variant="outline"
                    size="sm"
                    className="text-danger hover:bg-danger-soft"
                    disabled={busy === item.id}
                    aria-label={t("base.action.delete")}
                    icon={<Trash2 />}
                    onClick={() => setDeleting(item)}
                  />
                </div>
              </Card>
            );
          })}
        </div>
      )}

      {deleting && (
        <ConfirmationDialog
          open
          onOpenChange={(open) => { if (!open) setDeleting(null); }}
          title={deleting.name}
          description={t("integrations.message.confirm_delete", { name: deleting.name })}
          confirmLabel={t("base.action.delete")}
          cancelLabel={t("base.action.cancel")}
          confirmVariant="destructive"
          onConfirm={() => {
            const item = deleting;
            setDeleting(null);
            void handleDelete(item);
          }}
        />
      )}

      {showModal && (
        <Modal onClose={() => setShowModal(false)} scrollable>
          <DialogHeader className="mb-4">
            <DialogTitle>{t("integrations.view.create_title")}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleCreate} className="space-y-4">
            <div className="flex flex-col gap-1.5">
              <label htmlFor="integration-provider" className="text-sm font-medium text-foreground">
                {t("integrations.field.type")}
              </label>
              <Select
                value={form.provider}
                onValueChange={(provider) => setForm({ ...form, provider: provider as IntegrationProvider })}
              >
                <SelectTrigger id="integration-provider" />
                <SelectContent>
                  {(Object.keys(PROVIDER_LABEL_KEYS) as IntegrationProvider[]).map((provider) => {
                    const info = providerInfo(provider);
                    return (
                      <SelectItem key={provider} value={provider} disabled={info ? !info.available : false}>
                        {t(PROVIDER_LABEL_KEYS[provider])}
                        {info && !info.available ? " — " + t("integrations.state.unavailable") : ""}
                      </SelectItem>
                    );
                  })}
                </SelectContent>
              </Select>
              {selected && !selected.available && (
                <p className="text-xs text-warning">{selected.reason}</p>
              )}
            </div>

            <Input
              label={`${t("integrations.field.name")} *`}
              placeholder={t("integrations.field.name_placeholder")}
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              required
            />

            {!isOAuth && (
              <>
                <Input
                  type="url"
                  label={`${t("integrations.field.target_url")} *`}
                  placeholder="https://api.example.com/webhooks"
                  value={form.target_url}
                  onChange={(e) => setForm({ ...form, target_url: e.target.value })}
                  required
                />
                <Input
                  type="password"
                  label={t("integrations.field.secret")}
                  placeholder={t("integrations.field.secret_placeholder")}
                  value={form.secret_key}
                  onChange={(e) => setForm({ ...form, secret_key: e.target.value })}
                />
              </>
            )}

            {(form.provider === "google_drive" || form.provider === "dropbox") && (
              <>
                <Input
                  label={
                    form.provider === "google_drive"
                      ? t("integrations.field.drive_folder")
                      : t("integrations.field.dropbox_folder")
                  }
                  placeholder={
                    form.provider === "google_drive"
                      ? t("integrations.field.drive_folder_placeholder")
                      : t("integrations.field.dropbox_folder_placeholder")
                  }
                  value={form.folder}
                  onChange={(e) => setForm({ ...form, folder: e.target.value })}
                />
                <Checkbox
                  checked={form.auto_export}
                  onCheckedChange={(checked) => setForm({ ...form, auto_export: checked === true })}
                  label={t("integrations.field.auto_export")}
                />
              </>
            )}

            {form.provider === "google_meet" && (
              <Input
                label={t("integrations.field.calendar_id")}
                placeholder="primary"
                value={form.calendar_id}
                onChange={(e) => setForm({ ...form, calendar_id: e.target.value })}
                helperText={t("integrations.message.meet_via_calendar")}
              />
            )}

            {isOAuth && <p className="text-xs text-muted">{t("integrations.message.connect_after_save")}</p>}

            <div className="flex items-center gap-2 pt-2">
              <Button type="button" variant="secondary" className="w-1/2" onClick={() => setShowModal(false)}>
                {t("base.action.cancel")}
              </Button>
              <Button type="submit" className="w-1/2">
                {t("integrations.action.register")}
              </Button>
            </div>
          </form>
        </Modal>
      )}
    </div>
  );
}
