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
  Check, Code2, Copy, KeyRound, Plus, RefreshCw, Server, Shield, Smartphone, Trash2,
} from "lucide-react";
import {
  Alert,
  Badge,
  Button,
  Card,
  Checkbox,
  ConfirmationDialog,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  EmptyState,
  IconButton,
  Input,
  RadioGroup,
  RadioItem,
  Spinner,
  Textarea,
} from "@gerege-systems/ui";
import { api, type OAuth2Client, type OAuth2ClientDraft, type OAuth2Scope } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Modal } from "@/components/ui";
import { ReadOnlyNote, useAccess } from "@/lib/permissions";
import { formatDay } from "@/lib/datetime";

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
          <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
            <Code2 className="w-7 h-7 text-accent" />
            {t("sso_clients.view.title")}
          </h1>
          <p className="text-sm text-muted mt-1">{t("sso_clients.view.subtitle")}</p>
        </div>
        {canManage && (
          <Button leadingIcon={<Plus />} onClick={() => setEditing({ draft: { ...emptyDraft } })}>
            {t("sso_clients.action.create")}
          </Button>
        )}
      </header>

      {!canManage && <ReadOnlyNote permission="sso_clients.manage" />}

      <EndpointCard endpoints={endpoints} copied={copied} onCopy={copy} title={t("sso_clients.view.endpoints_title")} />

      {error && <Alert variant="danger" live>{error}</Alert>}

      {loading ? (
        <p className="p-12 text-center text-muted flex items-center justify-center gap-2" role="status">
          <Spinner size="md" decorative /> {t("sso_clients.message.loading")}
        </p>
      ) : apps.length === 0 ? (
        <Card padding="none" className="border-dashed">
          <EmptyState
            icon={<Shield />}
            title={t("sso_clients.view.empty_title")}
            description={t("sso_clients.view.empty_body")}
            headingLevel={2}
            className="p-12"
          />
        </Card>
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
        <ConfirmationDialog
          open
          onOpenChange={(open) => { if (!open) setConfirming(null); }}
          title={confirming.app.client_name}
          description={confirming.action === "delete" ? t("sso_clients.message.delete_warning") : t("sso_clients.message.rotate_warning")}
          confirmVariant={confirming.action === "delete" ? "destructive" : "primary"}
          confirmLabel={confirming.action === "delete" ? t("base.action.delete") : t("sso_clients.action.rotate")}
          cancelLabel={t("base.action.cancel")}
          onConfirm={() => void runConfirmed()}
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

/** The copy affordance beside a value: a tick for two seconds after it is pressed. */
function CopyIcon({ value, id, copied, onCopy }: {
  value: string; id: string; copied: string; onCopy: (value: string, id: string) => void;
}) {
  return (
    <IconButton
      size="sm"
      variant="ghost"
      className="shrink-0 -m-1"
      aria-label="copy"
      icon={copied === id ? <Check className="text-success" /> : <Copy />}
      onClick={() => onCopy(value, id)}
    />
  );
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
    <Card asChild padding="none" className="p-5 bg-surface-2">
      <section>
        <div className="flex items-center gap-3 mb-4">
          <Shield className="w-6 h-6 text-accent" />
          <h2 className="font-semibold text-sm text-foreground">{title}</h2>
          <Badge variant="outline" tone="success" dot className="ms-auto font-mono">
            Active SSO Provider
          </Badge>
        </div>
        <dl className="grid sm:grid-cols-2 gap-x-6 gap-y-2">
          {rows.map(([label, url]) => (
            <div key={label} className="flex items-center gap-2 min-w-0">
              <dt className="text-xs uppercase tracking-wide text-muted w-24 shrink-0">{label}</dt>
              <dd className="font-mono text-xs text-foreground truncate flex-1">{url}</dd>
              <CopyIcon value={url} id={label} copied={copied} onCopy={onCopy} />
            </div>
          ))}
        </dl>
      </section>
    </Card>
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
    <Card asChild padding="none" className={`p-5 space-y-4 ${app.disabled ? "opacity-70" : ""}`}>
      <article>
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h3 className="font-semibold text-foreground truncate">{app.client_name}</h3>
            <p className="text-xs text-muted mt-0.5">
              {app.last_used_at
                ? `${t("sso_clients.field.last_used")}: ${formatDay(app.last_used_at)}`
                : t("sso_clients.message.never_used")}
            </p>
          </div>
          <div className="flex items-center gap-1.5 shrink-0">
            {app.disabled && <Badge tone="neutral">{t("sso_clients.message.disabled")}</Badge>}
            <Badge tone="accent" icon={app.client_type === "public" ? <Smartphone /> : <Server />}>
              {app.client_type}
            </Badge>
          </div>
        </div>

        <div className="text-xs font-mono bg-surface-2 p-3 rounded-lg border border-line space-y-2">
          <div className="flex items-center justify-between gap-2">
            <span className="text-muted shrink-0">client_id</span>
            <span className="text-foreground font-semibold truncate">{app.client_id}</span>
            <CopyIcon value={app.client_id} id={app.client_id} copied={copied} onCopy={onCopy} />
          </div>
          <div className="flex items-center justify-between gap-2">
            <span className="text-muted shrink-0">client_secret</span>
            <span className="text-muted italic font-sans text-xs">
              {app.client_type === "public" ? "—" : t("sso_clients.message.secret_hidden")}
            </span>
          </div>
        </div>

        <Field label={t("sso_clients.field.redirect_uris")}>
          {app.redirect_uris.map((uri) => (
            <Badge className="break-all font-mono" tone="neutral" key={uri}>{uri}</Badge>
          ))}
        </Field>

        {/* Only shown once there is one: an application that never ends a session
            here has no reason to carry an empty row about it. */}
        {(app.post_logout_redirect_uris || []).length > 0 && (
          <Field label={t("sso_clients.field.post_logout_redirect_uris")}>
            {app.post_logout_redirect_uris.map((uri) => (
              <Badge className="break-all font-mono" tone="neutral" key={uri}>{uri}</Badge>
            ))}
          </Field>
        )}

        <Field label={t("sso_clients.field.scopes")}>
          {app.scopes.map((scope) => (
            <Badge className="break-all font-mono" key={scope} tone={sensitive.has(scope) ? "warning" : "info"}>{scope}</Badge>
          ))}
        </Field>

        <Field label={t("sso_clients.field.grant_types")}>
          {app.grant_types.map((grant) => (
            <Badge className="break-all font-mono" key={grant} tone="neutral">{grant}</Badge>
          ))}
        </Field>

        {canManage && (
          <div className="flex gap-2 pt-3 border-t border-line">
            <Button variant="ghost" size="sm" onClick={onEdit}>
              {t("sso_clients.view.edit_title")}
            </Button>
            {app.client_type !== "public" && (
              <Button variant="ghost" size="sm" className="text-warning hover:bg-warning-soft" leadingIcon={<RefreshCw />} onClick={onRotate}>
                {t("sso_clients.action.rotate")}
              </Button>
            )}
            <Button variant="ghost" size="sm" className="ms-auto text-danger hover:bg-danger-soft" leadingIcon={<Trash2 />} onClick={onDelete}>
              {t("base.action.delete")}
            </Button>
          </div>
        )}
      </article>
    </Card>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <span className="text-xs font-semibold text-foreground block mb-1">{label}</span>
      <div className="flex flex-wrap gap-1">{children}</div>
    </div>
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
    // The shared dialog: Escape closes it, and a backdrop click does too until
    // something has been typed — a stray click must not lose a half-filled form.
    <Modal onClose={onCancel} size="lg" scrollable>
      <form onSubmit={submit} className="space-y-4">
        <DialogHeader>
          <DialogTitle>{isNew ? t("sso_clients.view.create_title") : t("sso_clients.view.edit_title")}</DialogTitle>
        </DialogHeader>

        <Input
          label={`${t("sso_clients.field.name")} *`}
          value={draft.client_name}
          onChange={(e) => setDraft({ ...draft, client_name: e.target.value })}
          required
        />

        <Input
          label={t("sso_clients.field.homepage")}
          value={draft.client_uri || ""}
          onChange={(e) => setDraft({ ...draft, client_uri: e.target.value })}
          placeholder="https://example.mn"
          className="font-mono"
        />

        {isNew && (
          <fieldset>
            <legend className="text-sm font-medium text-foreground mb-1.5">{t("sso_clients.field.client_type")}</legend>
            <RadioGroup
              value={draft.client_type}
              onValueChange={(type) => setDraft({ ...draft, client_type: type as OAuth2ClientDraft["client_type"] })}
              className="grid sm:grid-cols-2 gap-2"
            >
              {(["confidential", "public"] as const).map((type) => (
                <RadioItem
                  key={type}
                  value={type}
                  className={`border rounded-lg p-3 ${draft.client_type === type ? "border-accent bg-accent-soft" : "border-line"}`}
                  label={
                    <span className="flex items-center gap-1.5 text-xs font-semibold text-foreground">
                      {type === "public" ? <Smartphone className="w-3.5 h-3.5" /> : <Server className="w-3.5 h-3.5" />}
                      {type === "public" ? t("sso_clients.type.public") : t("sso_clients.type.confidential")}
                    </span>
                  }
                  description={type === "public" ? t("sso_clients.type.public_hint") : t("sso_clients.type.confidential_hint")}
                />
              ))}
            </RadioGroup>
          </fieldset>
        )}

        <Textarea
          label={`${t("sso_clients.field.redirect_uris")} *`}
          value={uris}
          onChange={(e) => setUris(e.target.value)}
          rows={3}
          placeholder={"https://app.example.mn/callback\nhttp://localhost:3000/callback"}
          className="font-mono"
          // Exact matching is what stops a code being delivered somewhere else.
          helperText="one per line · https only, except on localhost"
        />

        <Textarea
          label={t("sso_clients.field.post_logout_redirect_uris")}
          value={logoutUris}
          onChange={(e) => setLogoutUris(e.target.value)}
          rows={2}
          placeholder={"https://app.example.mn/"}
          className="font-mono"
          // Matched exactly too: an unchecked return address would make the
          // logout endpoint an open redirector.
          helperText={t("sso_clients.hint.post_logout_redirect_uris")}
        />

        <fieldset>
          <legend className="text-sm font-medium text-foreground mb-1.5">{t("sso_clients.field.grant_types")}</legend>
          <div className="flex flex-wrap gap-x-4 gap-y-2">
            {grantTypes.map((grant) => (
              <Checkbox
                key={grant}
                checked={Boolean(draft.grant_types?.includes(grant))}
                onCheckedChange={() => toggle("grant_types", grant)}
                label={<span className="font-mono text-xs">{grant}</span>}
              />
            ))}
          </div>
          <p className="text-xs text-muted mt-1">{t("sso_clients.message.pkce_note")}</p>
        </fieldset>

        <fieldset>
          <legend className="text-sm font-medium text-foreground mb-1.5">{t("sso_clients.field.scopes")}</legend>
          <div className="space-y-1.5 max-h-52 overflow-y-auto pe-1">
            {scopes.map((scope) => (
              <Checkbox
                key={scope.name}
                checked={Boolean(draft.scopes?.includes(scope.name))}
                onCheckedChange={() => toggle("scopes", scope.name)}
                label={
                  <span className="text-xs">
                    <span className="font-mono text-foreground">{scope.name}</span>
                    {scope.sensitive && <span className="ms-1.5"><Badge tone="warning">sensitive</Badge></span>}
                  </span>
                }
                description={describe(scope)}
              />
            ))}
          </div>
        </fieldset>

        {!isNew && (
          <Checkbox
            checked={Boolean(draft.disabled)}
            onCheckedChange={(checked) => setDraft({ ...draft, disabled: checked === true })}
            label={t("sso_clients.action.disable")}
          />
        )}

        <div className="flex justify-end gap-2 pt-2">
          <Button variant="ghost" type="button" onClick={onCancel}>
            {t("base.action.cancel")}
          </Button>
          <Button type="submit">
            {t("base.action.save")}
          </Button>
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
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto">
        <div className="space-y-4">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <KeyRound className="size-5 text-warning" aria-hidden />
              {t("sso_clients.message.secret_once_title")}
            </DialogTitle>
            <DialogDescription>{t("sso_clients.message.secret_once_body")}</DialogDescription>
          </DialogHeader>

          <div className="space-y-2">
            <ReadOnlyField label="client_id" value={clientID} copied={copied} onCopy={onCopy} />
            <ReadOnlyField label="client_secret" value={secret} copied={copied} onCopy={onCopy} highlight />
          </div>

          <div className="flex justify-end">
            <Button onClick={onClose}>{t("sso_clients.action.done")}</Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function ReadOnlyField({ label, value, copied, onCopy, highlight }: {
  label: string; value: string; copied: string; highlight?: boolean;
  onCopy: (value: string, id: string) => void;
}) {
  return (
    <div className={`flex items-center gap-2 p-3 rounded-lg border ${highlight ? "bg-warning-soft border-warning-border" : "bg-surface-2 border-line"}`}>
      <span className="text-xs font-semibold text-muted w-24 shrink-0">{label}</span>
      <code className="text-xs font-mono text-foreground break-all flex-1">{value}</code>
      <CopyIcon value={value} id={label} copied={copied} onCopy={onCopy} />
    </div>
  );
}
