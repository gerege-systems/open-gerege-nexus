"use client";

import React, { useEffect, useState } from "react";
import { AlertTriangle, CheckCircle2, Plug, ServerCog, XCircle } from "lucide-react";
import { esign, type HSMSettings, type Probe } from "@/lib/esign";
import { useI18n } from "@/lib/i18n";
import { Banner, Loading, PageHeader } from "@/components/ui";
import { Card, useErrorMessage } from "@/components/esign/shared";

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
  if (!hsm) return <Banner tone="error" message={error ?? t("base.message.error")} />;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ServerCog className="w-7 h-7 text-indigo-600" />}
        title={t("esign.view.hsm_title")}
        subtitle={t("esign.view.hsm_subtitle")}
        actions={
          <button
            onClick={test}
            disabled={testing}
            className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white text-xs font-semibold px-4 py-2 rounded-lg flex items-center gap-2 shadow-sm"
          >
            <Plug className="w-4 h-4" />
            {testing ? t("esign.message.testing") : t("esign.action.test_connection")}
          </button>
        }
      />

      {error && <Banner tone="error" message={error} onDismiss={() => setError(null)} />}

      {hsm.mock_mode && <Banner tone="info" message={t("esign.message.hsm_mock_mode")} />}
      {!hsm.mock_mode && !hsm.has_token && (
        <Banner tone="error" message={t("esign.message.hsm_no_token")} />
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
                  <span className="text-amber-700 font-semibold">{t("esign.state.mock")}</span>
                ) : (
                  <span className="text-emerald-700 font-semibold">{t("esign.state.live")}</span>
                )
              }
            />
            <Row
              label={t("esign.field.token")}
              value={
                hsm.has_token ? (
                  <span className="inline-flex items-center gap-1.5 text-emerald-700 font-semibold">
                    <CheckCircle2 className="w-4 h-4" />
                    {t("esign.state.token_present")}
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1.5 text-muted">
                    <XCircle className="w-4 h-4" />
                    {t("esign.state.token_missing")}
                  </span>
                )
              }
            />
          </dl>
          <p className="px-4 py-3 text-[11px] text-muted border-t border-line flex items-start gap-1.5">
            <AlertTriangle className="w-3.5 h-3.5 mt-0.5 shrink-0" />
            {t("esign.message.hsm_env_managed")}
          </p>
        </Card>

        <Card title={t("esign.view.hsm_last_probe")}>
          {probe ? (
            <div className="p-4 space-y-3">
              <div
                className={`p-3 rounded-lg border text-sm flex items-start gap-2 ${
                  probe.ok
                    ? "bg-emerald-50 border-emerald-200 text-emerald-700"
                    : "bg-red-50 border-red-200 text-red-700"
                }`}
              >
                {probe.ok ? (
                  <CheckCircle2 className="w-4 h-4 mt-0.5 shrink-0" />
                ) : (
                  <XCircle className="w-4 h-4 mt-0.5 shrink-0" />
                )}
                <span>{probe.message}</span>
              </div>
              <dl className="divide-y divide-line text-sm border-t border-line">
                <Row label={t("esign.field.latency")} value={<span className="font-mono">{probe.latency_ms} ms</span>} />
                <Row label={t("esign.field.checked_at")} value={new Date(probe.checked_at).toLocaleString()} />
                {probe.checked_by && <Row label={t("esign.field.checked_by")} value={probe.checked_by} />}
              </dl>
            </div>
          ) : (
            <p className="p-6 text-sm text-muted text-center italic">{t("esign.message.no_probe_yet")}</p>
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
