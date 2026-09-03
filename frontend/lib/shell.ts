"use client";

/**
 * shell — "Native Shell + Web Work Area" архитектурын web талын гэрээ.
 *
 * Native бүрхүүл (Swift/AppKit, C#/WPF, Kotlin/Android) нь нэвтрэлт, толгой хэсэг,
 * цэс, хөл, төхөөрөмжийн хандалтыг өөртөө авдаг. Тэр үед web app нь өөрийн
 * chrome-оо нуугаад зөвхөн ажлын муж болж рендерлэгдэнэ. Хөтөч дээр shell
 * байхгүй тул энэ файлын бүх зүйл `null`/`false` буцаана — өөрөөр хэлбэл
 * хөтчийн горим ямар ч нөхцөлд өөрчлөгдөхгүй.
 *
 * Гэрээний бүрэн бүртгэл: `docs/SHELL_CONTRACT.md`.
 */

import { useEffect, useState, useSyncExternalStore } from "react";

export type ShellPlatform = "macos" | "windows" | "ios" | "android" | "kiosk" | "pos";
export type ShellFormFactor = "desktop" | "mobile" | "tablet" | "kiosk" | "pos";

/**
 * Төхөөрөмжийн шугам — хаяг юуг нэрлэдэг вэ.
 *
 * `ShellPlatform`-тай ЗОРИУДААР ижил биш. Платформ гэдэг нь бүрхүүл өөрийгөө
 * юу гэж хэлж байгаа нь (macOS уу, Windows уу); шугам гэдэг нь тухайн
 * төхөөрөмжтэй хүн хэрхэн харьцаж байгаа нь. Ширээн дээрх Mac ба ширээн
 * дээрх Windows хоёр НЭГ шугам — тэдгээрийн ялгаа нь дэлгэцийн нягтрал,
 * товчны хэмжээ, юуг эхэнд тавихад нөлөөлдөггүй.
 *
 * `tablet` энд байхгүй: Android-ийн таблет flavor нь `mobile` шугам дээр
 * ажиллана. `kiosk` ба `pos` нь тусдаа: нэг Windows машин дээр ажлын
 * ширээний клиент ба киоск зэрэг ажиллаж болох тул тэдэнд host-only
 * cookie-гийн тусгаарлалт хэрэгтэй.
 */
export type ShellLine = "desktop" | "mobile" | "kiosk" | "pos";

export interface GeregeShell {
  /** Гэрээний semver. */
  version: string;
  platform: ShellPlatform;
  formFactor: ShellFormFactor;
  /** Тухайн бүрхүүлд ҮНЭХЭЭР хэрэгжсэн чадварууд. Зарлагдаагүй method дуудвал
   *  invoke() reject хийнэ. */
  capabilities: string[];
  invoke<T>(method: string, params?: Record<string, unknown>): Promise<T>;
  /** Буцаах утга нь бүртгэлээ цуцлах функц. */
  on(event: string, handler: (payload: unknown) => void): () => void;
}

declare global {
  interface Window {
    GeregeShell?: GeregeShell;
  }
}

/** Гэрээгээр тогтсон method нэрс. Хаана ч мөрөн дотор бичихгүй — нэр солигдвол
 *  энэ нэг газраас л өөрчлөгдөнө. */
export const SHELL_METHODS = {
  /** Session дуусахад бүрхүүлийн нэвтрэлтийн урсгалыг эхлүүлнэ. */
  AUTH_RE_LOGIN: "auth.reLogin",
  /** Lifecycle method-ууд capability шаардахгүй. */
  AUTH_LOCK: "auth.lock",
  NOTIFY_SHOW: "notify.show",
  BADGE_SET: "badge.set",
  BIOMETRIC_AUTHENTICATE: "biometric.authenticate",
  EXTERNAL_OPEN: "external.open",
  PRINT_SYSTEM: "print.system",
  FS_SAVE_AS: "fs.saveAs",
  /** Тенантын цэс өөрчлөгдсөнийг мэдэгдэнэ — бүрхүүл өөрийн native цэсээ дахин
   *  татах боломжтой болно. */
  MENU_CHANGED: "menu.changed",
  /** Бүрхүүлийн эзэмшдэг дэлгэц рүү ХҮРЭЭН ДОТОР шилжинэ (`{ pane }`). Цонх
   *  нээх дуудлага БИШ — гэрээний §1a. Reject ирвэл web тал өөрийн хуудсаа
   *  үзүүлнэ. */
  SHELL_OPEN_PANE: "shell.openPane",
  ESCPOS_PRINT: "escpos.print",
  ESCPOS_DRAWER: "escpos.drawer",
  SCANNER_START: "scanner.start",
  SCANNER_STOP: "scanner.stop",
  SERIAL_TRANSACT: "serial.transact",
  DEVICE_IDENTITY: "device.identity",
  CAMERA_SCAN: "camera.scan",
  KIOSK_LOCKDOWN: "kiosk.lockdown",
  TELEMETRY_EMIT: "telemetry.emit",
  PAYMENT_START: "payments.start",
} as const;

/** Гэрээгээр тогтсон event нэрс. Бүрхүүлээс web рүү чиглэнэ. */
export const SHELL_EVENTS = {
  NAVIGATE: "shell:navigate",
  SEARCH: "shell:search",
  MENU_REFRESH: "shell:menu-refresh",
  AUTH_CHANGED: "shell:auth-changed",
  CAPABILITIES_CHANGED: "shell:capabilities-changed",
} as const;

/** Гэрээгээр тогтсон capability нэрс. */
export const SHELL_CAPABILITIES = {
  BIOMETRIC: "biometric",
  PRINT_SYSTEM: "print.system",
  /** `fs.saveAs` method-ын чадвар. Чадвар нь боломж, method нь дуудлага. */
  FS_SAVE: "fs.save",
  NOTIFY: "notify",
  BADGE: "badge",
  EXTERNAL_OPEN: "external.open",
  SECURE_STORE: "secure-store",
  MENU_NATIVE: "menu.native",
  /** `shell.openPane` method-ын чадвар. */
  SHELL_PANE: "shell.pane",
  ESCPOS: "escpos",
  SCANNER: "scanner",
  SERIAL: "serial",
  NFC: "nfc",
  CAMERA_SCAN: "camera.scan",
  DEVICE_IDENTITY: "device.identity",
  KIOSK_LOCKDOWN: "kiosk.lockdown",
  TELEMETRY: "telemetry",
  PAYMENTS: "payments",
} as const;

export interface ShellNavigatePayload {
  path: string;
}

export interface ShellSearchPayload {
  query: string;
}

/**
 * Одоогийн бүрхүүл. SSR-д үргэлж `null` — `window` байхгүй.
 *
 * Бүрхүүл объектыг document start дээр inject хийдэг тул hydration-ы үед аль
 * хэдийн байрандаа байдаг; ижил объектын лавлагаа буцдаг нь
 * `useSyncExternalStore`-ын snapshot-ыг тогтвортой байлгана.
 */
export function getShell(): GeregeShell | null {
  if (typeof window === "undefined") return null;
  return window.GeregeShell ?? null;
}

export function hasCapability(cap: string): boolean {
  return getShell()?.capabilities?.includes(cap) ?? false;
}

// Бүрхүүл ажиллах хугацаандаа солигддоггүй (inject хийгээд л дуусдаг), тул
// захиалга нь юу ч хийхгүй — snapshot нэг л удаа уншигдана.
const subscribeToShell = () => () => {};
const getServerShell = () => null;

/**
 * React хук: бүрхүүл ба түүн дотор ажиллаж байгаа эсэх.
 *
 * Сервер дээр `inShell` нь үргэлж `false` тул анхны markup хөтчийнхтэй адил
 * гарч, hydration-ы дараа л бүрхүүлийн харагдац руу шилжинэ.
 */
export function useShell(): { shell: GeregeShell | null; inShell: boolean } {
  const shell = useSyncExternalStore(subscribeToShell, getShell, getServerShell);
  const [mountedShell, setMountedShell] = useState<GeregeShell | null>(null);

  useEffect(() => {
    setMountedShell(getShell());
  }, []);

  const activeShell = shell ?? mountedShell;
  return { shell: activeShell, inShell: activeShell !== null };
}

export type ShellInvokeResult<T> =
  | { ok: true; value: T }
  | { ok: false; reason: "no-shell" | "error" | "timeout"; error?: Error };

/**
 * Хэзээ ч throw хийхгүй, хэзээ ч мөнхөд өлгөөстэй үлдэхгүй invoke.
 *
 * Дуудагч тал ихэвчлэн "бүрхүүл үүнийг барьж авсан уу, үгүй юу" гэдгийг л
 * мэдэх шаардлагатай байдаг — бүрхүүл дэмжихгүй бол, алдвал, эсвэл огт хариу
 * буцаахгүй бол бүгд адилхан "web талын fallback-аа ажиллуул" гэсэн утгатай.
 */
export async function invokeShell<T>(
  method: string,
  params?: Record<string, unknown>,
  timeoutMs = 20000,
): Promise<ShellInvokeResult<T>> {
  const shell = getShell();
  if (!shell) return { ok: false, reason: "no-shell" };

  let timer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<{ ok: false; reason: "timeout" }>((resolve) => {
    timer = setTimeout(() => resolve({ ok: false, reason: "timeout" }), timeoutMs);
  });

  try {
    const invocation = shell
      .invoke<T>(method, params)
      .then((value) => ({ ok: true, value }) as const)
      .catch((error: unknown) => ({
        ok: false,
        reason: "error",
        error: error instanceof Error ? error : new Error(String(error)),
      }) as const);
    return await Promise.race([invocation, timeout]);
  } finally {
    clearTimeout(timer);
  }
}
