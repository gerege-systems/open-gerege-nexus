/**
 * The account menu has to end above the bottom of the screen.
 *
 * It shipped capped at `100dvh` minus a fixed guess, measured from the top of
 * the viewport rather than from where the panel actually starts. On a phone
 * with several organisations that put the last rows — sign out among them —
 * below the fold: they scrolled into view under a dragging finger and the
 * page rubber-banded them straight back out. The menu looked scrollable and
 * was not usable, which is the worst of the two.
 *
 * So the assertion is arithmetic, not appearance: whatever the menu ends up
 * being, its bottom edge must sit inside the viewport.
 *
 * The same fix's other half is here too: signing out is now also reachable from
 * the menu's header, which never scrolls away.
 */

import { expect, test, vi, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

vi.mock("@/lib/i18n", () => import("./helpers/i18n"));

import { ThemeProvider } from "@/lib/theme";
import UserMenu from "@/components/UserMenu";

const PANEL_TOP = 96; // header height plus the menu's own margin

const realRect = Element.prototype.getBoundingClientRect;

afterEach(() => {
  Element.prototype.getBoundingClientRect = realRect;
});

function openMenuWithViewport(height: number) {
  window.innerHeight = height;
  Element.prototype.getBoundingClientRect = function () {
    return { ...realRect.call(this), top: PANEL_TOP } as DOMRect;
  };

  render(
    <ThemeProvider>
      <UserMenu user={{ name: "Цэнддорж Эрдэнэбат", email: "cs@example.mn" }} onLogout={() => {}} showTenants={false} />
    </ThemeProvider>,
  );
}

test("the menu is capped by the room below the button, not by the whole screen", async () => {
  openMenuWithViewport(700);
  await userEvent.click(screen.getByRole("button", { expanded: false }));

  const panel = screen.getByRole("menu") as HTMLElement;
  const capped = Number.parseInt(panel.style.maxHeight, 10);

  expect(Number.isNaN(capped)).toBe(false);
  // Bottom edge = where it starts + how tall it may grow. It must fit.
  expect(PANEL_TOP + capped).toBeLessThanOrEqual(700);
  // And it must not have been capped against the full viewport height, which
  // is the bug: 700 - 80 = 620 would push the last rows off the screen.
  expect(capped).toBeLessThan(620);
});

test("a viewport too short to be worth capping still leaves a usable menu", async () => {
  openMenuWithViewport(150);
  await userEvent.click(screen.getByRole("button", { expanded: false }));

  const panel = screen.getByRole("menu") as HTMLElement;
  expect(Number.parseInt(panel.style.maxHeight, 10)).toBe(200);
});

test("signing out is reachable without scrolling to the bottom of the menu", async () => {
  const signedOut = vi.fn();
  window.innerHeight = 700;
  render(
    <ThemeProvider>
      <UserMenu user={{ name: "Цэнддорж Эрдэнэбат", email: "cs@example.mn" }} onLogout={signedOut} showTenants={false} />
    </ThemeProvider>,
  );
  await userEvent.click(screen.getByRole("button", { expanded: false }));

  // Two ways out, and the one in the header comes before the organisations.
  const exits = screen.getAllByRole("menuitem", { name: "web.action.logout" });
  expect(exits.length).toBe(2);

  await userEvent.click(exits[0]);
  expect(signedOut).toHaveBeenCalledTimes(1);
});
