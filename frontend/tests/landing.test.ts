/**
 * The landing page's shape, which is configuration rather than code.
 *
 * `LANDING_SECTIONS` lets a deployment say which parts of the argument apply
 * to it — a single sign-on deployment has no catalogue of business
 * applications to advertise. The rules that matter are the ones about being
 * wrong: a typo in a deployment file should cost that deployment a section,
 * never its front door.
 */

import { expect, test, vi } from "vitest";

/**
 * The reader's environment, as `process.env` is typed here: Next declares
 * NODE_ENV as required, and a test that says only what it is setting should
 * not have to say that too.
 */
const env = (vars: Record<string, string> = {}) => vars as NodeJS.ProcessEnv;

import {
  LANDING_SECTIONS,
  SECTION_PATHS,
  firstLinkedSection,
  landingSectionsFromEnv,
  sectionByAnchor,
} from "@/lib/landing";

test("a deployment that says nothing gets the whole page", () => {
  expect(landingSectionsFromEnv(env())).toEqual([...LANDING_SECTIONS]);
  expect(landingSectionsFromEnv(env({ LANDING_SECTIONS: "   " }))).toEqual([...LANDING_SECTIONS]);
});

test("the order given is the order rendered", () => {
  expect(landingSectionsFromEnv(env({ LANDING_SECTIONS: "trust hero platform" }))).toEqual([
    "trust",
    "hero",
    "platform",
  ]);
  // Commas, whitespace, and case are all how somebody writes a list.
  expect(landingSectionsFromEnv(env({ LANDING_SECTIONS: "Hero, TRUST" }))).toEqual(["hero", "trust"]);
});

test("a section named twice is rendered once", () => {
  // Two of the same section is two elements with one id, which is invalid
  // markup and a menu item that jumps to whichever the browser saw first.
  expect(landingSectionsFromEnv(env({ LANDING_SECTIONS: "trust trust hero" }))).toEqual(["trust", "hero"]);
});

test("a typo costs a section, and a page of typos costs nothing", () => {
  const warned = vi.spyOn(console, "warn").mockImplementation(() => {});

  expect(landingSectionsFromEnv(env({ LANDING_SECTIONS: "hero heroo trust" }))).toEqual(["hero", "trust"]);
  expect(warned).toHaveBeenCalled();

  // The landing page is the one screen a visitor sees before signing in.
  // Answering a misspelt variable with a blank sheet is the worst of the
  // available failures.
  expect(landingSectionsFromEnv(env({ LANDING_SECTIONS: "heroo trast" }))).toEqual([...LANDING_SECTIONS]);
});

test("the hero's button points at the first section below it that can be linked to", () => {
  expect(firstLinkedSection(["hero", "applications", "platform", "trust"])).toBe("platform");
  // applications, technology and capabilities are read on the way down rather
  // than jumped to; nothing linkable means nothing to point at, and the
  // button is not drawn.
  expect(firstLinkedSection(["hero", "applications"])).toBeUndefined();
  expect(firstLinkedSection([])).toBeUndefined();
});

test("an address is only a section if this deployment has that section", () => {
  expect(sectionByAnchor(["hero", "trust"], "trust")).toBe("trust");
  // Dropped by this deployment: nothing links there and it must answer 404,
  // not render words the deployment took down.
  expect(sectionByAnchor(["hero"], "trust")).toBeUndefined();
  expect(sectionByAnchor([...LANDING_SECTIONS], "applications")).toBeUndefined();
});

test("the public paths are every linked section, whatever one deployment turned off", () => {
  // The shell reads this to know a page is public. A path missing from it is
  // a 404 handed to the signed-in shell, which then asks for a session to
  // show a page that does not exist.
  expect(SECTION_PATHS).toEqual(["/architecture", "/platform", "/trust"]);
});
