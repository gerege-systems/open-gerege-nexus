"use client";

/**
 * How the platform behaves, and what is switched on.
 *
 * Two lists on one screen because they are the same question asked twice —
 * "what is this deployment doing right now" — and an operator during an
 * incident should not have to remember which of two pages holds the answer.
 *
 * Every field says where its value came from. A setting reading "environment"
 * is one somebody set in a file and forgot; one reading "database" was chosen
 * here, and the history says by whom and why.
 */

import React, { useCallback, useEffect, useState } from "react";
import { History, KeyRound, RotateCcw, ToggleLeft, ToggleRight, X } from "lucide-react";

import { useAction } from "@/components/cp/Action";
import { cp, type Credential, type Flag, type Setting, type SettingChange } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";
import {
  Badge,
  Button,
  Card,
  CardHeader,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  IconButton,
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
import { formatMoment } from "@/lib/datetime";

export default function Configuration() {
  const { t } = useI18n();
  const action = useAction();

  const [settings, setSettings] = useState<Setting[]>([]);
  const [warnings, setWarnings] = useState<string[]>([]);
  const [flags, setFlags] = useState<Flag[]>([]);
  const [failure, setFailure] = useState("");
  const [editing, setEditing] = useState<Setting | null>(null);
  const [history, setHistory] = useState<{ key: string; changes: SettingChange[] } | null>(null);
  const [newFlag, setNewFlag] = useState(false);
  const [credentials, setCredentials] = useState<Credential[]>([]);
  const [sealing, setSealing] = useState(true);
  const [editingCredential, setEditingCredential] = useState<Credential | null>(null);
  const [loaded, setLoaded] = useState(false);

  const load = useCallback(async () => {
    try {
      const [config, flagList, keys] = await Promise.all([cp.settings(), cp.flags(), cp.credentials()]);
      setSettings(config.settings);
      setWarnings(config.warnings ?? []);
      setFlags(flagList.flags);
      setCredentials(keys.credentials);
      setSealing(keys.sealing_configured);
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

  async function openHistory(key: string) {
    const result = await cp.settingHistory(key);
    setHistory({ key, changes: result.changes });
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-foreground">{t("cp.section.config")}</h1>
        <p className="mt-1 text-sm text-muted">{t("cp.hint.config")}</p>
      </div>

      {warnings.map((warning) => (
        <p key={warning} className="text-sm rounded-lg bg-warning-soft border border-warning-border text-warning px-4 py-3">
          {warning}
        </p>
      ))}
      {failure && (
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
      )}

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.settings")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.setting")}</TableHead>
              <TableHead>{t("cp.field.value")}</TableHead>
              <TableHead>{t("cp.field.source")}</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {settings.map((setting) => (
              <TableRow key={setting.key}>
                <TableCell>
                  <span className="font-mono text-xs text-foreground">{setting.key}</span>
                  <span className="block text-xs text-muted">{setting.description}</span>
                </TableCell>
                <TableCell className="font-mono text-xs">{setting.current === "" ? "—" : setting.current}</TableCell>
                <TableCell>
                  <Badge tone={setting.source === "database" ? "success" : "neutral"}>
                    {t(`cp.source.${setting.source}`)}
                  </Badge>
                </TableCell>
                <TableCell>
                  <span className="flex gap-2">
                    <Button variant="outline" size="sm" onClick={() => setEditing(setting)}>
                      {t("cp.action.change")}
                    </Button>
                    <IconButton
                      variant="outline"
                      size="sm"
                      onClick={() => void openHistory(setting.key)}
                      aria-label={`${t("cp.section.history")}: ${setting.key}`}
                      icon={<History />}
                    />
                  </span>
                </TableCell>
              </TableRow>
            ))}
            {settings.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="py-8 text-center text-muted">
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

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.credentials")}</h2>
        </CardHeader>
        <p className="px-4 pt-4 text-sm text-muted">{t("cp.hint.credentials")}</p>
        {!sealing && (
          <p className="mx-4 mt-3 text-sm rounded-lg bg-warning-soft text-warning border border-warning-border px-3 py-2">
            {t("cp.message.sealing_off")}
          </p>
        )}
        {/* Breathing room before the table: the hint and the warning above are
            inset by mx-4, and a table header flush against them reads as one
            block glued together. */}
        <div className="h-4" aria-hidden="true" />
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.credential")}</TableHead>
              <TableHead>{t("cp.field.source")}</TableHead>
              <TableHead>{t("cp.field.updated")}</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {credentials.map((credential) => (
              <TableRow key={credential.name}>
                <TableCell>
                  <span className="font-mono text-xs text-foreground">{credential.name}</span>
                  <span className="block text-xs text-muted">{credential.description}</span>
                  <span className="block text-xs text-muted font-mono">{credential.env}</span>
                </TableCell>
                <TableCell>
                  <span className="inline-flex items-center gap-2">
                    <Badge tone={credential.source === "database" ? "success" : credential.source === "environment" ? "neutral" : "danger"}>
                      {t(`cp.source.${credential.source}`)}
                    </Badge>
                    {/* The last four characters, and only of a value long enough that
                        four does not give it away. It is how an operator tells two
                        keys apart and sees that a rotation landed. */}
                    {credential.hint && <span className="font-mono text-xs text-muted">…{credential.hint}</span>}
                  </span>
                </TableCell>
                <TableCell className="text-xs text-muted">{credential.updated_at ? formatMoment(credential.updated_at) : "—"}</TableCell>
                <TableCell>
                  <span className="flex gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={!sealing}
                      onClick={() => setEditingCredential(credential)}
                      leadingIcon={<KeyRound />}
                    >
                      {t("cp.action.set_credential")}
                    </Button>
                    {credential.source === "database" && (
                      <IconButton
                        variant="outline"
                        size="sm"
                        onClick={() =>
                          action.run({
                            title: t("cp.action.clear_credential"),
                            detail: credential.name,
                            danger: true,
                            perform: (reason) => cp.clearCredential(credential.name, reason),
                            onDone: load,
                          })
                        }
                        aria-label={`${t("cp.action.clear_credential")}: ${credential.name}`}
                        icon={<X />}
                      />
                    )}
                  </span>
                </TableCell>
              </TableRow>
            ))}
            {credentials.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="py-8 text-center text-muted">
                  {!loaded ? (
                    <span role="status" className="inline-flex items-center gap-2">
                      <Spinner size="sm" decorative />
                      {t("base.message.loading")}
                    </span>
                  ) : (
                    t("cp.message.no_credentials")
                  )}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.flags")}</h2>
        </CardHeader>
        <div className="p-4">
          <Button onClick={() => setNewFlag(true)}>{t("cp.action.new_flag")}</Button>
        </div>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.flag")}</TableHead>
              <TableHead>{t("cp.field.kind")}</TableHead>
              <TableHead>{t("cp.field.rollout")}</TableHead>
              <TableHead>{t("cp.field.expires")}</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {flags.map((flag) => (
              <TableRow key={flag.key}>
                <TableCell>
                  <span className="font-mono text-xs text-foreground">{flag.key}</span>
                  <span className="block text-xs text-muted">
                    {flag.description}
                    {flag.owner ? ` · ${flag.owner}` : ""}
                  </span>
                </TableCell>
                <TableCell>
                  <Badge tone={flag.kind === "kill_switch" ? "danger" : "neutral"}>
                    {t(`cp.kind.${flag.kind}`)}
                  </Badge>
                </TableCell>
                <TableCell className="tabular-nums text-xs">{flag.enabled ? `${flag.rollout}%` : t("cp.state.off")}</TableCell>
                <TableCell>
                  <span className={flag.expires_at && new Date(flag.expires_at) < new Date() ? "text-warning" : ""}>
                    {formatMoment(flag.expires_at) || "—"}
                  </span>
                </TableCell>
                <TableCell>
                  <span className="flex gap-2">
                    <IconButton
                      variant="outline"
                      size="sm"
                      onClick={() =>
                        action.run({
                          title: flag.enabled ? t("cp.action.turn_off") : t("cp.action.turn_on"),
                          detail: flag.key,
                          danger: flag.kind === "kill_switch" && !flag.enabled,
                          perform: (reason) =>
                            cp.saveFlag({ ...flag, enabled: !flag.enabled, reason }),
                          onDone: load,
                        })
                      }
                      aria-label={`${flag.enabled ? t("cp.action.turn_off") : t("cp.action.turn_on")}: ${flag.key}`}
                      icon={flag.enabled ? <ToggleRight className="text-success" /> : <ToggleLeft />}
                    />
                    <IconButton
                      variant="outline"
                      size="sm"
                      onClick={() =>
                        action.run({
                          title: t("cp.action.delete_flag"),
                          detail: flag.key,
                          danger: true,
                          perform: (reason) => cp.deleteFlag(flag.key, reason),
                          onDone: load,
                        })
                      }
                      aria-label={`${t("cp.action.delete_flag")}: ${flag.key}`}
                      icon={<X />}
                    />
                  </span>
                </TableCell>
              </TableRow>
            ))}
            {flags.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="py-8 text-center text-muted">
                  {!loaded ? (
                    <span role="status" className="inline-flex items-center gap-2">
                      <Spinner size="sm" decorative />
                      {t("base.message.loading")}
                    </span>
                  ) : (
                    t("cp.message.no_flags")
                  )}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      {editing && (
        <SettingDialog
          setting={editing}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            void load();
          }}
        />
      )}

      {editingCredential && (
        <CredentialDialog
          credential={editingCredential}
          onClose={() => setEditingCredential(null)}
          onSaved={() => {
            setEditingCredential(null);
            void load();
          }}
        />
      )}

      {history && (
        <Dialog open onOpenChange={(open) => { if (!open) setHistory(null); }}>
          <DialogContent size="lg" aria-describedby={undefined} className="max-h-[90dvh] overflow-y-auto">
            <DialogHeader>
              <DialogTitle className="font-mono text-sm">{history.key}</DialogTitle>
            </DialogHeader>
            <Table containerClassName="rounded-none border-0">
              <TableHeader>
                <TableRow>
                  <TableHead>{t("cp.field.when")}</TableHead>
                  <TableHead>{t("cp.field.value")}</TableHead>
                  <TableHead>{t("cp.field.operator")}</TableHead>
                  <TableHead>{t("cp.field.reason")}</TableHead>
                  <TableHead />
                </TableRow>
              </TableHeader>
              <TableBody>
                {history.changes.map((change) => (
                  <TableRow key={change.id}>
                    <TableCell>{formatMoment(change.changed_at)}</TableCell>
                    <TableCell className="font-mono text-xs">{change.previous_value ?? "—"} → {change.new_value}</TableCell>
                    <TableCell>{change.changed_by}</TableCell>
                    <TableCell>{change.reason}</TableCell>
                    <TableCell>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => {
                          setHistory(null);
                          action.run({
                            title: t("cp.action.rollback"),
                            detail: `${change.key}: ${change.previous_value ?? "—"}`,
                            perform: (reason) => cp.rollbackSetting(change.id, reason),
                            onDone: load,
                          });
                        }}
                        leadingIcon={<RotateCcw />}
                      >
                        {t("cp.action.rollback")}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {history.changes.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="py-8 text-center text-muted">
                      {t("cp.message.no_activity")}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
            <div className="flex justify-end pt-4">
              <Button variant="ghost" onClick={() => setHistory(null)}>
                {t("cp.action.cancel")}
              </Button>
            </div>
          </DialogContent>
        </Dialog>
      )}

      {newFlag && (
        <FlagDialog
          onClose={() => setNewFlag(false)}
          onSaved={() => {
            setNewFlag(false);
            void load();
          }}
        />
      )}

      {action.dialog}
    </div>
  );
}

/**
 * Setting a credential.
 *
 * The field starts empty and there is nothing to prefill it with: no route
 * returns a stored value, so what is on screen is what is being typed now. A
 * dialog that showed the current key would be the one place in this console
 * where a stolen session is worth more than the actions it can take.
 */
function CredentialDialog({
  credential,
  onClose,
  onSaved,
}: {
  credential: Credential;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { t } = useI18n();
  const [value, setValue] = useState("");
  const [reason, setReason] = useState("");
  const [code, setCode] = useState("");
  const [needsCode, setNeedsCode] = useState(false);
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setFailure("");
    try {
      if (needsCode) await cp.stepUp(code);
      await cp.setCredential(credential.name, value, reason);
      onSaved();
    } catch (error) {
      if (error instanceof Error && error.name === "StepUpRequired") {
        setNeedsCode(true);
        setFailure(t("cp.message.step_up"));
        return;
      }
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto">
      <form onSubmit={submit} className="space-y-4">
        <DialogHeader className="pb-0">
          <DialogTitle className="font-mono text-sm">{credential.name}</DialogTitle>
          <DialogDescription>{credential.description}</DialogDescription>
          {credential.docs && (
            <a
              href={credential.docs}
              target="_blank"
              rel="noreferrer"
              className="mt-1 inline-block text-xs text-info hover:underline"
            >
              {credential.docs}
            </a>
          )}
        </DialogHeader>

        <p className="text-xs text-muted">{t("cp.message.credential_write_only")}</p>

        {failure && (
          <p className="text-sm rounded-lg bg-warning-soft text-warning border border-warning-border px-3 py-2">
            {failure}
          </p>
        )}

        <Input
          type="password"
          label={t("cp.field.value")}
          autoComplete="off"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          required
          className="[&_input]:font-mono"
        />

        <Input label={t("cp.field.reason")} value={reason} onChange={(event) => setReason(event.target.value)} required />

        {needsCode && (
          <Input
            label={t("cp.field.code")}
            value={code}
            onChange={(event) => setCode(event.target.value)}
            inputMode="numeric"
            className="[&_input]:font-mono [&_input]:tracking-widest"
          />
        )}

        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>
            {t("cp.action.cancel")}
          </Button>
          <Button type="submit" loading={busy}>
            {t("cp.action.set_credential")}
          </Button>
        </div>
      </form>
      </DialogContent>
    </Dialog>
  );
}

function SettingDialog({
  setting,
  onClose,
  onSaved,
}: {
  setting: Setting;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { t } = useI18n();
  const [value, setValue] = useState(setting.current);
  const [reason, setReason] = useState("");
  const [code, setCode] = useState("");
  const [needsCode, setNeedsCode] = useState(false);
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setFailure("");
    try {
      if (needsCode) await cp.stepUp(code);
      await cp.setSetting(setting.key, value, reason);
      onSaved();
    } catch (error) {
      if (error instanceof Error && error.name === "StepUpRequired") {
        setNeedsCode(true);
        setFailure(t("cp.message.step_up"));
        return;
      }
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto">
      <form onSubmit={submit} className="space-y-4">
        <DialogHeader className="pb-0">
          <DialogTitle className="font-mono text-sm">{setting.key}</DialogTitle>
          <DialogDescription>{setting.description}</DialogDescription>
        </DialogHeader>

        {failure && (
          <p className="text-sm rounded-lg bg-warning-soft text-warning border border-warning-border px-3 py-2">
            {failure}
          </p>
        )}

        {setting.kind === "enum" || setting.kind === "bool" ? (
          <div className="flex flex-col gap-1.5">
            <label htmlFor="setting-value" className="text-sm font-medium text-foreground">
              {t("cp.field.value")}
            </label>
            <Select value={value} onValueChange={setValue}>
              <SelectTrigger id="setting-value" aria-describedby="setting-value-default" />
              <SelectContent>
                {(setting.kind === "bool" ? ["false", "true"] : (setting.options ?? [])).map((option) => (
                  <SelectItem key={option} value={option}>
                    {option}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <span id="setting-value-default" className="text-xs text-muted">
              {t("cp.field.default")}: <span className="font-mono">{setting.default || "—"}</span>
              {setting.env ? ` · ${setting.env}` : ""}
            </span>
          </div>
        ) : (
          <Input
            label={t("cp.field.value")}
            value={value}
            onChange={(event) => setValue(event.target.value)}
            className="[&_input]:font-mono [&_input]:text-sm"
            helperText={
              <>
                {t("cp.field.default")}: <span className="font-mono">{setting.default || "—"}</span>
                {setting.env ? ` · ${setting.env}` : ""}
              </>
            }
          />
        )}

        <Input label={t("cp.field.reason")} required value={reason} onChange={(event) => setReason(event.target.value)} />

        {needsCode && (
          <Input
            label={t("cp.field.code")}
            inputMode="numeric"
            maxLength={6}
            required
            value={code}
            onChange={(event) => setCode(event.target.value.replace(/\D/g, ""))}
            className="[&_input]:font-mono [&_input]:tracking-[0.4em]"
          />
        )}

        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>
            {t("cp.action.cancel")}
          </Button>
          <Button type="submit" loading={busy}>
            {t("cp.action.confirm")}
          </Button>
        </div>
      </form>
      </DialogContent>
    </Dialog>
  );
}

function FlagDialog({ onClose, onSaved }: { onClose: () => void; onSaved: () => void }) {
  const { t } = useI18n();
  const [key, setKey] = useState("");
  const [description, setDescription] = useState("");
  const [owner, setOwner] = useState("");
  const [kind, setKind] = useState<"release" | "kill_switch" | "experiment">("release");
  const [rollout, setRollout] = useState("100");
  const [expires, setExpires] = useState("");
  const [reason, setReason] = useState("");
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setFailure("");
    try {
      await cp.saveFlag({
        key,
        description,
        owner,
        kind,
        enabled: false,
        rollout: Number(rollout),
        // A flag with no date is a flag nobody will remember to remove, so the
        // field is offered every time one is created — the console warns about
        // the ones that lapse, and it can only warn about the ones that have a
        // date to lapse.
        expires_at: expires ? new Date(expires).toISOString() : null,
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
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent aria-describedby={undefined} className="max-h-[90dvh] overflow-y-auto">
      <DialogHeader>
        <DialogTitle>{t("cp.action.new_flag")}</DialogTitle>
      </DialogHeader>
      <form onSubmit={submit} className="space-y-3">

        {failure && (
          <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
        )}

        <Line label={t("cp.field.flag")} value={key} onChange={setKey} required mono />
        <Line label={t("cp.field.description")} value={description} onChange={setDescription} />
        <Line label={t("cp.field.owner")} value={owner} onChange={setOwner} />

        <div className="flex flex-col gap-1.5">
          <label htmlFor="flag-kind" className="text-sm font-medium text-foreground">
            {t("cp.field.kind")}
          </label>
          <Select value={kind} onValueChange={(next) => setKind(next as typeof kind)}>
            <SelectTrigger id="flag-kind" />
            <SelectContent>
              <SelectItem value="release">{t("cp.kind.release")}</SelectItem>
              <SelectItem value="kill_switch">{t("cp.kind.kill_switch")}</SelectItem>
              <SelectItem value="experiment">{t("cp.kind.experiment")}</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <Line label={t("cp.field.rollout")} value={rollout} onChange={setRollout} />

        <Input type="date" label={t("cp.field.expires")} value={expires} onChange={(event) => setExpires(event.target.value)} />

        <Line label={t("cp.field.reason")} value={reason} onChange={setReason} required />

        <div className="flex justify-end gap-2 pt-1">
          <Button variant="ghost" onClick={onClose}>
            {t("cp.action.cancel")}
          </Button>
          <Button type="submit" loading={busy}>
            {t("cp.action.create")}
          </Button>
        </div>
      </form>
      </DialogContent>
    </Dialog>
  );
}

function Line({
  label,
  value,
  onChange,
  required,
  mono,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  required?: boolean;
  mono?: boolean;
}) {
  return (
    <Input
      label={label}
      required={required}
      value={value}
      onChange={(event) => onChange(event.target.value)}
      className={mono ? "[&_input]:font-mono [&_input]:text-sm" : undefined}
    />
  );
}
