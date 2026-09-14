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
import { Modal } from "@/components/ui";
import { cp, type CreatedOperator, type OperatorSummary } from "@/lib/cp";
import { formatMoment } from "@/lib/datetime";
import { useI18n } from "@/lib/i18n";
import {
  Badge,
  Button,
  Card,
  CardHeader,
  DialogHeader,
  DialogTitle,
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

const ROLES = ["superadmin", "operator", "support", "auditor"] as const;

export default function Operators() {
  const { t } = useI18n();
  const { operator: me } = useConsole();
  const action = useAction();
  const [operators, setOperators] = useState<OperatorSummary[]>([]);
  const [loaded, setLoaded] = useState(false);
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
    } finally {
      setLoaded(true);
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
          <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
            <ShieldCheck className="w-6 h-6 text-accent" />
            {t("cp.section.operators")}
          </h1>
          <p className="mt-1 text-sm text-muted">{t("cp.hint.operators")}</p>
        </div>
        <Button variant="outline" onClick={() => setChanging(true)} leadingIcon={<KeyRound />}>
          {t("cp.action.change_password")}
        </Button>
        {superadmin && (
          <Button onClick={() => setAdding(true)} leadingIcon={<UserPlus />}>
            {t("cp.action.add_operator")}
          </Button>
        )}
      </div>

      {failure && (
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
      )}

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.operators")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.operator")}</TableHead>
              <TableHead>{t("cp.field.role")}</TableHead>
              <TableHead>{t("cp.field.status")}</TableHead>
              <TableHead>{t("cp.field.last_login")}</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {operators.map((row) => (
              <TableRow key={row.id}>
                <TableCell className="min-w-0">
                  <strong className="text-foreground">{row.name}</strong>
                  <span className="block text-xs text-muted font-mono">{row.email}</span>
                </TableCell>
                <TableCell>
                  <Badge tone={row.role === "superadmin" ? "warning" : "neutral"}>
                    {t(`cp.role.${row.role}` as "cp.role.operator")}
                  </Badge>
                </TableCell>
                <TableCell>
                  <Badge tone={row.disabled_at ? "danger" : row.enrolled ? "success" : "warning"}>
                    {row.disabled_at
                      ? t("cp.state.disabled")
                      : row.enrolled
                        ? t("cp.state.normal")
                        : t("cp.state.enrolment_pending")}
                  </Badge>
                </TableCell>
                <TableCell>
                  {formatMoment(row.last_login_at) || <span className="text-xs text-muted">{t("cp.state.never")}</span>}
                </TableCell>
                <TableCell>
                  {superadmin && row.id !== me.id ? (
                    <span className="flex items-center gap-2">
                      <Select
                        value={row.role}
                        onValueChange={(role) =>
                          action.run({
                            title: t("cp.action.change_role"),
                            detail: `${row.email} → ${t(`cp.role.${role}` as "cp.role.operator")}`,
                            perform: (reason) => cp.setOperatorRole(row.id, role, reason),
                            onDone: load,
                          })
                        }
                      >
                        <SelectTrigger size="sm" aria-label={t("cp.field.role")} className="w-36" />
                        <SelectContent>
                          {ROLES.map((role) => (
                            <SelectItem key={role} value={role}>
                              {t(`cp.role.${role}` as "cp.role.operator")}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() =>
                          action.run({
                            title: row.disabled_at ? t("cp.action.enable") : t("cp.action.disable"),
                            detail: row.email,
                            danger: !row.disabled_at,
                            perform: (reason) => cp.setOperatorEnabled(row.id, !!row.disabled_at, reason),
                            onDone: load,
                          })
                        }
                      >
                        {row.disabled_at ? t("cp.action.enable") : t("cp.action.disable")}
                      </Button>
                    </span>
                  ) : (
                    <span className="text-xs text-muted">{row.id === me.id ? t("cp.state.you") : "—"}</span>
                  )}
                </TableCell>
              </TableRow>
            ))}
            {operators.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="py-8 text-center text-muted">
                  {!loaded ? (
                    <span role="status" className="inline-flex items-center gap-2">
                      <Spinner size="sm" decorative />
                      {t("base.message.loading")}
                    </span>
                  ) : (
                    t("cp.message.no_activity")
                  )}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
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
    <Modal scrollable onClose={onClose} label={t("cp.action.add_operator")}>
      <DialogHeader>
        <DialogTitle>{t("cp.action.add_operator")}</DialogTitle>
      </DialogHeader>
      <form onSubmit={submit} className="space-y-4">
        {failure && (
          <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
        )}
        {/* The register first: the two fields below are filled from its
            answer, and typed over only when it is wrong or silent. */}
        <div className="flex items-end gap-2">
          <div className="flex-1">
            <Input
              label={t("cp.field.registration")}
              value={registration}
              onChange={(event) => setRegistration(event.target.value)}
            />
          </div>
          <Button
            variant="outline"
            onClick={() => void lookUp()}
            disabled={!registration.trim()}
            loading={looking}
            leadingIcon={<Search />}
          >
            {t("cp.action.look_up")}
          </Button>
        </div>
        {found && (
          <p className="text-xs rounded-lg bg-accent-soft text-accent px-3 py-2">
            {t("cp.message.from_the_register", { name })}
          </p>
        )}

        <Input
          type="email"
          label={t("cp.field.email")}
          required
          value={email}
          onChange={(event) => setEmail(event.target.value)}
        />
        <Input label={t("cp.field.name")} required value={name} onChange={(event) => setName(event.target.value)} />
        <div className="flex flex-col gap-1.5">
          <label htmlFor="add-operator-role" className="text-sm font-medium text-foreground">
            {t("cp.field.role")}
          </label>
          <Select value={role} onValueChange={setRole}>
            <SelectTrigger id="add-operator-role" />
            <SelectContent>
              {ROLES.map((option) => (
                <SelectItem key={option} value={option}>
                  {t(`cp.role.${option}` as "cp.role.operator")}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <Input label={t("cp.field.reason")} required value={reason} onChange={(event) => setReason(event.target.value)} />
        {/* The console asks for the second factor before it mints an account;
            the API refuses without it. Kept in the same dialog so the step-up
            does not throw away what has been typed. */}
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
          className="[&_input]:font-mono [&_input]:tracking-[0.4em]"
        />
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={onClose}>
            {t("cp.action.cancel")}
          </Button>
          <Button type="submit" loading={busy}>
            {t("cp.action.add_operator")}
          </Button>
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
    <Modal scrollable onClose={onClose} label={t("cp.view.handover")}>
      <DialogHeader>
        <DialogTitle>{t("cp.view.handover")}</DialogTitle>
      </DialogHeader>
      <div className="space-y-4">
        <p className="text-sm rounded-lg bg-warning-soft border border-warning-border text-warning px-3 py-2">
          {t("cp.message.handover_once")}
        </p>

        <div className="grid gap-3 sm:grid-cols-[auto_minmax(0,1fr)] items-start">
          <div className="rounded-lg border border-line p-3 bg-surface">
            <QRCodeSVG value={created.uri} size={148} />
          </div>
          <div className="space-y-2 min-w-0">
            <Field label={t("cp.field.email")} value={created.email} />
            <Field label={t("cp.field.password")} value={created.password} />
            <Field label={t("cp.field.secret")} value={created.secret} />
          </div>
        </div>

        {confirmed ? (
          <p className="text-sm rounded-lg bg-success-soft border border-success-border text-success px-3 py-2">
            {t("cp.message.enrolled")}
          </p>
        ) : (
          <form onSubmit={confirm} className="space-y-3">
            <p className="text-sm text-muted">{t("cp.hint.confirm_enrolment")}</p>
            {failure && (
              <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
            )}
            <div className="flex items-end gap-2">
              <div className="w-40">
                <Input
                  label={t("cp.field.code")}
                  hideLabel
                  inputMode="numeric"
                  pattern="[0-9]*"
                  maxLength={6}
                  required
                  value={code}
                  onChange={(event) => setCode(event.target.value.replace(/\D/g, ""))}
                  className="[&_input]:font-mono [&_input]:tracking-[0.4em]"
                />
              </div>
              <Button type="submit" loading={busy}>
                {t("cp.action.confirm")}
              </Button>
            </div>
          </form>
        )}

        <div className="flex justify-end">
          <Button variant="outline" onClick={onClose}>
            {t("base.action.close")}
          </Button>
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
    <Modal scrollable onClose={onClose} label={t("cp.action.change_password")}>
      <DialogHeader>
        <DialogTitle>{t("cp.action.change_password")}</DialogTitle>
      </DialogHeader>
      <form onSubmit={submit} className="space-y-4">
        {failure && (
          <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
        )}
        {[
          { label: t("cp.field.current_password"), value: current, set: setCurrent, autoComplete: "current-password" },
          { label: t("cp.field.new_password"), value: next, set: setNext, autoComplete: "new-password" },
          { label: t("cp.field.repeat_password"), value: again, set: setAgain, autoComplete: "new-password" },
        ].map((field) => (
          <Input
            key={field.label}
            type="password"
            label={field.label}
            required
            autoComplete={field.autoComplete}
            value={field.value}
            onChange={(event) => field.set(event.target.value)}
          />
        ))}
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={onClose}>
            {t("cp.action.cancel")}
          </Button>
          <Button type="submit" loading={busy}>
            {t("base.action.save")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <p className="text-xs font-semibold uppercase tracking-wider text-muted">{label}</p>
      <p className="font-mono text-sm text-foreground break-all select-all">{value}</p>
    </div>
  );
}
