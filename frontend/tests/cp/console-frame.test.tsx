/**
 * The console's frame: one place that decides whether anybody is signed in.
 *
 * It moved into `app/cp/layout.tsx` on 2026-08-29 because it had been inside
 * each page, and every navigation inside the console tore the whole shell down
 * and built it again — the loading state flashed, `cp.me()` was asked on every
 * screen, and the folded menu groups reset. A layout survives between the
 * routes under it, so the tests here render the frame once and change the route
 * beneath it, which is the arrangement the bug could not survive.
 */

import { expect, test, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const api = vi.hoisted(() => ({ me: vi.fn(), signIn: vi.fn(), signOut: vi.fn() }));
const route = vi.hoisted(() => ({ pathname: "/cp", push: vi.fn() }));

vi.mock("@/lib/i18n", () => import("../helpers/i18n"));
vi.mock("@/lib/cp", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/cp")>()),
  cp: api,
}));
vi.mock("next/navigation", () => ({
  usePathname: () => route.pathname,
  useRouter: () => ({ push: route.push, replace: vi.fn(), refresh: vi.fn(), back: vi.fn() }),
}));

import { Unauthorized } from "@/lib/cp";
import { ThemeProvider } from "@/lib/theme";
import Console from "@/components/cp/Console";

const OPERATOR = { id: "op-me", email: "me@example.test", name: "Мөнх Оператор", role: "superadmin" as const };

function frame(pathname = "/cp") {
  route.pathname = pathname;
  return render(
    <ThemeProvider>
      <Console>
        <p>дэлгэцийн агуулга</p>
      </Console>
    </ThemeProvider>,
  );
}

test("nobody signed in is shown the door, not an error page", async () => {
  api.me.mockRejectedValue(new Unauthorized());

  frame();

  expect(await screen.findByText("cp.login.title")).toBeTruthy();
  expect(screen.queryByText("дэлгэцийн агуулга")).toBeNull();
});

test("a session that cannot be read at all is also shown the door", async () => {
  // Anything other than "not signed in" — a proxy's error page, a database the
  // console cannot reach — still has to end somewhere an operator can act.
  // An error screen with no way to sign in from it is a dead end.
  api.me.mockRejectedValue(new Error("bad gateway"));
  const complained = vi.spyOn(console, "error").mockImplementation(() => {});

  frame();

  expect(await screen.findByText("cp.login.title")).toBeTruthy();
  expect(complained).toHaveBeenCalled();
});

test("the door is a landing page, and the form is on it", async () => {
  // The console's front door used to be one card on a pale screen: somebody who
  // arrived at this hostname without a session was told nothing about what was
  // behind it. It is now the platform's own landing page with the form in the
  // hero, so the two have to be true at once — the argument is there, and the
  // form is still reachable without scrolling past it.
  api.me.mockRejectedValue(new Unauthorized());

  frame();

  // The form, unchanged.
  expect(await screen.findByLabelText("cp.field.email")).toBeTruthy();
  expect(screen.getByLabelText("cp.field.code")).toBeTruthy();
  expect(screen.getByRole("button", { name: "cp.action.sign_in" })).toBeTruthy();

  // And the page around it: what the console is, and the three claims it makes.
  expect(screen.getByText("cp.landing.lede")).toBeTruthy();
  expect(screen.getByText("cp.landing.card1_title")).toBeTruthy();
  expect(screen.getByText("cp.landing.card3_title")).toBeTruthy();
  // The five conditions on impersonation are the page's strongest claim, so a
  // silent drop of one of them should fail here rather than in a screenshot.
  for (const claim of ["imp_1", "imp_2", "imp_3", "imp_4", "imp_5"]) {
    expect(screen.getByText(`cp.landing.${claim}`)).toBeTruthy();
  }
});

test("a refused sign-in says one thing and takes the code away", async () => {
  api.me.mockRejectedValue(new Unauthorized());
  api.signIn.mockRejectedValue(new Error("invalid credentials"));
  const person = userEvent.setup();

  frame();
  await screen.findByText("cp.login.title");

  await person.type(screen.getByLabelText("cp.field.email"), "me@example.test");
  await person.type(screen.getByLabelText("cp.field.password"), "hunter2");
  const code = screen.getByLabelText("cp.field.code") as HTMLInputElement;
  await person.type(code, "111111");
  await person.click(screen.getByRole("button", { name: "cp.action.sign_in" }));

  // Which of the three was wrong is deliberately not said — the API does not
  // distinguish them either, and a console that does is an oracle.
  expect(await screen.findByText("cp.login.failed")).toBeTruthy();
  expect(screen.queryByText("invalid credentials")).toBeNull();
  // A one-time code is spent whether or not it worked; leaving it in the box
  // invites a second attempt with a value that cannot succeed.
  expect(code.value).toBe("");
});

test("a signed-in operator gets the page, and the frame asks once", async () => {
  api.me.mockResolvedValue({ operator: OPERATOR, expires_at: "", stepped_up: false });

  frame();

  expect(await screen.findByText("дэлгэцийн агуулга")).toBeTruthy();
  expect(screen.getAllByText(OPERATOR.name).length).toBeGreaterThan(0);
  expect(api.me).toHaveBeenCalledTimes(1);
});

test("the rail lights the app the route belongs to, not the one whose prefix matches", async () => {
  api.me.mockResolvedValue({ operator: OPERATOR, expires_at: "", stepped_up: false });

  frame("/cp/ops/alerts");
  await screen.findByText("дэлгэцийн агуулга");

  // Both "/cp" and "/cp/ops" prefix-match this route. The longest wins, or the
  // console tile lights on every screen in the deployment.
  const ops = screen.getByRole("link", { name: "cp.app.ops" });
  expect(ops.getAttribute("aria-current")).toBe("page");
  // Two links carry that name — the brand mark and the console's own tile —
  // and on this route neither is the page being shown.
  for (const link of screen.getAllByRole("link", { name: "cp.view.title" })) {
    expect(link.getAttribute("aria-current")).toBeNull();
  }
});

test("the front page's entry is exact, so it does not light on every screen", async () => {
  api.me.mockResolvedValue({ operator: OPERATOR, expires_at: "", stepped_up: false });

  // A route inside the console's own app, so the panel holding both entries is
  // the one on screen.
  frame("/cp/audit");
  await screen.findByText("дэлгэцийн агуулга");

  const health = screen.getByRole("link", { name: "cp.section.health" });
  expect(health.getAttribute("aria-current")).toBeNull();
  expect(screen.getByRole("link", { name: "cp.section.audit" }).getAttribute("aria-current")).toBe("page");
});

test("a group folded in a previous session comes back folded", async () => {
  api.me.mockResolvedValue({ operator: OPERATOR, expires_at: "", stepped_up: false });
  localStorage.setItem("gerege_cp_sidebar_groups", JSON.stringify(["cp.group.platform"]));

  frame();
  await screen.findByText("дэлгэцийн агуулга");

  const folded = await screen.findByRole("button", { name: "cp.group.platform", expanded: false });
  // A link folded away is still a link, and Tab must not walk into it.
  expect(document.getElementById(folded.getAttribute("aria-controls")!)?.hasAttribute("inert")).toBe(true);
  expect(screen.getByRole("button", { name: "cp.group.watch", expanded: true })).toBeTruthy();
});

test("half-written storage does not take the console down with it", async () => {
  api.me.mockResolvedValue({ operator: OPERATOR, expires_at: "", stepped_up: false });
  localStorage.setItem("gerege_cp_sidebar_groups", "{not json");

  frame();

  expect(await screen.findByText("дэлгэцийн агуулга")).toBeTruthy();
  expect(screen.getByRole("button", { name: "cp.group.watch", expanded: true })).toBeTruthy();
});

test("search finds a destination in the other app and goes there", async () => {
  api.me.mockResolvedValue({ operator: OPERATOR, expires_at: "", stepped_up: false });
  const person = userEvent.setup();

  frame("/cp");
  await screen.findByText("дэлгэцийн агуулга");

  // The console's own app is showing; the search reaches the whole console,
  // which is the point of it.
  await person.type(screen.getByPlaceholderText("web.view.search_placeholder"), "cp.section.backups");
  await person.click(await screen.findByRole("button", { name: /cp\.section\.backups/ }));

  expect(route.push).toHaveBeenCalledWith("/cp/ops/backups");
});
