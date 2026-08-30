"use client";

import { useEffect, useState } from "react";
import { Inbox, Search, Send } from "lucide-react";

import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Banner, EmptyState, fieldClass, Loading, PageHeader, TableCard, tableHeadClass } from "@/components/ui";

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

  const [round, setRound] = useState(0);
  const reload = () => setRound((n) => n + 1);

  useEffect(() => {
    let alive = true;
    void api
      .getMyItems()
      .then((answer) => alive && setItems(answer.items || []))
      .catch((err: unknown) => alive && setError(err instanceof Error ? err.message : "—"));
    return () => {
      alive = false;
    };
  }, [round]);

  return (
    <main className="p-6 space-y-6">
      <PageHeader
        icon={<Inbox className="w-6 h-6 text-[var(--gerege-blue)]" />}
        title={t("me.view.requests_title")}
        subtitle={t("me.view.requests_subtitle")}
      />

      <AskToJoin onAsked={reload} />

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

/**
 * Асуух талбар.
 *
 * Байгууллагыг **slug-аар** нэрлэнэ — хүнд «манайд ирээрэй» гэж хэлэхэд өгдөг
 * нэр, тэдний үйлчилдэг дэлгэц бүрийн хаяган дотор байдаг үг. Сонгох жагсаалт
 * биш: суулгац дээрх бүх байгууллагын жагсаалт бол **лавлах** бөгөөд иргэн юуг
 * тоолж болох тухай тусдаа шийдвэр. Тэр өдөр ирэхэд энэ талбар сонголт болно.
 */
function AskToJoin({ onAsked }: { onAsked: () => void }) {
  const { t } = useI18n();
  const [slug, setSlug] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState("");
  // Нээлттэй байгууллагад орсон хүний хараа: хүсэлт биш, гишүүнчлэл. Нэрийг
  // нь барьж байгаа нь мессежид хэрэгтэй тул — «нэгдлээ» гэдэг өгүүлбэр
  // хаана нэгдсэнээ хэлэхгүй бол хагас мэдээлэл.
  const [joined, setJoined] = useState<{ id: string; name: string } | null>(null);
  // Хайлт нь slug-ийн **тусламж**, орлуулга биш. Байгууллагаа мэддэг хүн
  // шууд бичээд явна; мэдэхгүй хүн үйлчилгээгээрээ хайж, олсноо талбарт
  // хийнэ. Хоёр зам нэг товч руу нийлнэ.
  const [lookingFor, setLookingFor] = useState("");
  const [found, setFound] = useState<{ slug: string; name: string; code: string; title: string }[] | null>(null);

  async function search(event: React.FormEvent) {
    event.preventDefault();
    setFailed("");
    try {
      setFound((await api.searchDirectory(lookingFor.trim())).providers || []);
    } catch (err: unknown) {
      setFailed(err instanceof Error ? err.message : "—");
    }
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setFailed("");
    try {
      const answer = await api.askToJoin(slug.trim(), message.trim());
      setSlug("");
      setMessage("");
      // Нээлттэй байгууллага энэ товчийг дарсан агшинд шийдчихсэн. Дараалалд
      // орсон гэж хэлэх нь худал байх тул хоёр өөр хариу.
      if (answer.joined) {
        setJoined({ id: answer.workspace_id, name: answer.workspace_name });
      }
      onAsked();
    } catch (err: unknown) {
      setFailed(err instanceof Error ? err.message : "—");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="bg-white rounded-xl border border-slate-200 shadow-sm p-4 space-y-3">
      <div>
        <h2 className="text-sm font-bold text-slate-900">{t("me.view.ask_title")}</h2>
        <p className="text-xs text-slate-500 mt-0.5">{t("me.view.ask_subtitle")}</p>
      </div>

      {/* Шилжих товч нь чимэглэл биш: хүн энэ агшинд гишүүн болсон бөгөөд
          дараагийн зүйл нь тэр байгууллага руугаа орох явдал. Бүрхүүл өөрийн
          мужийн жагсаалтыг ачаалах үедээ уншсан тул хуудсыг дахин ачаалж
          байж шинэ бичлэг харагдана. */}
      {joined && (
        <div className="rounded-lg border border-[var(--gerege-blue)] bg-[var(--gerege-blue-soft)] px-3 py-2.5 flex items-center gap-3">
          <p className="flex-1 text-sm text-[var(--gerege-blue-text)]">
            {t("me.message.joined", { name: joined.name })}
          </p>
          <button
            type="button"
            className="rounded-lg bg-[var(--gerege-blue)] px-3 py-1.5 text-xs font-semibold text-[var(--gerege-on-blue)]"
            onClick={() => {
              void api.switchTenant(joined.id).then(() => window.location.assign("/"));
            }}
          >
            {t("me.action.open_workspace")}
          </button>
        </div>
      )}
      <div className="flex flex-col gap-2 sm:flex-row">
        <input
          required
          value={slug}
          onChange={(e) => setSlug(e.target.value)}
          placeholder={t("me.field.slug_placeholder")}
          className={`${fieldClass} sm:w-64`}
        />
        <input
          value={message}
          maxLength={500}
          onChange={(e) => setMessage(e.target.value)}
          placeholder={t("me.field.message_placeholder")}
          className={`${fieldClass} flex-1`}
        />
        <button
          disabled={busy || slug.trim() === ""}
          className="rounded-lg bg-[var(--gerege-blue)] px-4 py-2 text-sm font-semibold text-[var(--gerege-on-blue)] disabled:opacity-50"
        >
          <Send className="inline w-4 h-4 mr-1" />
          {t("me.action.ask")}
        </button>
      </div>
      {failed && <Banner tone="error" message={failed} />}

      {/* Хэнд хандахаа мэдэхгүй хүнд зориулсан хайлт. Тусдаа form: Enter
          дарахад хайх ёстой болохоос хүсэлт илгээх ёсгүй. */}
      <div className="border-t border-slate-100 pt-3">
        <p className="text-xs text-slate-500 mb-2">{t("me.view.lookup_hint")}</p>
        <div className="flex gap-2">
          <input
            value={lookingFor}
            onChange={(e) => setLookingFor(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && void search(e)}
            placeholder={t("me.field.lookup_placeholder")}
            className={`${fieldClass} flex-1`}
          />
          <button type="button" onClick={(e) => void search(e)} className="rounded-lg border border-slate-200 px-3 py-2 text-sm font-semibold text-slate-600">
            <Search className="inline w-4 h-4 mr-1" />
            {t("me.action.lookup")}
          </button>
        </div>
        {found?.length === 0 && <p className="mt-2 text-xs text-slate-500 italic">{t("me.message.no_providers")}</p>}
        {found && found.length > 0 && (
          <ul className="mt-2 divide-y divide-slate-100 rounded-lg border border-slate-200">
            {found.map((one) => (
              <li key={one.slug + one.code} className="flex items-center justify-between gap-3 px-3 py-2">
                <span className="min-w-0">
                  <strong className="block text-sm truncate">{one.name}</strong>
                  <small className="text-xs text-slate-500">{one.title || one.code}</small>
                </span>
                <button
                  type="button"
                  onClick={() => { setSlug(one.slug); setFound(null); }}
                  className="shrink-0 rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-semibold text-[var(--gerege-blue)]"
                >
                  {t("me.action.choose")}
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </form>
  );
}
