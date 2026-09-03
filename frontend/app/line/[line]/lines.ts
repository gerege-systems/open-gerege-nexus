import type { ShellLine } from "@/lib/shell";

/**
 * Шугам бүрийн нүүр дэлгэцийн агуулга.
 *
 * Шугамууд өнгө, гарчгаараа биш **байрлалаараа** ялгаатай: хүн тухайн
 * төхөөрөмжтэй бие махбодоор хэрхэн харьцдаг вэ гэдэг нь дэлгэцийн нягтрал,
 * товчны хэмжээ, юуг эхэнд тавихыг шийднэ. Ширээн дээрх Mac ба зогсож байгаа
 * хүний хүрдэг kiosk хоёр ижил дэлгэц байх ёсгүй.
 *
 * Яг тэр шалтгаанаар Mac ба Windows хоёр НЭГ дэлгэц: хүн тэр хоёрын аль
 * дээр нь ч сандал дээр суугаад, гар талдаа байлгаж ажилладаг. Тэдгээрийг
 * тусад нь бичих гэсэн оролдлого нь ижил дэлгэцийг өөр үгээр хоёр удаа
 * бичихэд хүргэсэн. Платформ ялгах шаардлага үнэхээр гарвал бүрхүүл
 * `window.GeregeShell.platform`-оор өөрийгөө хэлдэг — дэлгэц дотроос
 * салаалж болно.
 */
export type LinePosture = "desk" | "hand" | "public";

export interface LineAction {
  label: string;
  hint: string;
  href: string;
  /** lucide-ийн дүрсний нэр. `page.tsx` доторх зурагт харгалзана. */
  icon: string;
}

export interface LineContent {
  /** Тамганы дээрх мөр — шугамыг нэрлэнэ. */
  eyebrow: string;
  title: string;
  lede: string;
  posture: LinePosture;
  /**
   * Тухайн шугамын хайлш. Гэрэгэ нь зэрэг тус бүрдээ өөр металлаар цутгагдаж
   * байсан — хараад л алийг нь барьж байгаагаа мэднэ. Энд ч мөн адил: парк
   * удирддаг хүн зургаан дэлгэцийг өнгөөр нь ялгана.
   *
   * Гэрэлтэй ба харанхуй хоёуланд уншигдахын тулд бүгд дунд өнгөтэй.
   */
  alloy: string;
  alloyRGB: string;
  actions: LineAction[];
}

export const LINE_ORDER: ShellLine[] = ["desktop", "mobile", "kiosk", "pos"];

export const LINES: Record<ShellLine, LineContent> = {
  desktop: {
    eyebrow: "ГЭРЭГЭ · DESKTOP ШУГАМ",
    title: "Ажлын ширээ",
    lede: "Энд desktop аппын web хэсгийг хөгжүүлнэ. Урт ээлж, гар талдаа: байгууллагын өдөр тутмын ажил — модуль, тохиргоо, нөөц нэг хүрээн дотор, цонх нэмэгдэхгүй.",
    posture: "desk",
    alloy: "#6B7A99",
    alloyRGB: "107 122 153",
    actions: [
      { label: "Апп дэлгүүр", hint: "Модуль асаах, унтраах", href: "/apps", icon: "grid" },
      { label: "SSO клиентүүд", hint: "OAuth2 клиент бүртгэл", href: "/sso-clients", icon: "key" },
      { label: "Холбогч", hint: "Интеграц тохиргоо", href: "/module/integrations/connectors", icon: "link" },
      { label: "Хандах эрх", hint: "Хэрэглэгч, үүрэг", href: "/settings/access", icon: "shield" },
      { label: "Төхөөрөмжийн парк", hint: "Бүртгэсэн төхөөрөмжүүд", href: "/settings/devices", icon: "monitor" },
    ],
  },
  mobile: {
    eyebrow: "ГЭРЭГЭ · MOBILE ШУГАМ",
    title: "Гарын алганд",
    lede: "Хөдөлгөөнд байхад хэрэгтэй нь: зөвшөөрөх, хянах, уншуулаад бүртгэх. Гар оролт багатай.",
    posture: "hand",
    alloy: "#0E9AA7",
    alloyRGB: "14 154 167",
    actions: [
      { label: "Апп дэлгүүр", hint: "Модуль асаах, унтраах", href: "/apps", icon: "grid" },
      { label: "Профайл", hint: "Хэл, төхөөрөмж, нэвтрэлт", href: "/profile", icon: "settings" },
      { label: "Хандах эрх", hint: "Хэрэглэгч, үүрэг", href: "/settings/access", icon: "shield" },
      { label: "Төхөөрөмж", hint: "Бүртгэсэн төхөөрөмжүүд", href: "/settings/devices", icon: "monitor" },
    ],
  },
  kiosk: {
    eyebrow: "ГЭРЭГЭ · KIOSK ШУГАМ",
    title: "Өөртөө үйлчлэх цэг",
    lede: "Иргэн өөрөө eID-ээрээ баталгаажаад үйлчилгээгээ авна. Ээлж дуусахад дэлгэц өөрөө цэвэрлэгдэнэ.",
    posture: "public",
    alloy: "#C8791A",
    alloyRGB: "200 121 26",
    actions: [
      { label: "Үйлчилгээ эхлүүлэх", hint: "eID-аар таниулж эхэлнэ", href: "/kiosk", icon: "scan" },
    ],
  },
  pos: {
    eyebrow: "ГЭРЭГЭ · POS ШУГАМ",
    title: "Кассын цэг",
    lede: "Ээлжийн ажилтан PIN-ээр нэвтэрч, борлуулалтаа бүртгээд баримтаа хэвлэнэ.",
    posture: "public",
    alloy: "#C2410C",
    alloyRGB: "194 65 12",
    actions: [
      { label: "Дэлгэц түгжих", hint: "Киоск горим", href: "/kiosk", icon: "shield-check" },
      { label: "Төхөөрөмж", hint: "Бүртгэл, ээлж, тохиргоо", href: "/settings/devices", icon: "monitor-cog" },
    ],
  },
};

export function isLine(value: string): value is ShellLine {
  return value in LINES;
}
