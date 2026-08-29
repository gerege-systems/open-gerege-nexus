/**
 * Which addresses the shell may draw without a session.
 *
 * The rule this file checks is one line of code and two weeks of a Google
 * button that did nothing. `/login/bind` — where a first Google sign-in lands
 * to be verified once with eID — was not in the list, so the shell asked
 * `/api/v1/me` for somebody who by definition has no session, took the 401 and
 * pushed them back to `/login`. Nothing failed and nothing was reported: the
 * screen the flow was carrying them to was simply never drawn.
 *
 * So the list is asserted by intent rather than by contents. Each name below
 * says why that address must render for somebody who is not signed in, which
 * is the question the next person adding a screen to the sign-in area has to
 * answer.
 */

import { expect, test } from "vitest";

import { isPublicPath } from "@/lib/publicRoutes";

test("the sign-in area is public, all of it", () => {
  expect(isPublicPath("/login")).toBe(true);
  // A first sign-in from an external provider: nobody here recognises the
  // account yet, which is the whole reason this screen exists.
  expect(isPublicPath("/login/bind")).toBe(true);
  // An invitation or a password reset. Somebody who cannot sign in is, by
  // definition, not signed in.
  expect(isPublicPath("/login/set-password")).toBe(true);
  // And whatever the sign-in area grows next.
  expect(isPublicPath("/login/anything-added-later")).toBe(true);
});

test("the screens that create a session are public, because they run before one", () => {
  // The wizard is authorised by the setup token; there is no organisation yet
  // to hold a session.
  expect(isPublicPath("/setup")).toBe(true);
  // eID returns the browser here, before the cookie is set.
  expect(isPublicPath("/auth/eid/callback")).toBe(true);
  // An operator's handover becomes a session by being spent on this page. Sent
  // to /login while it is spending, the handover expires instead.
  expect(isPublicPath("/impersonate")).toBe(true);
});

test("the front door and its menu are public", () => {
  expect(isPublicPath("/")).toBe(true);
  for (const section of ["/architecture", "/platform", "/trust"]) {
    expect(isPublicPath(section), section).toBe(true);
  }
});

test("the console is drawn without the tenant shell, and holds its own session", () => {
  // Not public — it authenticates against its own API. It is here so the shell
  // does not ask /api/v1/me for an operator who holds no tenant session and
  // redirect the console to the platform's sign-in screen.
  expect(isPublicPath("/cp")).toBe(true);
  expect(isPublicPath("/cp/operators")).toBe(true);
});

test("a device line draws its own home rather than the web sign-in screen", () => {
  expect(isPublicPath("/line/kiosk")).toBe(true);
});

test("everything behind the door still asks who is asking", () => {
  for (const path of ["/profile", "/settings/access", "/apps", "/me", "/organisation", "/module/x"]) {
    expect(isPublicPath(path), path).toBe(false);
  }
  // A look-alike is not the sign-in area: only what is under it is.
  expect(isPublicPath("/loginx")).toBe(false);
  expect(isPublicPath("/cpx")).toBe(false);
});
