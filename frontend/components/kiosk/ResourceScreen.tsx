"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import { kioskList, kioskCreate, kioskUpdate, kioskRemove } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import type { KioskResource } from "@/lib/kiosk/types";
import { Plus, Pencil, Trash2, RefreshCw, Inbox } from "lucide-react";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  ConfirmationDialog,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  ErrorState,
  IconButton,
  Input,
  Pagination,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Textarea,
} from "@gerege-systems/ui";
import { Modal } from "@/components/ui";
import { MediaCell, MediaViewer } from "./MediaCell";

const PAGE_SIZES = [25, 50, 100, 200, 500];
const DEFAULT_PAGE_SIZE = 50;

/**
 * The screen every declared kiosk resource renders through: list, create, edit
 * and delete, driven entirely by the resource definition.
 *
 * Errors are shown rather than swallowed. Most kiosk endpoints proxy other
 * Gerege services, so "cannot reach Core" is a common and very different thing
 * from "no records" — an empty table for both would be a lie.
 */
export default function ResourceScreen({ resource }: { resource: KioskResource }) {
  const { t } = useI18n();
  const idKey = resource.idKey ?? "id";
  const base = resource.app ?? "kiosk";

  const [rows, setRows] = useState<Record<string, any>[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
  // False when the endpoint answered with the whole table; the slice is then
  // ours to take rather than the server's.
  const [serverPaged, setServerPaged] = useState(true);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const [editing, setEditing] = useState<Record<string, any> | null>(null);
  const [isNew, setIsNew] = useState(false);
  const [form, setForm] = useState<Record<string, any>>({});
  const [formError, setFormError] = useState("");
  const [confirming, setConfirming] = useState<Record<string, any> | null>(null);
  const [viewing, setViewing] = useState<{ url: string; name?: string } | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      // Sent unconditionally. Every module built on the go_grc paginator
      // defaults to 50 whether or not it is asked, so a screen that stayed
      // silent got a silent truncation — 837 terminals reported and 50 shown,
      // with no way to reach the rest. The endpoints that ignore these
      // parameters are unaffected by them.
      const params: Record<string, string | number> = {
        ...(resource.listParams ?? {}),
        page_size: pageSize,
        page_number: page,
      };
      const res = await kioskList(resource.list, params, base);
      setRows(res.items || []);
      setTotal(res.total || 0);
      setServerPaged(res.serverPaged);
    } catch (err: any) {
      setError(err?.message || t("kiosk.message.load_failed"));
      setRows([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [resource, base, page, pageSize, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const openCreate = () => {
    const blank: Record<string, any> = {};
    for (const f of resource.fields ?? []) {
      if (f.readOnly) continue;
      blank[f.key] = f.type === "boolean" ? false : "";
    }
    setForm(blank);
    setEditing({});
    setIsNew(true);
    setFormError("");
  };

  const openEdit = (row: Record<string, any>) => {
    const filled: Record<string, any> = {};
    for (const f of resource.fields ?? []) {
      const v = row[f.key];
      filled[f.key] = f.type === "json" && v && typeof v === "object" ? JSON.stringify(v, null, 2) : v ?? "";
    }
    filled[idKey] = row[idKey];
    setForm(filled);
    setEditing(row);
    setIsNew(false);
    setFormError("");
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setFormError("");
    try {
      const payload: Record<string, any> = {};
      for (const f of resource.fields ?? []) {
        if (f.readOnly) continue;
        let v = form[f.key];
        if (f.type === "number") v = v === "" || v === null ? undefined : Number(v);
        if (f.type === "json" && typeof v === "string" && v.trim() !== "") {
          // Rejected here rather than sent, so a typo reads as a form error and
          // not as an opaque failure from the API.
          try {
            v = JSON.parse(v);
          } catch {
            throw new Error(`${t(f.label)}: ${t("kiosk.message.invalid_json")}`);
          }
        }
        if (v !== undefined) payload[f.key] = v;
      }
      if (!isNew) payload[idKey] = form[idKey];

      if (isNew) await kioskCreate(resource.create!, payload, base);
      else await kioskUpdate(resource.update!, payload, base);

      setEditing(null);
      await load();
    } catch (err: any) {
      setFormError(err?.message || t("kiosk.message.save_failed"));
    } finally {
      setBusy(false);
    }
  };

  const doRemove = async () => {
    if (!confirming) return;
    setBusy(true);
    try {
      await kioskRemove(resource.remove!, confirming[idKey], idKey, base);
      setConfirming(null);
      await load();
    } catch (err: any) {
      setError(err?.message || t("kiosk.message.delete_failed"));
      setConfirming(null);
    } finally {
      setBusy(false);
    }
  };

  const pages = Math.max(1, Math.ceil(total / pageSize));
  // A server-paged response is already the page. A bare array is the whole
  // table, so the slice happens here.
  const visible = serverPaged ? rows : rows.slice((page - 1) * pageSize, page * pageSize);

  const canWrite = Boolean(resource.create || resource.update || resource.remove);
  const actionCount = useMemo(
    () => (resource.update ? 1 : 0) + (resource.remove ? 1 : 0),
    [resource.update, resource.remove],
  );

  return (
    // The AI Copilot is a fixed button in the bottom-right corner, and the
    // pager is the last thing on the page — scrolled to the end, the two landed
    // on top of each other and the page buttons could not be clicked. The
    // padding keeps the end of the content clear of it.
    <div className="space-y-6 pb-24">
      <header className="flex items-start justify-between border-b border-line pb-4 gap-4">
        <div className="min-w-0">
          <h1 className="text-2xl font-semibold text-foreground">{t(resource.title)}</h1>
          <p className="text-sm text-muted">{t(resource.description)}</p>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <IconButton
            variant="outline"
            onClick={() => void load()}
            aria-label={t("kiosk.action.refresh")}
            icon={<RefreshCw />}
          />
          {resource.create && (
            <Button onClick={openCreate} leadingIcon={<Plus />}>
              {t("kiosk.action.create")}
            </Button>
          )}
        </div>
      </header>

      {!canWrite && resource.readOnlyReason && (
        <Alert variant="info">{t(resource.readOnlyReason)}</Alert>
      )}

      {/* A failed delete leaves the rows that were already on screen, so the
          reason sits above them; a failed load has no rows and says it in
          place of the table. */}
      {error && rows.length > 0 && (
        <Alert variant="danger" live className="wrap-break-word">{error}</Alert>
      )}

      {loading ? (
        <div className="space-y-3 py-4" role="status" aria-live="polite" aria-busy="true">
          <span className="sr-only">{t("kiosk.message.loading")}</span>
          {Array.from({ length: 6 }, (_, row) => (
            <div key={row} className="flex items-center gap-3">
              <Skeleton variant="circle" className="size-4 shrink-0" />
              <Skeleton className="h-4 flex-1" />
              <Skeleton className="h-4 w-24 shrink-0" />
            </div>
          ))}
        </div>
      ) : rows.length === 0 && error ? (
        <ErrorState
          title={t("kiosk.message.load_failed")}
          description={<span className="wrap-break-word">{error}</span>}
          live
          action={
            <Button variant="outline" onClick={() => void load()}>
              {t("kiosk.action.refresh")}
            </Button>
          }
        />
      ) : rows.length === 0 ? (
        <EmptyState icon={<Inbox />} title={t("kiosk.message.empty")} />
      ) : (
        <>
          <Card padding="none" className="overflow-hidden">
            <Table containerClassName="rounded-none border-0" scrollLabel={t(resource.title)}>
              <TableHeader>
                <TableRow>
                  {resource.media && <TableHead className="w-px">{t("kiosk.field.preview")}</TableHead>}
                  {resource.columns.map((c) => (
                    <TableHead key={c.key} className="whitespace-nowrap">{t(c.label)}</TableHead>
                  ))}
                  {actionCount > 0 && <TableHead className="w-px" />}
                </TableRow>
              </TableHeader>
              <TableBody>
                {visible.map((row, i) => (
                  <TableRow key={String(row[idKey] ?? i)}>
                    {resource.media && (
                      <TableCell className="py-2">
                        <MediaCell
                          url={String(row[resource.media.urlKey] ?? "")}
                          name={resource.media.nameKey ? String(row[resource.media.nameKey] ?? "") : undefined}
                          onOpen={() =>
                            setViewing({
                              url: String(row[resource.media!.urlKey] ?? ""),
                              name: resource.media!.nameKey ? String(row[resource.media!.nameKey] ?? "") : undefined,
                            })
                          }
                        />
                      </TableCell>
                    )}
                    {resource.columns.map((c) => (
                      <TableCell key={c.key} className="max-w-xs truncate">
                        {c.render ? c.render(row) : format(row[c.key])}
                      </TableCell>
                    ))}
                    {actionCount > 0 && (
                      <TableCell>
                        <div className="flex items-center gap-1 justify-end">
                          {resource.update && (
                            <IconButton
                              size="sm"
                              variant="ghost"
                              onClick={() => openEdit(row)}
                              aria-label={t("kiosk.action.edit")}
                              icon={<Pencil />}
                            />
                          )}
                          {resource.remove && (
                            <IconButton
                              size="sm"
                              variant="ghost"
                              className="hover:text-danger hover:bg-danger-soft"
                              onClick={() => setConfirming(row)}
                              aria-label={t("kiosk.action.delete")}
                              icon={<Trash2 />}
                            />
                          )}
                        </div>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Card>

          <Pagination
            page={page}
            pageCount={pages}
            totalItems={total}
            pageSize={pageSize}
            pageSizeOptions={PAGE_SIZES}
            onPageChange={setPage}
            onPageSizeChange={(size) => { setPageSize(size); setPage(1); }}
          />
        </>
      )}

      {editing && resource.fields && (
        <Modal onClose={() => setEditing(null)} scrollable>
          <form onSubmit={submit} className="space-y-4">
            <DialogHeader>
              <DialogTitle>
                {isNew ? t("kiosk.action.create") : t("kiosk.action.edit")} — {t(resource.title)}
              </DialogTitle>
            </DialogHeader>
            {formError && <Alert variant="danger" live className="wrap-break-word">{formError}</Alert>}
            {resource.fields.filter((f) => !f.readOnly).map((f) => {
              const label = `${t(f.label)}${f.required ? " *" : ""}`;
              if (f.type === "boolean") {
                return (
                  <Checkbox
                    key={f.key}
                    label={label}
                    checked={Boolean(form[f.key])}
                    onCheckedChange={(checked) => setForm({ ...form, [f.key]: checked === true })}
                  />
                );
              }
              if (f.type === "textarea" || f.type === "json") {
                return (
                  <Textarea
                    key={f.key}
                    label={label}
                    value={form[f.key] ?? ""}
                    onChange={(e) => setForm({ ...form, [f.key]: e.target.value })}
                    required={f.required}
                    rows={f.type === "json" ? 6 : 3}
                    placeholder={f.placeholder}
                    className="font-mono"
                  />
                );
              }
              return (
                <Input
                  key={f.key}
                  label={label}
                  type={f.type === "number" ? "number" : f.type === "date" ? "date" : "text"}
                  value={form[f.key] ?? ""}
                  onChange={(e) => setForm({ ...form, [f.key]: e.target.value })}
                  required={f.required}
                  placeholder={f.placeholder}
                />
              );
            })}
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setEditing(null)}>
                {t("kiosk.action.cancel")}
              </Button>
              <Button type="submit" disabled={busy}>
                {busy ? t("kiosk.message.saving") : t("kiosk.action.save")}
              </Button>
            </DialogFooter>
          </form>
        </Modal>
      )}

      {viewing && <MediaViewer url={viewing.url} name={viewing.name} onClose={() => setViewing(null)} />}

      {confirming && (
        <ConfirmationDialog
          open
          onOpenChange={(open) => { if (!open) setConfirming(null); }}
          title={t("kiosk.action.delete")}
          description={t("kiosk.message.confirm_delete")}
          confirmLabel={t("kiosk.action.delete")}
          cancelLabel={t("kiosk.action.cancel")}
          confirmVariant="destructive"
          loading={busy}
          onConfirm={doRemove}
        />
      )}
    </div>
  );
}

function format(value: unknown): React.ReactNode {
  if (value === null || value === undefined || value === "") return "—";
  if (typeof value === "boolean") return value ? "✓" : "—";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}
