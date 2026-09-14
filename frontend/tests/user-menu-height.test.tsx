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
 * being, its bottom edge must sit inside the viewport. The menu is a
 * design-system DropdownMenu now, and the room it may take is what Radix
 * measures below the button and hands the panel as a CSS variable — that
 * number is what is read here.
 *
 * The same fix's other half is here too: signing out is now also reachable from
 * the menu's header, which never scrolls away.
 */

import { expect, test, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
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

/** How tall Radix lets the panel grow, once it has measured. */
async function roomForMenu() {
  const panel = screen.getByRole("menu");
  // The number lives on the positioned wrapper Radix puts around the panel;
  // the panel's own max-height refers to it by name.
  expect(panel.style.getPropertyValue("--radix-dropdown-menu-content-available-height")).toContain("--radix-popper-available-height");
  return waitFor(() => {
    const room = Number.parseInt(panel.parentElement!.style.getPropertyValue("--radix-popper-available-height"), 10);
    expect(Number.isNaN(room)).toBe(false);
    return room;
  });
}

test("the menu is capped by the room below the button, not by the whole screen", async () => {
  openMenuWithViewport(700);
  await userEvent.click(screen.getByRole("button", { expanded: false }));

  const capped = await roomForMenu();

  // Bottom edge = where it starts + how tall it may grow. It must fit.
  expect(PANEL_TOP + capped).toBeLessThanOrEqual(700);
  // And it must not have been capped against the full viewport height, which
  // is the bug: 700 - 80 = 620 would push the last rows off the screen.
  expect(capped).toBeLessThan(620);
});

test("a viewport too short to be worth capping still leaves a usable menu", async () => {
  openMenuWithViewport(150);
  await userEvent.click(screen.getByRole("button", { expanded: false }));

  // Below the button there is no room to speak of; the panel goes wherever
  // there is more, and what it gets is still something — a menu that fits
  // and scrolls, not one capped to nothing.
  expect(await roomForMenu()).toBeGreaterThan(0);
  expect(screen.getAllByRole("menuitem", { name: "web.action.logout" }).length).toBeGreaterThan(0);
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
