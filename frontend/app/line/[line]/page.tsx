"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { Boxes, FileText, KeyRound, Landmark, LayoutGrid, Link2, MonitorCog, Package, PenTool, Receipt, ScanLine, Settings, ShieldCheck, Users, Wallet } from "lucide-react";
import { invokeShell, useShell, SHELL_METHODS } from "@/lib/shell";
import { api } from "@/lib/api";
import { formatDay } from "@/lib/datetime";
import { LINES, isLine, type LineContent } from "./lines";

/**
 * `lines.ts`-ийн дүрсний нэр → зураг.
 *
 * Энд БАЙХГҮЙ нэр нь алдаа өгдөггүй, зүгээр л хоосон дөрвөлжин зурдаг —
 * `key`, `link`, `settings`, `shield-check`, `monitor-cog` тав нь яг тэр
 * байдлаар дөрвөн шугам дээр хоосон хайрцаг болж сууж байв. Шинэ үйлдэл
 * нэмэхдээ дүрсээ ЭНД мөн нэм.
 */
const ICONS: Record<string, React.ReactNode> = {
  grid: <LayoutGrid />, file: <FileText />, pen: <PenTool />, landmark: <Landmark />,
  users: <Users />, monitor: <MonitorCog />, wallet: <Wallet />, boxes: <Boxes />,
  shield: <ShieldCheck />, package: <Package />, scan: <ScanLine />, receipt: <Receipt />,
  key: <KeyRound />, link: <Link2 />, settings: <Settings />,
  "shield-check": <ShieldCheck />, "monitor-cog": <MonitorCog />,
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

  const actions = (
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
  );

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

      {content.posture === "public" ? (
        <div className="line-body">
          {/*
            Гэрэгэ — энэ аппын нэрийг үүрсэн эд. Монголын эзэнт гүрэн элчдээ
            олгодог байсан төмөр пайз: эзэмшигчийн эрх, хаана хүчинтэйг нь сийлж
            бичсэн байдаг. Олон нийтийн терминалд хүн биш ТӨХӨӨРӨМЖ нь гол
            баримт учир зөвхөн энд үлдэв — ширээ, гарын алган дээр тэр нь
            дөрвөн зураас болж, хоосон чимэг үлдээж байсан.
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
              <p className="paiza-seal">{shell ? `Гэрээ v${shell.version}` : "Олгоогүй"}</p>
            </div>
            <figcaption className="paiza-note">
              {shell
                ? "Энэ төхөөрөмж бүрхүүлээр нэвтэрсэн. Дугаар, байршлыг Тохиргоо → Төхөөрөмж дээрээс бүртгэнэ."
                : "Хөтчөөр нээсэн байна. Гэрэгэ нь зөвхөн native бүрхүүлд олгогдоно."}
            </figcaption>
          </figure>

          {actions}
        </div>
      ) : (
        <>
          <Person person={person} session={session} note={personNote} />
          {actions}
        </>
      )}
    </div>
  );
}

/**
 * Нэвтэрсэн хүн — ширээ ба гарын алган дээрх нүүрийн ГОЛ агуулга.
 *
 * Бүрхүүл нэвтрэлтээ өөрөө эзэмшдэг тул ажлын мужид «намайг хэн гэж уншиж
 * байна» гэдгийг хэлэх газар өөр байхгүй. Тиймээс энэ нь хажуугийн жижиг
 * хайрцаг биш, хуудсыг эзэлсэн нэр: хүн нээсэн даруйдаа зөв данснаас
 * харагдаж байгаагаа уншина.
 *
 * Session байхгүй үед хуудас ЗОГСОНО — 401 нь энэ дэлгэцийн хувьд алдаа биш.
 */
function Person({ person, session, note }: { person: Person | null; session: Session | null; note: string }) {
  if (!person || !session) {
    return (
      <section className="person" aria-label="Нэвтэрсэн хүн">
        <p className="person-eyebrow">Нэвтэрсэн хүн</p>
        <p className="person-empty">{note || "Уншиж байна…"}</p>
      </section>
    );
  }

  return (
    <section className="person" aria-label="Нэвтэрсэн хүн">
      <p className="person-eyebrow">Нэвтэрсэн хүн</p>
      <h2 className="person-name">{person.name}</h2>
      <p className="person-sub">{person.email}</p>

      <dl className="person-facts">
        <div><dt>Ажлын муж</dt><dd>{session.tenant_name}</dd></div>
        <div><dt>Эрх</dt><dd>{person.is_admin ? "Админ" : "Гишүүн"}</dd></div>
        <div><dt>Байгууллага</dt><dd>{person.organisations.length}</dd></div>
        <div><dt>Идэвхтэй session</dt><dd>{person.active_sessions}</dd></div>
        <div><dt>Бүртгүүлсэн</dt><dd>{when(person.created_at)}</dd></div>
      </dl>

      <dl className="person-rows">
        <div><dt>Дугаар</dt><dd>{person.id}</dd></div>
        <div><dt>Мужийн төрөл</dt><dd>{session.workspace_kind || EMPTY}</dd></div>
        <div><dt>Гэрийн муж</dt><dd>{person.home ? `${person.home.name} (${person.home.slug})` : EMPTY}</dd></div>
        <div>
          <dt>Байгууллага</dt>
          <dd>{person.organisations.map((one) => `${one.name} (${one.slug})`).join(", ") || EMPTY}</dd>
        </div>
        <div><dt>Эрх</dt><dd>{session.permissions?.join(", ") || EMPTY}</dd></div>
        {session.impersonated && <div><dt>Нэрийн өмнөөс</dt><dd>тийм</dd></div>}
        {/* Таних тэмдэг бүр нэг мөр: провайдер, түүн дэх дугаар, хэзээ холбогдсон.
            Түүхий claims-ыг ХАССАН — оношлоход хэрэгтэй ч энэ хуудасны ажил биш. */}
        {person.identities.map((one) => (
          <div key={`${one.kind}:${one.issuer ?? ""}:${one.subject}`}>
            <dt>{one.provider}</dt>
            <dd>
              <span>{one.subject}</span>
              <small>{[one.email, `холбосон ${when(one.linked_at)}`, `сүүлд ${when(one.last_seen_at)}`].filter(Boolean).join(" · ")}</small>
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
