"use client";

/**
 * SSO clients — OAuth2 / OIDC client management.
 *
 * The client secret is readable exactly once, in the response that mints it,
 * so the create and rotate paths both end in a modal the user has to
 * acknowledge. Every other view of a client shows no secret at all, because
 * the server has only a digest and could not show one if it wanted to.
 */

import { useEffect, useMemo, useState } from "react";
import {
  AlertTriangle, Check, Code2, Copy, KeyRound, Loader2, Plus,
  RefreshCw, Server, Shield, Smartphone, Trash2, X,
} from "lucide-react";
import { api, type OAuth2Client, type OAuth2ClientDraft, type OAuth2Scope } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Modal as Dialog } from "@/components/ui";
import { ReadOnlyNote, useAccess } from "@/lib/permissions";

const emptyDraft: OAuth2ClientDraft = {
  client_name: "",
  client_uri: "",
  client_type: "confidential",
  redirect_uris: [],
  post_logout_redirect_uris: [],
  grant_types: ["authorization_code", "refresh_token"],
  scopes: ["openid", "profile", "email"],
};

export default function SSOClientsPage() {
  const { t, locale } = useI18n();
  // Registering, editing, rotating and deleting all need developer.manage;
  // a member with only developer.read gets the list and nothing else.
  const { allowed: canManage } = useAccess("sso_clients.manage");
  const [apps, setApps] = useState<OAuth2Client[]>([]);
  const [scopes, setScopes] = useState<OAuth2Scope[]>([]);
  const [grantTypes, setGrantTypes] = useState<string[]>([]);
  const [endpoints, setEndpoints] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [editing, setEditing] = useState<{ draft: OAuth2ClientDraft; clientID?: string } | null>(null);
  const [revealed, setRevealed] = useState<OAuth2Client | null>(null);
  const [confirming, setConfirming] = useState<{ app: OAuth2Client; action: "delete" | "rotate" } | null>(null);
  const [copied, setCopied] = useState("");

  async function load() {
    setLoading(true);
    setError("");
    try {
      const [list, vocabulary, urls] = await Promise.all([
        api.getSSOClients(),
        api.getSSOClientScopes(),
        api.getSSOClientEndpoints(),
      ]);
      setApps(list || []);
      setScopes(vocabulary.scopes || []);
      setGrantTypes(vocabulary.grant_types || []);
      setEndpoints(urls || {});
    } catch (err) {
      setError(err instanceof Error ? err.message : t("base.message.error"));
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { void load(); }, []);

  function copy(value: string, id: string) {
    void navigator.clipboard.writeText(value);
    setCopied(id);
    setTimeout(() => setCopied(""), 2000);
  }

  async function save(draft: OAuth2ClientDraft, clientID?: string) {
    setError("");
    try {
      const saved = clientID
        ? await api.updateSSOClient(clientID, draft)
        : await api.createSSOClient(draft);
      setEditing(null);
      await load();
      // Only a fresh registration carries a secret worth showing.
      if (saved.client_secret) setRevealed(saved);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("base.message.error"));
    }
  }

  async function runConfirmed() {
    if (!confirming) return;
    const { app, action } = confirming;
    setConfirming(null);
    setError("");
    try {
      if (action === "delete") {
        await api.deleteSSOClient(app.client_id);
      } else {
        setRevealed(await api.rotateSSOClientSecret(app.client_id));
      }
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("base.message.error"));
    }
  }

  const describe = (scope: OAuth2Scope) => (locale === "mn" ? scope.description_mn : scope.description);

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold text-slate-900 flex items-center gap-2">
            <Code2 className="w-7 h-7 text-indigo-600" />
            {t("sso_clients.view.title")}
          </h1>
          <p className="text-sm text-slate-500 mt-1">{t("sso_clients.view.subtitle")}</p>
        </div>
        {canManage && <button
          onClick={() => setEditing({ draft: { ...emptyDraft } })}
          className="bg-indigo-600 hover:bg-indigo-700 text-white px-4 py-2 rounded-lg text-sm font-semibold flex items-center gap-2 shadow-sm"
        >
          <Plus className="w-4 h-4" />
          {t("sso_clients.action.create")}
        </button>}
      </header>

      {!canManage && <ReadOnlyNote permission="sso_clients.manage" />}

      <EndpointCard endpoints={endpoints} copied={copied} onCopy={copy} title={t("sso_clients.view.endpoints_title")} />

      {error && (
        <p className="text-sm text-rose-700 bg-rose-50 border border-rose-200 rounded-lg px-3 py-2 flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 shrink-0" /> {error}
        </p>
      )}

      {loading ? (
        <p className="p-12 text-center text-slate-400 flex items-center justify-center gap-2">
          <Loader2 className="w-4 h-4 animate-spin" /> {t("sso_clients.message.loading")}
        </p>
      ) : apps.length === 0 ? (
        <div className="p-12 text-center border border-dashed border-slate-300 rounded-xl bg-white">
          <Shield className="w-9 h-9 text-slate-300 mx-auto mb-3" />
          <h2 className="font-bold text-slate-900">{t("sso_clients.view.empty_title")}</h2>
          <p className="text-sm text-slate-500 mt-1 max-w-md mx-auto">{t("sso_clients.view.empty_body")}</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
          {apps.map((app) => (
            <AppCard
              key={app.id}
              app={app}
              scopes={scopes}
              copied={copied}
              onCopy={copy}
              onEdit={() => setEditing({ clientID: app.client_id, draft: toDraft(app) })}
              onRotate={() => setConfirming({ app, action: "rotate" })}
              onDelete={() => setConfirming({ app, action: "delete" })}
              canManage={canManage}
            />
          ))}
        </div>
      )}

      {editing && (
        <AppForm
          initial={editing.draft}
          isNew={!editing.clientID}
          scopes={scopes}
          grantTypes={grantTypes}
          describe={describe}
          onCancel={() => setEditing(null)}
          onSave={(draft) => save(draft, editing.clientID)}
        />
      )}

      {revealed?.client_secret && (
        <SecretModal secret={revealed.client_secret} clientID={revealed.client_id} copied={copied} onCopy={copy} onClose={() => setRevealed(null)} />
      )}

      {confirming && (
        <ConfirmModal
          title={confirming.app.client_name}
          body={confirming.action === "delete" ? t("sso_clients.message.delete_warning") : t("sso_clients.message.rotate_warning")}
          danger={confirming.action === "delete"}
          confirmLabel={confirming.action === "delete" ? t("base.action.delete") : t("sso_clients.action.rotate")}
          cancelLabel={t("base.action.cancel")}
          onCancel={() => setConfirming(null)}
          onConfirm={runConfirmed}
        />
      )}
    </div>
  );
}

function toDraft(app: OAuth2Client): OAuth2ClientDraft {
  return {
    client_name: app.client_name,
    client_uri: app.client_uri || "",
    client_type: app.client_type,
    redirect_uris: app.redirect_uris,
    post_logout_redirect_uris: app.post_logout_redirect_uris || [],
    grant_types: app.grant_types,
    scopes: app.scopes,
    disabled: app.disabled,
  };
}

function EndpointCard({ endpoints, copied, onCopy, title }: {
  endpoints: Record<string, string>; copied: string; title: string;
  onCopy: (value: string, id: string) => void;
}) {
  const rows: [string, string][] = [
    ["Discovery", endpoints.discovery],
    ["Authorization", endpoints.authorization_endpoint],
    ["Token", endpoints.token_endpoint],
    ["UserInfo", endpoints.userinfo_endpoint],
    ["JWKS", endpoints.jwks_uri],
  ].filter(([, url]) => Boolean(url)) as [string, string][];
  if (rows.length === 0) return null;

  return (
    <section className="bg-slate-900 text-white rounded-xl border border-slate-800 p-5">
      <div className="flex items-center gap-3 mb-4">
        <Shield className="w-6 h-6 text-cyan-400" />
        <h2 className="font-semibold text-sm text-white">{title}</h2>
        <span className="ml-auto text-[11px] bg-emerald-500/20 text-emerald-300 font-mono px-3 py-1 rounded-full border border-emerald-500/30">
          Active SSO Provider
        </span>
      </div>
      <dl className="grid sm:grid-cols-2 gap-x-6 gap-y-2">
        {rows.map(([label, url]) => (
          <div key={label} className="flex items-center gap-2 min-w-0">
            <dt className="text-[11px] uppercase tracking-wide text-slate-500 w-24 shrink-0">{label}</dt>
            <dd className="font-mono text-xs text-slate-300 truncate flex-1">{url}</dd>
            <button onClick={() => onCopy(url, label)} className="text-slate-500 hover:text-white shrink-0">
              {copied === label ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            </button>
          </div>
        ))}
      </dl>
    </section>
  );
}

function AppCard({ app, scopes, copied, onCopy, onEdit, onRotate, onDelete, canManage }: {
  app: OAuth2Client; scopes: OAuth2Scope[]; copied: string;
  onCopy: (value: string, id: string) => void;
  onEdit: () => void; onRotate: () => void; onDelete: () => void; canManage: boolean;
}) {
  const { t } = useI18n();
  const sensitive = useMemo(
    () => new Set(scopes.filter((s) => s.sensitive).map((s) => s.name)),
    [scopes],
  );

  return (
    <article className={`bg-white p-5 rounded-xl border shadow-sm space-y-4 ${app.disabled ? "border-slate-200 opacity-70" : "border-slate-200"}`}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="font-bold text-slate-900 truncate">{app.client_name}</h3>
          <p className="text-[11px] text-slate-400 mt-0.5">
            {app.last_used_at
              ? `${t("sso_clients.field.last_used")}: ${new Date(app.last_used_at).toLocaleDateString()}`
              : t("sso_clients.message.never_used")}
          </p>
        </div>
        <div className="flex items-center gap-1.5 shrink-0">
          {app.disabled && (
            <span className="text-[11px] bg-slate-100 text-slate-500 px-2 py-0.5 rounded-full">
              {t("sso_clients.message.disabled")}
            </span>
          )}
          <span className="text-[11px] bg-indigo-50 text-indigo-700 font-medium px-2 py-0.5 rounded-full flex items-center gap-1">
            {app.client_type === "public" ? <Smartphone className="w-3 h-3" /> : <Server className="w-3 h-3" />}
            {app.client_type}
          </span>
        </div>
      </div>

      <div className="text-xs font-mono bg-slate-50 p-3 rounded-lg border border-slate-200 space-y-2">
        <div className="flex items-center justify-between gap-2">
          <span className="text-slate-500 shrink-0">client_id</span>
          <span className="text-slate-900 font-bold truncate">{app.client_id}</span>
          <button onClick={() => onCopy(app.client_id, app.client_id)} className="shrink-0">
            {copied === app.client_id
              ? <Check className="w-3.5 h-3.5 text-emerald-600" />
              : <Copy className="w-3.5 h-3.5 text-slate-400" />}
          </button>
        </div>
        <div className="flex items-center justify-between gap-2">
          <span className="text-slate-500 shrink-0">client_secret</span>
          <span className="text-slate-400 italic font-sans text-[11px]">
            {app.client_type === "public" ? "—" : t("sso_clients.message.secret_hidden")}
          </span>
        </div>
      </div>

      <Field label={t("sso_clients.field.redirect_uris")}>
        {app.redirect_uris.map((uri) => (
          <Chip key={uri} mono>{uri}</Chip>
        ))}
      </Field>

      {/* Only shown once there is one: an application that never ends a session
          here has no reason to carry an empty row about it. */}
      {(app.post_logout_redirect_uris || []).length > 0 && (
        <Field label={t("sso_clients.field.post_logout_redirect_uris")}>
          {app.post_logout_redirect_uris.map((uri) => (
            <Chip key={uri} mono>{uri}</Chip>
          ))}
        </Field>
      )}

      <Field label={t("sso_clients.field.scopes")}>
        {app.scopes.map((scope) => (
          <Chip key={scope} mono tone={sensitive.has(scope) ? "amber" : "blue"}>{scope}</Chip>
        ))}
      </Field>

      <Field label={t("sso_clients.field.grant_types")}>
        {app.grant_types.map((grant) => <Chip key={grant} mono tone="slate">{grant}</Chip>)}
      </Field>

      <div className={`flex gap-2 pt-1 border-t border-slate-100 ${canManage ? "" : "hidden"}`}>
        <button onClick={onEdit} className="text-xs font-semibold text-slate-600 hover:bg-slate-100 px-3 py-1.5 rounded-lg mt-2">
          {t("sso_clients.view.edit_title")}
        </button>
        {app.client_type !== "public" && (
          <button onClick={onRotate} className="text-xs font-semibold text-amber-700 hover:bg-amber-50 px-3 py-1.5 rounded-lg mt-2 flex items-center gap-1.5">
            <RefreshCw className="w-3.5 h-3.5" /> {t("sso_clients.action.rotate")}
          </button>
        )}
        <button onClick={onDelete} className="text-xs font-semibold text-rose-700 hover:bg-rose-50 px-3 py-1.5 rounded-lg mt-2 ml-auto flex items-center gap-1.5">
          <Trash2 className="w-3.5 h-3.5" /> {t("base.action.delete")}
        </button>
      </div>
    </article>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <span className="text-[11px] font-semibold text-slate-700 block mb-1">{label}</span>
      <div className="flex flex-wrap gap-1">{children}</div>
    </div>
  );
}

function Chip({ children, mono, tone = "slate" }: {
  children: React.ReactNode; mono?: boolean; tone?: "slate" | "blue" | "amber";
}) {
  const tones = {
    slate: "bg-slate-100 text-slate-600",
    blue: "bg-blue-50 text-blue-700",
    amber: "bg-amber-50 text-amber-700 border border-amber-200",
  };
  return (
    <span className={`text-[11px] px-2 py-0.5 rounded ${tones[tone]} ${mono ? "font-mono" : ""} break-all`}>
      {children}
    </span>
  );
}

function AppForm({ initial, isNew, scopes, grantTypes, describe, onCancel, onSave }: {
  initial: OAuth2ClientDraft; isNew: boolean; scopes: OAuth2Scope[]; grantTypes: string[];
  describe: (scope: OAuth2Scope) => string;
  onCancel: () => void; onSave: (draft: OAuth2ClientDraft) => void;
}) {
  const { t } = useI18n();
  const [draft, setDraft] = useState(initial);
  const [uris, setUris] = useState(initial.redirect_uris.join("\n"));
  const [logoutUris, setLogoutUris] = useState((initial.post_logout_redirect_uris || []).join("\n"));

  function toggle(field: "scopes" | "grant_types", value: string) {
    setDraft((d) => {
      const current = d[field] || [];
      return { ...d, [field]: current.includes(value) ? current.filter((v) => v !== value) : [...current, value] };
    });
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    onSave({
      ...draft,
      redirect_uris: uris.split("\n").map((s) => s.trim()).filter(Boolean),
      post_logout_redirect_uris: logoutUris.split("\n").map((s) => s.trim()).filter(Boolean),
    });
  }

  return (
    <Modal onClose={onCancel} wide label={isNew ? t("sso_clients.view.create_title") : t("sso_clients.view.edit_title")}>
      <form onSubmit={submit} className="space-y-4">
        <h2 className="text-lg font-bold text-slate-900">
          {isNew ? t("sso_clients.view.create_title") : t("sso_clients.view.edit_title")}
        </h2>

        <label className="block">
          <span className="text-xs font-semibold text-slate-700">{t("sso_clients.field.name")} *</span>
          <input
            value={draft.client_name}
            onChange={(e) => setDraft({ ...draft, client_name: e.target.value })}
            required
            className="mt-1 w-full px-3 py-2 text-sm border border-slate-300 rounded-lg focus:ring-2 focus:ring-indigo-500 outline-none"
          />
        </label>

        <label className="block">
          <span className="text-xs font-semibold text-slate-700">{t("sso_clients.field.homepage")}</span>
          <input
            value={draft.client_uri || ""}
            onChange={(e) => setDraft({ ...draft, client_uri: e.target.value })}
            placeholder="https://example.mn"
            className="mt-1 w-full px-3 py-2 text-sm border border-slate-300 rounded-lg font-mono focus:ring-2 focus:ring-indigo-500 outline-none"
          />
        </label>

        {isNew && (
          <fieldset>
            <legend className="text-xs font-semibold text-slate-700 mb-1">{t("sso_clients.field.client_type")}</legend>
            <div className="grid sm:grid-cols-2 gap-2">
              {(["confidential", "public"] as const).map((type) => (
                <label
                  key={type}
                  className={`border rounded-lg p-3 cursor-pointer text-xs ${draft.client_type === type ? "border-indigo-500 bg-indigo-50" : "border-slate-200"}`}
                >
                  <input
                    type="radio"
                    name="client_type"
                    className="sr-only"
                    checked={draft.client_type === type}
                    onChange={() => setDraft({ ...draft, client_type: type })}
                  />
                  <span className="font-semibold text-slate-900 flex items-center gap-1.5">
                    {type === "public" ? <Smartphone className="w-3.5 h-3.5" /> : <Server className="w-3.5 h-3.5" />}
                    {type === "public" ? t("sso_clients.type.public") : t("sso_clients.type.confidential")}
                  </span>
                  <span className="text-slate-500 block mt-1">
                    {type === "public" ? t("sso_clients.type.public_hint") : t("sso_clients.type.confidential_hint")}
                  </span>
                </label>
              ))}
            </div>
          </fieldset>
        )}

        <label className="block">
          <span className="text-xs font-semibold text-slate-700">{t("sso_clients.field.redirect_uris")} *</span>
          <textarea
            value={uris}
            onChange={(e) => setUris(e.target.value)}
            rows={3}
            placeholder={"https://app.example.mn/callback\nhttp://localhost:3000/callback"}
            className="mt-1 w-full px-3 py-2 text-sm border border-slate-300 rounded-lg font-mono focus:ring-2 focus:ring-indigo-500 outline-none"
          />
          <span className="text-[11px] text-slate-400">
            {/* Exact matching is what stops a code being delivered somewhere else. */}
            one per line · https only, except on localhost
          </span>
        </label>

        <label className="block">
          <span className="text-xs font-semibold text-slate-700">{t("sso_clients.field.post_logout_redirect_uris")}</span>
          <textarea
            value={logoutUris}
            onChange={(e) => setLogoutUris(e.target.value)}
            rows={2}
            placeholder={"https://app.example.mn/"}
            className="mt-1 w-full px-3 py-2 text-sm border border-slate-300 rounded-lg font-mono focus:ring-2 focus:ring-indigo-500 outline-none"
          />
          <span className="text-[11px] text-slate-400">
            {/* Matched exactly too: an unchecked return address would make the
                logout endpoint an open redirector. */}
            {t("sso_clients.hint.post_logout_redirect_uris")}
          </span>
        </label>

        <fieldset>
          <legend className="text-xs font-semibold text-slate-700 mb-1">{t("sso_clients.field.grant_types")}</legend>
          <div className="flex flex-wrap gap-2">
            {grantTypes.map((grant) => (
              <label
                key={grant}
                className={`text-[11px] font-mono px-2.5 py-1 rounded-lg border cursor-pointer ${draft.grant_types?.includes(grant) ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-slate-200 text-slate-500"}`}
              >
                <input type="checkbox" className="sr-only" checked={draft.grant_types?.includes(grant)} onChange={() => toggle("grant_types", grant)} />
                {grant}
              </label>
            ))}
          </div>
          <p className="text-[11px] text-slate-400 mt-1">{t("sso_clients.message.pkce_note")}</p>
        </fieldset>

        <fieldset>
          <legend className="text-xs font-semibold text-slate-700 mb-1">{t("sso_clients.field.scopes")}</legend>
          <div className="space-y-1.5 max-h-52 overflow-y-auto pr-1">
            {scopes.map((scope) => (
              <label key={scope.name} className="flex items-start gap-2 text-xs cursor-pointer">
                <input
                  type="checkbox"
                  checked={draft.scopes?.includes(scope.name)}
                  onChange={() => toggle("scopes", scope.name)}
                  className="mt-0.5"
                />
                <span>
                  <span className="font-mono text-slate-800">{scope.name}</span>
                  {scope.sensitive && (
                    <span className="ml-1.5 text-[10px] bg-amber-50 text-amber-700 border border-amber-200 px-1 rounded">
                      sensitive
                    </span>
                  )}
                  <span className="block text-slate-500">{describe(scope)}</span>
                </span>
              </label>
            ))}
          </div>
        </fieldset>

        {!isNew && (
          <label className="flex items-center gap-2 text-xs text-slate-700">
            <input type="checkbox" checked={Boolean(draft.disabled)} onChange={(e) => setDraft({ ...draft, disabled: e.target.checked })} />
            {t("sso_clients.action.disable")}
          </label>
        )}

        <div className="flex justify-end gap-2 pt-2">
          <button type="button" onClick={onCancel} className="px-4 py-2 text-sm text-slate-600 hover:bg-slate-100 rounded-lg">
            {t("base.action.cancel")}
          </button>
          <button type="submit" className="px-4 py-2 text-sm bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg font-semibold">
            {t("base.action.save")}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function SecretModal({ secret, clientID, copied, onCopy, onClose }: {
  secret: string; clientID: string; copied: string;
  onCopy: (value: string, id: string) => void; onClose: () => void;
}) {
  const { t } = useI18n();
  return (
    <Modal onClose={onClose} label={t("sso_clients.message.secret_once_title")}>
      <div className="space-y-4">
        <h2 className="text-lg font-bold text-slate-900 flex items-center gap-2">
          <KeyRound className="w-5 h-5 text-amber-600" />
          {t("sso_clients.message.secret_once_title")}
        </h2>
        <p className="text-sm text-slate-600">{t("sso_clients.message.secret_once_body")}</p>

        <div className="space-y-2">
          <ReadOnlyField label="client_id" value={clientID} copied={copied} onCopy={onCopy} />
          <ReadOnlyField label="client_secret" value={secret} copied={copied} onCopy={onCopy} highlight />
        </div>

        <div className="flex justify-end">
          <button onClick={onClose} className="px-4 py-2 text-sm bg-slate-900 hover:bg-slate-800 text-white rounded-lg font-semibold">
            {t("sso_clients.action.done")}
          </button>
        </div>
      </div>
    </Modal>
  );
}

function ReadOnlyField({ label, value, copied, onCopy, highlight }: {
  label: string; value: string; copied: string; highlight?: boolean;
  onCopy: (value: string, id: string) => void;
}) {
  return (
    <div className={`flex items-center gap-2 p-3 rounded-lg border ${highlight ? "bg-amber-50 border-amber-200" : "bg-slate-50 border-slate-200"}`}>
      <span className="text-[11px] font-semibold text-slate-500 w-24 shrink-0">{label}</span>
      <code className="text-xs font-mono text-slate-900 break-all flex-1">{value}</code>
      <button onClick={() => onCopy(value, label)} className="shrink-0">
        {copied === label ? <Check className="w-4 h-4 text-emerald-600" /> : <Copy className="w-4 h-4 text-slate-400" />}
      </button>
    </div>
  );
}

function ConfirmModal({ title, body, danger, confirmLabel, cancelLabel, onCancel, onConfirm }: {
  title: string; body: string; danger?: boolean; confirmLabel: string; cancelLabel: string;
  onCancel: () => void; onConfirm: () => void;
}) {
  return (
    <Modal onClose={onCancel} label={title}>
      <div className="space-y-4">
        <h2 className="text-lg font-bold text-slate-900 flex items-center gap-2">
          <AlertTriangle className={`w-5 h-5 ${danger ? "text-rose-600" : "text-amber-600"}`} />
          {title}
        </h2>
        <p className="text-sm text-slate-600">{body}</p>
        <div className="flex justify-end gap-2">
          <button onClick={onCancel} className="px-4 py-2 text-sm text-slate-600 hover:bg-slate-100 rounded-lg">
            {cancelLabel}
          </button>
          <button
            onClick={onConfirm}
            className={`px-4 py-2 text-sm text-white rounded-lg font-semibold ${danger ? "bg-rose-600 hover:bg-rose-700" : "bg-amber-600 hover:bg-amber-700"}`}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </Modal>
  );
}

// The platform's dialog, with the close affordance these three need. The shared
// one deliberately has no dismissal — most dialogs here hold typed input — but a
// secret shown once, and a confirmation, are read and dismissed, so they carry
// an X. What comes from the shared component is what no dialog should be
// without: the role, the name, focus taken on open and returned on close, and
// Tab that cannot walk out into the page behind.
function Modal({ children, onClose, wide, label }: { children: React.ReactNode; onClose: () => void; wide?: boolean; label?: string }) {
  return (
    <Dialog size={wide ? "lg" : "md"} scrollable label={label} className="my-8 relative">
      <button type="button" onClick={onClose} className="absolute top-4 right-4 text-slate-400 hover:text-slate-600" aria-label={label ? undefined : "close"}>
        <X className="w-4 h-4" />
      </button>
      {children}
    </Dialog>
  );
}
