/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

"use client";

import React, { useCallback, useEffect, useState } from "react";
import { Megaphone, Plus, Trash2 } from "lucide-react";

import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Banner, fieldClass } from "@/components/ui";
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
        <h1 className="flex items-center gap-2 text-2xl font-bold text-slate-900">
          <Megaphone className="w-6 h-6 text-[var(--gerege-blue)]" />
          {t("core.view.services_title")}
        </h1>
        <p className="mt-1 text-sm text-slate-500">{t("core.view.services_hint")}</p>
      </header>

      <section className="rounded-2xl border border-slate-200 bg-white overflow-hidden">
      {failed && <div className="p-4"><Banner tone="error" message={failed} /></div>}
      {services.length === 0
        ? <p className="p-6 text-center text-sm italic text-slate-500">{t("core.message.no_services")}</p>
        : <ul className="divide-y">{services.map((one) => (
            <li key={one.id} className="flex items-center justify-between gap-3 p-4">
              <span className="min-w-0"><strong className="block text-sm">{one.title || one.code}</strong><code className="text-xs text-slate-500">{one.code}</code></span>
              {canManage && <button disabled={busy} onClick={() => void withdraw(one.id)} aria-label={t("core.action.withdraw")} className="rounded-lg border border-red-200 p-2 text-red-600 disabled:opacity-50"><Trash2 className="w-4 h-4" /></button>}
            </li>))}
          </ul>}
      {canManage && (
        <div className="flex flex-col gap-2 border-t p-4 sm:flex-row">
          <input value={code} onChange={(e) => setCode(e.target.value)} placeholder={t("core.field.service_code")} className={`${fieldClass} sm:w-56`} />
          <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder={t("core.field.service_title")} className={`${fieldClass} flex-1`} />
          <button disabled={busy || code.trim() === ""} onClick={() => void publish()} className="rounded-lg bg-[var(--gerege-blue)] px-4 py-2 text-sm font-semibold text-[var(--gerege-on-blue)] disabled:opacity-50">
            <Plus className="inline w-4 h-4 mr-1" />{t("core.action.publish")}
          </button>
        </div>
      )}
    </section>
    </div>
  );
}
