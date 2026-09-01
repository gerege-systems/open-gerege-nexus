"use client";

/**
 * Access audit — what this tenant's clients currently hold.
 *
 * Two questions this answers that nothing did before: which credential is
 * nobody using any more (delete it), and which user granted a client access
 * they have since forgotten about (withdraw it).
 */

import { useEffect, useState } from "react";
import { ScrollText, ShieldOff, UserMinus, Users } from "lucide-react";
import { api, type ClientActivity, type ConsentRecord } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import {
  Chip, ConfirmDialog, Empty, ErrorNote, Loading, Panel, ReadOnlyNote, Screen,
  relativeDate, useAccess,
} from "../shared";
import { formatDay } from "@/lib/datetime";

type Pending =
  | { kind: "tokens"; client: ClientActivity }
  | { kind: "consent"; consent: ConsentRecord };

export default function AccessAuditPage() {
  const { t, locale } = useI18n();
  // Revoking a token and withdrawing a consent are both mutations.
  const { allowed: canManage } = useAccess("sso_clients.manage");
  const [clients, setClients] = useState<ClientActivity[]>([]);
  const [consents, setConsents] = useState<ConsentRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState<Pending | null>(null);

  async function load() {
    setLoading(true);
    setError("");
    try {
      const data = await api.getSSOClientAudit();
      setClients(data.clients || []);
      setConsents(data.consents || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("base.message.error"));
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { void load(); }, []);

  async function runPending() {
    if (!pending) return;
    const target = pending;
    setPending(null);
    setError("");
    try {
      if (target.kind === "tokens") {
        const { revoked } = await api.revokeSSOClientTokens(target.client.client_id);
        setNotice(t("sso_clients.audit.revoked_count", { n: revoked }));
        setTimeout(() => setNotice(""), 4000);
      } else {
        await api.withdrawSSOClientConsent(target.consent.client_id, target.consent.user_id);
      }
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("base.message.error"));
    }
  }

  return (
    <Screen
      icon={<ScrollText className="w-5 h-5" />}
      title={t("sso_clients.audit.title")}
      subtitle={t("sso_clients.audit.subtitle")}
    >
      {!canManage && <ReadOnlyNote permission="sso_clients.manage" />}
      {error && <ErrorNote>{error}</ErrorNote>}
      {notice && (
        <p className="text-sm text-emerald-700 bg-emerald-50 border border-emerald-200 rounded-lg px-3 py-2">
          {notice}
        </p>
      )}

      {loading ? (
        <Loading label={t("sso_clients.message.loading")} />
      ) : clients.length === 0 ? (
        <Empty icon={<ScrollText className="w-9 h-9 mx-auto" />}>{t("sso_clients.audit.no_activity")}</Empty>
      ) : (
        <Panel className="overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-surface-2 border-b border-line text-left">
                <tr className="text-[11px] uppercase tracking-wide text-muted">
                  <th className="px-4 py-2.5 font-semibold">{t("sso_clients.field.name")}</th>
                  <th className="px-4 py-2.5 font-semibold text-right">{t("sso_clients.audit.active_access")}</th>
                  <th className="px-4 py-2.5 font-semibold text-right">{t("sso_clients.audit.active_refresh")}</th>
                  <th className="px-4 py-2.5 font-semibold text-right">{t("sso_clients.audit.consented")}</th>
                  <th className="px-4 py-2.5 font-semibold">{t("sso_clients.field.last_used")}</th>
                  <th className="px-4 py-2.5" />
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {clients.map((client) => {
                  const live = client.active_access_tokens + client.active_refresh_tokens;
                  return (
                    <tr key={client.client_id} className="hover:bg-slate-50/60">
                      <td className="px-4 py-3">
                        <div className="font-semibold text-foreground flex items-center gap-2">
                          {client.client_name}
                          {client.disabled && <Chip tone="rose">{t("sso_clients.message.disabled")}</Chip>}
                        </div>
                        <div className="text-[11px] font-mono text-muted">{client.client_id}</div>
                      </td>
                      <td className="px-4 py-3 text-right tabular-nums font-semibold text-foreground">
                        {client.active_access_tokens}
                      </td>
                      <td className="px-4 py-3 text-right tabular-nums font-semibold text-foreground">
                        {client.active_refresh_tokens}
                      </td>
                      <td className="px-4 py-3 text-right tabular-nums text-muted">{client.consented_users}</td>
                      <td className="px-4 py-3 text-muted text-xs">
                        {relativeDate(client.last_used_at, t("sso_clients.message.never_used"), locale)}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          onClick={() => setPending({ kind: "tokens", client })}
                          disabled={live === 0 || !canManage}
                          hidden={!canManage}
                          className="text-xs font-semibold text-rose-700 hover:bg-rose-50 px-2.5 py-1.5 rounded-lg inline-flex items-center gap-1.5 disabled:opacity-30 disabled:hover:bg-transparent"
                        >
                          <ShieldOff className="w-3.5 h-3.5" /> {t("sso_clients.audit.revoke_tokens")}
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </Panel>
      )}

      <section className="space-y-3">
        <h2 className="text-sm font-semibold text-foreground flex items-center gap-2">
          <Users className="w-4 h-4 text-muted" /> {t("sso_clients.audit.consents_title")}
        </h2>
        {loading ? null : consents.length === 0 ? (
          <Empty icon={<Users className="w-9 h-9 mx-auto" />}>{t("sso_clients.audit.no_consents")}</Empty>
        ) : (
          <Panel className="divide-y divide-line">
            {consents.map((consent) => (
              <div key={`${consent.client_id}:${consent.user_id}`} className="p-4 flex flex-wrap items-start gap-3">
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-semibold text-foreground">
                    {consent.user_name || consent.user_email}
                    <span className="font-normal text-muted"> → </span>
                    {consent.client_name}
                  </p>
                  <p className="text-[11px] text-muted">
                    {consent.user_email} · {formatDay(consent.granted_at)}
                  </p>
                  <div className="flex flex-wrap gap-1 mt-1.5">
                    {consent.scopes.map((scope) => <Chip key={scope} mono tone="blue">{scope}</Chip>)}
                  </div>
                </div>
                <button
                  onClick={() => setPending({ kind: "consent", consent })}
                  hidden={!canManage}
                  className="text-xs font-semibold text-rose-700 hover:bg-rose-50 px-3 py-1.5 rounded-lg flex items-center gap-1.5"
                >
                  <UserMinus className="w-3.5 h-3.5" /> {t("sso_clients.audit.withdraw")}
                </button>
              </div>
            ))}
          </Panel>
        )}
      </section>

      {pending && (
        <ConfirmDialog
          danger
          title={pending.kind === "tokens" ? pending.client.client_name : pending.consent.user_email}
          body={pending.kind === "tokens" ? t("sso_clients.audit.revoke_warning") : t("sso_clients.audit.withdraw_warning")}
          confirmLabel={pending.kind === "tokens" ? t("sso_clients.audit.revoke_tokens") : t("sso_clients.audit.withdraw")}
          onCancel={() => setPending(null)}
          onConfirm={runPending}
        />
      )}
    </Screen>
  );
}
