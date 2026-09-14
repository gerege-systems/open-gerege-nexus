/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

"use client";

import React, { useCallback, useEffect, useState } from "react";
import { Megaphone, Plus, Trash2 } from "lucide-react";
import { Alert, Button, Card, EmptyState, IconButton, Input } from "@gerege-systems/ui";

import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { useAccess } from "@/lib/permissions";

/**
 * Юуг олон нийтэд зарлаж байгаа.
 *
 * Хүсэлт хүлээж авах нь **дотоод** шийдвэр — байгууллага өөрийн дараалалдаа
 * ямар төрлийн ажил оруулахаа тохируулж байна. Энд гарах нь **гадаад** амлалт:
 * танихгүй хүн лавлахаас олоод хандаж болно. Хоёрыг нэг үйлдэл болговол
 * байгууллага дотоод урсгалаа тохируулаад олон нийтийн үйлчилгээ санамсаргүй
 * зарласан байна — тиймээс тусдаа.
 */
export default function PublishedServices() {
  const { t } = useI18n();
  const { isAdmin: canManage } = useAccess();
  const [services, setServices] = useState<{ id: string; code: string; title: string }[]>([]);
  const [code, setCode] = useState("");
  const [title, setTitle] = useState("");
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState("");

  const load = useCallback(async () => {
    try {
      setServices((await api.getPublishedServices()).services || []);
    } catch {
      // Хоосон нь ердийн байдал. Энэ хэсгийн алдаа дээрх хуудсыг унагаах ёсгүй.
      setServices([]);
    }
  }, []);
  useEffect(() => { void load(); }, [load]);

  async function publish() {
    setBusy(true);
    setFailed("");
    try {
      await api.publishService(code.trim(), title.trim());
      setCode("");
      setTitle("");
      await load();
    } catch (err: unknown) {
      setFailed(err instanceof Error ? err.message : "—");
    } finally {
      setBusy(false);
    }
  }

  async function withdraw(id: string) {
    setBusy(true);
    try {
      await api.withdrawService(id);
      await load();
    } catch (err: unknown) {
      setFailed(err instanceof Error ? err.message : "—");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="w-full space-y-6">
      <header>
        <h1 className="flex items-center gap-2 text-2xl font-semibold text-foreground">
          <Megaphone className="w-6 h-6 text-accent" aria-hidden="true" />
          {t("core.view.services_title")}
        </h1>
        <p className="mt-1 text-sm text-muted">{t("core.view.services_hint")}</p>
      </header>

      <Card asChild padding="none" className="overflow-hidden">
        <section>
          {failed && <div className="p-4"><Alert variant="danger" live>{failed}</Alert></div>}
          {services.length === 0
            ? <EmptyState icon={<Megaphone className="size-6" />} title={t("core.message.no_services")} className="py-8" />
            : <ul className="divide-y divide-line">{services.map((one) => (
                <li key={one.id} className="flex items-center justify-between gap-3 p-4">
                  <span className="min-w-0"><strong className="block text-sm">{one.title || one.code}</strong><code className="text-xs text-muted">{one.code}</code></span>
                  {canManage && (
                    <IconButton
                      variant="outline"
                      className="border-danger-border text-danger hover:bg-danger-soft"
                      disabled={busy}
                      onClick={() => void withdraw(one.id)}
                      aria-label={t("core.action.withdraw")}
                      icon={<Trash2 />}
                    />
                  )}
                </li>))}
              </ul>}
          {canManage && (
            <div className="flex flex-col gap-2 border-t border-line p-4 sm:flex-row sm:items-center">
              <Input
                label={t("core.field.service_code")}
                hideLabel
                value={code}
                onChange={(e) => setCode(e.target.value)}
                placeholder={t("core.field.service_code")}
                className="sm:w-56"
              />
              <Input
                label={t("core.field.service_title")}
                hideLabel
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder={t("core.field.service_title")}
                className="flex-1"
              />
              <Button type="button" disabled={code.trim() === ""} loading={busy} leadingIcon={<Plus />} onClick={() => void publish()}>
                {t("core.action.publish")}
              </Button>
            </div>
          )}
        </section>
      </Card>
    </div>
  );
}
