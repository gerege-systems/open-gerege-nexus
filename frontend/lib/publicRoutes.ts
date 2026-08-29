import { SECTION_PATHS } from "./landing";

/**
 * Which addresses the workspace shell may draw without a session.
 *
 * The shell asks `/api/v1/me` on mount for everything else, and sends whoever
 * gets a 401 to `/login`. That is right for a screen behind the door and wrong
 * for the door itself — and the difference is decided here, by address, before
 * any request is made.
 *
 * It lives in its own module rather than inside components/Layout.tsx because
 * the rule is a fact about this deployment's addresses, not about how the shell
 * is drawn: a test can ask it what it thinks without rendering a page, which is
 * the check that was missing when /login/bind was added.
 */

/**
 * Routes that render without the ERP chrome. /oauth/consent is signed-in but
 * belongs here too: it is an identity handoff to another product, and framing
 * it in this one's navigation invites the user to wander off mid-flow.
 *
 * /setup is public in the only sense that matters here: it runs on a
 * deployment with no organisation, so there is nobody to hold a session and
 * asking /me for one would push the wizard to a sign-in screen that cannot
 * work. What authorises it is the setup token, not a session.
 *
 * /impersonate is where an operator's handover becomes a session. The person
 * arriving has none yet — spending the token is what gives them one — so the
 * shell must not send them to /login while the page is still spending it.
 */
export const PUBLIC_ROUTES = [
  "/",
  "/login",
  "/setup",
  "/auth/eid/callback",
  "/oauth/consent",
  "/kiosk",
  "/impersonate",
];

/**
 * isPublicPath decides whether the shell may render a route without asking who
 * is looking at it.
 *
 * The sign-in area is matched by prefix, not by name. `/login` is not one
 * screen: a first sign-in from Google lands on `/login/bind` to be verified
 * once with eID, and an invitation or a password reset lands on
 * `/login/set-password`. Both are, by definition, for somebody who is not
 * signed in — and both were private under an exact-match rule, so the shell
 * asked /me, took the 401 and pushed them back to `/login`. From the outside
 * that is a Google button that does nothing and an invitation link that leads
 * to a sign-in screen; the reason was never on the screen, because nothing had
 * failed. Every screen under /login is the door, so the door is the prefix.
 *
 * Шугамын нүүр дэлгэц нэвтрэлт шаардахгүй. Тэр нь ажлын мужид web-ийн нэвтрэх
 * дэлгэц гарч ирэхийг ОРЛОХЫН тулд байгаа тул session байхгүй үед ч зогсох
 * ёстой — эс бөгөөс дахин /login руу түлхэж, шийдэх гэсэн асуудлаа өөрөө
 * үүсгэнэ.
 *
 * `/cp` is chromeless for a different reason from the rest: it is not public at
 * all, it is the operator console, and it authenticates with its own session
 * against its own API. Left out of this list it would have been given the
 * tenant shell — which asks /api/v1/me on mount, gets a 401 because an operator
 * holds no tenant session, and redirects the console to the platform's login
 * screen before it can draw its own.
 *
 * The landing page's menu items are pages of their own now, and they are as
 * public as the page they were carved out of. Left out, the shell asked
 * /api/v1/me for a visitor who has no session, took the 401 and redirected the
 * front door's own menu to a sign-in screen.
 */
export function isPublicPath(path: string): boolean {
  return (
    PUBLIC_ROUTES.includes(path) ||
    path.startsWith("/login/") ||
    SECTION_PATHS.includes(path) ||
    path.startsWith("/line/") ||
    path === "/cp" ||
    path.startsWith("/cp/")
  );
}
