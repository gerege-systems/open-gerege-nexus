/**
 * What the address bar is for.
 *
 * A filter, a search term and a page number describe what the reader is
 * looking at, so they belong in the URL and not in a component's memory. Held
 * in `useState` they lose every one of these: refresh loses the filter, Back
 * leaves the screen instead of undoing it, and a link sent to a colleague
 * opens a different list from the one being discussed. Each test below is one
 * of those three.
 */

import { expect, test, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { signedInAs } from "./helpers/console";

const api = vi.hoisted(() => ({ audit: vi.fn(), stepUp: vi.fn() }));

vi.mock("@/lib/i18n", () => import("./helpers/i18n"));
vi.mock("next/navigation", () => import("./helpers/navigation"));
vi.mock("@/components/cp/Console", () => import("./helpers/console"));
vi.mock("@/lib/cp", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/cp")>()),
  cp: api,
}));

import AuditTrail from "@/app/cp/audit/page";

test("a filter the operator applied is written into the address", async () => {
  signedInAs({ role: "superadmin" });
  api.audit.mockResolvedValue({ entries: [] });
  const person = userEvent.setup();

  render(<AuditTrail />);
  await screen.findByText("cp.audit.empty");

  await person.type(screen.getByLabelText("cp.field.target_id"), "ten-1");
  await person.click(screen.getByRole("button", { name: "cp.action.search" }));

  await waitFor(() => expect(window.location.search).toContain("target_id=ten-1"));
});

test("a link carrying a filter opens on that filter, not on everything", async () => {
  signedInAs({ role: "superadmin" });
  api.audit.mockResolvedValue({ entries: [] });
  // What arriving from a pasted link looks like: the query is there before the
  // screen mounts, and nothing has been typed.
  window.history.replaceState(null, "", "/cp/audit?action=tenant.suspend");

  render(<AuditTrail />);

  await waitFor(() =>
    expect(api.audit).toHaveBeenLastCalledWith({
      action: "tenant.suspend",
      target_type: "",
      target_id: "",
    }),
  );
  // And the box shows it, so the reader can see why the list is short.
  expect(screen.getByLabelText("cp.field.action")).toHaveProperty("value", "tenant.suspend");
});

test("clearing the filters takes them back out of the address", async () => {
  signedInAs({ role: "superadmin" });
  api.audit.mockResolvedValue({ entries: [] });
  window.history.replaceState(null, "", "/cp/audit?target_id=ten-1");
  const person = userEvent.setup();

  render(<AuditTrail />);
  await waitFor(() => expect(api.audit).toHaveBeenCalled());

  await person.click(screen.getByRole("button", { name: "cp.action.clear" }));

  // A default is an absence, not an empty value: `?target_id=` in a shared
  // link would say a filter is set when none is.
  await waitFor(() => expect(window.location.search).toBe(""));
});

test("an empty list says which kind of empty it is", async () => {
  signedInAs({ role: "superadmin" });
  api.audit.mockResolvedValue({ entries: [] });

  const { unmount } = render(<AuditTrail />);
  // Nothing filtered: the trail itself has no entries.
  expect(await screen.findByText("cp.audit.empty")).toBeTruthy();
  unmount();

  window.history.replaceState(null, "", "/cp/audit?action=tenant.suspend");
  render(<AuditTrail />);

  // Filtered: the entries exist, this operator hid them. Telling them nothing
  // has ever been recorded would be false.
  expect(await screen.findByText("cp.audit.empty_filtered")).toBeTruthy();
});
