"use client";

/**
 * API keys — machine-to-machine credentials.
 *
 * Not a second credential system beside OAuth2: a key here *is* a confidential
 * client registered for the client_credentials grant. Keeping them in one store
 * means a key can be audited, scoped and revoked by the same machinery as
 * everything else, and the screen says so rather than pretending otherwise.
 */

import { useEffect, useMemo, useState } from "react";
import { KeyRound, Plus, RefreshCw, Terminal, Trash2 } from "lucide-react";
import {
  Alert, Badge, Button, Card, Checkbox, ConfirmationDialog, DialogHeader, DialogTitle, EmptyState, Input, Spinner,
} from "@gerege-systems/ui";
import { api, type OAuth2Client, type OAuth2Scope } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { ReadOnlyNote, useAccess } from "@/lib/permissions";
import { Modal, PageHeader } from "@/components/ui";
import { relativeDate } from "@/components/module/kit";
import { CopyButton, SecretDialog, useCopy } from "../shared";

export default function ApiKeysPage() {
  const { t, locale } = useI18n();
  const { copied, copy } = useCopy();
  // Every mutation below maps to developer.manage in the gate middleware, so
  // the screen asks the same question before offering the control.
  const { allowed: canManage } = useAccess("sso_clients.manage");
  const [clients, setClients] = useState<OAuth2Client[]>([]);
  const [scopes, setScopes] = useState<OAuth2Scope[]>([]);
  const [endpoints, setEndpoints] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [creating, setCreating] = useState(false);
  const [revealed, setRevealed] = useState<OAuth2Client | null>(null);
  const [confirming, setConfirming] = useState<{ client: OAuth2Client; action: "rotate" | "delete" } | null>(null);

  async function load() {
    setLoading(true);
    setError("");
    try {
      const [apps, vocabulary, urls] = await Promise.all([
        api.getSSOClients(),
        api.getSSOClientScopes(),
        api.getSSOClientEndpoints(),
      ]);
      setClients(apps || []);
      setScopes(vocabulary.scopes || []);
      setEndpoints(urls || {});
    } catch (err) {
      setError(err instanceof Error ? err.message : t("base.message.error"));
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { void load(); }, []);

  // A machine credential is exactly a confidential client that can run the
  // client_credentials grant; interactive apps live on the SSO clients screen.
  const keys = useMemo(
    () => clients.filter((c) => c.client_type === "confidential" && c.grant_types.includes("client_credentials")),
    [clients],
  );

  // Identity scopes need a user to be about, so they are not offered here.
  const machineScopes = useMemo(
    () => scopes.filter((s) => !["openid", "profile", "email", "phone", "offline_access"].includes(s.name)),
    [scopes],
  );

  async function create(name: string, chosen: string[]) {
    setError("");
    try {
      const created = await api.createSSOClient({
        client_name: name,
        client_type: "confidential",
        redirect_uris: [],
        grant_types: ["client_credentials"],
        scopes: chosen,
      });
      setCreating(false);
      await load();
      if (created.client_secret) setRevealed(created);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("base.message.error"));
    }
  }

  async function runConfirmed() {
    if (!confirming) return;
    const { client, action } = confirming;
    setConfirming(null);
    setError("");
    try {
      if (action === "delete") await api.deleteSSOClient(client.client_id);
      else setRevealed(await api.rotateSSOClientSecret(client.client_id));
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("base.message.error"));
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<KeyRound className="w-5 h-5" />}
        title={t("sso_clients.keys.title")}
        subtitle={t("sso_clients.keys.subtitle")}
        actions={canManage && (
          <Button leadingIcon={<Plus />} onClick={() => setCreating(true)}>
            {t("sso_clients.keys.create")}
          </Button>
        )}
      />
      <Card padding="none" className="p-4 bg-surface-2">
        <p className="text-xs text-muted leading-relaxed">{t("sso_clients.keys.explainer")}</p>
      </Card>

      {!canManage && <ReadOnlyNote permission="sso_clients.manage" />}
      {error && <Alert variant="danger" live>{error}</Alert>}

      {loading ? (
        <p className="flex items-center justify-center gap-2 p-12 text-center text-muted" role="status">
          <Spinner size="md" decorative />
          {t("sso_clients.message.loading")}
        </p>
      ) : keys.length === 0 ? (
        <EmptyState icon={<KeyRound />} title={t("sso_clients.keys.empty")} />
      ) : (
        <div className="space-y-3">
          {keys.map((key) => (
            <Card padding="none" key={key.id} className="p-4">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                  <h3 className="font-semibold text-foreground flex items-center gap-2">
                    {key.client_name}
                    {key.disabled && <Badge className="break-all" tone="danger">{t("sso_clients.message.disabled")}</Badge>}
                  </h3>
                  <div className="flex items-center gap-2 mt-1 text-xs font-mono text-muted">
                    {key.client_id}
                    <CopyButton value={key.client_id} id={key.client_id} copied={copied} onCopy={copy} />
                  </div>
                  <p className="text-xs text-muted mt-1">
                    {t("sso_clients.field.last_used")}:{" "}
                    {relativeDate(key.last_used_at, t("sso_clients.message.never_used"), locale)}
                  </p>
                </div>
                {canManage && (
                  <div className="flex gap-1">
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-warning hover:bg-warning-soft"
                      leadingIcon={<RefreshCw />}
                      onClick={() => setConfirming({ client: key, action: "rotate" })}
                    >
                      {t("sso_clients.action.rotate")}
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-danger hover:bg-danger-soft"
                      leadingIcon={<Trash2 />}
                      onClick={() => setConfirming({ client: key, action: "delete" })}
                    >
                      {t("base.action.delete")}
                    </Button>
                  </div>
                )}
              </div>

              <div className="flex flex-wrap gap-1 mt-3">
                {key.scopes.map((scope) => (
                  <Badge
                    className="break-all font-mono"
                    key={scope}
                    tone={scopes.find((s) => s.name === scope)?.sensitive ? "warning" : "info"}
                  >
                    {scope}
                  </Badge>
                ))}
              </div>

              {endpoints.token_endpoint && (
                <details className="mt-3 group">
                  <summary className="text-xs font-semibold text-muted cursor-pointer flex items-center gap-1.5">
                    <Terminal className="w-3.5 h-3.5" /> {t("sso_clients.keys.curl")}
                  </summary>
                  <pre className="mt-2 text-xs bg-surface-2 text-foreground border border-line rounded-lg p-3 overflow-x-auto">
{`curl -X POST ${endpoints.token_endpoint} \\
  -u '${key.client_id}:YOUR_SECRET' \\
  -d 'grant_type=client_credentials'`}
                  </pre>
                </details>
              )}
            </Card>
          ))}
        </div>
      )}

      {creating && (
        <CreateKeyDialog scopes={machineScopes} onCancel={() => setCreating(false)} onCreate={create} />
      )}
      {revealed?.client_secret && (
        <SecretDialog clientID={revealed.client_id} secret={revealed.client_secret} onClose={() => setRevealed(null)} />
      )}
      {confirming && (
        <ConfirmationDialog
          open
          onOpenChange={(open) => { if (!open) setConfirming(null); }}
          title={confirming.client.client_name}
          description={confirming.action === "delete" ? t("sso_clients.message.delete_warning") : t("sso_clients.message.rotate_warning")}
          confirmLabel={confirming.action === "delete" ? t("base.action.delete") : t("sso_clients.action.rotate")}
          cancelLabel={t("base.action.cancel")}
          confirmVariant={confirming.action === "delete" ? "destructive" : "primary"}
          onConfirm={() => void runConfirmed()}
        />
      )}
    </div>
  );
}

function CreateKeyDialog({ scopes, onCancel, onCreate }: {
  scopes: OAuth2Scope[]; onCancel: () => void; onCreate: (name: string, scopes: string[]) => void;
}) {
  const { t, locale } = useI18n();
  const [name, setName] = useState("");
  const [chosen, setChosen] = useState<string[]>(["erp.read"]);

  return (
    <Modal onClose={onCancel} scrollable>
      <form
        onSubmit={(e) => { e.preventDefault(); onCreate(name, chosen); }}
        className="space-y-4"
      >
        <DialogHeader>
          <DialogTitle>{t("sso_clients.keys.create")}</DialogTitle>
        </DialogHeader>

        <Input
          label={`${t("sso_clients.field.name")} *`}
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
          placeholder="Warehouse sync job"
        />

        <fieldset>
          <legend className="text-sm font-medium text-foreground mb-1.5">{t("sso_clients.field.scopes")}</legend>
          <div className="space-y-1.5">
            {scopes.map((scope) => (
              <Checkbox
                key={scope.name}
                checked={chosen.includes(scope.name)}
                onCheckedChange={() =>
                  setChosen((c) => (c.includes(scope.name) ? c.filter((s) => s !== scope.name) : [...c, scope.name]))
                }
                label={
                  <span className="text-xs">
                    <span className="font-mono text-foreground">{scope.name}</span>
                    {scope.sensitive && <span className="ms-1.5"><Badge tone="warning">sensitive</Badge></span>}
                  </span>
                }
                description={locale === "mn" ? scope.description_mn : scope.description}
              />
            ))}
          </div>
        </fieldset>

        <div className="flex justify-end gap-2 pt-2">
          <Button variant="ghost" type="button" onClick={onCancel}>
            {t("base.action.cancel")}
          </Button>
          <Button type="submit" disabled={chosen.length === 0}>
            {t("base.action.create")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
