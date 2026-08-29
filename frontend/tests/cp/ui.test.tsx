/**
 * The console's small shared pieces, and the account menu it borrows from the
 * product.
 *
 * They are shared, so a mistake in one of them is a mistake on every screen at
 * once — which is also why they are worth their own tests rather than being
 * asserted incidentally through a page.
 */

import { expect, test, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";

vi.mock("@/lib/i18n", () => import("../helpers/i18n"));

import { Badge, formatMoment, Table } from "@/components/cp/ui";
import { ThemeProvider } from "@/lib/theme";
import UserMenu from "@/components/UserMenu";

test("a moment nobody recorded is blank, not the epoch", () => {
  // Every table here passes a nullable timestamp straight in. `new Date(null)`
  // is 1 January 1970, which is a date, and a date is what a reader believes.
  for (const nothing of [null, undefined, ""]) {
    expect(formatMoment(nothing, "mn")).toBe("");
  }
  expect(formatMoment("not a date", "mn")).toBe("");
});

test("a moment is written in the reader's own language and time zone", () => {
  const written = formatMoment("2026-08-29T02:00:00Z", "mn");

  expect(written).not.toBe("");
  expect(written).toContain("2026");
  // The API sends RFC 3339 with an offset; the browser is the only party that
  // knows where the person reading it is sitting.
  expect(written).not.toContain("2026-08-29T02:00:00Z");
});

test("an empty table says why it is empty, across the whole width", () => {
  render(<Table head={["a", "b", "c"]} rows={[]} empty="cp.message.no_activity" />);

  const cell = screen.getByText("cp.message.no_activity");
  expect(cell.getAttribute("colspan")).toBe("3");
});

test("a table with rows does not also show its empty state", () => {
  render(<Table head={["a"]} rows={[[<span key="x">мөр</span>]]} empty="cp.message.no_activity" />);

  expect(screen.getByText("мөр")).toBeTruthy();
  expect(screen.queryByText("cp.message.no_activity")).toBeNull();
});

test("every tone a screen asks for is a tone the badge has", () => {
  // `green` and `emerald` are the same colour under two names, because two
  // screens were written months apart. A tone with no entry renders
  // `class="... undefined"`, which is an invisible badge.
  for (const tone of ["red", "amber", "emerald", "green", "slate"] as const) {
    const { unmount } = render(<Badge tone={tone}>{tone}</Badge>);
    expect(screen.getByText(tone).className).not.toContain("undefined");
    unmount();
  }
});

test("the avatar is two letters, and the name is not cut in half", () => {
  render(
    <ThemeProvider>
      <UserMenu
        user={{ name: "Мөнх Оператор", email: "me@example.test" }}
        onLogout={() => {}}
        showTenants={false}
        links={[]}
        subtitle="cp.role.superadmin"
      />
    </ThemeProvider>,
  );

  // One initial is the same letter for most of a Mongolian directory.
  const button = screen.getByRole("button");
  expect(within(button).getByText("МО")).toBeTruthy();
  expect(within(button).getByText("Мөнх Оператор")).toBeTruthy();
});

test("a one-word name still fills both letters", () => {
  render(
    <ThemeProvider>
      <UserMenu
        user={{ name: "Болд", email: "b@example.test" }}
        onLogout={() => {}}
        showTenants={false}
        links={[]}
      />
    </ThemeProvider>,
  );

  expect(within(screen.getByRole("button")).getByText("БО")).toBeTruthy();
});
