/**
 * What is being produced, and what is being kept.
 *
 * The rules under test are the ones that make these three screens worth
 * opening rather than reading off the front page: the busiest organisation
 * first, the broken schedule counted and the working one not, and a backup
 * history that distinguishes "we have never tested a restore" from "the last
 * one was fine".
 */

import { expect, test, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { aBackup, aPlatformUsage, aSchedule } from "../helpers/fixtures";

const api = vi.hoisted(() => ({
  platformUsage: vi.fn(),
  reportSchedules: vi.fn(),
  backups: vi.fn(),
  recordRestoreTest: vi.fn(),
  stepUp: vi.fn(),
}));

vi.mock("@/lib/i18n", () => import("../helpers/i18n"));
vi.mock("@/lib/cp", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/cp")>()),
  cp: api,
}));

import Usage from "@/app/cp/ops/usage/page";
import Schedules from "@/app/cp/ops/schedules/page";
import Backups from "@/app/cp/ops/backups/page";

const line = (name: string, metrics: Record<string, number>, collected: string | null = "2026-08-29T02:00:00+08:00") => ({
  tenant_id: `id-${name}`,
  tenant_name: name,
  slug: name.toLowerCase(),
  suspended: false,
  metrics,
  collected,
});

test("the busiest organisation is the first one read, not the first one named", async () => {
  api.platformUsage.mockResolvedValue(
    aPlatformUsage({
      metrics: ["documents", "storage_mb"],
      totals: { documents: 130, storage_mb: 1300 },
      tenants: [
        line("Alpha", { documents: 10, storage_mb: 100 }),
        line("Zulu", { documents: 100, storage_mb: 1000 }),
        line("Mike", { documents: 20, storage_mb: 200 }),
      ],
    }),
  );

  render(<Usage />);
  await screen.findByText("Zulu");

  const names = screen.getAllByRole("link").map((link) => link.textContent);
  expect(names).toEqual(["Zulu", "Mike", "Alpha"]);
});

test("an organisation nothing has ever been counted for does not read as a row of zeroes", async () => {
  api.platformUsage.mockResolvedValue(
    aPlatformUsage({
      metrics: ["documents"],
      totals: { documents: 0 },
      tenants: [line("Шинэ ХХК", {}, null)],
    }),
  );

  render(<Usage />);

  const row = (await screen.findByText("Шинэ ХХК")).closest("tr")!;
  // The metric column is genuinely zero; the collection column is not zero,
  // it is absent, and the two mean different things to whoever is deciding
  // whether this deployment is being used.
  expect(within(row).getByText("0")).toBeTruthy();
  expect(within(row).getByText("cp.state.never_counted")).toBeTruthy();
});

test("a suspended organisation is still counted, and marked", async () => {
  api.platformUsage.mockResolvedValue(
    aPlatformUsage({
      metrics: ["documents"],
      totals: { documents: 5 },
      tenants: [{ ...line("Түдгэлзсэн ХХК", { documents: 5 }), suspended: true }],
    }),
  );

  render(<Usage />);

  const row = (await screen.findByText("Түдгэлзсэн ХХК")).closest("tr")!;
  expect(within(row).getByText("cp.state.suspended")).toBeTruthy();
  expect(within(row).getByText("5")).toBeTruthy();
});

test("only live schedules are counted as trouble", async () => {
  api.reportSchedules.mockResolvedValue({
    schedules: [
      aSchedule({ id: "1", last_status: "smtp refused" }),
      aSchedule({ id: "2", last_run_at: null, last_status: "" }),
      // Switched off on purpose, and failing when it was on. Counting it would
      // send somebody to fix a report nobody asked for.
      aSchedule({ id: "3", active: false, last_status: "smtp refused" }),
      aSchedule({ id: "4" }),
    ],
  });

  render(<Schedules />);

  expect(await screen.findByText("cp.message.schedules_trouble {failing=1 never=1}")).toBeTruthy();
});

test("a schedule that has never run, one that is off and one that failed read as three states", async () => {
  api.reportSchedules.mockResolvedValue({
    schedules: [
      aSchedule({ id: "1", name: "Хэзээ ч", last_run_at: null, last_status: "" }),
      aSchedule({ id: "2", name: "Унтраалттай", active: false }),
      aSchedule({ id: "3", name: "Уналттай", last_status: "smtp refused" }),
      aSchedule({ id: "4", name: "Хэвийн" }),
    ],
  });

  render(<Schedules />);

  const stateOf = async (name: string) =>
    within((await screen.findByText(name)).closest("tr")!);
  expect((await stateOf("Хэзээ ч")).getAllByText("cp.state.never").length).toBeGreaterThan(0);
  expect((await stateOf("Унтраалттай")).getByText("cp.state.off")).toBeTruthy();
  // The provider's own words, not a badge saying "failed": what is needed at
  // this point is what it said.
  expect((await stateOf("Уналттай")).getByText("smtp refused")).toBeTruthy();
  expect((await stateOf("Хэвийн")).getByText("cp.state.normal")).toBeTruthy();
});

test("a deleted organisation's schedule still names something", async () => {
  api.reportSchedules.mockResolvedValue({ schedules: [aSchedule({ tenant_name: "" })] });

  render(<Schedules />);

  expect(await screen.findByText("cp.state.deleted")).toBeTruthy();
});

test("a backup nobody has ever restored from is not shown as a backup that works", async () => {
  api.backups.mockResolvedValue({
    backups: [aBackup()],
    status: {
      configured: true,
      last_backup_at: "2026-08-29T02:00:00+08:00",
      last_size_mb: 512,
      last_ok: true,
      last_detail: "",
      last_restore_test_at: null,
    },
  });

  render(<Backups />);

  const never = await screen.findByText("cp.state.never");
  // An untested backup is not a backup, so the tile says so in the colour the
  // rest of the console uses for "this needs attention".
  expect(never.className).toContain("text-amber-700");
  expect(screen.getAllByText("512.0 MB").length).toBeGreaterThan(0);
});

test("a deployment with no backup configured is told, rather than shown an empty history", async () => {
  api.backups.mockResolvedValue({
    backups: [],
    status: {
      configured: false,
      last_backup_at: null,
      last_size_mb: 0,
      last_ok: false,
      last_detail: "",
      last_restore_test_at: null,
    },
  });

  render(<Backups />);

  expect(await screen.findByText("cp.message.no_backups_configured")).toBeTruthy();
  expect(screen.getByText("cp.message.no_backups")).toBeTruthy();
});

test("recording a restore test asks why and writes what the operator said", async () => {
  api.backups.mockResolvedValue({
    backups: [],
    status: {
      configured: true,
      last_backup_at: "2026-08-29T02:00:00+08:00",
      last_size_mb: 512,
      last_ok: true,
      last_detail: "",
      last_restore_test_at: null,
    },
  });
  api.recordRestoreTest.mockResolvedValue({ status: "ok" });
  const person = userEvent.setup();

  render(<Backups />);
  await screen.findByText("cp.section.history");
  await person.click(screen.getByRole("button", { name: "cp.action.record_restore_test" }));

  const dialog = await screen.findByRole("dialog", { name: "cp.action.record_restore_test" });
  await person.type(within(dialog).getByRole("textbox"), "staging дээр сэргээв");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  // Only a person can say a restore worked, so what they typed is both the
  // audit reason and the detail the history shows.
  await waitFor(() =>
    expect(api.recordRestoreTest).toHaveBeenCalledWith("staging дээр сэргээв", "staging дээр сэргээв"),
  );
  // And the history is read again, so the row appears without a refresh.
  expect(api.backups).toHaveBeenCalledTimes(2);
});
