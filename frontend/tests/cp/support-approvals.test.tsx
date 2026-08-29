/**
 * The help desk, and the screen where a second person agrees.
 *
 * Both are places where an operator acts on somebody else's account, and both
 * put the same three things in the way: say why, prove yourself if the window
 * has closed, leave a row behind. What is asserted here is that the screens do
 * not skip that — and, on the help desk, that a search box does not put one
 * request on the wire per keystroke.
 */

import { expect, test, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const api = vi.hoisted(() => ({
  people: vi.fn(),
  unlock: vi.fn(),
  revokeSessions: vi.fn(),
  credentialLink: vi.fn(),
  approvals: vi.fn(),
  approve: vi.fn(),
  reject: vi.fn(),
  stepUp: vi.fn(),
}));

vi.mock("@/lib/i18n", () => import("../helpers/i18n"));
vi.mock("@/lib/cp", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/cp")>()),
  cp: api,
}));

import Support from "@/app/cp/support/page";
import Approvals from "@/app/cp/approvals/page";

const PERSON = {
  id: "usr-1",
  email: "bat@example.test",
  name: "Бат Дорж",
  locked_until: null as string | null,
  failed_logins: 0,
  sessions: 2,
  memberships: [
    { tenant_id: "ten-1", tenant_name: "Гэрэгэ ХХК", tenant_slug: "gerege", roles: ["admin"], suspended: false },
  ],
};

const APPROVAL = {
  id: "apr-1",
  action: "tenant.delete",
  target_type: "tenant",
  target_id: "ten-1",
  target_name: "Гэрэгэ ХХК",
  requested_by: "op-2",
  requested_by_name: "Дорж Оператор",
  requested_reason: "гэрээ дууссан",
  requested_at: "2026-08-28T14:00:00+08:00",
  expires_at: "2026-09-04T14:00:00+08:00",
};

test("a search waits for the typing to stop", async () => {
  api.people.mockResolvedValue({ people: [] });
  const person = userEvent.setup();

  render(<Support />);
  await waitFor(() => expect(api.people).toHaveBeenCalled());
  await person.type(screen.getByPlaceholderText("cp.field.email"), "bat");

  await waitFor(() => expect(api.people).toHaveBeenLastCalledWith("bat"));
  // The whole point of the delay: three letters are not three requests, on a
  // screen whose search reads across every organisation on the deployment.
  expect(api.people.mock.calls.length).toBeLessThan(3);
});

test("an empty search says nothing rather than saying nobody was found", async () => {
  api.people.mockResolvedValue({ people: [] });

  render(<Support />);
  await waitFor(() => expect(api.people).toHaveBeenCalled());

  // Before anybody has typed a name, "no such person" is not an answer to a
  // question that was asked.
  expect(screen.queryByText("cp.message.no_people")).toBeNull();
});

test("ending somebody's sessions is destructive and says whose", async () => {
  api.people.mockResolvedValue({ people: [PERSON] });
  api.revokeSessions.mockResolvedValue({ status: "ok" });
  const person = userEvent.setup();

  render(<Support />);
  await screen.findByText(PERSON.email);
  await person.click(screen.getByRole("button", { name: "cp.action.revoke_sessions" }));

  const dialog = await screen.findByRole("dialog", { name: "cp.action.revoke_sessions" });
  expect(within(dialog).getByText(PERSON.email)).toBeTruthy();
  await person.type(within(dialog).getByRole("textbox"), "төхөөрөмж алдагдсан");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  await waitFor(() => expect(api.revokeSessions).toHaveBeenCalledWith("usr-1", "төхөөрөмж алдагдсан"));
});

test("a password link is offered only to somebody an organisation can send it for", async () => {
  api.people.mockResolvedValue({ people: [{ ...PERSON, memberships: [] }] });
  const { unmount } = render(<Support />);
  await screen.findByText(PERSON.email);

  // The verification service counts its quota per organisation, so the mail
  // cannot be sent on behalf of none. Offering the button anyway would be a
  // help-desk action that fails at the server every time.
  expect(screen.queryByRole("button", { name: "cp.action.send_reset" })).toBeNull();
  unmount();

  api.people.mockResolvedValue({ people: [PERSON] });
  api.credentialLink.mockResolvedValue({ status: "ok" });
  const person = userEvent.setup();
  render(<Support />);
  await screen.findByText(PERSON.email);
  await person.click(screen.getByRole("button", { name: "cp.action.send_reset" }));

  const dialog = await screen.findByRole("dialog", { name: "cp.action.send_reset" });
  await person.type(within(dialog).getByRole("textbox"), "нууц үгээ мартсан");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  await waitFor(() =>
    expect(api.credentialLink).toHaveBeenCalledWith("usr-1", {
      tenant_id: "ten-1",
      purpose: "reset",
      reason: "нууц үгээ мартсан",
    }),
  );
});

test("nothing waiting for a second person says so", async () => {
  api.approvals.mockResolvedValue({ approvals: [] });

  render(<Approvals />);

  expect(await screen.findByText("cp.message.no_approvals")).toBeTruthy();
});

test("a request shows who asked, why, and for what", async () => {
  api.approvals.mockResolvedValue({ approvals: [APPROVAL] });

  render(<Approvals />);

  // The second person is agreeing to somebody else's judgement, so what that
  // judgement was has to be on the screen they agree from.
  expect(await screen.findByText("гэрээ дууссан")).toBeTruthy();
  expect(screen.getByText("Дорж Оператор")).toBeTruthy();
  expect(screen.getByRole("link", { name: "Гэрэгэ ХХК" }).getAttribute("href")).toBe("/cp/tenants/ten-1");
});

test("agreeing to a deletion is the destructive half, and still asks why", async () => {
  api.approvals.mockResolvedValue({ approvals: [APPROVAL] });
  api.approve.mockResolvedValue({ status: "ok" });
  const person = userEvent.setup();

  render(<Approvals />);
  await screen.findByText("гэрээ дууссан");
  await person.click(screen.getByRole("button", { name: "cp.action.approve" }));

  const dialog = await screen.findByRole("dialog", { name: "cp.action.approve" });
  expect(api.approve).not.toHaveBeenCalled();
  await person.type(within(dialog).getByRole("textbox"), "шалгасан");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  await waitFor(() => expect(api.approve).toHaveBeenCalledWith("apr-1", "шалгасан"));
  // Read again afterwards: an approval that stays on the screen is one the
  // next operator agrees to a second time.
  expect(api.approvals).toHaveBeenCalledTimes(2);
});

test("refusing a request is not destructive, and is not the same call", async () => {
  api.approvals.mockResolvedValue({ approvals: [APPROVAL] });
  api.reject.mockResolvedValue({ status: "ok" });
  const person = userEvent.setup();

  render(<Approvals />);
  await screen.findByText("гэрээ дууссан");
  await person.click(screen.getByRole("button", { name: "cp.action.reject" }));

  const dialog = await screen.findByRole("dialog", { name: "cp.action.reject" });
  await person.type(within(dialog).getByRole("textbox"), "буруу байгууллага");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  await waitFor(() => expect(api.reject).toHaveBeenCalledWith("apr-1", "буруу байгууллага"));
  expect(api.approve).not.toHaveBeenCalled();
});
