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
import {
  Alert, Badge, Button, Card, ConfirmationDialog, EmptyState, Spinner,
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@gerege-systems/ui";
import { api, type ClientActivity, type ConsentRecord } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { ReadOnlyNote, useAccess } from "@/lib/permissions";
import { PageHeader } from "@/components/ui";
import { relativeDate } from "@/components/module/kit";
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
    <div className="space-y-6">
      <PageHeader
        icon={<ScrollText className="w-5 h-5" />}
        title={t("sso_clients.audit.title")}
        subtitle={t("sso_clients.audit.subtitle")}
      />
      {!canManage && <ReadOnlyNote permission="sso_clients.manage" />}
      {error && <Alert variant="danger" live>{error}</Alert>}
      {notice && <Alert variant="success" live>{notice}</Alert>}

      {loading ? (
        <p className="flex items-center justify-center gap-2 p-12 text-center text-muted" role="status">
          <Spinner size="md" decorative />
          {t("sso_clients.message.loading")}
        </p>
      ) : clients.length === 0 ? (
        <EmptyState icon={<ScrollText />} title={t("sso_clients.audit.no_activity")} />
      ) : (
        <Card padding="none" className="overflow-hidden">
          <Table containerClassName="rounded-none border-0">
            <TableHeader>
              <TableRow>
                <TableHead>{t("sso_clients.field.name")}</TableHead>
                <TableHead align="right">{t("sso_clients.audit.active_access")}</TableHead>
                <TableHead align="right">{t("sso_clients.audit.active_refresh")}</TableHead>
                <TableHead align="right">{t("sso_clients.audit.consented")}</TableHead>
                <TableHead>{t("sso_clients.field.last_used")}</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {clients.map((client) => {
                const live = client.active_access_tokens + client.active_refresh_tokens;
                return (
                  <TableRow key={client.client_id}>
                    <TableCell>
                      <div className="font-semibold text-foreground flex items-center gap-2">
                        {client.client_name}
                        {client.disabled && <Badge className="break-all" tone="danger">{t("sso_clients.message.disabled")}</Badge>}
                      </div>
                      <div className="text-xs font-mono text-muted">{client.client_id}</div>
                    </TableCell>
                    <TableCell align="right" className="tabular-nums font-semibold">{client.active_access_tokens}</TableCell>
                    <TableCell align="right" className="tabular-nums font-semibold">{client.active_refresh_tokens}</TableCell>
                    <TableCell align="right" className="tabular-nums text-muted">{client.consented_users}</TableCell>
                    <TableCell className="text-muted text-xs">
                      {relativeDate(client.last_used_at, t("sso_clients.message.never_used"), locale)}
                    </TableCell>
                    <TableCell align="right">
                      {canManage && (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="text-danger hover:bg-danger-soft"
                          leadingIcon={<ShieldOff />}
                          disabled={live === 0}
                          onClick={() => setPending({ kind: "tokens", client })}
                        >
                          {t("sso_clients.audit.revoke_tokens")}
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </Card>
      )}

      <section className="space-y-3">
        <h2 className="text-sm font-semibold text-foreground flex items-center gap-2">
          <Users className="w-4 h-4 text-muted" /> {t("sso_clients.audit.consents_title")}
        </h2>
        {loading ? null : consents.length === 0 ? (
          <EmptyState icon={<Users />} title={t("sso_clients.audit.no_consents")} />
        ) : (
          <Card padding="none" className="divide-y divide-line">
            {consents.map((consent) => (
              <div key={`${consent.client_id}:${consent.user_id}`} className="p-4 flex flex-wrap items-start gap-3">
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-semibold text-foreground">
                    {consent.user_name || consent.user_email}
                    <span className="font-normal text-muted"> → </span>
                    {consent.client_name}
                  </p>
                  <p className="text-xs text-muted">
                    {consent.user_email} · {formatDay(consent.granted_at)}
                  </p>
                  <div className="flex flex-wrap gap-1 mt-1.5">
                    {consent.scopes.map((scope) => (
                      <Badge className="break-all font-mono" key={scope} tone="info">{scope}</Badge>
                    ))}
                  </div>
                </div>
                {canManage && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-danger hover:bg-danger-soft"
                    leadingIcon={<UserMinus />}
                    onClick={() => setPending({ kind: "consent", consent })}
                  >
                    {t("sso_clients.audit.withdraw")}
                  </Button>
                )}
              </div>
            ))}
          </Card>
        )}
      </section>

      {pending && (
        <ConfirmationDialog
          open
          onOpenChange={(open) => { if (!open) setPending(null); }}
          confirmVariant="destructive"
          title={pending.kind === "tokens" ? pending.client.client_name : pending.consent.user_email}
          description={pending.kind === "tokens" ? t("sso_clients.audit.revoke_warning") : t("sso_clients.audit.withdraw_warning")}
          confirmLabel={pending.kind === "tokens" ? t("sso_clients.audit.revoke_tokens") : t("sso_clients.audit.withdraw")}
          cancelLabel={t("base.action.cancel")}
          onConfirm={() => void runPending()}
        />
      )}
    </div>
  );
}
