"use client";

/**
 * Every organisation on the deployment.
 *
 * The console's front page was this list until CP-4 gave it a health summary;
 * the list moved here rather than being squeezed onto it, because they answer
 * different questions — "is the platform well" and "who is on it" — and a
 * screen that tries to do both does neither at a glance.
 */

import React, { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Building2, Plus, Search } from "lucide-react";

import Console, { useConsole } from "@/components/cp/Console";
import { formatMoment } from "@/components/cp/ui";
import { cp, type TenantSummary } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";
import { Modal } from "@/components/ui";

export default function ControlPlaneTenantsPage() {
  return (
    <Console>
      <Tenants />
    </Console>
  );
}

function Tenants() {
  const { t, locale } = useI18n();
  const { operator } = useConsole();
  const [creating, setCreating] = useState(false);
  const [search, setSearch] = useState("");
  const [tenants, setTenants] = useState<TenantSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState("");

  const load = useCallback(async (query: string) => {
    setLoading(true);
    try {
      const result = await cp.tenants(query);
      setTenants(result.tenants);
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    // Debounced, so typing a registration number is one query rather than
    // eleven.
    const timer = setTimeout(() => void load(search), 250);
    return () => clearTimeout(timer);
  }, [search, load]);

  return (
    <div className="space-y-6">
      <div className="flex items-start gap-3">
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-slate-900">{t("cp.section.tenants")}</h1>
          <p className="mt-1 text-sm text-slate-500">{t("cp.view.subtitle")}</p>
        </div>
        {(operator.role === "superadmin" || operator.role === "operator") && (
          <button
            type="button"
            onClick={() => setCreating(true)}
            className="inline-flex items-center gap-2 rounded-lg bg-[var(--gerege-blue)] px-3 py-2 text-sm font-medium text-white hover:brightness-105"
          >
            <Plus className="w-4 h-4" />
            {t("cp.action.new_tenant")}
          </button>
        )}
      </div>

      <p className="text-sm rounded-xl bg-amber-50 border border-amber-200 text-amber-900 px-4 py-3">
        {t("cp.message.read_only")}
      </p>

      <div className="relative">
        <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
        <input
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={t("cp.field.search")}
          className="w-full rounded-xl border border-slate-300 bg-white pl-9 pr-3 py-2.5 focus:outline-none focus:ring-2 focus:ring-slate-900/10"
        />
      </div>

      {failure && (
        <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">
          {t("cp.message.load_failed")}
        </p>
      )}

      <div className="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 text-slate-600">
              <tr>
                <th className="text-left font-medium px-4 py-3">{t("cp.field.organisation")}</th>
                <th className="text-left font-medium px-4 py-3">{t("cp.field.registration")}</th>
                <th className="text-right font-medium px-4 py-3">{t("cp.field.users")}</th>
                <th className="text-right font-medium px-4 py-3">{t("cp.field.apps")}</th>
                <th className="text-left font-medium px-4 py-3">{t("cp.field.last_activity")}</th>
                <th className="text-left font-medium px-4 py-3">{t("cp.field.state")}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {tenants.map((tenant) => (
                <tr key={tenant.id} className="hover:bg-slate-50">
                  <td className="px-4 py-3">
                    <Link
                      href={`/cp/tenants/${tenant.id}`}
                      className="flex items-center gap-2 font-medium text-slate-900 hover:underline"
                    >
                      <Building2 className="w-4 h-4 text-slate-400" />
                      {tenant.name}
                    </Link>
                    <span className="text-xs text-slate-400">{tenant.slug}</span>
                  </td>
                  <td className="px-4 py-3 text-slate-600">{tenant.registration_number || "—"}</td>
                  <td className="px-4 py-3 text-right tabular-nums">{tenant.user_count}</td>
                  <td className="px-4 py-3 text-right tabular-nums">{tenant.app_count}</td>
                  <td className="px-4 py-3 text-slate-600">
                    {formatMoment(tenant.last_activity_at, locale) || t("cp.message.never")}
                  </td>
                  <td className="px-4 py-3">
                    {tenant.deletion_scheduled_at ? (
                      <span className="text-xs font-medium rounded-full px-2 py-0.5 bg-red-100 text-red-800">
                        {t("cp.state.deleting")}
                      </span>
                    ) : tenant.suspended_at ? (
                      <span className="text-xs font-medium rounded-full px-2 py-0.5 bg-amber-100 text-amber-900">
                        {t("cp.state.suspended")}
                      </span>
                    ) : (
                      <span className="text-xs font-medium rounded-full px-2 py-0.5 bg-emerald-100 text-emerald-800">
                        {t("cp.state.active")}
                      </span>
                    )}
                  </td>
                </tr>
              ))}
              {!loading && tenants.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-10 text-center text-slate-500">
                    {t("cp.message.no_tenants")}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
      {creating && (
        <NewTenantDialog
          onClose={() => setCreating(false)}
          onCreated={() => {
            setCreating(false);
            void load(search);
          }}
        />
      )}
    </div>
  );
}

/**
 * Opening an organisation.
 *
 * The apps are a comma-separated list of catalogue slugs rather than a picker,
 * because the catalogue a deployment carries is its own and a hard-coded set of
 * checkboxes here would be wrong on the first deployment that adds a module.
 * The first administrator gets an invitation, never a password: see
 * lifecycle.go for why an operator must not be able to choose one.
 */
function NewTenantDialog({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const { t } = useI18n();
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [registration, setRegistration] = useState("");
  const [apps, setApps] = useState("");
  const [adminEmail, setAdminEmail] = useState("");
  const [reason, setReason] = useState("");
  const [failure, setFailure] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setFailure("");
    try {
      const created = await cp.createTenant({
        name,
        slug,
        registration_number: registration,
        apps: apps.split(",").map((app) => app.trim()).filter(Boolean),
        admin_email: adminEmail,
        reason,
      });
      // An organisation created with an app that would not install, or an
      // invitation that could not be sent, is still an organisation — and the
      // operator has to be told which parts did not land rather than finding
      // out when the customer calls.
      if (created.failed.length || !created.invited) {
        setNotice(
          [
            created.failed.length ? `${t("cp.field.apps")}: ${created.failed.join(", ")}` : "",
            created.invited ? "" : created.invite_error || "",
          ]
            .filter(Boolean)
            .join(" · "),
        );
        return;
      }
      onCreated();
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal label={t("cp.action.new_tenant")}>
      <form onSubmit={submit} className="p-5 space-y-4">
        <h2 className="text-lg font-semibold text-slate-900">{t("cp.action.new_tenant")}</h2>

        {failure && (
          <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
        )}
        {notice && (
          <div className="space-y-2">
            <p className="text-sm rounded-lg bg-amber-50 text-amber-900 border border-amber-200 px-3 py-2">
              {notice}
            </p>
            <button
              type="button"
              onClick={onCreated}
              className="rounded-lg bg-[var(--gerege-blue)] px-4 py-2 text-sm font-medium text-white hover:brightness-105"
            >
              {t("cp.action.back")}
            </button>
          </div>
        )}

        {!notice && (
          <>
            <TextField label={t("cp.field.name")} value={name} onChange={setName} required />
            <TextField label={t("cp.field.slug")} value={slug} onChange={(value) => setSlug(value.toLowerCase())} required />
            <TextField label={t("cp.field.registration")} value={registration} onChange={setRegistration} />
            <TextField label={t("cp.field.install_apps")} value={apps} onChange={setApps} />
            <TextField label={t("cp.field.admin_email")} value={adminEmail} onChange={setAdminEmail} required type="email" />
            <TextField label={t("cp.field.reason")} value={reason} onChange={setReason} required />

            <div className="flex justify-end gap-2">
              <button type="button" onClick={onClose} className="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100">
                {t("cp.action.cancel")}
              </button>
              <button
                type="submit"
                disabled={busy}
                className="rounded-lg bg-[var(--gerege-blue)] px-4 py-2 text-sm font-medium text-white hover:brightness-105 disabled:opacity-60"
              >
                {t("cp.action.create")}
              </button>
            </div>
          </>
        )}
      </form>
    </Modal>
  );
}

function TextField({
  label,
  value,
  onChange,
  required,
  type = "text",
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  required?: boolean;
  type?: string;
}) {
  return (
    <label className="block text-sm">
      <span className="text-slate-600">{label}</span>
      <input
        type={type}
        required={required}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 focus:outline-none focus:ring-2 focus:ring-slate-900/10"
      />
    </label>
  );
}
