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

import {
  Badge,
  Button,
  Card,
  CardHeader,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  Spinner,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@gerege-systems/ui";

import { useConsole } from "@/components/cp/Console";
import { useAction } from "@/components/cp/Action";
// The console's Modal is the library Dialog plus the one rule this product
// adds: a stray backdrop click does not throw away what has been typed.
import { Modal } from "@/components/ui";
import { cp, type Quota, type TenantDetail, type VerifiedPerson } from "@/lib/cp";
import { formatMoment } from "@/lib/datetime";
import { useI18n } from "@/lib/i18n";

export default function Detail() {
  const { t } = useI18n();
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
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">
          {t("cp.message.load_failed")}
        </p>
      </div>
    );
  }
  if (!tenant) {
    return (
      <p role="status" className="flex items-center gap-2 text-sm text-muted">
        <Spinner size="md" decorative />
        {t("base.message.loading")}
      </p>
    );
  }

  const suspended = !!tenant.suspended_at;
  const deleting = !!tenant.deletion_scheduled_at;
  const may = (capability: string) => allowed(operator.role, capability);

  return (
    <div className="space-y-6">
      <BackLink label={t("cp.action.back")} />

      <div className="flex flex-wrap items-start gap-3">
        <Building2 className="w-6 h-6 text-muted mt-1" />
        <div className="flex-1 min-w-0">
          <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
            {tenant.name}
            <StateBadge tenant={tenant} />
          </h1>
          <p className="text-sm text-muted">
            {tenant.slug}
            {tenant.legal_name ? ` · ${tenant.legal_name}` : ""}
          </p>
          {suspended && tenant.suspension_reason && (
            <p className="mt-1 text-sm text-warning">{tenant.suspension_reason}</p>
          )}
          {deleting && (
            <p className="mt-1 text-sm text-danger">
              {formatMoment(tenant.deletion_scheduled_at)}
            </p>
          )}
        </div>
      </div>

      <section className="bg-surface rounded-lg border border-line p-4">
        <h2 className="text-sm font-medium text-muted mb-3">{t("cp.section.actions")}</h2>
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
          <Button variant="outline" asChild>
            <Link href={`/cp/tenants/${tenant.id}/usage`}>
              <BarChart3 className="w-4 h-4" />
              {t("cp.action.usage")}
            </Link>
          </Button>
          {may("tenant.delete") && (
            <Button variant="outline" asChild>
              <a href={cp.exportURL(tenant.id)}>
                <Download className="w-4 h-4" />
                {t("cp.action.export")}
              </a>
            </Button>
          )}
        </div>
      </section>

      <dl className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Fact label={t("cp.field.registration")} value={tenant.registration_number || "—"} />
        <Fact label={t("cp.field.tax_number")} value={tenant.tax_number || "—"} />
        <Fact label={t("cp.field.created")} value={formatMoment(tenant.created_at)} />
        <Fact
          label={t("cp.field.users")}
          value={
            tenant.quota.max_users === null
              ? String(tenant.quota.users)
              : `${tenant.quota.users} / ${tenant.quota.max_users}`
          }
        />
      </dl>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.apps")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.apps")}</TableHead>
              <TableHead>{t("cp.field.version")}</TableHead>
              <TableHead>{t("cp.field.status")}</TableHead>
              <TableHead>{t("cp.field.installed")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {tenant.apps.map((app, index) => (
              <TableRow key={index}>
                <TableCell>{app.name}</TableCell>
                <TableCell>{app.version}</TableCell>
                <TableCell>{app.enabled ? app.status : `${app.status} · off`}</TableCell>
                <TableCell>{formatMoment(app.installed_at)}</TableCell>
              </TableRow>
            ))}
            {tenant.apps.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="py-8 text-center text-muted">
                  {t("cp.message.no_activity")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.members")}</h2>
          {!suspended && (
            <Button variant="outline" size="sm" onClick={() => setAddingPerson(true)} leadingIcon={<UserPlus />}>
              {t("cp.action.add_person")}
            </Button>
          )}
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.email")}</TableHead>
              <TableHead>{t("cp.field.person")}</TableHead>
              <TableHead>{t("cp.field.roles")}</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {tenant.members.map((member) => (
              <TableRow key={member.user_id}>
                <TableCell>{member.email}</TableCell>
                <TableCell>{member.name}</TableCell>
                <TableCell>{member.roles.length ? member.roles.join(", ") : "—"}</TableCell>
                <TableCell>
                  {/* Looking at the platform as somebody is a decision about *which*
                      somebody. It used to be a button on the action bar above that
                      took tenant.members[0] — whoever the list happened to start
                      with — so the operator got an arbitrary person and the reason
                      they typed named a different one. */}
                  {may("user.impersonate") && !suspended && (
                    <Button
                      variant="outline"
                      size="sm"
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
                      leadingIcon={<Eye />}
                    >
                      {t("cp.action.impersonate")}
                    </Button>
                  )}
                </TableCell>
              </TableRow>
            ))}
            {tenant.members.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="py-8 text-center text-muted">
                  {t("cp.message.no_activity")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.impersonations")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.when")}</TableHead>
              <TableHead>{t("cp.field.operator")}</TableHead>
              <TableHead>{t("cp.field.person")}</TableHead>
              <TableHead>{t("cp.field.reason")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {tenant.impersonations.map((visit, index) => (
              <TableRow key={index}>
                <TableCell>{formatMoment(visit.created_at)}</TableCell>
                <TableCell>{visit.operator_email}</TableCell>
                <TableCell>{visit.user_email}</TableCell>
                <TableCell>{visit.reason}</TableCell>
              </TableRow>
            ))}
            {tenant.impersonations.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="py-8 text-center text-muted">
                  {t("cp.message.no_activity")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.activity")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.when")}</TableHead>
              <TableHead>{t("cp.field.action")}</TableHead>
              <TableHead>{t("cp.field.resource")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {tenant.activity.map((entry, index) => (
              <TableRow key={index}>
                <TableCell>{formatMoment(entry.created_at)}</TableCell>
                <TableCell>{entry.action}</TableCell>
                <TableCell>{entry.resource}</TableCell>
              </TableRow>
            ))}
            {tenant.activity.length === 0 && (
              <TableRow>
                <TableCell colSpan={3} className="py-8 text-center text-muted">
                  {t("cp.message.no_activity")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.operator_actions")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.when")}</TableHead>
              <TableHead>{t("cp.field.operator")}</TableHead>
              <TableHead>{t("cp.field.action")}</TableHead>
              <TableHead>{t("cp.field.reason")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {tenant.operator_actions.map((entry, index) => (
              <TableRow key={index}>
                <TableCell>{formatMoment(entry.created_at)}</TableCell>
                <TableCell>{entry.operator_email}</TableCell>
                <TableCell>{entry.action}</TableCell>
                <TableCell>{entry.reason || "—"}</TableCell>
              </TableRow>
            ))}
            {tenant.operator_actions.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="py-8 text-center text-muted">
                  {t("cp.message.no_activity")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
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
    return <Badge tone="danger">{t("cp.state.deleting")}</Badge>;
  }
  if (tenant.suspended_at) {
    return <Badge tone="warning">{t("cp.state.suspended")}</Badge>;
  }
  return <Badge tone="success">{t("cp.state.active")}</Badge>;
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
    <Button
      variant="outline"
      onClick={onClick}
      leadingIcon={icon}
      className={danger ? "border-danger-border text-danger hover:bg-danger-soft" : undefined}
    >
      {label}
    </Button>
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
    <Modal onClose={onClose} label={t("cp.section.limits")}>
      <form onSubmit={submit} className="p-5 space-y-4">
        <h2 className="text-lg font-semibold text-foreground">{t("cp.section.limits")}</h2>

        {failure && (
          <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
        )}

        <Field label={t("cp.field.max_users")} value={users} onChange={setUsers} />
        <Field label={t("cp.field.max_storage")} value={storage} onChange={setStorage} hint={t("cp.hint.not_enforced")} />
        <Field label={t("cp.field.max_ai")} value={ai} onChange={setAI} hint={t("cp.hint.not_enforced")} />

        <div className="flex flex-col gap-1.5">
          <label htmlFor="quota-enforcement" className="text-sm font-medium text-foreground">
            {t("cp.field.enforcement")}
          </label>
          <Select value={enforcement} onValueChange={(next) => setEnforcement(next as "soft" | "hard")}>
            <SelectTrigger id="quota-enforcement" />
            <SelectContent>
              <SelectItem value="soft">{t("cp.state.soft")}</SelectItem>
              <SelectItem value="hard">{t("cp.state.hard")}</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <Input label={t("cp.field.reason")} required value={reason} onChange={(event) => setReason(event.target.value)} />

        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>
            {t("cp.action.cancel")}
          </Button>
          <Button type="submit" loading={busy}>
            {t("cp.action.confirm")}
          </Button>
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
    <Input
      label={label}
      inputMode="numeric"
      value={value}
      onChange={(event) => onChange(event.target.value.replace(/\D/g, ""))}
      helperText={hint && <span className="text-warning">{hint}</span>}
    />
  );
}

function BackLink({ label }: { label: string }) {
  return (
    <Link href="/cp/tenants" className="inline-flex items-center gap-1.5 text-sm text-muted hover:text-foreground">
      <ArrowLeft className="w-4 h-4" />
      {label}
    </Link>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-surface rounded-lg border border-line px-4 py-3">
      <dt className="text-xs uppercase tracking-wide text-muted">{label}</dt>
      <dd className="mt-1 text-sm text-foreground">{value}</dd>
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
  const { t } = useI18n();
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
    <Modal onClose={onClose} label={t("cp.action.add_person")}>
      <form onSubmit={submit} className="p-5 space-y-4">
        <h2 className="text-lg font-semibold text-foreground">{t("cp.action.add_person")}</h2>
        <p className="text-xs text-muted">{t("cp.hint.member_is_chosen")}</p>

        {failure && (
          <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
        )}

        {chosen ? (
          <div className="flex items-center gap-3 rounded-lg border border-accent bg-accent-soft px-3 py-2">
            <span className="min-w-0 flex-1">
              <strong className="block text-sm text-foreground truncate">{chosen.name}</strong>
              <span className="block text-xs text-muted truncate">{chosen.email}</span>
            </span>
            <Button variant="outline" size="sm" onClick={() => setChosen(null)}>
              {t("cp.action.change")}
            </Button>
          </div>
        ) : (
          <div className="space-y-2">
            <Input
              type="search"
              label={t("cp.field.search_people")}
              hideLabel
              value={search}
              onChange={(event) => {
                setSearch(event.target.value);
                void loadPeople(event.target.value);
              }}
              placeholder={t("cp.field.search_people")}
            />
            <div className="max-h-48 overflow-y-auto divide-y divide-line rounded-lg border border-line">
              {people.map((person) => (
                <button
                  key={person.user_id}
                  type="button"
                  onClick={() => setChosen(person)}
                  className="w-full text-start px-3 py-2 hover:bg-surface-hover"
                >
                  <strong className="block text-sm text-foreground truncate">{person.name}</strong>
                  <span className="block text-xs text-muted truncate">
                    {person.email}
                    {person.reg_number ? ` · ${person.reg_number}` : ""}
                    {" · "}
                    {formatMoment(person.last_seen_at)}
                  </span>
                </button>
              ))}
              {people.length === 0 && (
                <p className="px-3 py-3 text-sm text-muted">{t("cp.message.no_verified_people")}</p>
              )}
            </div>
          </div>
        )}

        <Input label={t("cp.field.reason")} required value={reason} onChange={(event) => setReason(event.target.value)} />

        {/* The API asks for the second factor before it hands anybody the keys
            to an organisation's data; confirming it here keeps what has been
            typed. */}
        <Input
          label={t("cp.field.code")}
          helperText={t("cp.hint.step_up")}
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
          className="font-mono tracking-[0.4em]"
        />

        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>
            {t("cp.action.cancel")}
          </Button>
          <Button type="submit" disabled={!chosen} loading={busy}>
            {t("cp.action.add_person")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
