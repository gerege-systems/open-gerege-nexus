"use client";

/**
 * OAuth scopes — the vocabulary, and who asks for what.
 *
 * The descriptions here are the exact strings the consent screen renders, read
 * from the same API the picker uses, so what a developer selects and what a
 * user is asked to approve cannot drift apart.
 */

import { useEffect, useMemo, useState } from "react";
import { ShieldCheck } from "lucide-react";
import { Alert, Badge, Card, EmptyState, Spinner } from "@gerege-systems/ui";
import { api, type OAuth2Client, type OAuth2Scope } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { PageHeader } from "@/components/ui";

export default function OAuthScopesPage() {
  const { t, locale } = useI18n();
  const [scopes, setScopes] = useState<OAuth2Scope[]>([]);
  const [clients, setClients] = useState<OAuth2Client[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    void (async () => {
      try {
        const [vocabulary, apps] = await Promise.all([api.getSSOClientScopes(), api.getSSOClients()]);
        setScopes(vocabulary.scopes || []);
        setClients(apps || []);
      } catch (err) {
        setError(err instanceof Error ? err.message : t("base.message.error"));
      } finally {
        setLoading(false);
      }
    })();
  }, [t]);

  const usage = useMemo(() => {
    const map = new Map<string, OAuth2Client[]>();
    for (const client of clients) {
      for (const scope of client.scopes) {
        map.set(scope, [...(map.get(scope) || []), client]);
      }
    }
    return map;
  }, [clients]);

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ShieldCheck className="w-5 h-5" />}
        title={t("sso_clients.scopes.title")}
        subtitle={t("sso_clients.scopes.subtitle")}
      />
      {error && <Alert variant="danger" live>{error}</Alert>}

      <Card padding="none" className="p-4 bg-surface-2">
        <p className="text-xs text-muted">{t("sso_clients.scopes.sensitive_note")}</p>
      </Card>

      {loading ? (
        <p className="flex items-center justify-center gap-2 p-12 text-center text-muted" role="status">
          <Spinner size="md" decorative />
          {t("sso_clients.message.loading")}
        </p>
      ) : scopes.length === 0 ? (
        <EmptyState icon={<ShieldCheck />} title={t("base.message.error")} />
      ) : (
        <div className="grid gap-3 lg:grid-cols-2">
          {scopes.map((scope) => {
            const users = usage.get(scope.name) || [];
            return (
              <Card padding="none" key={scope.name} className={`p-4 ${scope.sensitive ? "border-warning-border" : ""}`}>
                <div className="flex items-start justify-between gap-2">
                  <code className="text-sm font-mono font-semibold text-foreground">{scope.name}</code>
                  {scope.sensitive && <Badge className="break-all" tone="warning">{t("oauth.consent.sensitive")}</Badge>}
                </div>

                <p className="text-xs text-muted mt-2">{t("sso_clients.scopes.consent_preview")}</p>
                <p className="text-sm text-foreground border-s-2 border-line ps-3 mt-1">
                  {locale === "mn" ? scope.description_mn : scope.description}
                </p>

                <div className="mt-3 pt-3 border-t border-line">
                  <span className="text-xs font-semibold text-muted">
                    {t("sso_clients.scopes.used_by")}
                  </span>
                  {users.length === 0 ? (
                    <p className="text-xs text-muted mt-1">{t("sso_clients.scopes.unused")}</p>
                  ) : (
                    <div className="flex flex-wrap gap-1 mt-1">
                      {users.map((client) => (
                        <Badge className="break-all" key={client.client_id} tone="neutral">{client.client_name}</Badge>
                      ))}
                    </div>
                  )}
                </div>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
