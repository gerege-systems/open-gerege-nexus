import { redirect } from "next/navigation";

/**
 * Where PDF signing used to live.
 *
 * The screen moved when the E-Sign app was absorbed into Documents: one app has
 * one slug, and every one of its screens hangs off `/module/documents/*`. The
 * page itself did not change — only its address did.
 *
 * This stays because an address people have bookmarked, pinned to a kiosk, or
 * written into a device line's home screen is not ours to invalidate for a
 * reorganisation they did not ask for. The old address is not gone; it has a
 * forwarding note.
 *
 * The note itself is served by `proxy.ts`, which answers 308 before any of this
 * renders. It has to be, and this file is the evidence: a redirect from a page
 * under a streaming root layout arrives after the response has begun, so Next
 * can only finish it as a client-side navigation — a 200 with an instruction
 * in it. Browsers follow that; crawlers and link checkers read the 200.
 *
 * What is left here is the backstop for anything that reaches rendering anyway.
 *
 * The API kept its own prefix (`/api/v1/esign`) and needed no note at all —
 * see documents.RegisterRoutes.
 */
export default function ESignMoved() {
  redirect("/module/documents/pdf");
}
