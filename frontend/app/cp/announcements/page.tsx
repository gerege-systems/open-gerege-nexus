"use client";

/**
 * Telling everybody something.
 *
 * An announcement appears as a banner above the tenant application's chrome,
 * for everybody or for one organisation, between two dates. It expires by
 * itself — which is the property that makes people willing to write them: a
 * notice that has to be taken down by hand is a notice nobody puts up on a
 * Friday.
 */

import React, { useCallback, useEffect, useState } from "react";
import { Megaphone } from "lucide-react";

import { useAction } from "@/components/cp/Action";
import { Badge, Card, formatMoment, Table } from "@/components/cp/ui";
import { cp, type Announcement } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";
import { Modal } from "@/components/ui";

export default function Announcements() {
  const { t, locale } = useI18n();
  const action = useAction();
  const [announcements, setAnnouncements] = useState<Announcement[]>([]);
  const [writing, setWriting] = useState(false);
  const [failure, setFailure] = useState("");

  const load = useCallback(async () => {
    try {
      setAnnouncements((await cp.announcements()).announcements);
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="space-y-6">
      <div className="flex items-start gap-3">
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-slate-900">{t("cp.section.announcements")}</h1>
        </div>
        <button
          type="button"
          onClick={() => setWriting(true)}
          className="inline-flex items-center gap-2 rounded-lg bg-[var(--gerege-blue)] px-3 py-2 text-sm font-medium text-[var(--gerege-on-blue)] hover:brightness-105"
        >
          <Megaphone className="w-4 h-4" />
          {t("cp.action.announce")}
        </button>
      </div>

      {failure && (
        <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
      )}

      <Card title={t("cp.section.announcements")}>
        <Table
          head={[t("cp.field.title"), t("cp.field.kind"), t("cp.field.organisation"), t("cp.field.until"), ""]}
          rows={announcements.map((announcement) => [
            <span key="t">
              <strong className="text-slate-900">{announcement.title}</strong>
              {announcement.body && <span className="block text-xs text-slate-500">{announcement.body}</span>}
            </span>,
            <Badge key="k" tone={announcement.kind === "maintenance" ? "red" : announcement.kind === "warning" ? "amber" : "slate"}>
              {t(`cp.kind.${announcement.kind}`)}
            </Badge>,
            announcement.tenant_id ?? t("cp.state.everyone"),
            formatMoment(announcement.ends_at, locale) || "—",
            <button
              key="w"
              type="button"
              onClick={() =>
                action.run({
                  title: t("cp.action.withdraw"),
                  detail: announcement.title,
                  perform: (reason) => cp.withdraw(announcement.id, reason),
                  onDone: load,
                })
              }
              className="text-xs rounded-lg border border-slate-300 px-2 py-1 hover:bg-slate-50"
            >
              {t("cp.action.withdraw")}
            </button>,
          ])}
          empty={t("cp.message.no_announcements")}
        />
      </Card>

      {writing && (
        <WriteDialog
          onClose={() => setWriting(false)}
          onPublished={() => {
            setWriting(false);
            void load();
          }}
        />
      )}
      {action.dialog}
    </div>
  );
}

function WriteDialog({ onClose, onPublished }: { onClose: () => void; onPublished: () => void }) {
  const { t } = useI18n();
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [kind, setKind] = useState<"info" | "warning" | "maintenance">("info");
  const [tenantID, setTenantID] = useState("");
  const [endsAt, setEndsAt] = useState("");
  const [reason, setReason] = useState("");
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setFailure("");
    try {
      await cp.announce({
        title,
        body,
        kind,
        // Empty means everybody. The field is an organisation's id rather than
        // a picker because this screen is reached from a list where the id is
        // already in the operator's clipboard.
        tenant_id: tenantID || null,
        ends_at: endsAt ? new Date(endsAt).toISOString() : null,
        reason,
      });
      onPublished();
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal label={t("cp.action.announce")}>
      <form onSubmit={submit} className="p-5 space-y-3">
        <h2 className="text-lg font-semibold text-slate-900">{t("cp.action.announce")}</h2>

        {failure && (
          <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
        )}

        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.title")}</span>
          <input
            required
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
          />
        </label>

        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.body")}</span>
          <textarea
            rows={3}
            value={body}
            onChange={(event) => setBody(event.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
          />
        </label>

        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.kind")}</span>
          <select
            value={kind}
            onChange={(event) => setKind(event.target.value as typeof kind)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
          >
            <option value="info">{t("cp.kind.info")}</option>
            <option value="warning">{t("cp.kind.warning")}</option>
            <option value="maintenance">{t("cp.kind.maintenance")}</option>
          </select>
        </label>

        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.organisation")}</span>
          <input
            value={tenantID}
            onChange={(event) => setTenantID(event.target.value)}
            placeholder={t("cp.state.everyone")}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 font-mono text-xs"
          />
        </label>

        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.until")}</span>
          <input
            type="datetime-local"
            value={endsAt}
            onChange={(event) => setEndsAt(event.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
          />
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

        <div className="flex justify-end gap-2 pt-1">
          <button type="button" onClick={onClose} className="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100">
            {t("cp.action.cancel")}
          </button>
          <button
            type="submit"
            disabled={busy}
            className="rounded-lg bg-[var(--gerege-blue)] px-4 py-2 text-sm font-medium text-[var(--gerege-on-blue)] hover:brightness-105 disabled:opacity-60"
          >
            {t("cp.action.announce")}
          </button>
        </div>
      </form>
    </Modal>
  );
}
