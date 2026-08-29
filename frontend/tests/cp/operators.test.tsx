/**
 * Who may reach the console, and what the screen refuses to let happen.
 *
 * The rules asserted here are the API's — the console cannot grant itself an
 * account, and a superadmin cannot demote the last one — so nothing on this
 * screen is a security boundary; the server refuses all of it again. What the
 * screen owes is that it never *offers* an action the server will refuse, and
 * that the one moment a password and an authenticator secret exist is not
 * quietly thrown away. Both are what this file holds.
 */

import { expect, test, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { anOperator } from "../helpers/fixtures";
import { signedInAs } from "../helpers/console";

const api = vi.hoisted(() => ({
  operators: vi.fn(),
  addOperator: vi.fn(),
  confirmEnrolment: vi.fn(),
  setOperatorEnabled: vi.fn(),
  setOperatorRole: vi.fn(),
  changePassword: vi.fn(),
  stepUp: vi.fn(),
}));

vi.mock("@/lib/i18n", () => import("../helpers/i18n"));
vi.mock("@/components/cp/Console", () => import("../helpers/console"));
vi.mock("@/lib/cp", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/cp")>()),
  cp: api,
}));

import { StepUpRequired } from "@/lib/cp";
import Operators from "@/app/cp/operators/page";

const CREATED = {
  id: "op-new",
  email: "shine@example.test",
  name: "Шинэ Оператор",
  role: "operator",
  secret: "JBSWY3DPEHPK3PXP",
  uri: "otpauth://totp/Nexus:shine@example.test?secret=JBSWY3DPEHPK3PXP",
  password: "correct-horse-battery-staple",
};

function listing(...operators: ReturnType<typeof anOperator>[]) {
  api.operators.mockResolvedValue({ operators });
}

/** The list has loaded when the first operator's address is on the screen. */
async function shown(email: string) {
  return screen.findByText(email);
}

test("a role that cannot add operators is not shown the button", async () => {
  signedInAs({ role: "auditor" });
  listing(anOperator());

  render(<Operators />);
  await shown("bat@example.test");

  expect(screen.queryByRole("button", { name: "cp.action.add_operator" })).toBeNull();
  // Nor the two controls that change somebody else's account.
  expect(screen.queryByRole("combobox", { name: "cp.field.role" })).toBeNull();
  expect(screen.queryByRole("button", { name: "cp.action.disable" })).toBeNull();
  // Their own password is still theirs to change.
  expect(screen.getByRole("button", { name: "cp.action.change_password" })).toBeTruthy();
});

test("a superadmin is offered no way to change or disable their own account", async () => {
  const me = signedInAs({ role: "superadmin" });
  listing(anOperator({ id: me.id, email: me.email, name: me.name, role: "superadmin" }), anOperator());

  render(<Operators />);
  await shown(me.email);

  // One dropdown and one button on the screen, both on the other operator's
  // row: locking yourself out is the mistake this screen must not offer.
  expect(screen.getAllByRole("combobox", { name: "cp.field.role" })).toHaveLength(1);
  expect(screen.getAllByRole("button", { name: "cp.action.disable" })).toHaveLength(1);
  expect(screen.getByText("cp.state.you")).toBeTruthy();
});

test("an account that has not finished enrolling is marked, not counted as normal", async () => {
  signedInAs();
  listing(anOperator({ enrolled: false, last_login_at: null }));

  render(<Operators />);
  await shown("bat@example.test");

  // The account exists and cannot sign in. Showing it as normal is how a
  // half-finished handover is mistaken for a working account.
  expect(screen.getByText("cp.state.enrolment_pending")).toBeTruthy();
  expect(screen.queryByText("cp.state.normal")).toBeNull();
  expect(screen.getByText("cp.state.never")).toBeTruthy();
});

test("a listing that cannot be read says so instead of showing an empty console", async () => {
  signedInAs();
  api.operators.mockRejectedValue(new Error("operator role is missing SELECT"));

  render(<Operators />);

  expect(await screen.findByText("operator role is missing SELECT")).toBeTruthy();
});

test("two different new passwords are refused before the API is called", async () => {
  signedInAs();
  listing(anOperator());
  const person = userEvent.setup();

  render(<Operators />);
  await shown("bat@example.test");
  await person.click(screen.getByRole("button", { name: "cp.action.change_password" }));

  const dialog = screen.getByRole("dialog", { name: "cp.action.change_password" });
  await person.type(within(dialog).getByLabelText("cp.field.current_password"), "one");
  await person.type(within(dialog).getByLabelText("cp.field.new_password"), "two");
  await person.type(within(dialog).getByLabelText("cp.field.repeat_password"), "three");
  await person.click(within(dialog).getByRole("button", { name: "base.action.save" }));

  expect(await within(dialog).findByText("cp.message.passwords_differ")).toBeTruthy();
  expect(api.changePassword).not.toHaveBeenCalled();
});

test("the handover is shown once and the enrolment is finished from it", async () => {
  signedInAs();
  listing(anOperator());
  api.addOperator.mockResolvedValue(CREATED);
  api.confirmEnrolment.mockResolvedValue({ status: "ok" });
  const person = userEvent.setup();

  render(<Operators />);
  await shown("bat@example.test");
  await person.click(screen.getByRole("button", { name: "cp.action.add_operator" }));

  const form = screen.getByRole("dialog", { name: "cp.action.add_operator" });
  await person.type(within(form).getByLabelText("cp.field.email"), CREATED.email);
  await person.type(within(form).getByLabelText("cp.field.name"), CREATED.name);
  await person.type(within(form).getByLabelText("cp.field.reason"), "шинэ ажилтан");
  await person.click(within(form).getByRole("button", { name: "cp.action.add_operator" }));

  expect(api.addOperator).toHaveBeenCalledWith({
    email: CREATED.email,
    name: CREATED.name,
    role: "operator",
    reason: "шинэ ажилтан",
  });

  // The password and the secret exist here and nowhere else on the server.
  const handover = await screen.findByRole("dialog", { name: "cp.view.handover" });
  expect(within(handover).getByText(CREATED.password)).toBeTruthy();
  expect(within(handover).getByText(CREATED.secret)).toBeTruthy();
  expect(within(handover).getByText("cp.message.handover_once")).toBeTruthy();

  await person.type(within(handover).getByRole("textbox"), "123456");
  await person.click(within(handover).getByRole("button", { name: "cp.action.confirm" }));

  await waitFor(() =>
    expect(api.confirmEnrolment).toHaveBeenCalledWith(CREATED.id, "123456", `enrolled ${CREATED.email}`),
  );
  expect(await within(handover).findByText("cp.message.enrolled")).toBeTruthy();
});

test("the enrolment code takes six digits and nothing else", async () => {
  signedInAs();
  listing(anOperator());
  api.addOperator.mockResolvedValue(CREATED);
  const person = userEvent.setup();

  render(<Operators />);
  await shown("bat@example.test");
  await person.click(screen.getByRole("button", { name: "cp.action.add_operator" }));

  // The hint sits inside the label, so the field's accessible name is the
  // label and the hint together.
  const code = within(screen.getByRole("dialog", { name: "cp.action.add_operator" })).getByLabelText(
    /cp\.field\.code/,
  ) as HTMLInputElement;
  await person.type(code, "12ab34cd5678");

  // What a phone shows is six digits, and what a paste of "123 456" carries is
  // a space. Neither should reach the API as part of the code.
  expect(code.value).toBe("123456");
});

test("changing somebody's role asks why, and sends the reason", async () => {
  const me = signedInAs({ role: "superadmin" });
  listing(anOperator({ id: "op-2" }));
  api.setOperatorRole.mockResolvedValue({ status: "ok" });
  const person = userEvent.setup();

  render(<Operators />);
  await shown("bat@example.test");
  await person.selectOptions(screen.getByRole("combobox", { name: "cp.field.role" }), "auditor");

  const dialog = await screen.findByRole("dialog", { name: "cp.action.change_role" });
  expect(api.setOperatorRole).not.toHaveBeenCalled();
  await person.type(within(dialog).getByRole("textbox"), "шилжсэн");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  await waitFor(() => expect(api.setOperatorRole).toHaveBeenCalledWith("op-2", "auditor", "шилжсэн"));
  expect(me.id).not.toBe("op-2");
});

test("a step-up asks for the code and then does what was asked, once", async () => {
  signedInAs({ role: "superadmin" });
  listing(anOperator({ id: "op-2", disabled_at: null }));
  api.setOperatorEnabled.mockRejectedValueOnce(new StepUpRequired()).mockResolvedValueOnce({ status: "ok" });
  api.stepUp.mockResolvedValue({ stepped_up_until: "2026-08-29T18:00:00+08:00" });
  const person = userEvent.setup();

  render(<Operators />);
  await shown("bat@example.test");
  await person.click(screen.getByRole("button", { name: "cp.action.disable" }));

  const dialog = await screen.findByRole("dialog", { name: "cp.action.disable" });
  await person.type(within(dialog).getByRole("textbox"), "гарсан");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  // The refusal is not an error the operator has to read and start again from:
  // the reason they typed is still there, and a code field has appeared.
  const code = await within(dialog).findByLabelText("cp.field.code");
  expect(within(dialog).getByText("cp.message.step_up")).toBeTruthy();

  await person.type(code, "654321");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  await waitFor(() => expect(api.stepUp).toHaveBeenCalledWith("654321"));
  // The step-up happens before the action, so the action is never attempted
  // with a window that has closed — and it is attempted exactly twice: the
  // refusal, and the retry.
  expect(api.setOperatorEnabled).toHaveBeenCalledTimes(2);
  // The second argument is the state asked for, not the state left behind: an
  // account that is not disabled is being disabled, so it is false.
  expect(api.setOperatorEnabled).toHaveBeenLastCalledWith("op-2", false, "гарсан");
});
