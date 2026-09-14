"use client";

import React, { useEffect, useState } from "react";
import { AlertTriangle, CheckCircle2, Plug, ServerCog, XCircle } from "lucide-react";
import { esign, type HSMSettings, type Probe } from "@/lib/esign";
import { useI18n } from "@/lib/i18n";
import { Loading, PageHeader } from "@/components/ui";
import { Card, useErrorMessage } from "@/components/esign/shared";
import { Alert, Badge, Button, EmptyState } from "@gerege-systems/ui";

/**
 * The HSM connection.
 *
 * Everything here is read-only. The endpoints, the mode and the token are
 * deployment facts held in the environment, not tenant preferences — showing
 * an editable value the running process is not actually using would make this
 * screen a liar, and the token must never come back to a browser at all. What
 * the tenant *can* do is prove the connection works.
 */
export default function EsignHSMPage() {
  const { t } = useI18n();
  const describe = useErrorMessage();
  const [hsm, setHsm] = useState<HSMSettings | null>(null);
  const [probe, setProbe] = useState<Probe | null>(null);
  const [loading, setLoading] = useState(true);
  const [testing, setTesting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    esign
      .settings()
      .then((settings) => {
        setHsm(settings.hsm);
        setProbe(settings.hsm.last_probe ?? null);
      })
      .catch((err) => setError(describe(err, t("base.message.error"))))
      .finally(() => setLoading(false));
  }, [describe, t]);

  const test = async () => {
    setTesting(true);
    setError(null);
    try {
      setProbe(await esign.testHSM());
    } catch (err) {
      setError(describe(err, t("base.message.error")));
    } finally {
      setTesting(false);
    }
  };

  if (loading) return <Loading />;
  if (!hsm) return <Alert variant="danger" live>{error ?? t("base.message.error")}</Alert>;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ServerCog className="w-7 h-7 text-accent" />}
        title={t("esign.view.hsm_title")}
        subtitle={t("esign.view.hsm_subtitle")}
        actions={
          <Button onClick={test} loading={testing} leadingIcon={<Plug />}>
            {testing ? t("esign.message.testing") : t("esign.action.test_connection")}
          </Button>
        }
      />

      {error && <Alert variant="danger" live dismissible onDismiss={() => setError(null)}>{error}</Alert>}

      {hsm.mock_mode && <Alert variant="info" live>{t("esign.message.hsm_mock_mode")}</Alert>}
      {!hsm.mock_mode && !hsm.has_token && (
        <Alert variant="danger" live>{t("esign.message.hsm_no_token")}</Alert>
      )}

      <div className="grid lg:grid-cols-2 gap-6 items-start">
        <Card title={t("esign.view.hsm_connection")}>
          <dl className="divide-y divide-line text-sm">
            <Row label={t("esign.field.login_url")} value={<code className="text-xs break-all">{hsm.login_url}</code>} />
            <Row label={t("esign.field.sign_url")} value={<code className="text-xs break-all">{hsm.sign_url}</code>} />
            <Row
              label={t("esign.field.mode")}
              value={
                hsm.mock_mode ? (
                  <Badge tone="warning">{t("esign.state.mock")}</Badge>
                ) : (
                  <Badge tone="success">{t("esign.state.live")}</Badge>
                )
              }
            />
            <Row
              label={t("esign.field.token")}
              value={
                hsm.has_token ? (
                  <Badge tone="success" icon={<CheckCircle2 />}>{t("esign.state.token_present")}</Badge>
                ) : (
                  <Badge tone="neutral" icon={<XCircle />}>{t("esign.state.token_missing")}</Badge>
                )
              }
            />
          </dl>
          <p className="px-4 py-3 text-xs text-muted border-t border-line flex items-start gap-1.5">
            <AlertTriangle className="w-3.5 h-3.5 mt-0.5 shrink-0" aria-hidden />
            {t("esign.message.hsm_env_managed")}
          </p>
        </Card>

        <Card title={t("esign.view.hsm_last_probe")}>
          {probe ? (
            <div className="p-4 space-y-3">
              <Alert variant={probe.ok ? "success" : "danger"}>{probe.message}</Alert>
              <dl className="divide-y divide-line text-sm border-t border-line">
                <Row label={t("esign.field.latency")} value={<span className="font-mono">{probe.latency_ms} ms</span>} />
                <Row label={t("esign.field.checked_at")} value={new Date(probe.checked_at).toLocaleString()} />
                {probe.checked_by && <Row label={t("esign.field.checked_by")} value={probe.checked_by} />}
              </dl>
            </div>
          ) : (
            <div className="p-4">
              <EmptyState icon={<Plug />} title={t("esign.message.no_probe_yet")} />
            </div>
          )}
        </Card>
      </div>
    </div>
  );
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="px-4 py-3 flex items-start justify-between gap-4">
      <dt className="text-muted shrink-0">{label}</dt>
      <dd className="text-foreground text-right min-w-0">{value}</dd>
    </div>
  );
}
