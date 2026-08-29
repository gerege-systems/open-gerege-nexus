/**
 * One organisation, and everything an operator may do to it.
 *
 * Every button here is a lifecycle action on somebody else's data: close them
 * down, ask for them to be deleted, look at the platform as one of their
 * people. The server decides all of it and answers 403 whatever this screen
 * draws — so what is asserted is the other half: that the screen does not
 * offer what will be refused, does not offer two contradictory states at once,
 * and never writes without a reason.
 */

import { expect, test, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { signedInAs } from "../helpers/console";

const api = vi.hoisted(() => ({
  tenant: vi.fn(),
  suspend: vi.fn(),
  resume: vi.fn(),
  requestDeletion: vi.fn(),
  cancelDeletion: vi.fn(),
  impersonate: vi.fn(),
  maintenance: vi.fn(),
  setQuota: vi.fn(),
  exportURL: vi.fn(() => "/api/platform/v1/tenants/ten-1/export"),
  stepUp: vi.fn(),
}));

vi.mock("@/lib/i18n", () => import("../helpers/i18n"));
vi.mock("@/components/cp/Console", () => import("../helpers/console"));
vi.mock("@/lib/cp", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/cp")>()),
  cp: api,
}));
vi.mock("next/navigation", () => ({ useParams: () => ({ id: "ten-1" }) }));

import Detail from "@/app/cp/tenants/[id]/page";

function aTenant(overrides: Record<string, unknown> = {}) {
  return {
    id: "ten-1",
    slug: "gerege",
    name: "Гэрэгэ ХХК",
    legal_name: "Гэрэгэ ХХК",
    tax_number: "",
    registration_number: "6012345",
    created_at: "2026-01-10T09:00:00+08:00",
    user_count: 12,
    app_count: 3,
    last_activity_at: "2026-08-29T08:00:00+08:00",
    suspended_at: null,
    suspension_reason: "",
    deletion_scheduled_at: null,
    maintenance_at: null,
    apps: [],
    members: [{ user_id: "usr-1", email: "bat@example.test", name: "Бат Дорж", roles: ["admin"] }],
    activity: [],
    operator_actions: [],
    quota: {
      tenant_id: "ten-1",
      max_users: 50,
      max_storage_mb: null,
      max_ai_calls_monthly: null,
      enforcement: "soft",
      users: 12,
      enforced: ["max_users"],
    },
    impersonations: [],
    ...overrides,
  };
}

test("an auditor is shown the organisation and none of the buttons", async () => {
  signedInAs({ role: "auditor" });
  api.tenant.mockResolvedValue(aTenant());

  render(<Detail />);
  await screen.findByText("Гэрэгэ ХХК");

  // Reads everything, can do nothing — that is the point of the role.
  for (const action of ["cp.action.suspend", "cp.action.delete", "cp.action.quota", "cp.action.impersonate"]) {
    expect(screen.queryByRole("button", { name: action }), action).toBeNull();
  }
});

test("an operator may close an organisation but not ask for it to be deleted", async () => {
  signedInAs({ role: "operator" });
  api.tenant.mockResolvedValue(aTenant());

  render(<Detail />);
  await screen.findByText("Гэрэгэ ХХК");

  expect(screen.getByRole("button", { name: "cp.action.suspend" })).toBeTruthy();
  expect(screen.getByRole("button", { name: "cp.action.quota" })).toBeTruthy();
  // Deletion is a superadmin's request and a different superadmin's approval.
  expect(screen.queryByRole("button", { name: "cp.action.delete" })).toBeNull();
  // Looking inside is support's line to cross, not this role's.
  expect(screen.queryByRole("button", { name: "cp.action.impersonate" })).toBeNull();
});

test("support may look inside and may not touch the lifecycle", async () => {
  signedInAs({ role: "support" });
  api.tenant.mockResolvedValue(aTenant());

  render(<Detail />);
  await screen.findByText("Гэрэгэ ХХК");

  expect(screen.getByRole("button", { name: "cp.action.impersonate" })).toBeTruthy();
  expect(screen.queryByRole("button", { name: "cp.action.suspend" })).toBeNull();
  expect(screen.queryByRole("button", { name: "cp.action.quota" })).toBeNull();
  expect(screen.queryByRole("button", { name: "cp.action.delete" })).toBeNull();
});

test("a closed organisation offers resume rather than suspend", async () => {
  signedInAs({ role: "superadmin" });
  api.tenant.mockResolvedValue(aTenant({ suspended_at: "2026-08-20T10:00:00+08:00" }));

  render(<Detail />);
  await screen.findByText("Гэрэгэ ХХК");

  expect(screen.getByRole("button", { name: "cp.action.resume" })).toBeTruthy();
  expect(screen.queryByRole("button", { name: "cp.action.suspend" })).toBeNull();
  expect(screen.getByText("cp.state.suspended")).toBeTruthy();
  // Nobody signs in there, so there is nobody to be.
  expect(screen.queryByRole("button", { name: "cp.action.impersonate" })).toBeNull();
});

test("an organisation on its way out offers the way back, and not a second deletion", async () => {
  signedInAs({ role: "superadmin" });
  api.tenant.mockResolvedValue(aTenant({ deletion_scheduled_at: "2026-09-28T00:00:00+08:00" }));

  render(<Detail />);
  await screen.findByText("Гэрэгэ ХХК");

  expect(screen.getByRole("button", { name: "cp.action.cancel_deletion" })).toBeTruthy();
  expect(screen.queryByRole("button", { name: "cp.action.delete" })).toBeNull();
  expect(screen.getByText("cp.state.deleting")).toBeTruthy();
});

test("an organisation with nobody in it cannot be looked at as somebody", async () => {
  signedInAs({ role: "superadmin" });
  api.tenant.mockResolvedValue(aTenant({ members: [] }));

  render(<Detail />);
  await screen.findByText("Гэрэгэ ХХК");

  // The button impersonates the first member; with no members it would be a
  // button that reads `undefined.user_id`.
  expect(screen.queryByRole("button", { name: "cp.action.impersonate" })).toBeNull();
});

test("asking for a deletion is a request, and says so before it is sent", async () => {
  signedInAs({ role: "superadmin" });
  api.tenant.mockResolvedValue(aTenant());
  api.requestDeletion.mockResolvedValue({ status: "ok", approval_id: "apr-1", grace_days: 30 });
  const person = userEvent.setup();

  render(<Detail />);
  await screen.findByText("Гэрэгэ ХХК");
  await person.click(screen.getByRole("button", { name: "cp.action.delete" }));

  const dialog = await screen.findByRole("dialog", { name: "cp.action.delete" });
  // What the button does is ask a second superadmin, and the operator pressing
  // it should read that before typing a reason, not afterwards.
  expect(within(dialog).getByText("cp.message.deletion_requested")).toBeTruthy();
  await person.type(within(dialog).getByRole("textbox"), "гэрээ дууссан");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  await waitFor(() => expect(api.requestDeletion).toHaveBeenCalledWith("ten-1", "гэрээ дууссан"));
});

test("stepping inside opens a second window and leaves the console where it is", async () => {
  signedInAs({ role: "support" });
  api.tenant.mockResolvedValue(aTenant());
  api.impersonate.mockResolvedValue({ url: "https://nexus.example.test/impersonate?token=abc" });
  const opened = vi.spyOn(window, "open").mockReturnValue(null);
  const person = userEvent.setup();

  render(<Detail />);
  await screen.findByText("Гэрэгэ ХХК");
  await person.click(screen.getByRole("button", { name: "cp.action.impersonate" }));

  const dialog = await screen.findByRole("dialog", { name: "cp.action.impersonate" });
  await person.type(within(dialog).getByRole("textbox"), "хэрэглэгчийн гомдол");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  await waitFor(() =>
    expect(api.impersonate).toHaveBeenCalledWith("ten-1", "usr-1", "хэрэглэгчийн гомдол"),
  );
  // The operator is about to be two people at once and must not lose the
  // window that can end it — and the new tab cannot reach back into it.
  expect(opened).toHaveBeenCalledWith(
    "https://nexus.example.test/impersonate?token=abc",
    "_blank",
    "noopener",
  );
});

test("an organisation that cannot be read says so instead of rendering half of one", async () => {
  signedInAs({ role: "superadmin" });
  api.tenant.mockRejectedValue(new Error("no such organisation"));

  render(<Detail />);

  expect(await screen.findByText("cp.message.load_failed")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "cp.action.delete" })).toBeNull();
});
