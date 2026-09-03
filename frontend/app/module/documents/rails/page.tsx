"use client";

import React, { useEffect, useState } from "react";
import { Save, ShieldCheck } from "lucide-react";
import { esign, type Policy } from "@/lib/esign";
import { useI18n } from "@/lib/i18n";
import { Banner, Loading, PageHeader, fieldClass } from "@/components/ui";
import { Card, useErrorMessage } from "@/components/esign/shared";

/**
 * Signing policy — the rules that decide what counts as a valid signature here.
 *
 * The consequential one is "require eID". Only the eID rail produces a
 * qualified electronic signature; the HSM holds the key on the operator's side,
 * which is a weaker claim in law. Turning this on disables the HSM rail
 * everywhere, including for callers hitting the API directly.
 */
export default function EsignPoliciesPage() {
  const { t } = useI18n();
  const describe = useErrorMessage();
  const [policy, setPolicy] = useState<Policy | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => {
    esign
      .settings()
      .then((settings) => setPolicy(settings.policy))
      .catch((err) => setError(describe(err, t("base.message.error"))))
      .finally(() => setLoading(false));
  }, [describe, t]);

  const update = (patch: Partial<Policy>) => setPolicy((current) => (current ? { ...current, ...patch } : current));

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!policy) return;
    setSaving(true);
    setError(null);
    try {
      setPolicy(await esign.savePolicy(policy));
      setNotice(t("esign.message.policy_saved"));
    } catch (err) {
      setError(describe(err, t("base.message.error")));
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <Loading />;
  if (!policy) return <Banner tone="error" message={error ?? t("base.message.error")} />;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ShieldCheck className="w-7 h-7 text-indigo-600" />}
        title={t("esign.view.policies_title")}
        subtitle={t("esign.view.policies_subtitle")}
      />

      {error && <Banner tone="error" message={error} onDismiss={() => setError(null)} />}
      {notice && <Banner tone="success" message={notice} onDismiss={() => setNotice(null)} />}

      <form onSubmit={save} className="space-y-6 max-w-2xl">
        <Card title={t("esign.view.policy_rails")}>
          <div className="p-4 space-y-4">
            <div>
              <label htmlFor="policy-provider" className="block text-xs font-semibold text-foreground mb-1">
                {t("esign.field.default_provider")}
              </label>
              <select
                id="policy-provider"
                value={policy.default_provider}
                onChange={(event) => update({ default_provider: event.target.value as Policy["default_provider"] })}
                disabled={policy.require_eid}
                className="w-full h-[38px] px-3 py-2 text-sm border border-input rounded-lg bg-surface focus:ring-2 focus:ring-indigo-500 disabled:bg-surface-2"
              >
                <option value="EID">eID Mongolia (PIN2)</option>
                <option value="HSM">Gerege eSign HSM</option>
              </select>
              <p className="text-[11px] text-muted mt-1">{t("esign.field.default_provider_hint")}</p>
            </div>

            <Toggle
              id="policy-require-eid"
              label={t("esign.field.require_eid")}
              hint={t("esign.field.require_eid_hint")}
              checked={policy.require_eid}
              // Not disabled from here: whether eID is reachable is a
              // deployment fact the browser cannot see, so the server refuses
              // the save with EID_NOT_CONFIGURED and that message is shown.
              onChange={(require_eid) =>
                // Requiring eID while the default names the HSM would refuse
                // every signature it started, so the two move together.
                update(require_eid ? { require_eid, default_provider: "EID" } : { require_eid })
              }
            />

            <div>
              <label htmlFor="policy-level" className="block text-xs font-semibold text-foreground mb-1">
                {t("esign.field.min_certificate_level")}
              </label>
              <select
                id="policy-level"
                value={policy.min_certificate_level}
                onChange={(event) =>
                  update({ min_certificate_level: event.target.value as Policy["min_certificate_level"] })
                }
                className="w-full h-[38px] px-3 py-2 text-sm border border-input rounded-lg bg-surface focus:ring-2 focus:ring-indigo-500"
              >
                <option value="ADVANCED">ADVANCED</option>
                <option value="QUALIFIED">QUALIFIED</option>
                <option value="QSCD">QSCD</option>
              </select>
              <p className="text-[11px] text-muted mt-1">{t("esign.field.min_certificate_level_hint")}</p>
            </div>
          </div>
        </Card>

        <Card title={t("esign.view.policy_rules")}>
          <div className="p-4 space-y-4">
            <Toggle
              id="policy-onbehalf"
              label={t("esign.field.allow_on_behalf_of")}
              hint={t("esign.field.allow_on_behalf_of_hint")}
              checked={policy.allow_on_behalf_of}
              onChange={(allow_on_behalf_of) => update({ allow_on_behalf_of })}
            />
            <Toggle
              id="policy-selfsign"
              label={t("esign.field.allow_self_sign")}
              hint={t("esign.field.allow_self_sign_hint")}
              checked={policy.allow_self_sign}
              onChange={(allow_self_sign) => update({ allow_self_sign })}
            />

            <div className="grid sm:grid-cols-2 gap-4">
              <div>
                <label htmlFor="policy-retention" className="block text-xs font-semibold text-foreground mb-1">
                  {t("esign.field.retention_days")}
                </label>
                <input
                  id="policy-retention"
                  type="number"
                  min={0}
                  value={policy.retention_days}
                  onChange={(event) => update({ retention_days: Number(event.target.value) })}
                  className={fieldClass}
                />
                <p className="text-[11px] text-muted mt-1">{t("esign.field.retention_days_hint")}</p>
              </div>
              <div>
                <label htmlFor="policy-upload" className="block text-xs font-semibold text-foreground mb-1">
                  {t("esign.field.max_upload_mb")}
                </label>
                <input
                  id="policy-upload"
                  type="number"
                  min={1}
                  max={25}
                  value={policy.max_upload_mb}
                  onChange={(event) => update({ max_upload_mb: Number(event.target.value) })}
                  className={fieldClass}
                />
                <p className="text-[11px] text-muted mt-1">{t("esign.field.max_upload_mb_hint")}</p>
              </div>
            </div>
          </div>
        </Card>

        <button
          type="submit"
          disabled={saving}
          className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white text-xs font-semibold px-4 py-2 rounded-lg inline-flex items-center gap-1.5"
        >
          <Save className="w-3.5 h-3.5" />
          {saving ? t("base.message.saving") : t("base.action.save")}
        </button>
      </form>
    </div>
  );
}

function Toggle({
  id,
  label,
  hint,
  checked,
  disabled,
  onChange,
}: {
  id: string;
  label: string;
  hint: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label htmlFor={id} className={`flex items-start gap-3 ${disabled ? "opacity-50" : "cursor-pointer"}`}>
      <input
        id={id}
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
        className="mt-0.5 rounded border-input text-indigo-600 focus:ring-indigo-500"
      />
      <span className="min-w-0">
        <span className="block text-sm font-medium text-foreground">{label}</span>
        <span className="block text-[11px] text-muted mt-0.5">{hint}</span>
      </span>
    </label>
  );
}
