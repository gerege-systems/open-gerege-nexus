/**
 * The two read-first screens: the list of organisations, and the trail of what
 * operators have done.
 *
 * Neither writes anything, which is exactly why they are easy to get subtly
 * wrong and hard to notice: a search that fires per keystroke, a "nothing
 * found" that means "not asked yet", a draft filter that searches before the
 * operator pressed the button. All three read as working software.
 */

import { expect, test, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { signedInAs } from "../helpers/console";

const api = vi.hoisted(() => ({
  tenants: vi.fn(),
  createTenant: vi.fn(),
  audit: vi.fn(),
  stepUp: vi.fn(),
}));

vi.mock("@/lib/i18n", () => import("../helpers/i18n"));
vi.mock("@/components/cp/Console", () => import("../helpers/console"));
vi.mock("@/lib/cp", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/cp")>()),
  cp: api,
}));

import Tenants from "@/app/cp/tenants/page";
import AuditTrail from "@/app/cp/audit/page";

function aTenantSummary(overrides: Record<string, unknown> = {}) {
  return {
    id: "ten-1",
    slug: "gerege",
    name: "Гэрэгэ ХХК",
    registration_number: "6012345",
    created_at: "2026-01-10T09:00:00+08:00",
    user_count: 12,
    app_count: 3,
    last_activity_at: "2026-08-29T08:00:00+08:00",
    suspended_at: null,
    suspension_reason: "",
    deletion_scheduled_at: null,
    ...overrides,
  };
}

const ENTRY = {
  id: "aud-1",
  operator_id: "op-1",
  operator_email: "operator@example.test",
  action: "tenant.suspend",
  target_type: "tenant",
  target_id: "ten-1",
  reason: "төлбөр хоцорсон",
  before: { suspended_at: null },
  after: { suspended_at: "2026-08-20T10:00:00+08:00" },
  ip: "10.0.0.4",
  created_at: "2026-08-20T10:00:00+08:00",
};

test("an auditor may read the organisations and not open one", async () => {
  signedInAs({ role: "auditor" });
  api.tenants.mockResolvedValue({ tenants: [aTenantSummary()] });

  render(<Tenants />);
  await screen.findByText("Гэрэгэ ХХК");

  expect(screen.queryByRole("button", { name: "cp.action.new_tenant" })).toBeNull();
});

test("support may read the organisations and not open one either", async () => {
  signedInAs({ role: "support" });
  api.tenants.mockResolvedValue({ tenants: [aTenantSummary()] });

  render(<Tenants />);
  await screen.findByText("Гэрэгэ ХХК");

  // Support's remit is the people in an organisation, not the list of them.
  expect(screen.queryByRole("button", { name: "cp.action.new_tenant" })).toBeNull();
});

test("an operator may open an organisation", async () => {
  signedInAs({ role: "operator" });
  api.tenants.mockResolvedValue({ tenants: [] });

  render(<Tenants />);
  await waitFor(() => expect(api.tenants).toHaveBeenCalled());

  expect(screen.getByRole("button", { name: "cp.action.new_tenant" })).toBeTruthy();
});

test("typing a registration number is one query, not eleven", async () => {
  signedInAs({ role: "operator" });
  api.tenants.mockResolvedValue({ tenants: [] });
  const person = userEvent.setup();

  render(<Tenants />);
  await waitFor(() => expect(api.tenants).toHaveBeenCalled());
  await person.type(screen.getByPlaceholderText("cp.field.search"), "6012345");

  await waitFor(() => expect(api.tenants).toHaveBeenLastCalledWith("6012345"));
  expect(api.tenants.mock.calls.length).toBeLessThan(3);
});

test("the three states an organisation can be in read as three things", async () => {
  signedInAs({ role: "superadmin" });
  api.tenants.mockResolvedValue({
    tenants: [
      aTenantSummary({ id: "a", name: "Идэвхтэй" }),
      aTenantSummary({ id: "b", name: "Түдгэлзсэн", suspended_at: "2026-08-20T10:00:00+08:00" }),
      aTenantSummary({
        id: "c",
        name: "Устгагдах",
        suspended_at: "2026-08-20T10:00:00+08:00",
        deletion_scheduled_at: "2026-09-20T10:00:00+08:00",
      }),
    ],
  });

  render(<Tenants />);
  await screen.findByText("Идэвхтэй");

  expect(within(screen.getByText("Идэвхтэй").closest("tr")!).getByText("cp.state.active")).toBeTruthy();
  expect(within(screen.getByText("Түдгэлзсэн").closest("tr")!).getByText("cp.state.suspended")).toBeTruthy();
  // On its way out and closed at the same time. The one that matters is the
  // one that is about to be irreversible.
  const going = within(screen.getByText("Устгагдах").closest("tr")!);
  expect(going.getByText("cp.state.deleting")).toBeTruthy();
  expect(going.queryByText("cp.state.suspended")).toBeNull();
});

test("an organisation nobody has used yet says so rather than showing a blank", async () => {
  signedInAs({ role: "superadmin" });
  api.tenants.mockResolvedValue({ tenants: [aTenantSummary({ last_activity_at: null })] });

  render(<Tenants />);
  await screen.findByText("Гэрэгэ ХХК");

  expect(screen.getByText("cp.message.never")).toBeTruthy();
});

test("the audit trail asks with the filters the operator submitted, not the ones being typed", async () => {
  api.audit.mockResolvedValue({ entries: [ENTRY] });
  const person = userEvent.setup();

  render(<AuditTrail />);
  await screen.findByText("төлбөр хоцорсон");
  expect(api.audit).toHaveBeenCalledWith({ action: "", target_type: "", target_id: "" });

  await person.type(screen.getByLabelText("cp.field.action"), "tenant.delete");
  // Typed, not submitted: a filter that searched on every keystroke would put
  // one query per letter across the whole deployment's audit table.
  expect(api.audit).toHaveBeenCalledTimes(1);

  await person.click(screen.getByRole("button", { name: "cp.action.search" }));

  await waitFor(() =>
    expect(api.audit).toHaveBeenLastCalledWith({ action: "tenant.delete", target_type: "", target_id: "" }),
  );
});

test("clearing the filters asks again for everything", async () => {
  api.audit.mockResolvedValue({ entries: [] });
  const person = userEvent.setup();

  render(<AuditTrail />);
  await screen.findByText("cp.audit.empty");

  await person.type(screen.getByLabelText("cp.field.target_id"), "ten-1");
  await person.click(screen.getByRole("button", { name: "cp.action.search" }));
  await waitFor(() => expect(api.audit).toHaveBeenLastCalledWith({ action: "", target_type: "", target_id: "ten-1" }));

  await person.click(screen.getByRole("button", { name: "cp.action.clear" }));

  await waitFor(() =>
    expect(api.audit).toHaveBeenLastCalledWith({ action: "", target_type: "", target_id: "" }),
  );
  // And the boxes are empty, or the next search runs with filters the screen
  // shows and does not hold.
  expect(screen.getByLabelText("cp.field.target_id")).toHaveProperty("value", "");
});

test("an empty trail and an unread trail are not the same screen", async () => {
  let answer: (value: { entries: unknown[] }) => void = () => {};
  api.audit.mockReturnValue(new Promise((resolve) => (answer = resolve)));

  render(<AuditTrail />);

  // Nothing has come back yet, so the screen must not say the trail is empty —
  // an append-only record that reads as empty is the alarming version of
  // "still loading".
  expect(screen.queryByText("cp.audit.empty")).toBeNull();

  answer({ entries: [] });
  expect(await screen.findByText("cp.audit.empty")).toBeTruthy();
});
