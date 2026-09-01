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
import userEvent from "@testing-library/user-event";

vi.mock("@/lib/i18n", () => import("../helpers/i18n"));

import { Badge, formatMoment, Table } from "@/components/cp/ui";
import { Modal } from "@/components/ui";
import { ThemeProvider } from "@/lib/theme";
import UserMenu from "@/components/UserMenu";

test("a moment nobody recorded is blank, not the epoch", () => {
  // Every table here passes a nullable timestamp straight in. `new Date(null)`
  // is 1 January 1970, which is a date, and a date is what a reader believes.
  for (const nothing of [null, undefined, ""]) {
    expect(formatMoment(nothing)).toBe("");
  }
  expect(formatMoment("not a date")).toBe("");
});

test("a moment is written in one format, in Ulaanbaatar time", () => {
  // 02:00 UTC is 10:00 the same morning in Ulaanbaatar (UTC+8, no DST). The
  // assertion is on the exact string on purpose: the format is the contract —
  // sortable, identical in all seven languages, and the same for an operator
  // whose laptop is set to another zone.
  expect(formatMoment("2026-08-29T02:00:00Z")).toBe("2026-08-29 10:00");
  // Across midnight UTC, so the date rolls forward and not just the clock.
  expect(formatMoment("2026-08-29T20:30:00Z")).toBe("2026-08-30 04:30");
  // Midnight reads 00:00, never 24:00.
  expect(formatMoment("2026-08-29T16:00:00Z")).toBe("2026-08-30 00:00");
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

test("a dialog closes on Escape", async () => {
  // Escape is not a stray gesture — it is the one every dialog on the web
  // answers to. Without it a keyboard operator has no way to tell that the
  // dialog is closable at all.
  const close = vi.fn();
  const person = userEvent.setup();
  render(
    <Modal onClose={close} label="cp.action.new_tenant">
      <button type="button">cp.action.save</button>
    </Modal>,
  );

  await person.keyboard("{Escape}");

  expect(close).toHaveBeenCalledTimes(1);
});

test("a click outside dismisses an empty dialog and spares one being filled in", async () => {
  // The backdrop is the case the old component was right to be careful about:
  // a stray click must not throw away typed input. So it dismisses only while
  // there is nothing to lose.
  const close = vi.fn();
  const person = userEvent.setup();
  const { container } = render(
    <Modal onClose={close} label="cp.action.new_tenant">
      <input aria-label="cp.field.name" defaultValue="" />
    </Modal>,
  );
  const backdrop = container.firstElementChild as HTMLElement;

  await person.click(backdrop);
  expect(close).toHaveBeenCalledTimes(1);

  await person.type(screen.getByLabelText("cp.field.name"), "Гэрэгэ");
  await person.click(backdrop);

  expect(close).toHaveBeenCalledTimes(1);
});
