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
import { History, KeyRound, RotateCcw, ToggleLeft, ToggleRight } from "lucide-react";

import { useAction } from "@/components/cp/Action";
import { Badge, Card, formatMoment, Table } from "@/components/cp/ui";
import { cp, type Credential, type Flag, type Setting, type SettingChange } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";
import { Modal } from "@/components/ui";

export default function Configuration() {
  const { t, locale } = useI18n();
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
        <h1 className="text-2xl font-semibold text-slate-900">{t("cp.section.config")}</h1>
        <p className="mt-1 text-sm text-slate-500">{t("cp.hint.config")}</p>
      </div>

      {warnings.map((warning) => (
        <p key={warning} className="text-sm rounded-xl bg-amber-50 border border-amber-200 text-amber-900 px-4 py-3">
          {warning}
        </p>
      ))}
      {failure && (
        <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
      )}

      <Card title={t("cp.section.settings")}>
        <Table
          head={[t("cp.field.setting"), t("cp.field.value"), t("cp.field.source"), ""]}
          rows={settings.map((setting) => [
            <span key="k">
              <span className="font-mono text-xs text-slate-900">{setting.key}</span>
              <span className="block text-xs text-slate-500">{setting.description}</span>
            </span>,
            <span key="v" className="font-mono text-xs">
              {setting.current === "" ? "—" : setting.current}
            </span>,
            <Badge key="s" tone={setting.source === "database" ? "emerald" : "slate"}>
              {t(`cp.source.${setting.source}`)}
            </Badge>,
            <span key="a" className="flex gap-2">
              <button
                type="button"
                onClick={() => setEditing(setting)}
                className="text-xs rounded-lg border border-slate-300 px-2 py-1 hover:bg-slate-50"
              >
                {t("cp.action.change")}
              </button>
              <button
                type="button"
                onClick={() => void openHistory(setting.key)}
                aria-label={`${t("cp.section.history")}: ${setting.key}`}
                title={t("cp.section.history")}
                className="text-xs rounded-lg border border-slate-300 px-2 py-1 hover:bg-slate-50 inline-flex items-center gap-1"
              >
                <History className="w-3 h-3" />
              </button>
            </span>,
          ])}
          empty={t("cp.message.no_activity")}
        />
      </Card>

      <Card title={t("cp.section.credentials")}>
        <p className="px-4 pt-4 text-sm text-slate-500">{t("cp.hint.credentials")}</p>
        {!sealing && (
          <p className="mx-4 mt-3 text-sm rounded-lg bg-amber-50 text-amber-900 border border-amber-200 px-3 py-2">
            {t("cp.message.sealing_off")}
          </p>
        )}
        <Table
          head={[t("cp.field.credential"), t("cp.field.source"), t("cp.field.updated"), ""]}
          rows={credentials.map((credential) => [
            <span key="k">
              <span className="font-mono text-xs text-slate-900">{credential.name}</span>
              <span className="block text-xs text-slate-500">{credential.description}</span>
              <span className="block text-xs text-slate-400 font-mono">{credential.env}</span>
            </span>,
            <span key="s" className="inline-flex items-center gap-2">
              <Badge tone={credential.source === "database" ? "emerald" : credential.source === "environment" ? "slate" : "red"}>
                {t(`cp.source.${credential.source}`)}
              </Badge>
              {/* The last four characters, and only of a value long enough that
                  four does not give it away. It is how an operator tells two
                  keys apart and sees that a rotation landed. */}
              {credential.hint && <span className="font-mono text-xs text-slate-400">…{credential.hint}</span>}
            </span>,
            <span key="u" className="text-xs text-slate-500">
              {credential.updated_at ? formatMoment(credential.updated_at, locale) : "—"}
            </span>,
            <span key="a" className="flex gap-2">
              <button
                type="button"
                disabled={!sealing}
                onClick={() => setEditingCredential(credential)}
                className="text-xs rounded-lg border border-slate-300 px-2 py-1 hover:bg-slate-50 disabled:opacity-50 inline-flex items-center gap-1"
              >
                <KeyRound className="w-3 h-3" />
                {t("cp.action.set_credential")}
              </button>
              {credential.source === "database" && (
                <button
                  type="button"
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
                  title={t("cp.action.clear_credential")}
                  className="text-xs rounded-lg border border-slate-300 px-2 py-1 hover:bg-slate-50"
                >
                  ✕
                </button>
              )}
            </span>,
          ])}
          empty={t("cp.message.no_credentials")}
        />
      </Card>

      <Card title={t("cp.section.flags")}>
        <div className="p-4 pb-0">
          <button
            type="button"
            onClick={() => setNewFlag(true)}
            className="rounded-lg bg-[var(--gerege-blue)] px-3 py-2 text-sm font-medium text-[var(--gerege-on-blue)] hover:brightness-105"
          >
            {t("cp.action.new_flag")}
          </button>
        </div>
        <Table
          head={[t("cp.field.flag"), t("cp.field.kind"), t("cp.field.rollout"), t("cp.field.expires"), ""]}
          rows={flags.map((flag) => [
            <span key="k">
              <span className="font-mono text-xs text-slate-900">{flag.key}</span>
              <span className="block text-xs text-slate-500">
                {flag.description}
                {flag.owner ? ` · ${flag.owner}` : ""}
              </span>
            </span>,
            <Badge key="t" tone={flag.kind === "kill_switch" ? "red" : "slate"}>
              {t(`cp.kind.${flag.kind}`)}
            </Badge>,
            <span key="r" className="tabular-nums text-xs">
              {flag.enabled ? `${flag.rollout}%` : t("cp.state.off")}
            </span>,
            <span key="e" className={flag.expires_at && new Date(flag.expires_at) < new Date() ? "text-amber-700" : ""}>
              {formatMoment(flag.expires_at, locale) || "—"}
            </span>,
            <span key="a" className="flex gap-2">
              <button
                type="button"
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
                title={flag.enabled ? t("cp.action.turn_off") : t("cp.action.turn_on")}
                className="text-xs rounded-lg border border-slate-300 px-2 py-1 hover:bg-slate-50 inline-flex items-center gap-1"
              >
                {flag.enabled ? <ToggleRight className="w-4 h-4 text-emerald-600" /> : <ToggleLeft className="w-4 h-4" />}
              </button>
              <button
                type="button"
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
                title={t("cp.action.delete_flag")}
                className="text-xs rounded-lg border border-slate-300 px-2 py-1 hover:bg-slate-50"
              >
                ✕
              </button>
            </span>,
          ])}
          empty={t("cp.message.no_flags")}
        />
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
        <Modal label={history.key}>
          <div className="p-5 space-y-4">
            <h2 className="text-lg font-semibold text-slate-900 font-mono text-sm">{history.key}</h2>
            <Table
              head={[t("cp.field.when"), t("cp.field.value"), t("cp.field.operator"), t("cp.field.reason"), ""]}
              rows={history.changes.map((change) => [
                formatMoment(change.changed_at, locale),
                <span key="v" className="font-mono text-xs">
                  {change.previous_value ?? "—"} → {change.new_value}
                </span>,
                change.changed_by,
                change.reason,
                <button
                  key="r"
                  type="button"
                  onClick={() => {
                    setHistory(null);
                    action.run({
                      title: t("cp.action.rollback"),
                      detail: `${change.key}: ${change.previous_value ?? "—"}`,
                      perform: (reason) => cp.rollbackSetting(change.id, reason),
                      onDone: load,
                    });
                  }}
                  className="text-xs rounded-lg border border-slate-300 px-2 py-1 hover:bg-slate-50 inline-flex items-center gap-1"
                >
                  <RotateCcw className="w-3 h-3" />
                  {t("cp.action.rollback")}
                </button>,
              ])}
              empty={t("cp.message.no_activity")}
            />
            <div className="flex justify-end">
              <button
                type="button"
                onClick={() => setHistory(null)}
                className="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100"
              >
                {t("cp.action.cancel")}
              </button>
            </div>
          </div>
        </Modal>
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
    <Modal label={credential.name}>
      <form onSubmit={submit} className="p-5 space-y-4">
        <div>
          <h2 className="font-mono text-sm font-semibold text-slate-900">{credential.name}</h2>
          <p className="mt-1 text-sm text-slate-500">{credential.description}</p>
          {credential.docs && (
            <a
              href={credential.docs}
              target="_blank"
              rel="noreferrer"
              className="mt-1 inline-block text-xs text-blue-700 hover:underline"
            >
              {credential.docs}
            </a>
          )}
        </div>

        <p className="text-xs text-slate-500">{t("cp.message.credential_write_only")}</p>

        {failure && (
          <p className="text-sm rounded-lg bg-amber-50 text-amber-900 border border-amber-200 px-3 py-2">
            {failure}
          </p>
        )}

        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.value")}</span>
          <input
            type="password"
            autoComplete="off"
            value={value}
            onChange={(event) => setValue(event.target.value)}
            required
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 font-mono"
          />
        </label>

        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.reason")}</span>
          <input
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            required
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
          />
        </label>

        {needsCode && (
          <label className="block text-sm">
            <span className="text-slate-600">{t("cp.field.code")}</span>
            <input
              value={code}
              onChange={(event) => setCode(event.target.value)}
              inputMode="numeric"
              className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 font-mono tracking-widest"
            />
          </label>
        )}

        <div className="flex justify-end gap-2">
          <button type="button" onClick={onClose} className="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100">
            {t("cp.action.cancel")}
          </button>
          <button
            type="submit"
            disabled={busy}
            className="rounded-lg bg-[var(--gerege-blue)] px-4 py-2 text-sm font-medium text-[var(--gerege-on-blue)] hover:brightness-105 disabled:opacity-50"
          >
            {t("cp.action.set_credential")}
          </button>
        </div>
      </form>
    </Modal>
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
    <Modal label={setting.key}>
      <form onSubmit={submit} className="p-5 space-y-4">
        <div>
          <h2 className="font-mono text-sm font-semibold text-slate-900">{setting.key}</h2>
          <p className="mt-1 text-sm text-slate-500">{setting.description}</p>
        </div>

        {failure && (
          <p className="text-sm rounded-lg bg-amber-50 text-amber-900 border border-amber-200 px-3 py-2">
            {failure}
          </p>
        )}

        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.value")}</span>
          {setting.kind === "enum" ? (
            <select
              value={value}
              onChange={(event) => setValue(event.target.value)}
              className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
            >
              {(setting.options ?? []).map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </select>
          ) : setting.kind === "bool" ? (
            <select
              value={value}
              onChange={(event) => setValue(event.target.value)}
              className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
            >
              <option value="false">false</option>
              <option value="true">true</option>
            </select>
          ) : (
            <input
              value={value}
              onChange={(event) => setValue(event.target.value)}
              className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 font-mono text-sm"
            />
          )}
          <span className="mt-1 block text-xs text-slate-400">
            {t("cp.field.default")}: <span className="font-mono">{setting.default || "—"}</span>
            {setting.env ? ` · ${setting.env}` : ""}
          </span>
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

        {needsCode && (
          <label className="block text-sm">
            <span className="text-slate-600">{t("cp.field.code")}</span>
            <input
              inputMode="numeric"
              maxLength={6}
              required
              value={code}
              onChange={(event) => setCode(event.target.value.replace(/\D/g, ""))}
              className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 font-mono tracking-[0.4em]"
            />
          </label>
        )}

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
    <Modal label={t("cp.action.new_flag")}>
      <form onSubmit={submit} className="p-5 space-y-3">
        <h2 className="text-lg font-semibold text-slate-900">{t("cp.action.new_flag")}</h2>

        {failure && (
          <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
        )}

        <Line label={t("cp.field.flag")} value={key} onChange={setKey} required mono />
        <Line label={t("cp.field.description")} value={description} onChange={setDescription} />
        <Line label={t("cp.field.owner")} value={owner} onChange={setOwner} />

        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.kind")}</span>
          <select
            value={kind}
            onChange={(event) => setKind(event.target.value as typeof kind)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
          >
            <option value="release">{t("cp.kind.release")}</option>
            <option value="kill_switch">{t("cp.kind.kill_switch")}</option>
            <option value="experiment">{t("cp.kind.experiment")}</option>
          </select>
        </label>

        <Line label={t("cp.field.rollout")} value={rollout} onChange={setRollout} />

        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.expires")}</span>
          <input
            type="date"
            value={expires}
            onChange={(event) => setExpires(event.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
          />
        </label>

        <Line label={t("cp.field.reason")} value={reason} onChange={setReason} required />

        <div className="flex justify-end gap-2 pt-1">
          <button type="button" onClick={onClose} className="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100">
            {t("cp.action.cancel")}
          </button>
          <button
            type="submit"
            disabled={busy}
            className="rounded-lg bg-[var(--gerege-blue)] px-4 py-2 text-sm font-medium text-[var(--gerege-on-blue)] hover:brightness-105 disabled:opacity-60"
          >
            {t("cp.action.create")}
          </button>
        </div>
      </form>
    </Modal>
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
    <label className="block text-sm">
      <span className="text-slate-600">{label}</span>
      <input
        required={required}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className={`mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 ${mono ? "font-mono text-sm" : ""}`}
      />
    </label>
  );
}
