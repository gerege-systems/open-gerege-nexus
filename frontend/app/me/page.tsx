"use client";

import { useEffect, useState } from "react";
import { Inbox } from "lucide-react";

import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Banner, EmptyState, Loading, PageHeader, TableCard, tableHeadClass } from "@/components/ui";

/**
 * Юу гуйсан, хаана явна.
 *
 * Иргэн нийлүүлэгч байгууллагуудад хүсэлт гаргадаг ч тэдгээрийн мөр нь тухайн
 * байгууллагын мужид, тэдний мөрийн түвшний бодлогын ард байдаг — тэр нь
 * бодлого ажиллаж байгаагийн шинж. Тиймээс энэ дэлгэц зуун байгууллагыг
 * дамжин уншдаггүй. Нийлүүлэгч төлөв өөрчлөгдөх бүрд хүний **өөрийнх нь
 * мужид** проекц бичдэг бөгөөд энэ нь тэр мужийг уншиж байна.
 *
 * Иймд эндээс өөр байгууллагын юу ч харагдахгүй: `/api/v1/me/items` нь
 * `workspace.person_items`-ийг ямар ч tenant шүүлтгүйгээр уншдаг, учир нь
 * шүүлтийг RLS хийдэг. Дэлгэц нь тусгай эрхтэй биш — зүгээр л өөрийн мужаа
 * харж байгаа хүн.
 *
 * Гэрийн бүрхүүл тусдаа биш. Хүний бусад дэлгэц — профайл, төхөөрөмж,
 * харагдац — бүгд энэ бүрхүүлд байдаг тул хоёр дахь бүрхүүл эхний товшилт
 * дээрээ задарна. Оронд нь бүрхүүл өөрөө мужийн төрлөөр шийддэг:
 * lib/workspaceKind.mjs.
 */

type Item = {
  id: string;
  source_app: string;
  source_ref: string;
  provider: string;
  code: string;
  status: string;
  answer: string;
  opened_at: string;
  updated_at: string;
};

function when(iso: string) {
  const at = new Date(iso);
  return Number.isNaN(at.getTime()) ? "—" : at.toLocaleDateString();
}

export default function MyRequestsPage() {
  const { t } = useI18n();
  const [items, setItems] = useState<Item[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let alive = true;
    void api
      .getMyItems()
      .then((answer) => alive && setItems(answer.items || []))
      .catch((err: unknown) => alive && setError(err instanceof Error ? err.message : "—"));
    return () => {
      alive = false;
    };
  }, []);

  return (
    <main className="p-6 space-y-6">
      <PageHeader
        icon={<Inbox className="w-6 h-6 text-[var(--gerege-blue)]" />}
        title={t("me.view.requests_title")}
        subtitle={t("me.view.requests_subtitle")}
      />

      {error && <Banner tone="error" message={error} />}
      {!items && !error && <Loading />}

      {items && items.length === 0 && (
        // Хоосон байх нь энэ дэлгэцийн ердийн байдал, алдаа биш: хүсэлт
        // гаргаагүй хүн хоосон жагсаалттай байх ёстой бөгөөд суулгац
        // нийтэлдэг модульгүй бол бас хоосон. Хоёрын аль нь ч гэдгийг энэ
        // дэлгэц ялгаж мэдэхгүй тул амласан зүйлээ болиулж хэлэхгүй.
        <div className="bg-white rounded-xl border border-slate-200 shadow-sm">
          <EmptyState message={t("me.message.no_requests")} />
        </div>
      )}

      {items && items.length > 0 && (
        <TableCard
          head={
            <tr className={tableHeadClass}>
              <th className="px-4 py-2">{t("me.field.code")}</th>
              <th className="px-4 py-2">{t("me.field.provider")}</th>
              <th className="px-4 py-2">{t("me.field.status")}</th>
              <th className="px-4 py-2">{t("me.field.answer")}</th>
              <th className="px-4 py-2">{t("me.field.updated")}</th>
            </tr>
          }
        >
          {items.map((item) => (
            <tr key={item.id} className="hover:bg-[var(--gerege-surface-2)]">
              <td className="px-4 py-2 font-medium text-slate-900">{item.code}</td>
              {/* Хоосон байж болно: хэн ч аваагүй хүсэлт хаана ч заахгүй. */}
              <td className="px-4 py-2">{item.provider || "—"}</td>
              <td className="px-4 py-2">{item.status}</td>
              <td className="px-4 py-2 max-w-md truncate" title={item.answer}>
                {item.answer || "—"}
              </td>
              <td className="px-4 py-2 whitespace-nowrap">{when(item.updated_at)}</td>
            </tr>
          ))}
        </TableCard>
      )}
    </main>
  );
}
