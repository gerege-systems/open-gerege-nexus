"use client";

/**
 * Redirect policies — every registered callback, and the rules behind them.
 *
 * The authorization endpoint matches a redirect_uri exactly against this list,
 * so the list is the security boundary. Seeing it in one place is how you
 * notice the loopback URI somebody left on a production integration.
 */

import { useEffect, useMemo, useState } from "react";
import { Globe, Laptop, Route, Server, ShieldCheck, Smartphone } from "lucide-react";
import {
  Alert, Badge, Card, EmptyState, Spinner, Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@gerege-systems/ui";
import { api, type OAuth2Client } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { PageHeader } from "@/components/ui";
import { CopyButton, useCopy } from "../shared";

type Entry = { client: OAuth2Client; uri: string; kind: "https" | "loopback" | "custom" };

function classify(uri: string): Entry["kind"] {
  try {
    const parsed = new URL(uri);
    if (parsed.protocol === "https:") return "https";
    if (parsed.protocol === "http:") return "loopback";
    return "custom";
  } catch {
    return "custom";
  }
}

export default function RedirectPoliciesPage() {
  const { t } = useI18n();
  const { copied, copy } = useCopy();
  const [clients, setClients] = useState<OAuth2Client[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    void (async () => {
      try {
        setClients((await api.getSSOClients()) || []);
      } catch (err) {
        setError(err instanceof Error ? err.message : t("base.message.error"));
      } finally {
        setLoading(false);
      }
    })();
  }, [t]);

  const entries = useMemo<Entry[]>(
    () => clients.flatMap((client) => client.redirect_uris.map((uri) => ({ client, uri, kind: classify(uri) }))),
    [clients],
  );

  // A client registered only for client_credentials never receives a redirect,
  // so its absence from the list below is correct rather than a gap.
  const machineOnly = clients.filter((c) => c.redirect_uris.length === 0);

  const rules = [
    { icon: <ShieldCheck className="w-4 h-4" />, text: t("sso_clients.redirects.rule_exact") },
    { icon: <Globe className="w-4 h-4" />, text: t("sso_clients.redirects.rule_https") },
    { icon: <Route className="w-4 h-4" />, text: t("sso_clients.redirects.rule_fragment") },
    { icon: <Smartphone className="w-4 h-4" />, text: t("sso_clients.redirects.rule_custom") },
  ];

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Route className="w-5 h-5" />}
        title={t("sso_clients.redirects.title")}
        subtitle={t("sso_clients.redirects.subtitle")}
      />
      {error && <Alert variant="danger" live>{error}</Alert>}

      <Card padding="none" className="p-5">
        <h2 className="text-sm font-semibold text-foreground mb-3">{t("sso_clients.redirects.rules_title")}</h2>
        <ul className="space-y-2.5">
          {rules.map((rule, index) => (
            <li key={index} className="flex items-start gap-2.5 text-xs text-muted">
              <span className="text-accent shrink-0 mt-0.5">{rule.icon}</span>
              {rule.text}
            </li>
          ))}
        </ul>
      </Card>

      {loading ? (
        <p className="flex items-center justify-center gap-2 p-12 text-center text-muted" role="status">
          <Spinner size="md" decorative />
          {t("sso_clients.message.loading")}
        </p>
      ) : entries.length === 0 ? (
        <EmptyState icon={<Route />} title={t("sso_clients.redirects.none")} />
      ) : (
        <Card padding="none" className="overflow-hidden">
          <Table containerClassName="rounded-none border-0">
            <TableHeader>
              <TableRow>
                <TableHead>{t("sso_clients.field.name")}</TableHead>
                <TableHead>redirect_uri</TableHead>
                <TableHead>{t("base.field.type")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {entries.map(({ client, uri, kind }) => (
                <TableRow key={`${client.client_id}:${uri}`}>
                  <TableCell>
                    <div className="font-semibold text-foreground flex items-center gap-1.5">
                      {client.client_type === "public"
                        ? <Smartphone className="w-3.5 h-3.5 text-muted" />
                        : <Server className="w-3.5 h-3.5 text-muted" />}
                      {client.client_name}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <code className="text-xs font-mono text-foreground break-all">{uri}</code>
                      <CopyButton value={uri} id={`${client.client_id}:${uri}`} copied={copied} onCopy={copy} />
                    </div>
                  </TableCell>
                  <TableCell>
                    {kind === "https" && <Badge className="break-all" tone="success">https</Badge>}
                    {kind === "loopback" && (
                      <span className="inline-flex items-center gap-1">
                        <Laptop className="w-3.5 h-3.5 text-warning" />
                        <Badge className="break-all" tone="warning">{t("sso_clients.redirects.loopback")}</Badge>
                      </span>
                    )}
                    {kind === "custom" && <Badge className="break-all" tone="info">custom scheme</Badge>}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}

      {machineOnly.length > 0 && (
        <Card padding="none" className="p-4">
          <p className="text-xs font-semibold text-muted mb-2">
            {t("sso_clients.redirects.no_redirect_needed")}
          </p>
          <div className="flex flex-wrap gap-1">
            {machineOnly.map((client) => (
              <Badge className="break-all" tone="neutral" key={client.client_id}>{client.client_name}</Badge>
            ))}
          </div>
        </Card>
      )}
    </div>
  );
}
