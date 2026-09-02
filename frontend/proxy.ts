import { NextResponse, type NextRequest } from "next/server";
import { controlPlaneHostDecision } from "@/lib/controlPlaneHost.mjs";
import { DEVICE_LINE_HEADER, deviceLineFromHost, lineHomePath } from "@/lib/deviceLine";

/**
 * Төхөөрөмжийн domain шугамыг Host-оор нь салгана.
 *
 * (Next 16-д `middleware.ts` нь `proxy.ts` болж нэрлэгдсэн — функцийн нэр мөн
 * `proxy`. Үйлдэл нь өмнөхтэй ижил.)
 *
 * Хөтчийн шугам (`nexus.gerege.mn`) дээр энэ файл юу ч хийхгүй — толгой
 * нэмэхгүй, шилжүүлэхгүй. Хөтчийн горим ямар ч нөхцөлд өөрчлөгдөхгүй гэсэн
 * гэрээний нөхцөл эндээс эхэлнэ.
 *
 * Төхөөрөмжийн шугам дээр хийх зүйл гурав:
 *
 * 1. `/` дээр тухайн шугамын өөрийн нүүр дэлгэц рүү шилжүүлнэ. Шугам бүр
 *    өөрийн нүүрээ (`/line/<line>`) өөрөө хөгжүүлнэ.
 *
 *    Rewrite биш redirect: rewrite үед хөтчийн хаяг `/` хэвээр үлддэг тул
 *    client талын router динамик сегментийг олж харахгүй, `useParams()` хоосон
 *    ирнэ. Redirect нь тэр эргэлзээг арилгаад зогсохгүй, аль шугам дээр байгааг
 *    хаягаас нь шууд уншуулна — оношлоход хамаагүй хялбар.
 *
 * 2. `/login` руу орохыг хаана. Тэдгээр шугам дээр нэвтрэлт бол native UI —
 *    web-ийн нэвтрэх дэлгэц бүрхүүлийн дотор гарч ирвэл хэрэглэгч нэг апп
 *    дотроос хоёр өөр нэвтрэлт хардаг болно. Session байхгүй үед бүрхүүл
 *    өөрөө `auth.reLogin`-оор нэвтрэлтээ эхлүүлнэ.
 *
 * 3. Тухайн шугамын нэрийг доош дамжуулж, `Vary: Host` тавина. Шугамууд нэг
 *    ижил зам дээр өөр өөр агуулга үйлчилдэг тул үүнгүйгээр CDN нэг шугамын
 *    хариуг нөгөөд өгөх боломжтой.
 */
/**
 * Screens that changed address when two apps became one.
 *
 * Handled here rather than by a page that calls `redirect()`, because a page
 * cannot. The root layout is rendered per request and streams, so by the time a
 * nested page asks for a redirect the response has already begun and Next can
 * only finish it with a client-side navigation: a 200 carrying an instruction.
 * That is fine for a browser and no use to a crawler, a link checker, or
 * anything reading the status code — which is most of what a permanent move is
 * announced to. Middleware runs before any of that and can answer 308.
 *
 * The old page files stay as a backstop for anything that reaches rendering
 * anyway; this is what actually answers.
 */
const MOVED: Record<string, string> = {};

export function proxy(request: NextRequest) {
  const controlPlane = controlPlaneHostDecision(
    request.headers.get("host"),
    process.env.CONTROL_PLANE_HOST,
    request.nextUrl.pathname,
  );

  if (controlPlane === "redirect") {
    const target = request.nextUrl.clone();
    target.pathname = "/cp";
    target.search = "";
    const response = NextResponse.redirect(target, 308);
    response.headers.set("Vary", "Host");
    return response;
  }

  if (controlPlane === "not-found") {
    return new NextResponse(null, {
      status: 404,
      headers: { "Cache-Control": "no-store", Vary: "Host" },
    });
  }

  if (controlPlane === "allow") {
    const response = NextResponse.next();
    response.headers.set("Vary", "Host");
    return response;
  }

  const moved = MOVED[request.nextUrl.pathname];
  if (moved) {
    const target = request.nextUrl.clone();
    target.pathname = moved;
    return NextResponse.redirect(target, 308);
  }

  const line = deviceLineFromHost(request.headers.get("host"));
  if (!line) return NextResponse.next();

  const { pathname } = request.nextUrl;

  if (pathname === "/" || pathname === "/login") {
    const target = request.nextUrl.clone();
    target.pathname = lineHomePath(line);
    target.search = "";
    const redirect = NextResponse.redirect(target);
    redirect.headers.set("Vary", "Host");
    return redirect;
  }

  const headers = new Headers(request.headers);
  headers.set(DEVICE_LINE_HEADER, line.line);
  const response = NextResponse.next({ request: { headers } });
  response.headers.set(DEVICE_LINE_HEADER, line.line);
  response.headers.set("Vary", "Host");
  return response;
}

export const config = {
  // Статик хөрөнгө, зураг, manifest дээр ажиллуулах шаардлагагүй — тэдгээр нь
  // шугамаас үл хамааран ижил.
  matcher: ["/((?!_next/static|_next/image|favicon.ico|icons|brand.webp|manifest.webmanifest).*)"],
};
