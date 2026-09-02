/**
 * deviceLine — төхөөрөмжийн domain шугам.
 *
 * Backend цор ганц. Гэхдээ төхөөрөмж бүр өөрийн host-оор ханддаг:
 *
 *   nexus.gerege.mn          → хөтөч / PWA — web app өөрөө бүрэн апп
 *   desktop.nexus.gerege.mn  → macOS, Windows — ширээн дээрх ажлын муж
 *   mobile.nexus.gerege.mn   → iOS, Android — гарын алган дахь ажлын муж
 *   kiosk.nexus.gerege.mn    → Kiosk
 *   pos.nexus.gerege.mn      → POS
 *
 * **Хаяг нь form factor-ыг нэрлэнэ, платформыг биш.** Ширээн дээрх Mac ба
 * ширээн дээрх Windows хоёр нэг шугам: хүн тэр хоёртой ижил байдлаар
 * харьцдаг. Бүрхүүл өөрийгөө юу гэж хэлж байгаа нь тусдаа зүйл бөгөөд
 * `window.GeregeShell.platform` дээр хэвээр байна — шаардлагатай бол нэг
 * шугамын дэлгэц дотроос платформоор салаалж болно.
 *
 * `kiosk` ба `pos` нь desktop/mobile дотор ОРООГҮЙ: нэг Windows машин дээр
 * ажлын ширээний клиент ба киоск зэрэг ажиллаж болох тул тэдний хооронд
 * host-only cookie-гийн тусгаарлалт хэрэгтэй хэвээр. macOS ба Windows хоёр
 * нэг машин дээр хэзээ ч зэрэг ажиллахгүй тул тэдэнд хэрэггүй.
 *
 * Шугам бүр өөрийн host дээрээ `/api/v1`-ээ мөн үйлчилдэг (nginx нь бүгдийг
 * НЭГ ижил API upstream руу дамжуулна), тиймээс webview доторх дуудлага үргэлж
 * same-origin — session cookie нь `SameSite=Strict` хэвээр ажиллаж, CORS
 * preflight огт үүсэхгүй.
 *
 * Эх сурвалж: `native-apps/shared/device_lines.json`. Тэр файл нь native
 * талын, энэ нь web талын хуулбар — шугам нэмэхэд ХОЁУЛАНГ нь өөрчилнө.
 */

import type { ShellFormFactor, ShellLine } from "@/lib/shell";

export interface DeviceLine {
  line: ShellLine;
  formFactor: ShellFormFactor;
  /**
   * Native бүрхүүл нэвтэрсний дараа нээх эхний зам — шугамын үндэс.
   *
   * Тэнд тухайн шугамын өөрийн нүүр дэлгэц рендерлэгдэнэ: `proxy.ts` нь
   * `/`-ыг `/line/<line>` рүү rewrite хийнэ (redirect биш — хаяг нь `/`
   * хэвээр үлдэнэ). Тиймээс шугам бүр өөрийн нүүрээ өөрөө хөгжүүлнэ.
   */
  startRoute: string;
}

/**
 * Host-ын хамгийн зүүн шошго → шугам.
 *
 * Домэйн бүтнээр нь биш зөвхөн эхний шошгоор тааруулж байгаа нь санаатай:
 * `desktop.nexus.gerege.mn`, `desktop.nexus.staging.gerege.mn`,
 * `desktop.localhost` гурвуул ижил шугам. Ингэснээр staging/preview орчинд
 * энэ файлыг хөндөх шаардлагагүй.
 */
const LINES_BY_LABEL: Record<string, DeviceLine> = {
  desktop: { line: "desktop", formFactor: "desktop", startRoute: "/" },
  mobile: { line: "mobile", formFactor: "mobile", startRoute: "/" },
  kiosk: { line: "kiosk", formFactor: "kiosk", startRoute: "/" },
  pos: { line: "pos", formFactor: "pos", startRoute: "/" },
};

/** Шугамын нүүр дэлгэцийн бодит зам. `proxy.ts` `/`-ыг үүн рүү rewrite хийнэ. */
export function lineHomePath(line: DeviceLine): string {
  return `/line/${line.line}`;
}

/** Middleware-ээс доош дамжуулах толгой. Хөтчийн шугам дээр огт тавигдахгүй. */
export const DEVICE_LINE_HEADER = "x-gerege-device-line";

/**
 * Host-оос шугамыг тодорхойлно. Танихгүй бол `null` — өөрөөр хэлбэл хөтчийн
 * шугам, тэнд юу ч өөрчлөгдөхгүй.
 *
 * `mac`, `win`, `ios`, `android` шошгууд 2026-09-02-оос хойш ТАНИГДАХГҮЙ:
 * шугам нь платформ биш form factor болсон бөгөөд nginx тэдгээр нэрийг
 * үйлчлэхээ больсон.
 *
 * `Host` толгой нь порт агуулж болно (`desktop.localhost:3000`), мөн IPv6 хаяг
 * нь хаалтанд байдаг — хоёуланг нь тайрч байж шошгыг уншина.
 */
export function deviceLineFromHost(host: string | null | undefined): DeviceLine | null {
  if (!host) return null;
  const hostname = host.trim().toLowerCase().replace(/^\[.*\]/, "").replace(/:\d+$/, "");
  const label = hostname.split(".")[0];
  return LINES_BY_LABEL[label] ?? null;
}

/** Хөтөч дээр ажиллаж байгаа шугам. SSR-д үргэлж `null`. */
export function currentDeviceLine(): DeviceLine | null {
  if (typeof window === "undefined") return null;
  return deviceLineFromHost(window.location.host);
}
