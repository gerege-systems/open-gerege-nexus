/**
 * A header menu item as a page of its own.
 *
 * The sections used to be anchors into one long landing page: a visitor who
 * followed one had no address to share and a phone downloaded the whole
 * argument to read a third of it. Each is a page now, at the address its
 * anchor already named. Two guards decide whether it may be served at all, and
 * both are easy to lose in a refactor because neither shows up on the screen
 * when it is working.
 */

import { afterEach, expect, test, vi } from "vitest";
import { render, screen } from "@testing-library/react";

const setup = vi.hoisted(() => ({ required: vi.fn() }));

vi.mock("@/lib/i18n", () => import("./helpers/i18n"));
vi.mock("@/lib/setup", () => ({ setupRequiredOnServer: setup.required }));
vi.mock("next/navigation", () => ({
  // Next's own versions throw to unwind the render; these say which one was
  // reached, which is the whole assertion.
  redirect: (to: string) => {
    throw new Error(`redirect:${to}`);
  },
  notFound: () => {
    throw new Error("not-found");
  },
  usePathname: () => "/trust",
}));

import SectionScreen from "@/components/landing/SectionScreen";
import TrustPage from "@/app/trust/page";

afterEach(() => vi.unstubAllEnvs());

test("a deployment with no organisation yet sends the reader to the wizard", async () => {
  setup.required.mockResolvedValue(true);

  // Before there is an organisation nothing on this page is true yet, and the
  // only person who can be reading it is the one who should be in the wizard.
  await expect(SectionScreen({ section: "trust" })).rejects.toThrow("redirect:/setup");
});

test("a section this deployment turned off answers 404, not a page it took down", async () => {
  setup.required.mockResolvedValue(false);
  vi.stubEnv("LANDING_SECTIONS", "hero platform");

  await expect(SectionScreen({ section: "trust" })).rejects.toThrow("not-found");
});

test("a section this deployment keeps is served with the whole menu", async () => {
  setup.required.mockResolvedValue(false);
  vi.stubEnv("LANDING_SECTIONS", "hero architecture trust");

  render(await SectionScreen({ section: "trust" }));

  // The menu is the same menu on every section page — a reader who followed
  // one item can reach the others — so it carries the deployment's other
  // linked section as well as this one.
  expect(screen.getAllByRole("link", { name: "website.menu.trust" }).length).toBeGreaterThan(0);
  expect(screen.getAllByRole("link", { name: "website.menu.architecture" }).length).toBeGreaterThan(0);
  // Dropped by this deployment, so it is not in the menu either.
  expect(screen.queryByRole("link", { name: "website.menu.platform" })).toBeNull();
});

test("the trust page asks for the trust section", () => {
  // The three section pages differ by one word, and a copy-paste that leaves
  // the wrong one there renders a page that works and says something else.
  expect(TrustPage().props.section).toBe("trust");
});
