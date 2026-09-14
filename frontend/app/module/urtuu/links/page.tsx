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
import {
  Alert,
  Badge,
  Button,
  Card,
  Checkbox,
  ConfirmationDialog,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  IconButton,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Textarea,
} from "@gerege-systems/ui";

import { api, type UrtuuCode, type UrtuuPeer } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Modal, PageHeader } from "@/components/ui";
import { formatMoment } from "@/lib/datetime";

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
  const [revoking, setRevoking] = useState<UrtuuPeer | null>(null);

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

  if (loading) {
    return (
      <div className="space-y-3 py-4" role="status" aria-live="polite" aria-busy="true">
        <span className="sr-only">{t("base.message.loading")}</span>
        {Array.from({ length: 4 }, (_, row) => (
          <div key={row} className="flex items-center gap-3">
            <Skeleton variant="circle" className="size-4 shrink-0" />
            <Skeleton className="h-4 flex-1" />
            <Skeleton className="h-4 w-24 shrink-0" />
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Route className="w-7 h-7 text-accent" />}
        title={t("urtuu.view.title")}
        subtitle={t("urtuu.view.subtitle")}
      />

      {failure && <Alert variant="danger" live dismissible onDismiss={() => setFailure("")}>{failure}</Alert>}
      {notice && <Alert variant="success" live dismissible onDismiss={() => setNotice("")}>{notice}</Alert>}
      {!enabled && <Alert variant="warning">{t("urtuu.message.disabled")}</Alert>}

      {/* Who this installation is, cryptographically. */}
      {enabled && (
        <Card asChild padding="sm">
          <section>
            <h2 className="text-sm font-semibold text-foreground flex items-center gap-2 mb-1">
              <KeyRound className="w-4 h-4 text-accent" />
              {t("urtuu.view.identity")}
            </h2>
            <p className="text-xs text-muted mb-3">{t("urtuu.view.identity_hint")}</p>
            <dl className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
              <Fingerprint label={t("urtuu.view.installation_id")} value={identity.installation_id} />
              <Fingerprint label={t("urtuu.view.public_key")} value={identity.public_key} />
            </dl>
          </section>
        </Card>
      )}

      {/* The links. */}
      <Card asChild padding="sm">
        <section>
          <div className="flex flex-wrap items-start justify-between gap-3 mb-1">
            <h2 className="text-sm font-semibold text-foreground flex items-center gap-2">
              <Link2 className="w-4 h-4 text-accent" />
              {t("urtuu.section.links")}
            </h2>
            <div className="flex gap-2">
              <Button size="sm" disabled={!enabled} leadingIcon={<Plus />} onClick={() => setInviting(true)}>
                {t("urtuu.action.invite")}
              </Button>
              <Button size="sm" variant="outline" disabled={!enabled} onClick={() => setJoining(true)}>
                {t("urtuu.action.join")}
              </Button>
            </div>
          </div>
          <p className="text-xs text-muted mb-3">{t("urtuu.hint.links")}</p>

          {peers.length === 0 ? (
            <p className="text-sm text-muted">{t("urtuu.message.no_links")}</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("urtuu.field.name")}</TableHead>
                  <TableHead>{t("urtuu.field.role")}</TableHead>
                  <TableHead>{t("urtuu.field.status")}</TableHead>
                  <TableHead>{t("urtuu.field.last_seen")}</TableHead>
                  <TableHead />
                </TableRow>
              </TableHeader>
              <TableBody>
                {peers.map((peer) => (
                  <TableRow key={peer.id}>
                    <TableCell className="align-top">
                      <p className="font-semibold text-foreground">{peer.name || peer.id.slice(0, 8)}</p>
                      {peer.base_url && (
                        <p className="text-xs text-muted font-mono">{peer.base_url}</p>
                      )}
                      {/* The health of a link, said where somebody is already
                          looking: an undelivered count and the reason nothing
                          is moving. "Not delivered" with no reason is a
                          support ticket. */}
                      {peer.undelivered > 0 && (
                        <p className="text-xs text-warning">
                          {t("urtuu.message.undelivered", { count: peer.undelivered })}
                        </p>
                      )}
                      {peer.clock_skew_seconds !== 0 && (
                        <p className="text-xs text-muted">
                          {t("urtuu.message.clock_skew", { seconds: peer.clock_skew_seconds })}
                        </p>
                      )}
                      {peer.last_error && (
                        <p className="text-xs text-danger break-all">{peer.last_error}</p>
                      )}
                    </TableCell>
                    <TableCell className="align-top text-muted">{labels.role(peer.role)}</TableCell>
                    <TableCell className="align-top">
                      <StatusPill status={peer.status} label={labels.status(peer.status)} />
                    </TableCell>
                    <TableCell className="align-top text-xs text-muted">
                      {peer.last_seen_at
                        ? formatMoment(peer.last_seen_at)
                        : t("urtuu.message.never")}
                    </TableCell>
                    <TableCell className="align-top">
                      <div className="flex flex-wrap justify-end gap-2">
                        {peer.role === "parent" && peer.status === "pending" && peer.peer_public_key && (
                          <Button
                            size="sm"
                            variant="outline"
                            leadingIcon={<Check />}
                            onClick={() =>
                              act(() => api.confirmUrtuuPeer(peer.id), t("urtuu.message.confirmed"))
                            }
                          >
                            {t("urtuu.action.confirm")}
                          </Button>
                        )}
                        {peer.role === "parent" && peer.status === "active" && (
                          <Button size="sm" variant="outline" onClick={() => setOpeningFor(peer)}>
                            {t("urtuu.action.open_codes")}
                          </Button>
                        )}
                        {!peer.revoked_at && (
                          <Button
                            size="sm"
                            variant="outline"
                            className="border-danger-border text-danger hover:bg-danger-soft"
                            leadingIcon={<X />}
                            onClick={() => setRevoking(peer)}
                          >
                            {t("urtuu.action.revoke")}
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </section>
      </Card>

      {/* The vocabulary. */}
      <Card asChild padding="sm">
        <section>
          <div className="flex flex-wrap items-start justify-between gap-3 mb-1">
            <h2 className="text-sm font-semibold text-foreground">{t("urtuu.section.codes")}</h2>
            <div className="flex gap-2">
              <Button size="sm" variant="outline" leadingIcon={<Plus />} onClick={() => setAuthoring(true)}>
                {t("urtuu.action.create_code")}
              </Button>
              <Button
                size="sm"
                leadingIcon={<RefreshCw />}
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
              >
                {t("urtuu.action.ring_sync")}
              </Button>
            </div>
          </div>
          <p className="text-xs text-muted mb-3">{t("urtuu.hint.codes")}</p>
          {!ringConfigured && (
            <p className="text-xs text-warning mb-3">{t("urtuu.message.ring_off")}</p>
          )}

          {codes.length === 0 ? (
            <p className="text-sm text-muted">{t("urtuu.message.no_codes")}</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("urtuu.field.code")}</TableHead>
                  <TableHead>{t("urtuu.field.name")}</TableHead>
                  <TableHead>{t("urtuu.field.line")}</TableHead>
                  <TableHead>{t("urtuu.field.source")}</TableHead>
                  <TableHead>{t("urtuu.field.sla")}</TableHead>
                  <TableHead align="right">{t("urtuu.field.active")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {codes.map((code) => (
                  <TableRow key={code.id}>
                    <TableCell className="font-mono text-xs">{code.code}</TableCell>
                    <TableCell>{codeName(code)}</TableCell>
                    <TableCell className="text-xs text-muted">{labels.line(code.line)}</TableCell>
                    <TableCell className="text-xs text-muted">
                      {labels.source(code.source)}
                      {code.source_peer_name ? ` · ${code.source_peer_name}` : ""}
                    </TableCell>
                    <TableCell className="text-xs text-muted">
                      {code.default_sla_seconds
                        ? t("urtuu.field.sla_days", {
                            days: Math.round(code.default_sla_seconds / 86400),
                          })
                        : t("urtuu.field.sla_none")}
                    </TableCell>
                    <TableCell align="right">
                      {/* Whether this organisation uses a code is its own
                          decision even for one it did not author, so the
                          switch is offered on every row. */}
                      <Checkbox
                        className="inline-flex"
                        checked={code.active}
                        label={t("urtuu.field.active")}
                        hideLabel
                        onCheckedChange={(checked) =>
                          act(
                            () => api.updateUrtuuCode(code.id, { active: checked === true }),
                            t("urtuu.message.code_updated"),
                          )
                        }
                      />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </section>
      </Card>

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

      {revoking && (
        <ConfirmationDialog
          open
          onOpenChange={(open) => { if (!open) setRevoking(null); }}
          title={revoking.name || revoking.id}
          description={t("urtuu.message.confirm_revoke", { name: revoking.name || revoking.id })}
          confirmLabel={t("urtuu.action.revoke")}
          cancelLabel={t("base.action.cancel")}
          confirmVariant="destructive"
          onConfirm={() => {
            const peer = revoking;
            setRevoking(null);
            void act(() => api.revokeUrtuuPeer(peer.id), t("urtuu.message.revoked"));
          }}
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
        <p className="text-xs text-muted">{t("urtuu.message.no_links")}</p>
      )}
    </div>
  );
}

function Fingerprint({ label, value }: { label: string; value: string }) {
  const { t } = useI18n();
  return (
    <div>
      <dt className="text-xs uppercase tracking-wide text-muted">{label}</dt>
      <dd className="flex items-center gap-2">
        <code className="text-xs font-mono text-foreground break-all">{value}</code>
        <IconButton
          size="sm"
          variant="ghost"
          className="shrink-0 -m-1"
          title={t("urtuu.action.copy")}
          aria-label={t("urtuu.action.copy")}
          icon={<Copy />}
          onClick={() => navigator.clipboard?.writeText(value)}
        />
      </dd>
    </div>
  );
}

function StatusPill({ status, label }: { status: UrtuuPeer["status"]; label: string }) {
  const tone = status === "active" ? "success" : status === "revoked" ? "neutral" : "warning";
  return (
    <Badge variant="outline" tone={tone} dot>
      {label}
    </Badge>
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
    <Modal onClose={onClose}>
      <DialogHeader className="mb-3">
        <DialogTitle>{t("urtuu.modal.invite")}</DialogTitle>
      </DialogHeader>
      {code ? (
        <div className="space-y-3">
          <p className="text-xs text-muted">{t("urtuu.message.invite_hint")}</p>
          <div className="flex items-center gap-2">
            <code className="flex-1 bg-surface-2 border border-line rounded-lg px-3 py-2 font-mono text-sm tracking-widest text-foreground break-all">
              {code}
            </code>
            <Button size="sm" variant="outline" leadingIcon={<Copy />} onClick={() => navigator.clipboard?.writeText(code)}>
              {t("urtuu.action.copy")}
            </Button>
          </div>
          <div className="flex justify-end">
            <Button variant="ghost" onClick={onClose}>
              {t("base.action.close")}
            </Button>
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
          <Input
            autoFocus
            label={t("urtuu.field.name")}
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={onClose}>
              {t("base.action.cancel")}
            </Button>
            <Button type="submit" disabled={busy}>
              {t("urtuu.action.invite")}
            </Button>
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
    <Modal onClose={onClose}>
      <DialogHeader className="mb-3">
        <DialogTitle>{t("urtuu.modal.join")}</DialogTitle>
      </DialogHeader>
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
        <Input
          autoFocus
          required
          label={t("urtuu.field.base_url")}
          placeholder="https://nexus.example.mn"
          value={form.base_url}
          onChange={(event) => setForm({ ...form, base_url: event.target.value })}
        />
        <Input
          required
          label={t("urtuu.field.invite_code")}
          value={form.invite_code}
          onChange={(event) => setForm({ ...form, invite_code: event.target.value })}
          className="font-mono tracking-widest"
        />
        <Input
          label={t("urtuu.field.name")}
          value={form.name}
          onChange={(event) => setForm({ ...form, name: event.target.value })}
        />
        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("base.action.cancel")}
          </Button>
          <Button type="submit" disabled={busy}>
            {t("urtuu.action.join")}
          </Button>
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
    <Modal onClose={onClose} size="lg">
      <DialogHeader className="mb-3">
        <DialogTitle>{t("urtuu.modal.code")}</DialogTitle>
        <DialogDescription>{t("urtuu.message.local_prefix")}</DialogDescription>
      </DialogHeader>
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
          <Input
            autoFocus
            required
            label={t("urtuu.field.code")}
            value={form.code}
            onChange={(event) => setForm({ ...form, code: event.target.value })}
            className="font-mono"
          />
          <div className="flex flex-col gap-1.5">
            <label htmlFor="urtuu-code-line" className="text-sm font-medium text-foreground">
              {t("urtuu.field.line")}
            </label>
            <Select
              value={form.line}
              onValueChange={(line) => setForm({ ...form, line: line as "service" | "assignment" })}
            >
              <SelectTrigger id="urtuu-code-line" />
              <SelectContent>
                <SelectItem value="assignment">{t("urtuu.line.assignment")}</SelectItem>
                <SelectItem value="service">{t("urtuu.line.service")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <Input
            type="number"
            min={0}
            label={t("urtuu.field.sla")}
            placeholder={t("urtuu.field.sla_none")}
            value={form.days}
            onChange={(event) => setForm({ ...form, days: event.target.value })}
          />
          <Input
            required
            label={t("urtuu.field.mn_name")}
            value={form.mn}
            onChange={(event) => setForm({ ...form, mn: event.target.value })}
          />
          <Input
            label={t("urtuu.field.en_name")}
            value={form.en}
            onChange={(event) => setForm({ ...form, en: event.target.value })}
          />
        </div>
        <Textarea
          label={t("urtuu.field.schema")}
          rows={6}
          value={form.schema}
          onChange={(event) => setForm({ ...form, schema: event.target.value })}
          className="font-mono text-xs"
          error={schemaError || undefined}
        />
        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("base.action.cancel")}
          </Button>
          <Button type="submit" disabled={busy}>
            {t("urtuu.action.save")}
          </Button>
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
    <Modal onClose={onClose} size="lg" className="max-h-[80dvh] overflow-y-auto">
      <DialogHeader className="mb-3">
        <DialogTitle>{t("urtuu.modal.open_codes", { name: peer.name || peer.id.slice(0, 8) })}</DialogTitle>
      </DialogHeader>
      {codes.length === 0 ? (
        <p className="text-sm text-muted">{t("urtuu.message.no_codes")}</p>
      ) : (
        <ul className="space-y-1 mb-4">
          {codes.map((code) => (
            <li key={code.id} className="py-1">
              <Checkbox
                checked={selected.includes(code.code)}
                onCheckedChange={(checked) =>
                  setSelected((current) =>
                    checked === true
                      ? [...current, code.code]
                      : current.filter((value) => value !== code.code),
                  )
                }
                label={
                  <span className="text-sm text-foreground">
                    <span className="font-mono text-xs text-muted me-2">{code.code}</span>
                    {codeName(code)}
                  </span>
                }
              />
            </li>
          ))}
        </ul>
      )}
      <div className="flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={onClose}>
          {t("base.action.cancel")}
        </Button>
        <Button
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
        >
          {t("urtuu.action.save")}
        </Button>
      </div>
    </Modal>
  );
}
