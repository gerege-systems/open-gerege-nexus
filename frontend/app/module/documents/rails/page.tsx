"use client";

import React, { useEffect, useState } from "react";
import { Save, ShieldCheck } from "lucide-react";
import { esign, type Policy } from "@/lib/esign";
import { useI18n } from "@/lib/i18n";
import { Loading, PageHeader } from "@/components/ui";
import { Alert, Button, Input, Switch } from "@gerege-systems/ui";
import { SelectField } from "@/components/documents/shared";
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
  if (!policy) return <Alert variant="danger" live>{error ?? t("base.message.error")}</Alert>;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ShieldCheck className="w-7 h-7 text-accent" />}
        title={t("esign.view.policies_title")}
        subtitle={t("esign.view.policies_subtitle")}
      />

      {error && <Alert variant="danger" live dismissible onDismiss={() => setError(null)}>{error}</Alert>}
      {notice && <Alert variant="success" live dismissible onDismiss={() => setNotice(null)}>{notice}</Alert>}

      <form onSubmit={save} className="space-y-6 max-w-2xl">
        <Card title={t("esign.view.policy_rails")}>
          <div className="p-4 space-y-4">
            <SelectField
              id="policy-provider"
              label={t("esign.field.default_provider")}
              helperText={t("esign.field.default_provider_hint")}
              value={policy.default_provider}
              onValueChange={(value) => update({ default_provider: value as Policy["default_provider"] })}
              disabled={policy.require_eid}
              options={[
                { value: "EID", label: "eID Mongolia (PIN2)" },
                { value: "HSM", label: "Gerege eSign HSM" },
              ]}
            />

            <Switch
              id="policy-require-eid"
              label={t("esign.field.require_eid")}
              description={t("esign.field.require_eid_hint")}
              checked={policy.require_eid}
              // Not disabled from here: whether eID is reachable is a
              // deployment fact the browser cannot see, so the server refuses
              // the save with EID_NOT_CONFIGURED and that message is shown.
              onCheckedChange={(require_eid) =>
                // Requiring eID while the default names the HSM would refuse
                // every signature it started, so the two move together.
                update(require_eid ? { require_eid, default_provider: "EID" } : { require_eid })
              }
              className="items-start"
            />

            <SelectField
              id="policy-level"
              label={t("esign.field.min_certificate_level")}
              helperText={t("esign.field.min_certificate_level_hint")}
              value={policy.min_certificate_level}
              onValueChange={(value) => update({ min_certificate_level: value as Policy["min_certificate_level"] })}
              options={[
                { value: "ADVANCED", label: "ADVANCED" },
                { value: "QUALIFIED", label: "QUALIFIED" },
                { value: "QSCD", label: "QSCD" },
              ]}
            />
          </div>
        </Card>

        <Card title={t("esign.view.policy_rules")}>
          <div className="p-4 space-y-4">
            <Switch
              id="policy-onbehalf"
              label={t("esign.field.allow_on_behalf_of")}
              description={t("esign.field.allow_on_behalf_of_hint")}
              checked={policy.allow_on_behalf_of}
              onCheckedChange={(allow_on_behalf_of) => update({ allow_on_behalf_of })}
              className="items-start"
            />
            <Switch
              id="policy-selfsign"
              label={t("esign.field.allow_self_sign")}
              description={t("esign.field.allow_self_sign_hint")}
              checked={policy.allow_self_sign}
              onCheckedChange={(allow_self_sign) => update({ allow_self_sign })}
              className="items-start"
            />

            <div className="grid sm:grid-cols-2 gap-4">
              <Input
                id="policy-retention"
                type="number"
                min={0}
                label={t("esign.field.retention_days")}
                helperText={t("esign.field.retention_days_hint")}
                value={policy.retention_days}
                onChange={(event) => update({ retention_days: Number(event.target.value) })}
              />
              <Input
                id="policy-upload"
                type="number"
                min={1}
                max={25}
                label={t("esign.field.max_upload_mb")}
                helperText={t("esign.field.max_upload_mb_hint")}
                value={policy.max_upload_mb}
                onChange={(event) => update({ max_upload_mb: Number(event.target.value) })}
              />
            </div>
          </div>
        </Card>

        <Button type="submit" loading={saving} leadingIcon={<Save />}>
          {saving ? t("base.message.saving") : t("base.action.save")}
        </Button>
      </form>
    </div>
  );
}
