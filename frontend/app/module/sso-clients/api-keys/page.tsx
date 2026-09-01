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
import { api, type OAuth2Client, type OAuth2Scope } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import {
  Chip, ConfirmDialog, CopyButton, Empty, ErrorNote, Loading, Modal, Panel,
  ReadOnlyNote, Screen, SecretDialog, relativeDate, useAccess, useCopy,
} from "../shared";

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
    <Screen
      icon={<KeyRound className="w-5 h-5" />}
      title={t("sso_clients.keys.title")}
      subtitle={t("sso_clients.keys.subtitle")}
      action={canManage && (
        <button
          onClick={() => setCreating(true)}
          className="bg-indigo-600 hover:bg-indigo-700 text-white px-4 py-2 rounded-lg text-sm font-semibold flex items-center gap-2 shadow-sm"
        >
          <Plus className="w-4 h-4" /> {t("sso_clients.keys.create")}
        </button>
      )}
    >
      <Panel className="p-4 bg-surface-2 border-line">
        <p className="text-xs text-muted leading-relaxed">{t("sso_clients.keys.explainer")}</p>
      </Panel>

      {!canManage && <ReadOnlyNote permission="sso_clients.manage" />}
      {error && <ErrorNote>{error}</ErrorNote>}

      {loading ? (
        <Loading label={t("sso_clients.message.loading")} />
      ) : keys.length === 0 ? (
        <Empty icon={<KeyRound className="w-9 h-9 mx-auto" />}>{t("sso_clients.keys.empty")}</Empty>
      ) : (
        <div className="space-y-3">
          {keys.map((key) => (
            <Panel key={key.id} className="p-4">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                  <h3 className="font-semibold text-foreground flex items-center gap-2">
                    {key.client_name}
                    {key.disabled && <Chip tone="rose">{t("sso_clients.message.disabled")}</Chip>}
                  </h3>
                  <div className="flex items-center gap-2 mt-1 text-xs font-mono text-muted">
                    {key.client_id}
                    <CopyButton value={key.client_id} id={key.client_id} copied={copied} onCopy={copy} />
                  </div>
                  <p className="text-[11px] text-muted mt-1">
                    {t("sso_clients.field.last_used")}:{" "}
                    {relativeDate(key.last_used_at, t("sso_clients.message.never_used"), locale)}
                  </p>
                </div>
                <div className={`flex gap-1 ${canManage ? "" : "hidden"}`}>
                  <button
                    onClick={() => setConfirming({ client: key, action: "rotate" })}
                    className="text-xs font-semibold text-amber-700 hover:bg-amber-50 px-3 py-1.5 rounded-lg flex items-center gap-1.5"
                  >
                    <RefreshCw className="w-3.5 h-3.5" /> {t("sso_clients.action.rotate")}
                  </button>
                  <button
                    onClick={() => setConfirming({ client: key, action: "delete" })}
                    className="text-xs font-semibold text-rose-700 hover:bg-rose-50 px-3 py-1.5 rounded-lg flex items-center gap-1.5"
                  >
                    <Trash2 className="w-3.5 h-3.5" /> {t("base.action.delete")}
                  </button>
                </div>
              </div>

              <div className="flex flex-wrap gap-1 mt-3">
                {key.scopes.map((scope) => (
                  <Chip key={scope} mono tone={scopes.find((s) => s.name === scope)?.sensitive ? "amber" : "blue"}>
                    {scope}
                  </Chip>
                ))}
              </div>

              {endpoints.token_endpoint && (
                <details className="mt-3 group">
                  <summary className="text-[11px] font-semibold text-muted cursor-pointer flex items-center gap-1.5">
                    <Terminal className="w-3.5 h-3.5" /> {t("sso_clients.keys.curl")}
                  </summary>
                  <pre className="mt-2 text-[11px] bg-slate-900 text-slate-200 rounded-lg p-3 overflow-x-auto">
{`curl -X POST ${endpoints.token_endpoint} \\
  -u '${key.client_id}:YOUR_SECRET' \\
  -d 'grant_type=client_credentials'`}
                  </pre>
                </details>
              )}
            </Panel>
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
        <ConfirmDialog
          title={confirming.client.client_name}
          body={confirming.action === "delete" ? t("sso_clients.message.delete_warning") : t("sso_clients.message.rotate_warning")}
          confirmLabel={confirming.action === "delete" ? t("base.action.delete") : t("sso_clients.action.rotate")}
          danger={confirming.action === "delete"}
          onCancel={() => setConfirming(null)}
          onConfirm={runConfirmed}
        />
      )}
    </Screen>
  );
}

function CreateKeyDialog({ scopes, onCancel, onCreate }: {
  scopes: OAuth2Scope[]; onCancel: () => void; onCreate: (name: string, scopes: string[]) => void;
}) {
  const { t, locale } = useI18n();
  const [name, setName] = useState("");
  const [chosen, setChosen] = useState<string[]>(["erp.read"]);

  return (
    <Modal onClose={onCancel}>
      <form
        onSubmit={(e) => { e.preventDefault(); onCreate(name, chosen); }}
        className="space-y-4"
      >
        <h2 className="text-lg font-semibold text-foreground">{t("sso_clients.keys.create")}</h2>

        <label className="block">
          <span className="text-xs font-semibold text-foreground">{t("sso_clients.field.name")} *</span>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            placeholder="Warehouse sync job"
            className="mt-1 w-full px-3 py-2 text-sm border border-input rounded-lg"
          />
        </label>

        <fieldset>
          <legend className="text-xs font-semibold text-foreground mb-1">{t("sso_clients.field.scopes")}</legend>
          <div className="space-y-1.5">
            {scopes.map((scope) => (
              <label key={scope.name} className="flex items-start gap-2 text-xs cursor-pointer">
                <input
                  type="checkbox"
                  className="mt-0.5"
                  checked={chosen.includes(scope.name)}
                  onChange={() =>
                    setChosen((c) => (c.includes(scope.name) ? c.filter((s) => s !== scope.name) : [...c, scope.name]))
                  }
                />
                <span>
                  <span className="font-mono text-foreground">{scope.name}</span>
                  {scope.sensitive && <span className="ml-1.5"><Chip tone="amber">sensitive</Chip></span>}
                  <span className="block text-muted">
                    {locale === "mn" ? scope.description_mn : scope.description}
                  </span>
                </span>
              </label>
            ))}
          </div>
        </fieldset>

        <div className="flex justify-end gap-2 pt-2">
          <button type="button" onClick={onCancel} className="px-4 py-2 text-sm text-muted hover:bg-surface-hover rounded-lg">
            {t("base.action.cancel")}
          </button>
          <button
            type="submit"
            disabled={chosen.length === 0}
            className="px-4 py-2 text-sm bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg font-semibold disabled:opacity-50"
          >
            {t("base.action.create")}
          </button>
        </div>
      </form>
    </Modal>
  );
}
