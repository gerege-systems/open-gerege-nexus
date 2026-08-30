"use client";

/**
 * One organisation, and everything an operator may do to it.
 *
 * The read half is metadata: apps, people, the two audit trails, who has been
 * inside. The write half is the lifecycle — suspend, resume, ask for deletion,
 * cancel one, set limits, step inside — and every button on it goes through
 * useAction, which asks for a reason and, when the window has closed, for the
 * authenticator code. Nothing here writes without both.
 */

import React, { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import {
  ArrowLeft,
  BarChart3,
  Building2,
  Download,
  Eye,
  Gauge,
  Pause,
  Play,
  Trash2,
  Undo2,
  UserPlus,
  Wrench,
} from "lucide-react";

import { useConsole } from "@/components/cp/Console";
import { useAction } from "@/components/cp/Action";
import { Badge, Card, formatMoment, Table } from "@/components/cp/ui";
import { cp, type Quota, type TenantDetail, type VerifiedPerson } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";
import { Modal } from "@/components/ui";

export default function Detail() {
  const { t, locale } = useI18n();
  const { operator } = useConsole();
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const action = useAction();

  const [tenant, setTenant] = useState<TenantDetail | null>(null);
  const [failure, setFailure] = useState("");
  const [quotaOpen, setQuotaOpen] = useState(false);
  const [addingPerson, setAddingPerson] = useState(false);

  const load = useCallback(async () => {
    try {
      setTenant(await cp.tenant(id));
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    }
  }, [id]);

  useEffect(() => {
    if (id) void load();
  }, [id, load]);

  if (failure) {
    return (
      <div className="space-y-4">
        <BackLink label={t("cp.action.back")} />
        <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">
          {t("cp.message.load_failed")}
        </p>
      </div>
    );
  }
  if (!tenant) return <div className="text-slate-500">…</div>;

  const suspended = !!tenant.suspended_at;
  const deleting = !!tenant.deletion_scheduled_at;
  const may = (capability: string) => allowed(operator.role, capability);

  return (
    <div className="space-y-6">
      <BackLink label={t("cp.action.back")} />

      <div className="flex flex-wrap items-start gap-3">
        <Building2 className="w-6 h-6 text-slate-400 mt-1" />
        <div className="flex-1 min-w-0">
          <h1 className="text-2xl font-semibold text-slate-900 flex items-center gap-2">
            {tenant.name}
            <StateBadge tenant={tenant} />
          </h1>
          <p className="text-sm text-slate-500">
            {tenant.slug}
            {tenant.legal_name ? ` · ${tenant.legal_name}` : ""}
          </p>
          {suspended && tenant.suspension_reason && (
            <p className="mt-1 text-sm text-amber-700">{tenant.suspension_reason}</p>
          )}
          {deleting && (
            <p className="mt-1 text-sm text-red-700">
              {formatMoment(tenant.deletion_scheduled_at, locale)}
            </p>
          )}
        </div>
      </div>

      <section className="bg-white rounded-xl border border-slate-200 shadow-sm p-4">
        <h2 className="text-sm font-medium text-slate-500 mb-3">{t("cp.section.actions")}</h2>
        <div className="flex flex-wrap gap-2">
          {may("tenant.suspend") && !suspended && (
            <ActionButton
              icon={<Pause className="w-4 h-4" />}
              label={t("cp.action.suspend")}
              onClick={() =>
                action.run({
                  title: t("cp.action.suspend"),
                  detail: tenant.name,
                  danger: true,
                  perform: (reason) => cp.suspend(tenant.id, reason),
                  onDone: load,
                })
              }
            />
          )}
          {may("tenant.suspend") && suspended && !deleting && (
            <ActionButton
              icon={<Play className="w-4 h-4" />}
              label={t("cp.action.resume")}
              onClick={() =>
                action.run({
                  title: t("cp.action.resume"),
                  detail: tenant.name,
                  perform: (reason) => cp.resume(tenant.id, reason),
                  onDone: load,
                })
              }
            />
          )}
          {may("quota.write") && (
            <ActionButton
              icon={<Gauge className="w-4 h-4" />}
              label={t("cp.action.quota")}
              onClick={() => setQuotaOpen(true)}
            />
          )}
          {may("settings.write") && (
            <ActionButton
              icon={<Wrench className="w-4 h-4" />}
              label={tenant.maintenance_at ? t("cp.action.maintenance_off") : t("cp.action.maintenance_on")}
              onClick={() =>
                action.run({
                  title: tenant.maintenance_at ? t("cp.action.maintenance_off") : t("cp.action.maintenance_on"),
                  detail: tenant.name,
                  perform: (reason) =>
                    cp.maintenance(tenant.id, !tenant.maintenance_at, reason, reason),
                  onDone: load,
                })
              }
            />
          )}
          {may("tenant.delete") && !deleting && (
            <ActionButton
              icon={<Trash2 className="w-4 h-4" />}
              label={t("cp.action.delete")}
              danger
              onClick={() =>
                action.run({
                  title: t("cp.action.delete"),
                  detail: t("cp.message.deletion_requested"),
                  danger: true,
                  perform: (reason) => cp.requestDeletion(tenant.id, reason),
                  onDone: load,
                })
              }
            />
          )}
          {may("tenant.suspend") && deleting && (
            <ActionButton
              icon={<Undo2 className="w-4 h-4" />}
              label={t("cp.action.cancel_deletion")}
              onClick={() =>
                action.run({
                  title: t("cp.action.cancel_deletion"),
                  detail: tenant.name,
                  perform: (reason) => cp.cancelDeletion(tenant.id, reason),
                  onDone: load,
                })
              }
            />
          )}
          <Link
            href={`/cp/tenants/${tenant.id}/usage`}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 hover:bg-slate-50"
          >
            <BarChart3 className="w-4 h-4" />
            {t("cp.action.usage")}
          </Link>
          {may("tenant.delete") && (
            <a
              href={cp.exportURL(tenant.id)}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-300 px-3 py-2 text-sm hover:bg-slate-50"
            >
              <Download className="w-4 h-4" />
              {t("cp.action.export")}
            </a>
          )}
        </div>
      </section>

      <dl className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Fact label={t("cp.field.registration")} value={tenant.registration_number || "—"} />
        <Fact label={t("cp.field.tax_number")} value={tenant.tax_number || "—"} />
        <Fact label={t("cp.field.created")} value={formatMoment(tenant.created_at, locale)} />
        <Fact
          label={t("cp.field.users")}
          value={
            tenant.quota.max_users === null
              ? String(tenant.quota.users)
              : `${tenant.quota.users} / ${tenant.quota.max_users}`
          }
        />
      </dl>

      <Card title={t("cp.section.apps")}>
        <Table
          head={[t("cp.field.apps"), t("cp.field.version"), t("cp.field.status"), t("cp.field.installed")]}
          rows={tenant.apps.map((app) => [
            app.name,
            app.version,
            app.enabled ? app.status : `${app.status} · off`,
            formatMoment(app.installed_at, locale),
          ])}
          empty={t("cp.message.no_activity")}
        />
      </Card>

      <Card
        title={t("cp.section.members")}
        action={
          !suspended && (
            <button
              type="button"
              onClick={() => setAddingPerson(true)}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-300 px-2.5 py-1.5 text-xs text-slate-700 hover:bg-slate-50"
            >
              <UserPlus className="w-3.5 h-3.5" />
              {t("cp.action.add_person")}
            </button>
          )
        }
      >
        <Table
          head={[t("cp.field.email"), t("cp.field.person"), t("cp.field.roles"), ""]}
          rows={tenant.members.map((member) => [
            member.email,
            member.name,
            member.roles.length ? member.roles.join(", ") : "—",
            // Looking at the platform as somebody is a decision about *which*
            // somebody. It used to be a button on the action bar above that
            // took tenant.members[0] — whoever the list happened to start
            // with — so the operator got an arbitrary person and the reason
            // they typed named a different one.
            may("user.impersonate") && !suspended ? (
              <button
                key="i"
                type="button"
                onClick={() =>
                  action.run({
                    title: t("cp.action.impersonate"),
                    detail: member.email,
                    perform: async (reason) => {
                      const { url } = await cp.impersonate(tenant.id, member.user_id, reason);
                      // A new tab, so the console stays where it is: the
                      // operator is about to be two people at once and should
                      // not lose the window that can end it.
                      window.open(url, "_blank", "noopener");
                    },
                  })
                }
                className="inline-flex items-center gap-1.5 text-xs rounded-lg border border-slate-300 px-2 py-1 hover:bg-slate-50"
              >
                <Eye className="w-3.5 h-3.5" />
                {t("cp.action.impersonate")}
              </button>
            ) : (
              <span key="i" />
            ),
          ])}
          empty={t("cp.message.no_activity")}
        />
      </Card>

      <Card title={t("cp.section.impersonations")}>
        <Table
          head={[t("cp.field.when"), t("cp.field.operator"), t("cp.field.person"), t("cp.field.reason")]}
          rows={tenant.impersonations.map((visit) => [
            formatMoment(visit.created_at, locale),
            visit.operator_email,
            visit.user_email,
            visit.reason,
          ])}
          empty={t("cp.message.no_activity")}
        />
      </Card>

      <Card title={t("cp.section.activity")}>
        <Table
          head={[t("cp.field.when"), t("cp.field.action"), t("cp.field.resource")]}
          rows={tenant.activity.map((entry) => [
            formatMoment(entry.created_at, locale),
            entry.action,
            entry.resource,
          ])}
          empty={t("cp.message.no_activity")}
        />
      </Card>

      <Card title={t("cp.section.operator_actions")}>
        <Table
          head={[t("cp.field.when"), t("cp.field.operator"), t("cp.field.action"), t("cp.field.reason")]}
          rows={tenant.operator_actions.map((entry) => [
            formatMoment(entry.created_at, locale),
            entry.operator_email,
            entry.action,
            entry.reason || "—",
          ])}
          empty={t("cp.message.no_activity")}
        />
      </Card>

      {addingPerson && (
        <AddPersonDialog
          tenantID={tenant.id}
          onClose={() => setAddingPerson(false)}
          onAdded={() => {
            setAddingPerson(false);
            void load();
          }}
        />
      )}
      {quotaOpen && (
        <QuotaDialog
          tenantID={tenant.id}
          quota={tenant.quota}
          onClose={() => setQuotaOpen(false)}
          onSaved={() => {
            setQuotaOpen(false);
            void load();
          }}
        />
      )}
      {action.dialog}
    </div>
  );
}

/**
 * What each role may do, mirroring the capability table the server enforces.
 *
 * A copy, and it has to be: the server is the authority and answers 403
 * whatever this says. The copy exists so an operator is not shown buttons that
 * will refuse them — and it is deliberately the same shape as the Go map, so
 * the two can be compared by eye when a capability is added.
 */
const CAPABILITIES: Record<string, string[]> = {
  superadmin: ["tenant.suspend", "tenant.delete", "quota.write", "support.act",
    "user.impersonate", "approval.decide", "settings.write"],
  operator: ["tenant.suspend", "quota.write", "support.act", "settings.write"],
  support: ["support.act", "user.impersonate"],
  auditor: [],
};

function allowed(role: string, capability: string): boolean {
  return (CAPABILITIES[role] ?? []).includes(capability);
}

function StateBadge({ tenant }: { tenant: { suspended_at: string | null; deletion_scheduled_at: string | null } }) {
  const { t } = useI18n();
  if (tenant.deletion_scheduled_at) {
    return <Badge tone="red">{t("cp.state.deleting")}</Badge>;
  }
  if (tenant.suspended_at) {
    return <Badge tone="amber">{t("cp.state.suspended")}</Badge>;
  }
  return <Badge tone="emerald">{t("cp.state.active")}</Badge>;
}

function ActionButton({
  icon,
  label,
  onClick,
  danger,
}: {
  icon: React.ReactNode;
  label: string;
  onClick: () => void;
  danger?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`inline-flex items-center gap-2 rounded-lg border px-3 py-2 text-sm transition ${
        danger
          ? "border-red-200 text-red-700 hover:bg-red-50"
          : "border-slate-300 text-slate-700 hover:bg-slate-50"
      }`}
    >
      {icon}
      {label}
    </button>
  );
}

function QuotaDialog({
  tenantID,
  quota,
  onClose,
  onSaved,
}: {
  tenantID: string;
  quota: Quota;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { t } = useI18n();
  const [users, setUsers] = useState(quota.max_users?.toString() ?? "");
  const [storage, setStorage] = useState(quota.max_storage_mb?.toString() ?? "");
  const [ai, setAI] = useState(quota.max_ai_calls_monthly?.toString() ?? "");
  const [enforcement, setEnforcement] = useState<"soft" | "hard">(quota.enforcement);
  const [reason, setReason] = useState("");
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);

  // An empty field is no limit at all, which is not the same as zero — the
  // server keeps the distinction and so does this form.
  const number = (raw: string) => (raw.trim() === "" ? null : Number(raw));

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setFailure("");
    try {
      await cp.setQuota(tenantID, {
        max_users: number(users),
        max_storage_mb: number(storage),
        max_ai_calls_monthly: number(ai),
        enforcement,
        reason,
      });
      onSaved();
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal label={t("cp.section.limits")}>
      <form onSubmit={submit} className="p-5 space-y-4">
        <h2 className="text-lg font-semibold text-slate-900">{t("cp.section.limits")}</h2>

        {failure && (
          <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
        )}

        <Field label={t("cp.field.max_users")} value={users} onChange={setUsers} />
        <Field label={t("cp.field.max_storage")} value={storage} onChange={setStorage} hint={t("cp.hint.not_enforced")} />
        <Field label={t("cp.field.max_ai")} value={ai} onChange={setAI} hint={t("cp.hint.not_enforced")} />

        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.enforcement")}</span>
          <select
            value={enforcement}
            onChange={(event) => setEnforcement(event.target.value as "soft" | "hard")}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
          >
            <option value="soft">{t("cp.state.soft")}</option>
            <option value="hard">{t("cp.state.hard")}</option>
          </select>
        </label>

        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.reason")}</span>
          <input
            required
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
          />
        </label>

        <div className="flex justify-end gap-2">
          <button type="button" onClick={onClose} className="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100">
            {t("cp.action.cancel")}
          </button>
          <button
            type="submit"
            disabled={busy}
            className="rounded-lg bg-[var(--gerege-blue)] px-4 py-2 text-sm font-medium text-[var(--gerege-on-blue)] hover:brightness-105 disabled:opacity-60"
          >
            {t("cp.action.confirm")}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function Field({
  label,
  value,
  onChange,
  hint,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  hint?: string;
}) {
  return (
    <label className="block text-sm">
      <span className="text-slate-600">{label}</span>
      <input
        inputMode="numeric"
        value={value}
        onChange={(event) => onChange(event.target.value.replace(/\D/g, ""))}
        className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
      />
      {hint && <span className="mt-1 block text-xs text-amber-700">{hint}</span>}
    </label>
  );
}

function BackLink({ label }: { label: string }) {
  return (
    <Link href="/cp/tenants" className="inline-flex items-center gap-1.5 text-sm text-slate-600 hover:text-slate-900">
      <ArrowLeft className="w-4 h-4" />
      {label}
    </Link>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-white rounded-xl border border-slate-200 shadow-sm px-4 py-3">
      <dt className="text-xs uppercase tracking-wide text-slate-400">{label}</dt>
      <dd className="mt-1 text-sm text-slate-900">{value}</dd>
    </div>
  );
}

/**
 * Adding somebody to an organisation.
 *
 * The same list the first administrator is chosen from — people this
 * deployment has watched sign in with eID — and the same reason: an address
 * typed into a dialog is an address, and a choice is a person.
 *
 * They arrive with the smallest role the platform has. Anything above it is
 * granted by the organisation's own administrator, in their own access screen,
 * where the people who live with the decision can see it.
 */
function AddPersonDialog({
  tenantID,
  onClose,
  onAdded,
}: {
  tenantID: string;
  onClose: () => void;
  onAdded: () => void;
}) {
  const { t, locale } = useI18n();
  const [people, setPeople] = useState<VerifiedPerson[]>([]);
  const [search, setSearch] = useState("");
  const [chosen, setChosen] = useState<VerifiedPerson | null>(null);
  const [reason, setReason] = useState("");
  const [code, setCode] = useState("");
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);

  const loadPeople = useCallback(async (query: string) => {
    try {
      setPeople((await cp.verifiedPeople(query)).people);
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    }
  }, []);

  useEffect(() => {
    void loadPeople("");
  }, [loadPeople]);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    if (!chosen) return;
    setBusy(true);
    setFailure("");
    try {
      await cp.addMember(tenantID, chosen.user_id, reason);
      onAdded();
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal label={t("cp.action.add_person")}>
      <form onSubmit={submit} className="p-5 space-y-4">
        <h2 className="text-lg font-semibold text-slate-900">{t("cp.action.add_person")}</h2>
        <p className="text-xs text-slate-500">{t("cp.hint.member_is_chosen")}</p>

        {failure && (
          <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
        )}

        {chosen ? (
          <div className="flex items-center gap-3 rounded-lg border border-[var(--gerege-blue)] bg-[var(--gerege-blue-soft)] px-3 py-2">
            <span className="min-w-0 flex-1">
              <strong className="block text-sm text-slate-900 truncate">{chosen.name}</strong>
              <span className="block text-xs text-slate-600 truncate">{chosen.email}</span>
            </span>
            <button
              type="button"
              onClick={() => setChosen(null)}
              className="text-xs rounded-lg border border-slate-300 bg-white px-2 py-1 hover:bg-slate-50"
            >
              {t("cp.action.change")}
            </button>
          </div>
        ) : (
          <div className="space-y-2">
            <input
              value={search}
              onChange={(event) => {
                setSearch(event.target.value);
                void loadPeople(event.target.value);
              }}
              placeholder={t("cp.field.search_people")}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
            />
            <div className="max-h-48 overflow-y-auto divide-y divide-slate-100 rounded-lg border border-slate-200">
              {people.map((person) => (
                <button
                  key={person.user_id}
                  type="button"
                  onClick={() => setChosen(person)}
                  className="w-full text-left px-3 py-2 hover:bg-slate-50"
                >
                  <strong className="block text-sm text-slate-900 truncate">{person.name}</strong>
                  <span className="block text-xs text-slate-500 truncate">
                    {person.email}
                    {person.reg_number ? ` · ${person.reg_number}` : ""}
                    {" · "}
                    {formatMoment(person.last_seen_at, locale)}
                  </span>
                </button>
              ))}
              {people.length === 0 && (
                <p className="px-3 py-3 text-sm text-slate-500">{t("cp.message.no_verified_people")}</p>
              )}
            </div>
          </div>
        )}

        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.reason")}</span>
          <input
            required
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
          />
        </label>

        {/* The API asks for the second factor before it hands anybody the keys
            to an organisation's data; confirming it here keeps what has been
            typed. */}
        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.code")}</span>
          <input
            inputMode="numeric"
            pattern="[0-9]*"
            maxLength={6}
            value={code}
            onChange={(event) => setCode(event.target.value.replace(/\D/g, ""))}
            onBlur={async () => {
              if (code.length === 6) {
                try {
                  await cp.stepUp(code);
                  setCode("");
                  setFailure("");
                } catch (error) {
                  setFailure(error instanceof Error ? error.message : String(error));
                }
              }
            }}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 font-mono tracking-[0.4em]"
          />
          <small className="text-xs text-slate-500">{t("cp.hint.step_up")}</small>
        </label>

        <div className="flex justify-end gap-2">
          <button type="button" onClick={onClose} className="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100">
            {t("cp.action.cancel")}
          </button>
          <button
            type="submit"
            disabled={busy || !chosen}
            className="rounded-lg bg-[var(--gerege-blue)] px-4 py-2 text-sm font-medium text-[var(--gerege-on-blue)] hover:brightness-105 disabled:opacity-60"
          >
            {t("cp.action.add_person")}
          </button>
        </div>
      </form>
    </Modal>
  );
}
