"use client";

/**
 * Өртөө → Холбоосууд.
 *
 * Two things on one screen, because they are two halves of one subject: the
 * links this organisation has to the installations above and below it, and the
 * request codes work may be raised under. A link with no vocabulary carries
 * nothing, and a vocabulary with no link reaches nobody.
 *
 * The identity card is first and is not decoration. Establishing a link starts
 * with one administrator reading a key fingerprint to another, and the whole
 * signature scheme rests on that being the right key — so it is the first thing
 * on the page rather than something to go looking for.
 *
 * This lived at /settings/urtuu while the channel was the platform's, on the
 * argument that a link an administrator established has to outlive any app
 * being uninstalled. The channel left for client-gerege-nexus with the app it
 * was carrying for — its only caller in three months — so the screen is the
 * app's own, under the app's menu and behind urtuu.manage rather than behind
 * "is an administrator". A deployment without the app installed reaches
 * nothing here, which is the honest answer rather than an empty screen.
 */

import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Check, Copy, KeyRound, Link2, Plus, RefreshCw, Route, X } from "lucide-react";

import { api, type UrtuuCode, type UrtuuPeer } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import {
  Banner,
  LoadingBlock,
  Modal,
  PageHeader,
  cardClass,
  fieldClass,
  rowActionClass,
  tableHeadClass,
} from "@/components/ui";

/** The status and source values are closed sets, so the keys are literals. */
function useLabels() {
  const { t } = useI18n();
  return {
    status: (value: UrtuuPeer["status"]) =>
      value === "active"
        ? t("urtuu.status.active")
        : value === "revoked"
          ? t("urtuu.status.revoked")
          : t("urtuu.status.pending"),
    role: (value: UrtuuPeer["role"]) =>
      value === "parent" ? t("urtuu.role.parent") : t("urtuu.role.child"),
    line: (value: UrtuuCode["line"]) =>
      value === "service" ? t("urtuu.line.service") : t("urtuu.line.assignment"),
    source: (value: UrtuuCode["source"]) =>
      value === "ring"
        ? t("urtuu.source.ring")
        : value === "link"
          ? t("urtuu.source.link")
          : t("urtuu.source.local"),
  };
}

export default function UrtuuSettingsPage() {
  const { t, locale } = useI18n();
  const labels = useLabels();

  const [peers, setPeers] = useState<UrtuuPeer[]>([]);
  const [codes, setCodes] = useState<UrtuuCode[]>([]);
  const [enabled, setEnabled] = useState(true);
  const [ringConfigured, setRingConfigured] = useState(false);
  const [identity, setIdentity] = useState({ installation_id: "", public_key: "" });
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState("");
  const [notice, setNotice] = useState("");

  const [inviting, setInviting] = useState(false);
  const [invitation, setInvitation] = useState("");
  const [joining, setJoining] = useState(false);
  const [authoring, setAuthoring] = useState(false);
  const [openingFor, setOpeningFor] = useState<UrtuuPeer | null>(null);

  const load = useCallback(async () => {
    try {
      const [links, vocabulary] = await Promise.all([api.getUrtuuPeers(), api.getUrtuuCodes()]);
      setPeers(links.peers || []);
      setEnabled(links.enabled);
      setIdentity({ installation_id: links.installation_id, public_key: links.public_key });
      setCodes(vocabulary.codes || []);
      setRingConfigured(vocabulary.ring_configured);
    } catch (err) {
      setFailure(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const act = async (action: () => Promise<unknown>, message: string) => {
    setFailure("");
    try {
      await action();
      setNotice(message);
      await load();
    } catch (err) {
      setFailure(err instanceof Error ? err.message : String(err));
    }
  };

  /** A code's label in the reader's language, falling back the way the server does. */
  const codeName = useCallback(
    (code: UrtuuCode) => code.names?.[locale] || code.names?.mn || code.names?.en || code.code,
    [locale],
  );

  // Links this organisation is the parent on are the only ones a vocabulary can
  // be opened on: a child does not decide what it may be asked to do.
  const childLinks = useMemo(
    () => peers.filter((peer) => peer.role === "parent" && peer.status === "active"),
    [peers],
  );

  if (loading) return <LoadingBlock label={t("base.message.loading")} />;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Route className="w-7 h-7 text-indigo-600" />}
        title={t("urtuu.view.title")}
        subtitle={t("urtuu.view.subtitle")}
      />

      {failure && <Banner tone="error" message={failure} onDismiss={() => setFailure("")} />}
      {notice && <Banner tone="success" message={notice} onDismiss={() => setNotice("")} />}
      {!enabled && <Banner tone="warning" message={t("urtuu.message.disabled")} />}

      {/* Who this installation is, cryptographically. */}
      {enabled && (
        <section className={`${cardClass} p-4`}>
          <h2 className="text-sm font-semibold text-slate-800 flex items-center gap-2 mb-1">
            <KeyRound className="w-4 h-4 text-indigo-500" />
            {t("urtuu.view.identity")}
          </h2>
          <p className="text-xs text-slate-500 mb-3">{t("urtuu.view.identity_hint")}</p>
          <dl className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
            <Fingerprint label={t("urtuu.view.installation_id")} value={identity.installation_id} />
            <Fingerprint label={t("urtuu.view.public_key")} value={identity.public_key} />
          </dl>
        </section>
      )}

      {/* The links. */}
      <section className={`${cardClass} p-4`}>
        <div className="flex flex-wrap items-start justify-between gap-3 mb-1">
          <h2 className="text-sm font-semibold text-slate-800 flex items-center gap-2">
            <Link2 className="w-4 h-4 text-indigo-500" />
            {t("urtuu.section.links")}
          </h2>
          <div className="flex gap-2">
            <button
              disabled={!enabled}
              onClick={() => setInviting(true)}
              className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-40 text-white text-xs font-semibold px-3 py-1.5 rounded-lg flex items-center gap-1"
            >
              <Plus className="w-3.5 h-3.5" />
              {t("urtuu.action.invite")}
            </button>
            <button
              disabled={!enabled}
              onClick={() => setJoining(true)}
              className="bg-white border border-slate-200 hover:bg-slate-50 disabled:opacity-40 text-slate-700 text-xs font-semibold px-3 py-1.5 rounded-lg"
            >
              {t("urtuu.action.join")}
            </button>
          </div>
        </div>
        <p className="text-xs text-slate-500 mb-3">{t("urtuu.hint.links")}</p>

        {peers.length === 0 ? (
          <p className="text-sm text-slate-500">{t("urtuu.message.no_links")}</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className={tableHeadClass}>
                <tr>
                  <th className="px-3 py-2 text-left">{t("urtuu.field.name")}</th>
                  <th className="px-3 py-2 text-left">{t("urtuu.field.role")}</th>
                  <th className="px-3 py-2 text-left">{t("urtuu.field.status")}</th>
                  <th className="px-3 py-2 text-left">{t("urtuu.field.last_seen")}</th>
                  <th className="px-3 py-2" />
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {peers.map((peer) => (
                  <tr key={peer.id} className="hover:bg-slate-50 align-top">
                    <td className="px-3 py-2">
                      <p className="font-semibold text-slate-800">{peer.name || peer.id.slice(0, 8)}</p>
                      {peer.base_url && (
                        <p className="text-[11px] text-slate-500 font-mono">{peer.base_url}</p>
                      )}
                      {/* The health of a link, said where somebody is already
                          looking: an undelivered count and the reason nothing
                          is moving. "Not delivered" with no reason is a
                          support ticket. */}
                      {peer.undelivered > 0 && (
                        <p className="text-[11px] text-amber-600">
                          {t("urtuu.message.undelivered", { count: peer.undelivered })}
                        </p>
                      )}
                      {peer.clock_skew_seconds !== 0 && (
                        <p className="text-[11px] text-slate-500">
                          {t("urtuu.message.clock_skew", { seconds: peer.clock_skew_seconds })}
                        </p>
                      )}
                      {peer.last_error && (
                        <p className="text-[11px] text-rose-600 break-all">{peer.last_error}</p>
                      )}
                    </td>
                    <td className="px-3 py-2 text-slate-600">{labels.role(peer.role)}</td>
                    <td className="px-3 py-2">
                      <StatusPill status={peer.status} label={labels.status(peer.status)} />
                    </td>
                    <td className="px-3 py-2 text-xs text-slate-500">
                      {peer.last_seen_at
                        ? new Date(peer.last_seen_at).toLocaleString()
                        : t("urtuu.message.never")}
                    </td>
                    <td className="px-3 py-2">
                      <div className="flex flex-wrap justify-end gap-2">
                        {peer.role === "parent" && peer.status === "pending" && peer.peer_public_key && (
                          <button
                            className={rowActionClass}
                            onClick={() =>
                              act(() => api.confirmUrtuuPeer(peer.id), t("urtuu.message.confirmed"))
                            }
                          >
                            <Check className="w-3.5 h-3.5" />
                            <span>{t("urtuu.action.confirm")}</span>
                          </button>
                        )}
                        {peer.role === "parent" && peer.status === "active" && (
                          <button className={rowActionClass} onClick={() => setOpeningFor(peer)}>
                            <span>{t("urtuu.action.open_codes")}</span>
                          </button>
                        )}
                        {!peer.revoked_at && (
                          <button
                            className={`${rowActionClass} border-rose-200 text-rose-600 hover:bg-rose-50`}
                            onClick={() => {
                              if (!window.confirm(t("urtuu.message.confirm_revoke", { name: peer.name || peer.id })))
                                return;
                              act(() => api.revokeUrtuuPeer(peer.id), t("urtuu.message.revoked"));
                            }}
                          >
                            <X className="w-3.5 h-3.5" />
                            <span>{t("urtuu.action.revoke")}</span>
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* The vocabulary. */}
      <section className={`${cardClass} p-4`}>
        <div className="flex flex-wrap items-start justify-between gap-3 mb-1">
          <h2 className="text-sm font-semibold text-slate-800">{t("urtuu.section.codes")}</h2>
          <div className="flex gap-2">
            <button
              onClick={() => setAuthoring(true)}
              className="bg-white border border-slate-200 hover:bg-slate-50 text-slate-700 text-xs font-semibold px-3 py-1.5 rounded-lg flex items-center gap-1"
            >
              <Plus className="w-3.5 h-3.5" />
              {t("urtuu.action.create_code")}
            </button>
            <button
              disabled={!ringConfigured}
              title={ringConfigured ? undefined : t("urtuu.message.ring_off")}
              onClick={() =>
                act(async () => {
                  const result = await api.syncUrtuuRing();
                  // Nothing new is an answer, not a failure: the register
                  // publishes rarely and this button is pressed often.
                  setNotice(
                    result.unchanged
                      ? t("urtuu.message.ring_unchanged")
                      : t("urtuu.message.imported", { count: result.imported }),
                  );
                }, t("urtuu.message.imported", { count: 0 }))
              }
              className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-40 text-white text-xs font-semibold px-3 py-1.5 rounded-lg flex items-center gap-1"
            >
              <RefreshCw className="w-3.5 h-3.5" />
              {t("urtuu.action.ring_sync")}
            </button>
          </div>
        </div>
        <p className="text-xs text-slate-500 mb-3">{t("urtuu.hint.codes")}</p>
        {!ringConfigured && (
          <p className="text-xs text-amber-600 mb-3">{t("urtuu.message.ring_off")}</p>
        )}

        {codes.length === 0 ? (
          <p className="text-sm text-slate-500">{t("urtuu.message.no_codes")}</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className={tableHeadClass}>
                <tr>
                  <th className="px-3 py-2 text-left">{t("urtuu.field.code")}</th>
                  <th className="px-3 py-2 text-left">{t("urtuu.field.name")}</th>
                  <th className="px-3 py-2 text-left">{t("urtuu.field.line")}</th>
                  <th className="px-3 py-2 text-left">{t("urtuu.field.source")}</th>
                  <th className="px-3 py-2 text-left">{t("urtuu.field.sla")}</th>
                  <th className="px-3 py-2 text-right">{t("urtuu.field.active")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {codes.map((code) => (
                  <tr key={code.id} className="hover:bg-slate-50">
                    <td className="px-3 py-2 font-mono text-xs text-slate-700">{code.code}</td>
                    <td className="px-3 py-2 text-slate-800">{codeName(code)}</td>
                    <td className="px-3 py-2 text-xs text-slate-600">{labels.line(code.line)}</td>
                    <td className="px-3 py-2 text-xs text-slate-500">
                      {labels.source(code.source)}
                      {code.source_peer_name ? ` · ${code.source_peer_name}` : ""}
                    </td>
                    <td className="px-3 py-2 text-xs text-slate-600">
                      {code.default_sla_seconds
                        ? t("urtuu.field.sla_days", {
                            days: Math.round(code.default_sla_seconds / 86400),
                          })
                        : t("urtuu.field.sla_none")}
                    </td>
                    <td className="px-3 py-2 text-right">
                      {/* Whether this organisation uses a code is its own
                          decision even for one it did not author, so the
                          switch is offered on every row. */}
                      <input
                        type="checkbox"
                        checked={code.active}
                        onChange={(event) =>
                          act(
                            () => api.updateUrtuuCode(code.id, { active: event.target.checked }),
                            t("urtuu.message.code_updated"),
                          )
                        }
                        className="w-4 h-4 accent-indigo-600"
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {inviting && (
        <InviteDialog
          code={invitation}
          onCreate={async (name) => {
            const created = await api.inviteUrtuuPeer(name);
            setInvitation(created.invite_code);
            await load();
          }}
          onClose={() => {
            setInviting(false);
            setInvitation("");
          }}
        />
      )}

      {joining && (
        <JoinDialog
          onJoin={async (input) => {
            await act(() => api.joinUrtuuParent(input), t("urtuu.message.joined"));
            setJoining(false);
          }}
          onClose={() => setJoining(false)}
        />
      )}

      {authoring && (
        <CodeDialog
          onCreate={async (input) => {
            await act(() => api.createUrtuuCode(input), t("urtuu.message.code_created"));
            setAuthoring(false);
          }}
          onClose={() => setAuthoring(false)}
        />
      )}

      {openingFor && (
        <OpenCodesDialog
          peer={openingFor}
          codes={codes.filter((code) => code.active)}
          codeName={codeName}
          onSave={async (selected) => {
            await act(
              () => api.setUrtuuPeerCodes(openingFor.id, selected),
              t("urtuu.message.codes_saved"),
            );
            setOpeningFor(null);
          }}
          onClose={() => setOpeningFor(null)}
        />
      )}

      {/* Nothing to open a vocabulary on yet is worth saying once, quietly. */}
      {enabled && childLinks.length === 0 && codes.length > 0 && (
        <p className="text-xs text-slate-400">{t("urtuu.message.no_links")}</p>
      )}
    </div>
  );
}

function Fingerprint({ label, value }: { label: string; value: string }) {
  const { t } = useI18n();
  return (
    <div>
      <dt className="text-xs uppercase tracking-wide text-slate-400">{label}</dt>
      <dd className="flex items-center gap-2">
        <code className="text-xs font-mono text-slate-700 break-all">{value}</code>
        <button
          type="button"
          title={t("urtuu.action.copy")}
          onClick={() => navigator.clipboard?.writeText(value)}
          className="text-slate-400 hover:text-slate-700 shrink-0"
        >
          <Copy className="w-3.5 h-3.5" />
        </button>
      </dd>
    </div>
  );
}

function StatusPill({ status, label }: { status: UrtuuPeer["status"]; label: string }) {
  const tone =
    status === "active"
      ? "bg-emerald-50 text-emerald-700 border-emerald-200"
      : status === "revoked"
        ? "bg-slate-100 text-slate-500 border-slate-200"
        : "bg-amber-50 text-amber-700 border-amber-200";
  return (
    <span className={`inline-block border rounded-full px-2 py-0.5 text-[11px] font-semibold ${tone}`}>
      {label}
    </span>
  );
}

/**
 * The invitation dialog shows the code once and then stops being a form. It is
 * not stored anywhere, so a person who closes this without copying it revokes
 * the link and invites again — which is the correct cost of a single-use
 * credential.
 */
function InviteDialog({
  code,
  onCreate,
  onClose,
}: {
  code: string;
  onCreate: (name: string) => Promise<void>;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);

  return (
    <Modal label={t("urtuu.modal.invite")}>
      <h3 className="text-base font-semibold text-slate-800 mb-3">{t("urtuu.modal.invite")}</h3>
      {code ? (
        <div className="space-y-3">
          <p className="text-xs text-slate-500">{t("urtuu.message.invite_hint")}</p>
          <div className="flex items-center gap-2">
            <code className="flex-1 bg-slate-50 border border-slate-200 rounded-lg px-3 py-2 font-mono text-sm tracking-widest text-slate-800 break-all">
              {code}
            </code>
            <button
              type="button"
              onClick={() => navigator.clipboard?.writeText(code)}
              className={rowActionClass}
            >
              <Copy className="w-3.5 h-3.5" />
              <span>{t("urtuu.action.copy")}</span>
            </button>
          </div>
          <div className="flex justify-end">
            <button onClick={onClose} className="text-sm font-semibold text-slate-600 px-3 py-1.5">
              {t("base.action.close")}
            </button>
          </div>
        </div>
      ) : (
        <form
          className="space-y-3"
          onSubmit={async (event) => {
            event.preventDefault();
            setBusy(true);
            try {
              await onCreate(name);
            } finally {
              setBusy(false);
            }
          }}
        >
          <label className="block text-xs font-semibold text-slate-600">
            {t("urtuu.field.name")}
            <input
              autoFocus
              value={name}
              onChange={(event) => setName(event.target.value)}
              className={`${fieldClass} mt-1`}
            />
          </label>
          <div className="flex justify-end gap-2">
            <button type="button" onClick={onClose} className="text-sm font-semibold text-slate-600 px-3 py-1.5">
              {t("base.action.cancel")}
            </button>
            <button
              type="submit"
              disabled={busy}
              className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-40 text-white text-sm font-semibold px-4 py-1.5 rounded-lg"
            >
              {t("urtuu.action.invite")}
            </button>
          </div>
        </form>
      )}
    </Modal>
  );
}

function JoinDialog({
  onJoin,
  onClose,
}: {
  onJoin: (input: { invite_code: string; base_url: string; name: string }) => Promise<void>;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const [form, setForm] = useState({ invite_code: "", base_url: "", name: "" });
  const [busy, setBusy] = useState(false);

  return (
    <Modal label={t("urtuu.modal.join")}>
      <h3 className="text-base font-semibold text-slate-800 mb-3">{t("urtuu.modal.join")}</h3>
      <form
        className="space-y-3"
        onSubmit={async (event) => {
          event.preventDefault();
          setBusy(true);
          try {
            await onJoin(form);
          } finally {
            setBusy(false);
          }
        }}
      >
        <label className="block text-xs font-semibold text-slate-600">
          {t("urtuu.field.base_url")}
          <input
            autoFocus
            required
            placeholder="https://nexus.example.mn"
            value={form.base_url}
            onChange={(event) => setForm({ ...form, base_url: event.target.value })}
            className={`${fieldClass} mt-1`}
          />
        </label>
        <label className="block text-xs font-semibold text-slate-600">
          {t("urtuu.field.invite_code")}
          <input
            required
            value={form.invite_code}
            onChange={(event) => setForm({ ...form, invite_code: event.target.value })}
            className={`${fieldClass} mt-1 font-mono tracking-widest`}
          />
        </label>
        <label className="block text-xs font-semibold text-slate-600">
          {t("urtuu.field.name")}
          <input
            value={form.name}
            onChange={(event) => setForm({ ...form, name: event.target.value })}
            className={`${fieldClass} mt-1`}
          />
        </label>
        <div className="flex justify-end gap-2">
          <button type="button" onClick={onClose} className="text-sm font-semibold text-slate-600 px-3 py-1.5">
            {t("base.action.cancel")}
          </button>
          <button
            type="submit"
            disabled={busy}
            className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-40 text-white text-sm font-semibold px-4 py-1.5 rounded-lg"
          >
            {t("urtuu.action.join")}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function CodeDialog({
  onCreate,
  onClose,
}: {
  onCreate: (input: {
    code: string;
    line?: "service" | "assignment";
    names: Record<string, string>;
    schema?: unknown;
    default_sla_seconds?: number | null;
  }) => Promise<void>;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const [form, setForm] = useState({
    code: "local.",
    mn: "",
    en: "",
    days: "",
    schema: "{}",
    line: "assignment" as "service" | "assignment",
  });
  const [busy, setBusy] = useState(false);
  const [schemaError, setSchemaError] = useState("");

  return (
    <Modal label={t("urtuu.modal.code")} size="lg">
      <h3 className="text-base font-semibold text-slate-800 mb-1">{t("urtuu.modal.code")}</h3>
      <p className="text-xs text-slate-500 mb-3">{t("urtuu.message.local_prefix")}</p>
      <form
        className="space-y-3"
        onSubmit={async (event) => {
          event.preventDefault();
          let schema: unknown = {};
          try {
            schema = JSON.parse(form.schema || "{}");
          } catch (err) {
            setSchemaError(err instanceof Error ? err.message : String(err));
            return;
          }
          setSchemaError("");
          setBusy(true);
          try {
            await onCreate({
              code: form.code.trim(),
              line: form.line,
              names: { mn: form.mn.trim(), en: form.en.trim() || form.mn.trim() },
              schema,
              // Empty means the code names no norm, which is a different fact
              // from a norm of zero — so it is sent as null, not as 0.
              default_sla_seconds: form.days ? Number(form.days) * 86400 : null,
            });
          } finally {
            setBusy(false);
          }
        }}
      >
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <label className="block text-xs font-semibold text-slate-600">
            {t("urtuu.field.code")}
            <input
              autoFocus
              required
              value={form.code}
              onChange={(event) => setForm({ ...form, code: event.target.value })}
              className={`${fieldClass} mt-1 font-mono`}
            />
          </label>
          <label className="block text-xs font-semibold text-slate-600">
            {t("urtuu.field.line")}
            <select
              value={form.line}
              onChange={(event) =>
                setForm({ ...form, line: event.target.value as "service" | "assignment" })
              }
              className={`${fieldClass} mt-1`}
            >
              <option value="assignment">{t("urtuu.line.assignment")}</option>
              <option value="service">{t("urtuu.line.service")}</option>
            </select>
          </label>
          <label className="block text-xs font-semibold text-slate-600">
            {t("urtuu.field.sla")}
            <input
              type="number"
              min={0}
              placeholder={t("urtuu.field.sla_none")}
              value={form.days}
              onChange={(event) => setForm({ ...form, days: event.target.value })}
              className={`${fieldClass} mt-1`}
            />
          </label>
          <label className="block text-xs font-semibold text-slate-600">
            {t("urtuu.field.mn_name")}
            <input
              required
              value={form.mn}
              onChange={(event) => setForm({ ...form, mn: event.target.value })}
              className={`${fieldClass} mt-1`}
            />
          </label>
          <label className="block text-xs font-semibold text-slate-600">
            {t("urtuu.field.en_name")}
            <input
              value={form.en}
              onChange={(event) => setForm({ ...form, en: event.target.value })}
              className={`${fieldClass} mt-1`}
            />
          </label>
        </div>
        <label className="block text-xs font-semibold text-slate-600">
          {t("urtuu.field.schema")}
          <textarea
            rows={6}
            value={form.schema}
            onChange={(event) => setForm({ ...form, schema: event.target.value })}
            className={`${fieldClass} mt-1 font-mono text-xs`}
          />
        </label>
        {schemaError && <p className="text-xs text-rose-600">{schemaError}</p>}
        <div className="flex justify-end gap-2">
          <button type="button" onClick={onClose} className="text-sm font-semibold text-slate-600 px-3 py-1.5">
            {t("base.action.cancel")}
          </button>
          <button
            type="submit"
            disabled={busy}
            className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-40 text-white text-sm font-semibold px-4 py-1.5 rounded-lg"
          >
            {t("urtuu.action.save")}
          </button>
        </div>
      </form>
    </Modal>
  );
}

/**
 * Which codes a link may carry. The whole set is submitted, because that is
 * what the announcement downstream is — a snapshot, not a change list.
 */
function OpenCodesDialog({
  peer,
  codes,
  codeName,
  onSave,
  onClose,
}: {
  peer: UrtuuPeer;
  codes: UrtuuCode[];
  codeName: (code: UrtuuCode) => string;
  onSave: (selected: string[]) => Promise<void>;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const [selected, setSelected] = useState<string[]>(() =>
    codes.filter((code) => code.open_to?.includes(peer.id)).map((code) => code.code),
  );
  const [busy, setBusy] = useState(false);

  return (
    <Modal label={t("urtuu.modal.open_codes", { name: peer.name })} size="lg" className="max-h-[80vh] overflow-y-auto">
      <h3 className="text-base font-semibold text-slate-800 mb-3">
        {t("urtuu.modal.open_codes", { name: peer.name || peer.id.slice(0, 8) })}
      </h3>
      {codes.length === 0 ? (
        <p className="text-sm text-slate-500">{t("urtuu.message.no_codes")}</p>
      ) : (
        <ul className="space-y-1 mb-4">
          {codes.map((code) => (
            <li key={code.id}>
              <label className="flex items-center gap-2 text-sm text-slate-700 py-1">
                <input
                  type="checkbox"
                  checked={selected.includes(code.code)}
                  onChange={(event) =>
                    setSelected((current) =>
                      event.target.checked
                        ? [...current, code.code]
                        : current.filter((value) => value !== code.code),
                    )
                  }
                  className="w-4 h-4 accent-indigo-600"
                />
                <span className="font-mono text-xs text-slate-500">{code.code}</span>
                <span>{codeName(code)}</span>
              </label>
            </li>
          ))}
        </ul>
      )}
      <div className="flex justify-end gap-2">
        <button type="button" onClick={onClose} className="text-sm font-semibold text-slate-600 px-3 py-1.5">
          {t("base.action.cancel")}
        </button>
        <button
          type="button"
          disabled={busy}
          onClick={async () => {
            setBusy(true);
            try {
              await onSave(selected);
            } finally {
              setBusy(false);
            }
          }}
          className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-40 text-white text-sm font-semibold px-4 py-1.5 rounded-lg"
        >
          {t("urtuu.action.save")}
        </button>
      </div>
    </Modal>
  );
}
