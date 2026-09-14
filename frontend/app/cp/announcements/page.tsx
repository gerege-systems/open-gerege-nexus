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
import {
  Badge,
  Button,
  Card,
  CardHeader,
  Dialog,
  DialogContent,
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
  Textarea,
} from "@gerege-systems/ui";

import { useAction } from "@/components/cp/Action";
import { cp, type Announcement } from "@/lib/cp";
import { formatMoment } from "@/lib/datetime";
import { useI18n } from "@/lib/i18n";

export default function Announcements() {
  const { t } = useI18n();
  const action = useAction();
  const [announcements, setAnnouncements] = useState<Announcement[]>([]);
  const [writing, setWriting] = useState(false);
  const [failure, setFailure] = useState("");
  const [loaded, setLoaded] = useState(false);

  const load = useCallback(async () => {
    try {
      setAnnouncements((await cp.announcements()).announcements);
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

  return (
    <div className="space-y-6">
      <div className="flex items-start gap-3">
        <div className="flex-1">
          <h1 className="text-2xl font-semibold text-foreground">{t("cp.section.announcements")}</h1>
        </div>
        <Button onClick={() => setWriting(true)} leadingIcon={<Megaphone />}>
          {t("cp.action.announce")}
        </Button>
      </div>

      {failure && (
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
      )}

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.announcements")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.title")}</TableHead>
              <TableHead>{t("cp.field.kind")}</TableHead>
              <TableHead>{t("cp.field.organisation")}</TableHead>
              <TableHead>{t("cp.field.until")}</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {announcements.map((announcement) => (
              <TableRow key={announcement.id}>
                <TableCell>
                  <strong className="text-foreground">{announcement.title}</strong>
                  {announcement.body && <span className="block text-xs text-muted">{announcement.body}</span>}
                </TableCell>
                <TableCell>
                  <Badge tone={announcement.kind === "maintenance" ? "danger" : announcement.kind === "warning" ? "warning" : "neutral"}>
                    {t(`cp.kind.${announcement.kind}`)}
                  </Badge>
                </TableCell>
                <TableCell>{announcement.tenant_id ?? t("cp.state.everyone")}</TableCell>
                <TableCell>{formatMoment(announcement.ends_at) || "—"}</TableCell>
                <TableCell>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() =>
                      action.run({
                        title: t("cp.action.withdraw"),
                        detail: announcement.title,
                        perform: (reason) => cp.withdraw(announcement.id, reason),
                        onDone: load,
                      })
                    }
                  >
                    {t("cp.action.withdraw")}
                  </Button>
                </TableCell>
              </TableRow>
            ))}
            {announcements.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="py-8 text-center text-muted">
                  {!loaded ? (
                    <span role="status" className="inline-flex items-center gap-2">
                      <Spinner size="sm" decorative />
                      {t("base.message.loading")}
                    </span>
                  ) : (
                    t("cp.message.no_announcements")
                  )}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
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
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent aria-describedby={undefined} className="max-h-[90dvh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("cp.action.announce")}</DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          {failure && (
            <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
          )}

          <Input label={t("cp.field.title")} required value={title} onChange={(event) => setTitle(event.target.value)} />

          <Textarea label={t("cp.field.body")} rows={3} value={body} onChange={(event) => setBody(event.target.value)} />

          <div className="flex flex-col gap-1.5">
            <label htmlFor="announcement-kind" className="text-sm font-medium text-foreground">
              {t("cp.field.kind")}
            </label>
            <Select value={kind} onValueChange={(value) => setKind(value as typeof kind)}>
              <SelectTrigger id="announcement-kind" />
              <SelectContent>
                <SelectItem value="info">{t("cp.kind.info")}</SelectItem>
                <SelectItem value="warning">{t("cp.kind.warning")}</SelectItem>
                <SelectItem value="maintenance">{t("cp.kind.maintenance")}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <Input
            label={t("cp.field.organisation")}
            value={tenantID}
            onChange={(event) => setTenantID(event.target.value)}
            placeholder={t("cp.state.everyone")}
            className="[&_input]:font-mono [&_input]:text-xs"
          />

          <Input
            type="datetime-local"
            label={t("cp.field.until")}
            value={endsAt}
            onChange={(event) => setEndsAt(event.target.value)}
          />

          <Input label={t("cp.field.reason")} required value={reason} onChange={(event) => setReason(event.target.value)} />

          <div className="flex justify-end gap-2 pt-1">
            <Button type="button" variant="ghost" onClick={onClose}>
              {t("cp.action.cancel")}
            </Button>
            <Button type="submit" loading={busy}>
              {t("cp.action.announce")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
