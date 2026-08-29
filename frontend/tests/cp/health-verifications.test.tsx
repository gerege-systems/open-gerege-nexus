/**
 * The console's front page, and the ledger of what the platform has emailed.
 *
 * The front page is the screen somebody opens to answer "is anything wrong
 * anywhere", so what it must never do is answer "no" from numbers it does not
 * have. The ledger is read-only and belongs to the platform rather than to any
 * one organisation — its three states have to stay three, because a pending
 * verification and an expired one are different problems.
 */

import { expect, test, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { anOverview } from "../helpers/fixtures";
import { signedInAs } from "../helpers/console";

const api = vi.hoisted(() => ({
  health: vi.fn(),
  deploy: vi.fn(),
  recordRestoreTest: vi.fn(),
  syncCatalog: vi.fn(),
  verifications: vi.fn(),
  stepUp: vi.fn(),
}));

vi.mock("@/lib/i18n", () => import("../helpers/i18n"));
vi.mock("@/components/cp/Console", () => import("../helpers/console"));
vi.mock("@/lib/cp", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/cp")>()),
  cp: api,
}));

import Health from "@/app/cp/page";
import Verifications from "@/app/cp/email-verification/page";

const LEDGER = {
  stats: { total: 40, verified: 30, pending: 6, expired: 4, last_24h: 5, verified_pct: 75, tenants: 3 },
  recent: [
    {
      id: "ver-1",
      tenant_id: "ten-1",
      tenant_name: "Гэрэгэ ХХК",
      source: "signup",
      purpose: "verify",
      email: "bat@example.test",
      status: "PENDING" as const,
      created_at: "2026-08-29T08:00:00+08:00",
      verified_at: null,
    },
  ],
  service: {
    configured: true,
    reachable: true,
    provider_url: "https://verify.example.test",
    admin_url: "https://verify.example.test/admin",
  },
};

test("only a superadmin is offered the button that puts a tag into production", async () => {
  signedInAs({ role: "operator" });
  api.health.mockResolvedValue(anOverview());

  const { unmount } = render(<Health />);
  await screen.findByText("cp.section.health");
  // A console that can deploy is a console that can put any tag into
  // production; that is the whole supply chain in one button.
  expect(screen.queryByRole("button", { name: "cp.action.deploy" })).toBeNull();
  unmount();

  signedInAs({ role: "superadmin" });
  render(<Health />);
  await screen.findByText("cp.section.health");
  expect(screen.getByRole("button", { name: "cp.action.deploy" })).toBeTruthy();
});

test("deploying is destructive, asks why, and opens the run in its own tab", async () => {
  signedInAs({ role: "superadmin" });
  api.health.mockResolvedValue(anOverview());
  api.deploy.mockResolvedValue({ url: "https://github.example.test/runs/1" });
  const opened = vi.spyOn(window, "open").mockReturnValue(null);
  const person = userEvent.setup();

  render(<Health />);
  await screen.findByText("cp.section.health");
  await person.click(screen.getByRole("button", { name: "cp.action.deploy" }));

  const dialog = await screen.findByRole("dialog", { name: "cp.action.deploy" });
  await person.type(within(dialog).getByRole("textbox"), "хувилбар гаргах");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  await waitFor(() => expect(api.deploy).toHaveBeenCalledWith("main", "хувилбар гаргах"));
  expect(opened).toHaveBeenCalledWith("https://github.example.test/runs/1", "_blank", "noopener");
});

test("with no monitoring the front page shows no numbers at all", async () => {
  signedInAs({ role: "superadmin" });
  api.health.mockResolvedValue(anOverview({ monitoring: false }));

  render(<Health />);

  expect(await screen.findByText("cp.message.no_monitoring")).toBeTruthy();
  // Not three dashes and not three zeroes: the tiles are not drawn, because
  // this is the screen somebody checks before going back to bed.
  expect(screen.queryByText("cp.stat.rps")).toBeNull();
});

test("a configuration warning is said in the deployment's own words", async () => {
  signedInAs({ role: "superadmin" });
  api.health.mockResolvedValue(anOverview({ warnings: ["APPSTORE_SIGNING_KEY is not set"] }));

  render(<Health />);

  expect(await screen.findByText("APPSTORE_SIGNING_KEY is not set")).toBeTruthy();
});

test("a backup nobody has restored from is named as such on the front page", async () => {
  signedInAs({ role: "superadmin" });
  api.health.mockResolvedValue(
    anOverview({
      backups: {
        configured: true,
        last_backup_at: "2026-08-29T02:00:00+08:00",
        last_size_mb: 512,
        last_ok: true,
        last_detail: "",
        last_restore_test_at: null,
      },
    }),
  );

  render(<Health />);

  expect(await screen.findByText("cp.message.never_tested")).toBeTruthy();
});

test("the verification service's three states are three different sentences", async () => {
  api.verifications.mockResolvedValue({
    ...LEDGER,
    service: { ...LEDGER.service, configured: false, reachable: false },
  });
  const { unmount } = render(<Verifications />);
  expect(await screen.findByText("emailverify.message.not_configured")).toBeTruthy();
  unmount();

  api.verifications.mockResolvedValue({
    ...LEDGER,
    // Configured and refusing: what it said is the only useful thing on the
    // screen, so it is what is shown rather than a generic "unreachable".
    service: { ...LEDGER.service, reachable: false, health: "401 from the provider" },
  });
  const second = render(<Verifications />);
  expect(await screen.findByText("401 from the provider")).toBeTruthy();
  second.unmount();

  api.verifications.mockResolvedValue(LEDGER);
  render(<Verifications />);
  expect(await screen.findByText("emailverify.message.reachable")).toBeTruthy();
});

test("a verification's state is the state, not the time it was written", async () => {
  api.verifications.mockResolvedValue({
    ...LEDGER,
    recent: [
      { ...LEDGER.recent[0], id: "a", status: "PENDING" as const },
      { ...LEDGER.recent[0], id: "b", status: "VERIFIED" as const, verified_at: "2026-08-29T09:00:00+08:00" },
      { ...LEDGER.recent[0], id: "c", status: "EXPIRED" as const },
    ],
  });

  render(<Verifications />);
  await screen.findByText("emailverify.state.verified");

  // Three rows, three states. Collapsing expired into pending would leave a
  // person waiting for a mail that can no longer arrive.
  expect(screen.getByText("emailverify.state.pending")).toBeTruthy();
  expect(screen.getByText("emailverify.state.expired")).toBeTruthy();
});

test("a verification for an organisation that has since gone still names something", async () => {
  api.verifications.mockResolvedValue({
    ...LEDGER,
    recent: [{ ...LEDGER.recent[0], tenant_name: "" }],
  });

  render(<Verifications />);

  // The row stays after the organisation is deleted; an empty cell there reads
  // as a rendering fault rather than as a fact.
  expect(await screen.findByText("cp.state.deleted")).toBeTruthy();
});

test("the ledger is read only, and offers nothing to press", async () => {
  api.verifications.mockResolvedValue(LEDGER);

  render(<Verifications />);
  await screen.findByText("bat@example.test");

  // One organisation's administrator used to be able to edit this; it is the
  // platform's now, and this screen is where an operator reads it.
  const buttons = screen.getAllByRole("button").map((button) => button.textContent);
  expect(buttons).toEqual(["cp.action.refresh"]);
});
