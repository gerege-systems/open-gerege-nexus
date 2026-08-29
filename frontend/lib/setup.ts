/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

/**
 * The rule the first administrator's password is held to.
 *
 * It repeats tenants.MinAdminPasswordLength, which is the authority: the server
 * refuses a shorter one whatever this says. What it buys is the refusal
 * arriving before the form is submitted rather than after.
 */
export const MIN_SETUP_PASSWORD = 10;

/**
 * Байгууллагын богино нэр (slug) ямар байж болох вэ.
 *
 * Энэ нь tenants.slugPattern-ыг давтаж байгаа бөгөөд эрх мэдэл нь тэнд:
 * сервер өөр хэлбэрийг татгалзана. Энд байгаагийн ашиг нь татгалзал маягт
 * илгээгдэхээс өмнө ирэхэд оршино — шидтэний сүүлийн алхам дээр биш.
 *
 * Зураас нь `\-` гэж бичигдсэн: HTML-ийн `pattern` шинжийг хөтөч `v` тугтай
 * эмхэтгэдэг болсон бөгөөд тэнд ангийн доторх задгай `-` нь дүрмийн алдаа юм.
 * Алдаатай `pattern`-ыг хөтөч алгасдаг тул шалгалт нь чимээгүй унтардаг.
 */
export const SETUP_SLUG_PATTERN = "[a-z0-9][a-z0-9\\-]{1,62}[a-z0-9]";

/** Богино нэр дүрэмдээ таарч байна уу. */
export function isValidSetupSlug(slug: string): boolean {
  return new RegExp(`^(?:${SETUP_SLUG_PATTERN})$`).test(slug);
}

/**
 * Энэ суулгац байгууллагагүй хэвээр байна уу.
 *
 * Байгууллагагүй суулгац дээр нүүр хуудас бол худал: платформын танилцуулга,
 * «Нэвтрэх» товч, стор — гурвуулаа хэн ч нэвтэрч чадахгүй систем дээр зогсож
 * байна. Тэр төлөвт зочин байхгүй, зөвхөн суулгасан хүн байна, түүнд хэрэгтэй
 * ганц зүйл нь шидтэн.
 *
 * `lib/storefront.ts`-ийн шалтгаанаар серверийн зүгээс: хуудсыг илгээхээс өмнө
 * шийдэгдэх ёстой, эс бөгөөс нүүр хуудас нэг зурагдаад дараа нь үсэрнэ.
 *
 * Кэшлэхгүй. Энэ утга амьдралдаа нэг л удаа өөрчлөгддөг ба яг тэр агшинд
 * (шидтэнийг дуусгасны дараа) хуучирсан хариу нь дөнгөж тохируулсан суулгацыг
 * шидтэн рүүгээ буцаан шиднэ. Хүсэлт нь compose-ийн дотоод сүлжээгээр явдаг
 * тул үнэ нь бараг тэг.
 *
 * Аливаа алдаанд `false` — нэг fetch унасны улмаас нүүр хуудсаа алдах нь
 * тохируулаагүй суулгац дээр танилцуулга үзүүлэхээс дор.
 */
export async function setupRequiredOnServer(): Promise<boolean> {
  const base = process.env.API_INTERNAL_URL;
  if (!base) return false;
  try {
    const res = await fetch(`${base.replace(/\/$/, "")}/setup/status`, { cache: "no-store" });
    if (!res.ok) return false;
    const status = (await res.json()) as { required?: boolean };
    return status.required === true;
  } catch {
    return false;
  }
}

/**
 * The rule the first operator's password is held to.
 *
 * Longer than the tenant admin's, and for the reason the server gives
 * (operator.MinPasswordLength): there are a handful of console accounts and
 * they are the platform. Repeated here so the form can say the rule before
 * asking rather than after refusing; the server is still the authority.
 */
export const MIN_OPERATOR_PASSWORD = 12;
