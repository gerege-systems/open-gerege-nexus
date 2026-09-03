"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { Boxes, FileText, Landmark, LayoutGrid, MonitorCog, Package, PenTool, Receipt, ScanLine, ShieldCheck, Users, Wallet } from "lucide-react";
import { invokeShell, useShell, SHELL_METHODS } from "@/lib/shell";
import { api } from "@/lib/api";
import { formatDay } from "@/lib/datetime";
import { LINES, isLine, type LineContent } from "./lines";

const ICONS: Record<string, React.ReactNode> = {
  grid: <LayoutGrid />, file: <FileText />, pen: <PenTool />, landmark: <Landmark />,
  users: <Users />, monitor: <MonitorCog />, wallet: <Wallet />, boxes: <Boxes />,
  shield: <ShieldCheck />, package: <Package />, scan: <ScanLine />, receipt: <Receipt />,
};

/** `device.identity`-ийн хариу. Талбар бүр байхгүй байж болно. */
interface DeviceIdentity {
  id?: string;
  name?: string;
  site?: string;
  form_factor?: string;
}

const EMPTY = "—";

/**
 * Нэвтэрсэн хүний бүртгэл — хоёр эх сурвалж, хоёулаа session-ээр өөрөө
 * шийдэгддэг тул энэ хуудас хэний ч мэдээллийг заагаад асууж чадахгүй.
 * `/profile` нь хүн өөрөө (таних тэмдэг, гишүүнчлэл), `/auth/me` нь тухайн
 * агшны муж ба эрх. Хэлбэрийг нь клиентээс нь өөрөөс нь авч байгаа учир
 * сервер талбар нэмэхэд энд гараар дагах зүйл байхгүй.
 */
type Person = Awaited<ReturnType<typeof api.profile>>;
type Session = Awaited<ReturnType<typeof api.getMe>>;

function when(iso?: string) {
  if (!iso) return EMPTY;
  const at = new Date(iso);
  return Number.isNaN(at.getTime()) ? EMPTY : formatDay(at);
}

function Field({ label, value, wide }: { label: string; value?: React.ReactNode; wide?: boolean }) {
  return (
    <div className={`min-w-0 ${wide ? "sm:col-span-2 lg:col-span-3" : ""}`}>
      <dt className="text-[0.625rem] uppercase tracking-[0.12em] text-muted">{label}</dt>
      <dd className="mt-0.5 break-words font-mono text-[0.8125rem] text-foreground">{value || EMPTY}</dd>
    </div>
  );
}

/**
 * Шугамын нүүр дэлгэц.
 *
 * Энэ бол маркетингийн хуудас БИШ. Энэ нь native хүрээний дотор, ribbon болон
 * rail-ын дунд рендерлэгддэг тул өөрийн толгой хэсэг, навигаци зурахгүй —
 * тэдгээрийг бүрхүүл эзэмшинэ.
 *
 * Хуудас нь нэвтрэлт шаардахгүй: ажлын мужид web-ийн нэвтрэх дэлгэц гарч
 * ирэхийг орлохын тулд байгаа юм. Session байхгүй бол гэрэгэ нь "олгоогүй"
 * гэж уншигдана, харин дэлгэц өөрөө хэвээр зогсоно.
 */
export default function LineHomePage() {
  const params = useParams<{ line: string }>();
  const { shell } = useShell();
  const [identity, setIdentity] = useState<DeviceIdentity | null>(null);
  const [host, setHost] = useState("");
  const [person, setPerson] = useState<Person | null>(null);
  const [session, setSession] = useState<Session | null>(null);
  const [personNote, setPersonNote] = useState("");

  // Шугам нь доорх дэлгэцийн салаалалтад хэрэгтэй тул hook-уудаас ДЭЭР
  // тодорхойлогдоно: танихгүй шугамын эрт буцалт hook-ийн дараа байна.
  const line = isLine(params.line) ? params.line : null;
  const posture = line ? LINES[line].posture : null;

  useEffect(() => setHost(window.location.host), []);

  /**
   * Хэн нэвтэрсэн бэ.
   *
   * Олон нийтийн терминал дээр УНШИХГҮЙ: kiosk, POS-ын дэлгэцийг дараагийн
   * дугаарлаж байгаа хүн хардаг тул ээлжийн ажилтны и-мэйл, регистр тэнд
   * гарах ёсгүй. Ширээ ба гарын алга хоёр нь тухайн хүний өөрийнх нь дэлгэц.
   *
   * 401 бол алдаа биш: бүрхүүл нэвтрэхээс өмнө энэ хуудас зогсох ёстой тул
   * түүнийг «нэвтрээгүй» гэж уншина.
   */
  useEffect(() => {
    if (!posture || posture === "public") return;
    let cancelled = false;
    void Promise.all([api.profile(), api.getMe()])
      .then(([profile, me]) => {
        if (cancelled) return;
        setPerson(profile);
        setSession(me);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        const status = (err as { status?: number })?.status;
        setPersonNote(
          status === 401
            ? "Нэвтрээгүй байна. Бүрхүүлээр нэвтрэхэд хүний бүртгэл энд гарч ирнэ."
            : err instanceof Error ? err.message : "—",
        );
      });
    return () => { cancelled = true; };
  }, [posture]);
  useEffect(() => {
    if (!shell) return;
    let cancelled = false;
    void invokeShell<DeviceIdentity>(SHELL_METHODS.DEVICE_IDENTITY, {}).then((result) => {
      // Бүрхүүл `device.identity`-г зарлаагүй бол reject ирнэ — тэр үед гэрэгэ
      // нь бүрхүүлээс мэдэх зүйлээрээ (платформ, хэлбэр, гэрээ) уншигдана.
      if (!cancelled && result.ok) setIdentity(result.value);
    });
    return () => { cancelled = true; };
  }, [shell]);

  if (!line) {
    return <div className="line-home line-home--unknown">
      <p className="line-eyebrow">ГЭРЭГЭ</p>
      <h1 className="line-title">Энэ шугам бүртгэлгүй</h1>
      <p className="line-lede">
        Хаяг нь <code>{params.line}</code> гэсэн шугамыг заасан байна. Бүртгэлтэй шугамууд
        <code> native-apps/shared/device_lines.json</code> дотор байна.
      </p>
    </div>;
  }

  const content: LineContent = LINES[line];
  const formFactor = identity?.form_factor || shell?.formFactor || EMPTY;

  return (
    <div
      className={`line-home line-home--${content.posture}`}
      style={{ ["--line-alloy" as string]: content.alloy, ["--line-alloy-rgb" as string]: content.alloyRGB }}
    >
      <header className="line-mast">
        <p className="line-eyebrow">{content.eyebrow}</p>
        <h1 className="line-title">{content.title}</h1>
        <p className="line-lede">{content.lede}</p>
      </header>

      <div className="line-body">
        {/*
          Гэрэгэ — энэ аппын нэрийг үүрсэн эд. Монголын эзэнт гүрэн элчдээ
          олгодог байсан төмөр пайз: эзэмшигчийн эрх, хаана хүчинтэйг нь сийлж
          бичсэн байдаг. Төхөөрөмжийн бүртгэл яг үүнтэй ижил зүйл хийдэг тул
          энд түүнийг чимэг биш, төхөөрөмжийн үнэмлэх болгож харууллаа.
        */}
        <figure className="paiza" aria-label="Энэ төхөөрөмжид олгосон гэрэгэ">
          <span className="paiza-cord" aria-hidden="true" />
          <div className="paiza-face">
            <p className="paiza-mark">ГЭРЭГЭ</p>
            <dl className="paiza-inscription">
              <div><dt>Шугам</dt><dd>{host || EMPTY}</dd></div>
              <div><dt>Хэлбэр</dt><dd>{formFactor}</dd></div>
              <div><dt>Байршил</dt><dd>{identity?.site || EMPTY}</dd></div>
              <div><dt>Дугаар</dt><dd>{identity?.id || EMPTY}</dd></div>
            </dl>
            <p className="paiza-seal">
              {shell ? `Гэрээ v${shell.version}` : "Олгоогүй"}
            </p>
          </div>
          <figcaption className="paiza-note">
            {shell
              ? "Энэ төхөөрөмж бүрхүүлээр нэвтэрсэн. Дугаар, байршлыг Тохиргоо → Төхөөрөмж дээрээс бүртгэнэ."
              : "Хөтчөөр нээсэн байна. Гэрэгэ нь зөвхөн native бүрхүүлд олгогдоно."}
          </figcaption>
        </figure>

        <nav className="line-actions" aria-label="Хаанаас эхлэх">
          <p className="line-actions-head">Хаанаас эхлэх</p>
          <ul>
            {content.actions.map((action) => (
              <li key={action.label + action.href}>
                <Link href={action.href} className="line-action">
                  <span className="line-action-icon" aria-hidden="true">{ICONS[action.icon]}</span>
                  <span className="line-action-text">
                    <strong>{action.label}</strong>
                    <small>{action.hint}</small>
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </nav>
      </div>

      {/*
        Нэвтэрсэн хүн. Бүрхүүл нэвтрэлтээ өөрөө эзэмшдэг тул ажлын муж хэнийг
        авчирсныг харуулахгүй бол native талд амжилттай нэвтэрсэн эсэхийг web
        талаас нотлох арга байхгүй байсан — эхний дэлгэц нь яг тэр асуултад
        хариулна.
      */}
      {posture !== "public" && (
        <section className="mt-10 rounded-xl border border-line bg-surface p-5">
          <h2 className="text-sm font-semibold text-foreground">Нэвтэрсэн хүн</h2>
          <p className="mt-0.5 text-xs text-muted">
            Session-ээр уншсан бүртгэл — <code>/profile</code>, <code>/auth/me</code>.
          </p>

          {personNote && <p className="mt-3 text-sm text-muted">{personNote}</p>}

          {person && session && (
            <dl className="mt-4 grid gap-x-6 gap-y-3 sm:grid-cols-2 lg:grid-cols-3">
              <Field label="Нэр" value={person.name} />
              <Field label="И-мэйл" value={person.email} />
              <Field label="Дугаар" value={person.id} />
              <Field label="Бүртгүүлсэн" value={when(person.created_at)} />
              <Field label="Админ" value={person.is_admin ? "тийм" : "үгүй"} />
              <Field label="Идэвхтэй session" value={String(person.active_sessions)} />
              <Field label="Ажлын муж" value={`${session.tenant_name} (${session.tenant_id})`} />
              <Field label="Мужийн төрөл" value={session.workspace_kind} />
              <Field label="Нэрийн өмнөөс" value={session.impersonated ? "тийм" : "үгүй"} />
              <Field label="Гэрийн муж" value={person.home ? `${person.home.name} (${person.home.slug})` : EMPTY} />
              <Field
                label="Байгууллага"
                value={person.organisations.map((one) => `${one.name} (${one.slug})`).join(", ")}
                wide
              />
              <Field label="Эрх" value={session.permissions?.join(", ")} wide />
            </dl>
          )}

          {/* Таних тэмдэг бүрийг бүтнээр нь: claims нь провайдер юу баталсныг
              хэлдэг бөгөөд нэвтрэлт оношлоход хамгийн эхэнд хэрэгтэй хэсэг. */}
          {person && person.identities.length > 0 && (
            <ul className="mt-5 space-y-2">
              {person.identities.map((one) => (
                <li key={`${one.kind}:${one.issuer ?? ""}:${one.subject}`} className="rounded-lg border border-line p-3">
                  <p className="text-sm font-semibold text-foreground">
                    {one.provider} <span className="text-xs font-normal text-muted">({one.kind})</span>
                  </p>
                  <dl className="mt-2 grid gap-x-6 gap-y-2 sm:grid-cols-2 lg:grid-cols-3">
                    <Field label="Subject" value={one.subject} />
                    <Field label="И-мэйл" value={one.email} />
                    <Field label="Нэр" value={[one.name, one.surname].filter(Boolean).join(" ")} />
                    <Field label="Issuer" value={one.issuer} />
                    <Field label="Холбосон" value={when(one.linked_at)} />
                    <Field label="Сүүлд харагдсан" value={when(one.last_seen_at)} />
                  </dl>
                  {one.claims && (
                    <pre className="mt-2 overflow-x-auto rounded-lg bg-surface-2 p-2 text-[0.6875rem] text-muted">
                      {JSON.stringify(one.claims, null, 2)}
                    </pre>
                  )}
                </li>
              ))}
            </ul>
          )}
        </section>
      )}
    </div>
  );
}
