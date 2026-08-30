"use client";

/**
 * Who may reach this console.
 *
 * Adding an operator used to mean a shell on the production host — the
 * bootstrap command, run as the database owner. This screen replaces that for
 * everybody after the first: a superadmin, with a second factor, leaving an
 * audit row.
 *
 * The handover panel appears once. The password and the authenticator's secret
 * are not stored anywhere they can be read back, so closing it without writing
 * them down means adding the account again.
 */

import React, { useCallback, useEffect, useState } from "react";
import { KeyRound, Search, ShieldCheck, UserPlus } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";

import { useAction } from "@/components/cp/Action";
import { useConsole } from "@/components/cp/Console";
import { Badge, Card, formatMoment, Table } from "@/components/cp/ui";
import { cp, type CreatedOperator, type OperatorSummary } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";
import { Modal } from "@/components/ui";

const ROLES = ["superadmin", "operator", "support", "auditor"] as const;

export default function Operators() {
  const { t, locale } = useI18n();
  const { operator: me } = useConsole();
  const action = useAction();
  const [operators, setOperators] = useState<OperatorSummary[]>([]);
  const [failure, setFailure] = useState("");
  const [adding, setAdding] = useState(false);
  const [changing, setChanging] = useState(false);
  const [handover, setHandover] = useState<CreatedOperator | null>(null);

  const load = useCallback(async () => {
    try {
      setOperators((await cp.operators()).operators);
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const superadmin = me.role === "superadmin";

  return (
    <div className="space-y-6">
      <div className="flex items-start gap-3">
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-slate-900 flex items-center gap-2">
            <ShieldCheck className="w-6 h-6 text-[var(--gerege-blue)]" />
            {t("cp.section.operators")}
          </h1>
          <p className="mt-1 text-sm text-slate-500">{t("cp.hint.operators")}</p>
        </div>
        <button
          type="button"
          onClick={() => setChanging(true)}
          className="inline-flex items-center gap-2 rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 hover:bg-slate-50"
        >
          <KeyRound className="w-4 h-4" />
          {t("cp.action.change_password")}
        </button>
        {superadmin && (
          <button
            type="button"
            onClick={() => setAdding(true)}
            className="inline-flex items-center gap-2 rounded-lg bg-[var(--gerege-blue)] px-3 py-2 text-sm font-medium text-[var(--gerege-on-blue)] hover:brightness-105"
          >
            <UserPlus className="w-4 h-4" />
            {t("cp.action.add_operator")}
          </button>
        )}
      </div>

      {failure && (
        <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
      )}

      <Card title={t("cp.section.operators")}>
        <Table
          head={[
            t("cp.field.operator"),
            t("cp.field.role"),
            t("cp.field.status"),
            t("cp.field.last_login"),
            "",
          ]}
          rows={operators.map((row) => [
            <span key="n" className="min-w-0">
              <strong className="text-slate-900">{row.name}</strong>
              <span className="block text-xs text-slate-500 font-mono">{row.email}</span>
            </span>,
            <Badge key="r" tone={row.role === "superadmin" ? "amber" : "slate"}>
              {t(`cp.role.${row.role}` as "cp.role.operator")}
            </Badge>,
            <Badge key="s" tone={row.disabled_at ? "red" : row.enrolled ? "emerald" : "amber"}>
              {row.disabled_at
                ? t("cp.state.disabled")
                : row.enrolled
                  ? t("cp.state.normal")
                  : t("cp.state.enrolment_pending")}
            </Badge>,
            formatMoment(row.last_login_at, locale) || <span key="l" className="text-xs text-slate-400">{t("cp.state.never")}</span>,
            superadmin && row.id !== me.id ? (
              <span key="a" className="flex items-center gap-2">
                <select
                  value={row.role}
                  aria-label={t("cp.field.role")}
                  onChange={(event) => {
                    const role = event.target.value;
                    action.run({
                      title: t("cp.action.change_role"),
                      detail: `${row.email} → ${t(`cp.role.${role}` as "cp.role.operator")}`,
                      perform: (reason) => cp.setOperatorRole(row.id, role, reason),
                      onDone: load,
                    });
                  }}
                  className="rounded-lg border border-slate-300 px-2 py-1 text-xs"
                >
                  {ROLES.map((role) => (
                    <option key={role} value={role}>
                      {t(`cp.role.${role}` as "cp.role.operator")}
                    </option>
                  ))}
                </select>
                <button
                  type="button"
                  onClick={() =>
                    action.run({
                      title: row.disabled_at ? t("cp.action.enable") : t("cp.action.disable"),
                      detail: row.email,
                      danger: !row.disabled_at,
                      perform: (reason) => cp.setOperatorEnabled(row.id, !!row.disabled_at, reason),
                      onDone: load,
                    })
                  }
                  className="text-xs rounded-lg border border-slate-300 px-2 py-1 hover:bg-slate-50"
                >
                  {row.disabled_at ? t("cp.action.enable") : t("cp.action.disable")}
                </button>
              </span>
            ) : (
              <span key="a" className="text-xs text-slate-400">{row.id === me.id ? t("cp.state.you") : "—"}</span>
            ),
          ])}
          empty={t("cp.message.no_activity")}
        />
      </Card>

      {adding && (
        <AddDialog
          onClose={() => setAdding(false)}
          onAdded={(created) => {
            setAdding(false);
            setHandover(created);
            void load();
          }}
        />
      )}
      {handover && (
        <HandoverDialog
          created={handover}
          onClose={() => {
            setHandover(null);
            void load();
          }}
        />
      )}
      {changing && <PasswordDialog onClose={() => setChanging(false)} />}
      {action.dialog}
    </div>
  );
}

function AddDialog({ onClose, onAdded }: { onClose: () => void; onAdded: (created: CreatedOperator) => void }) {
  const { t } = useI18n();
  const [registration, setRegistration] = useState("");
  const [looking, setLooking] = useState(false);
  const [found, setFound] = useState(false);
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [role, setRole] = useState<string>("operator");
  const [reason, setReason] = useState("");
  const [code, setCode] = useState("");
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);

  // The register spells the name; a name typed into this dialog is a name
  // somebody transliterated. The address comes with it when the register holds
  // one — an operator without an address in the register types their own.
  async function lookUp() {
    if (!registration.trim()) return;
    setLooking(true);
    setFailure("");
    try {
      const person = await cp.findPerson(registration.trim());
      setName(person.name);
      if (person.email) setEmail(person.email);
      setRegistration(person.registration_number);
      setFound(true);
    } catch (error) {
      setFound(false);
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setLooking(false);
    }
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setFailure("");
    try {
      onAdded(await cp.addOperator({ email, name, role, reason }));
    } catch (error) {
      // A step-up asks for the code and comes back here; anything else is said
      // as it arrived.
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal label={t("cp.action.add_operator")}>
      <form onSubmit={submit} className="p-5 space-y-4">
        <h2 className="text-lg font-semibold text-slate-900">{t("cp.action.add_operator")}</h2>
        {failure && (
          <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
        )}
        {/* The register first: the two fields below are filled from its
            answer, and typed over only when it is wrong or silent. */}
        <div className="flex items-end gap-2">
          <label className="block text-sm flex-1">
            <span className="text-slate-600">{t("cp.field.registration")}</span>
            <input
              value={registration}
              onChange={(event) => setRegistration(event.target.value)}
              className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
            />
          </label>
          <button
            type="button"
            onClick={() => void lookUp()}
            disabled={looking || !registration.trim()}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 hover:bg-slate-50 disabled:opacity-50"
          >
            <Search className={`w-4 h-4 ${looking ? "animate-pulse" : ""}`} />
            {t("cp.action.look_up")}
          </button>
        </div>
        {found && (
          <p className="text-xs rounded-lg bg-[var(--gerege-blue-soft)] text-[var(--gerege-blue)] px-3 py-2">
            {t("cp.message.from_the_register", { name })}
          </p>
        )}

        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.email")}</span>
          <input
            type="email"
            required
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
          />
        </label>
        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.name")}</span>
          <input
            required
            value={name}
            onChange={(event) => setName(event.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
          />
        </label>
        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.role")}</span>
          <select
            value={role}
            onChange={(event) => setRole(event.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
          >
            {ROLES.map((option) => (
              <option key={option} value={option}>
                {t(`cp.role.${option}` as "cp.role.operator")}
              </option>
            ))}
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
        {/* The console asks for the second factor before it mints an account;
            the API refuses without it. Kept in the same dialog so the step-up
            does not throw away what has been typed. */}
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
          <button type="button" onClick={onClose} className="rounded-lg border border-slate-300 px-4 py-2 text-sm">
            {t("cp.action.cancel")}
          </button>
          <button
            type="submit"
            disabled={busy}
            className="rounded-lg bg-[var(--gerege-blue)] px-4 py-2 text-sm font-medium text-[var(--gerege-on-blue)] hover:brightness-105 disabled:opacity-60"
          >
            {t("cp.action.add_operator")}
          </button>
        </div>
      </form>
    </Modal>
  );
}

/**
 * The one moment these values exist.
 *
 * Nothing on the server can show the password or the secret again, so the
 * dialog says so and the enrolment is finished here, with the new operator's
 * own authenticator, before anybody walks away.
 */
function HandoverDialog({ created, onClose }: { created: CreatedOperator; onClose: () => void }) {
  const { t } = useI18n();
  const [code, setCode] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);

  async function confirm(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setFailure("");
    try {
      await cp.confirmEnrolment(created.id, code, `enrolled ${created.email}`);
      setConfirmed(true);
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal label={t("cp.view.handover")}>
      <div className="p-5 space-y-4">
        <h2 className="text-lg font-semibold text-slate-900">{t("cp.view.handover")}</h2>
        <p className="text-sm rounded-lg bg-amber-50 border border-amber-200 text-amber-900 px-3 py-2">
          {t("cp.message.handover_once")}
        </p>

        <div className="grid gap-3 sm:grid-cols-[auto_minmax(0,1fr)] items-start">
          <div className="rounded-xl border border-slate-200 p-3 bg-white">
            <QRCodeSVG value={created.uri} size={148} />
          </div>
          <div className="space-y-2 min-w-0">
            <Field label={t("cp.field.email")} value={created.email} />
            <Field label={t("cp.field.password")} value={created.password} />
            <Field label={t("cp.field.secret")} value={created.secret} />
          </div>
        </div>

        {confirmed ? (
          <p className="text-sm rounded-lg bg-emerald-50 border border-emerald-200 text-emerald-800 px-3 py-2">
            {t("cp.message.enrolled")}
          </p>
        ) : (
          <form onSubmit={confirm} className="space-y-3">
            <p className="text-sm text-slate-600">{t("cp.hint.confirm_enrolment")}</p>
            {failure && (
              <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
            )}
            <div className="flex items-center gap-2">
              <input
                inputMode="numeric"
                pattern="[0-9]*"
                maxLength={6}
                required
                value={code}
                onChange={(event) => setCode(event.target.value.replace(/\D/g, ""))}
                className="w-40 rounded-lg border border-slate-300 px-3 py-2 font-mono tracking-[0.4em]"
              />
              <button
                type="submit"
                disabled={busy}
                className="rounded-lg bg-[var(--gerege-blue)] px-4 py-2 text-sm font-medium text-[var(--gerege-on-blue)] hover:brightness-105 disabled:opacity-60"
              >
                {t("cp.action.confirm")}
              </button>
            </div>
          </form>
        )}

        <div className="flex justify-end">
          <button type="button" onClick={onClose} className="rounded-lg border border-slate-300 px-4 py-2 text-sm">
            {t("base.action.close")}
          </button>
        </div>
      </div>
    </Modal>
  );
}

function PasswordDialog({ onClose }: { onClose: () => void }) {
  const { t } = useI18n();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [again, setAgain] = useState("");
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    if (next !== again) {
      setFailure(t("cp.message.passwords_differ"));
      return;
    }
    setBusy(true);
    setFailure("");
    try {
      await cp.changePassword(current, next);
      onClose();
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal label={t("cp.action.change_password")}>
      <form onSubmit={submit} className="p-5 space-y-4">
        <h2 className="text-lg font-semibold text-slate-900">{t("cp.action.change_password")}</h2>
        {failure && (
          <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
        )}
        {[
          { label: t("cp.field.current_password"), value: current, set: setCurrent, autoComplete: "current-password" },
          { label: t("cp.field.new_password"), value: next, set: setNext, autoComplete: "new-password" },
          { label: t("cp.field.repeat_password"), value: again, set: setAgain, autoComplete: "new-password" },
        ].map((field) => (
          <label key={field.label} className="block text-sm">
            <span className="text-slate-600">{field.label}</span>
            <input
              type="password"
              required
              autoComplete={field.autoComplete}
              value={field.value}
              onChange={(event) => field.set(event.target.value)}
              className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
            />
          </label>
        ))}
        <div className="flex justify-end gap-2">
          <button type="button" onClick={onClose} className="rounded-lg border border-slate-300 px-4 py-2 text-sm">
            {t("cp.action.cancel")}
          </button>
          <button
            type="submit"
            disabled={busy}
            className="rounded-lg bg-[var(--gerege-blue)] px-4 py-2 text-sm font-medium text-[var(--gerege-on-blue)] hover:brightness-105 disabled:opacity-60"
          >
            {t("base.action.save")}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <p className="text-xs font-semibold uppercase tracking-wider text-slate-400">{label}</p>
      <p className="font-mono text-sm text-slate-900 break-all select-all">{value}</p>
    </div>
  );
}
