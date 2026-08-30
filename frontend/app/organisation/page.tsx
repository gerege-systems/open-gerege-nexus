"use client";

import React, { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Banner, fieldClass } from "@/components/ui";
import { useAccess } from "@/lib/permissions";
import { Building2, Megaphone, Plus, RefreshCw, Trash2 } from "lucide-react";

/**
 * The organisation as it is, rather than as it is labelled.
 *
 * The name in the sidebar has always existed; none of the rest has, and every
 * app that has to print who this organisation is — a signed document, an
 * invoice, a request to a ministry — has been going without it.
 */
type Organisation = Awaited<ReturnType<typeof api.getOrganisation>>;

// Grouped the way somebody fills them in, not the way they are stored.
const GROUPS: Array<{ title: string; fields: Array<{ key: keyof Organisation; label: string; placeholder?: string }> }> = [
  {
    title: "core.group.identity",
    fields: [
      { key: "name", label: "core.field.name" },
      { key: "legal_name", label: "core.field.legal_name" },
      { key: "registration_number", label: "core.field.registration_number", placeholder: "1234567" },
      { key: "tax_number", label: "core.field.tax_number" },
    ],
  },
  {
    title: "core.group.address",
    fields: [
      { key: "province", label: "core.field.province" },
      { key: "district", label: "core.field.district" },
      { key: "khoroo", label: "core.field.khoroo" },
      { key: "address_line", label: "core.field.address_line" },
      { key: "postal_code", label: "core.field.postal_code" },
    ],
  },
  {
    title: "core.group.contact",
    fields: [
      { key: "phone", label: "base.field.phone" },
      { key: "email", label: "base.field.email" },
      { key: "website", label: "core.field.website" },
      { key: "logo_url", label: "core.field.logo_url" },
    ],
  },
  {
    title: "core.group.defaults",
    fields: [
      { key: "timezone", label: "core.field.timezone", placeholder: "Asia/Ulaanbaatar" },
      { key: "locale", label: "base.field.language", placeholder: "mn" },
      { key: "currency", label: "core.field.currency", placeholder: "MNT" },
    ],
  },
];

export default function OrganisationPage() {
  const { t } = useI18n();
  const { isAdmin: canManage } = useAccess();
  const [organisation, setOrganisation] = useState<Organisation | null>(null);
  // Only organisations this person belongs to can be offered: the server will
  // refuse any other, and a selector full of choices that all fail is worse
  // than a short one.
  const [candidates, setCandidates] = useState<Array<{ id: string; name: string }>>([]);
  const [draft, setDraft] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  const load = useCallback(async () => {
    try {
      const [current, mine] = await Promise.all([api.getOrganisation(), api.getTenants().catch(() => null)]);
      setOrganisation(current);
      setCandidates((mine?.tenants || []).filter((x: any) => x.id !== current.tenant_id)
        .map((x: any) => ({ id: x.id, name: x.name })));
    } catch (err: any) {
      setMessage({ type: "error", text: err.message || t("base.message.error") });
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  // Only what was touched is sent. The server merges field by field, so an
  // edit to a phone number cannot blank a registration number.
  const save = async () => {
    setSaving(true);
    setMessage(null);
    try {
      await api.updateOrganisation(draft);
      setDraft({});
      await load();
      setMessage({ type: "success", text: t("core.message.saved") });
    } catch (err: any) {
      setMessage({ type: "error", text: err.message || t("base.message.error") });
    } finally {
      setSaving(false);
    }
  };

  // What the register holds, rather than what somebody typed once. Deliberately
  // a button: it overwrites fields an administrator can see on the screen in
  // front of them, and doing that on a timer would mean Tuesday's edit quietly
  // disappearing on Wednesday.
  const syncFromCore = async () => {
    setSyncing(true);
    setMessage(null);
    try {
      await api.syncOrganisationFromCore();
      setDraft({});
      await load();
      setMessage({ type: "success", text: t("core.message.core_synced") });
    } catch (err: any) {
      setMessage({ type: "error", text: err.message || t("base.message.error") });
    } finally {
      setSyncing(false);
    }
  };

  if (!organisation) {
    return <div className="py-12 text-center text-slate-500 text-sm">{t("base.message.loading")}</div>;
  }

  const value = (key: keyof Organisation) => draft[key] ?? (organisation[key] as string) ?? "";

  return (
    <div className="space-y-6">
      <div className="border-b border-slate-200 pb-4 flex items-center gap-3">
        <Building2 className="w-6 h-6 text-slate-600" />
        <div>
          <h1 className="text-2xl font-bold text-slate-900">{t("core.view.organisation_title")}</h1>
          <p className="text-sm text-slate-500">{t("core.view.organisation_subtitle")}</p>
        </div>
        {canManage && (
          <button
            type="button"
            onClick={() => void syncFromCore()}
            disabled={syncing || !value("registration_number")}
            title={t("core.hint.core_sync")}
            className="ml-auto inline-flex items-center gap-2 rounded-lg border border-slate-300 px-3 py-2 text-sm hover:bg-slate-50 disabled:opacity-50"
          >
            <RefreshCw className={`w-4 h-4 ${syncing ? "animate-spin" : ""}`} />
            {t("core.action.core_sync")}
          </button>
        )}
      </div>

      {message && <Banner tone={message.type} message={message.text} onDismiss={() => setMessage(null)} />}
      {!canManage && <Banner tone="info" message={t("base.message.admin_only_edit")} />}

      <div className="grid gap-6 md:grid-cols-2">
        {GROUPS.map((group) => (
          <section key={group.title} className="bg-white border border-slate-200 rounded-xl p-5 space-y-3">
            <h2 className="font-semibold text-slate-800">{t(group.title as any)}</h2>
            {group.fields.map((field) => (
              <label key={field.key} className="block">
                <span className="block text-xs font-medium text-slate-500 mb-1">{t(field.label as any)}</span>
                <input
                  value={value(field.key)}
                  placeholder={field.placeholder}
                  disabled={!canManage}
                  onChange={(e) => setDraft((d) => ({ ...d, [field.key]: e.target.value }))}
                  className="w-full px-3 py-2 text-sm border border-slate-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 disabled:bg-slate-50 disabled:text-slate-500"
                />
              </label>
            ))}
          </section>
        ))}
      </div>

      {/* Kept apart from the grouped fields, and last: it is the one field on
          this screen that names somebody else, and it is a statement about the
          world rather than a setting — recording it grants nothing and changes
          nothing about what this organisation can see. */}
      <section className="bg-white border border-slate-200 rounded-xl p-5 space-y-2">
        <h2 className="font-semibold text-slate-800">{t("core.group.affiliation")}</h2>
        <p className="text-xs text-slate-500">{t("core.message.parent_hint")}</p>
        <label className="block max-w-md">
          <span className="block text-xs font-medium text-slate-500 mb-1">{t("core.field.parent_organisation")}</span>
          <select
            value={draft.parent_tenant_id ?? organisation.parent_tenant_id ?? ""}
            disabled={!canManage}
            onChange={(e) => setDraft((d) => ({ ...d, parent_tenant_id: e.target.value }))}
            className="w-full px-3 py-2 text-sm border border-slate-300 rounded-lg bg-white disabled:bg-slate-50"
          >
            <option value="">{t("core.state.independent")}</option>
            {/* The organisation currently recorded stays on offer even if the
                person has since left it, or the selector would silently read
                as "independent" and the next save would make that true. */}
            {organisation.parent_tenant_id &&
              !candidates.some((c) => c.id === organisation.parent_tenant_id) && (
                <option value={organisation.parent_tenant_id}>{organisation.parent_name}</option>
              )}
            {candidates.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </label>
      </section>

      {/* Хаалга. Энэ хэсэг «Харьяалал»-ын дараа байгаа нь дараалал зөв: дээрх нь
          энэ байгууллага хэн бэ гэдгийг хэлдэг, энэ нь хэн орж болохыг.

          Хоёр сонголт, гурав биш. «Урилгаар л авна» гэдэг нь гурав дахь төлөв
          мэт харагддаг ч үнэндээ хүсэлт хүлээж авдаг байгууллагын өдөр тутмын
          хариулт — татгалзах — тул түүнд тусдаа тохиргоо хэрэггүй. */}
      <section className="bg-white border border-slate-200 rounded-xl p-5 space-y-2">
        <h2 className="font-semibold text-slate-800">{t("core.group.joining")}</h2>
        <p className="text-xs text-slate-500">{t("core.message.join_policy_hint")}</p>
        <label className="block max-w-md">
          <span className="block text-xs font-medium text-slate-500 mb-1">{t("core.field.join_policy")}</span>
          <select
            value={draft.join_policy ?? organisation.join_policy ?? "on_request"}
            disabled={!canManage}
            onChange={(e) => setDraft((d) => ({ ...d, join_policy: e.target.value }))}
            className="w-full px-3 py-2 text-sm border border-slate-300 rounded-lg bg-white disabled:bg-slate-50"
          >
            <option value="on_request">{t("core.state.join_on_request")}</option>
            <option value="open">{t("core.state.join_open")}</option>
          </select>
        </label>
        {/* Нээлттэй болгосон хүнд юу өгч байгааг нь хэлнэ. Энэ мөр нь
            чимэглэл биш: гишүүнчлэл дангаараа хоосон биш — платформын trigger
            шинэ гишүүн бүрд `user` роль өгдөг, тэр нь уншилтын зөвшөөрөл
            агуулна. «Хаалга нээх» ба «мэдээллээ харуулах» хоёрыг андуурах нь
            энэ дэлгэц дээр гарч болох хамгийн үнэтэй алдаа. */}
        {(draft.join_policy ?? organisation.join_policy) === "open" && (
          <p className="text-xs text-[var(--gerege-blue-text)]">{t("core.message.join_open_note")}</p>
        )}
      </section>

      {canManage && (
        <div className="flex items-center gap-3">
          <button
            onClick={save}
            disabled={saving || Object.keys(draft).length === 0}
            className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white font-medium text-sm py-2 px-5 rounded-lg"
          >
            {saving ? t("base.message.saving") : t("base.action.save")}
          </button>
          {Object.keys(draft).length > 0 && (
            <button onClick={() => setDraft({})} className="text-sm text-slate-500 hover:text-slate-700">
              {t("base.action.cancel")}
            </button>
          )}
        </div>
      )}
    </div>
  );
}
