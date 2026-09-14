/**
 * The mobile drawer opens.
 *
 * On a phone the drawer is the only way to an app's menu: the bottom tab bar
 * switches apps, but the groups and screens of the current one live behind the
 * hamburger. On 2026-08-10 a stylesheet change quietly parked the drawer off
 * screen with its "open" class applied, and nothing caught it for a fortnight —
 * there was no test that opened the menu at a phone's width, and by eye it
 * read as "this app has no menu" rather than as a control that does not work.
 *
 * The drawer is a design-system Sheet now, so there is no stylesheet rule to
 * weigh; what is asserted is the behaviour itself. Below `lg` the hamburger
 * opens a dialog that carries the current panel's screens, and inside the
 * native shell — where the panel is part of the layout, never an overlay —
 * there is no hamburger to press.
 */

import { beforeEach, expect, test, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const route = vi.hoisted(() => ({ pathname: "/documents", push: vi.fn() }));
const shell = vi.hoisted(() => ({ inShell: false }));
const api = vi.hoisted(() => ({
  getMe: vi.fn(async () => ({ id: "u-1", name: "Батболд", tenant_id: "t-1", tenant_name: "Гэрэгэ", workspace_kind: "organisation" })),
  getMenus: vi.fn(async () => [
    { id: "documents_modules", app_id: "io.gerege.documents", app_name: "Баримт", label: "Баримт", icon: "file-text", order: 10, app_order: 10 },
    { id: "documents_inbox", app_id: "io.gerege.documents", app_name: "Баримт", parent_id: "documents_modules", label: "Ирсэн", path: "/documents", icon: "inbox", order: 10, app_order: 10 },
    { id: "documents_contracts", app_id: "io.gerege.documents", app_name: "Баримт", parent_id: "documents_modules", label: "Гэрээ", path: "/documents/contracts", icon: "file-text", order: 20, app_order: 10 },
  ]),
}));

vi.mock("@/lib/i18n", () => import("./helpers/i18n"));
vi.mock("next/navigation", () => ({
  usePathname: () => route.pathname,
  useRouter: () => ({ push: route.push, replace: vi.fn(), refresh: vi.fn(), back: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("@/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
  api,
}));
vi.mock("@/lib/shell", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/shell")>()),
  useShell: () => ({ shell: null, inShell: shell.inShell }),
}));

import { TooltipProvider } from "@gerege-systems/ui";
import { ThemeProvider } from "@/lib/theme";
import Layout from "@/components/Layout";

beforeEach(() => {
  shell.inShell = false;
  route.pathname = "/documents";
  // A phone: nothing is 1024px wide. jsdom has no viewport, and the shell asks
  // this to decide whether the menu button folds the panel or opens the drawer.
  window.matchMedia = (query: string) =>
    ({ matches: !query.includes("1024"), media: query, onchange: null, addListener: () => {}, removeListener: () => {}, addEventListener: () => {}, removeEventListener: () => {}, dispatchEvent: () => false }) as MediaQueryList;
});

function shellOnAPhone() {
  return render(
    <ThemeProvider>
      <TooltipProvider>
        <Layout>
          <p>дэлгэцийн агуулга</p>
        </Layout>
      </TooltipProvider>
    </ThemeProvider>,
  );
}

test("the hamburger opens a drawer that carries the current app's screens", async () => {
  shellOnAPhone();
  await screen.findByText("дэлгэцийн агуулга");

  // Shut: no dialog, and the button that opens it says so.
  expect(screen.queryByRole("dialog")).toBeNull();
  const [hamburger] = screen.getAllByRole("button", { name: "web.action.toggle_menu" });
  expect(hamburger.getAttribute("aria-expanded")).toBe("false");

  await userEvent.click(hamburger);

  // Named after the app whose panel it is — its first screen's label, as the
  // rail names it.
  const drawer = await screen.findByRole("dialog", { name: "Ирсэн" });
  expect(within(drawer).getByRole("link", { name: /Гэрээ/ }).getAttribute("href")).toBe("/documents/contracts");
  await waitFor(() => expect(hamburger.getAttribute("aria-expanded")).toBe("true"));

  // Escape closes it, as it does every other layer in the shell.
  await userEvent.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
});

test("off every app's route the drawer carries the platform's own menu", async () => {
  route.pathname = "/settings/appearance";
  shellOnAPhone();
  await screen.findByText("дэлгэцийн агуулга");

  await userEvent.click(screen.getAllByRole("button", { name: "web.action.toggle_menu" })[0]);

  const drawer = await screen.findByRole("dialog", { name: "web.label.platform" });
  expect(within(drawer).getByRole("link", { name: /web\.menu\.appearance/ }).getAttribute("href")).toBe("/settings/appearance");
  expect(within(drawer).getByRole("link", { name: /web\.menu\.app_store/ }).getAttribute("href")).toBe("/apps");
});

test("choosing a screen inside the drawer closes it", async () => {
  shellOnAPhone();
  await screen.findByText("дэлгэцийн агуулга");
  await userEvent.click(screen.getAllByRole("button", { name: "web.action.toggle_menu" })[0]);
  const drawer = await screen.findByRole("dialog", { name: "Ирсэн" });

  // jsdom cannot navigate and says so loudly when a link is followed; the
  // shell's own handler runs regardless, and it is the one under test.
  document.addEventListener("click", (event) => event.preventDefault(), { capture: true, once: true });
  await userEvent.click(within(drawer).getByRole("link", { name: /Гэрээ/ }));

  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
});

test("inside the native shell the panel is part of the layout, never a drawer", async () => {
  // The workarea is the native client's own chrome: it draws no header, so
  // there is no hamburger, and the rail and panel are simply on screen.
  shell.inShell = true;
  shellOnAPhone();
  await screen.findByText("дэлгэцийн агуулга");

  expect(screen.queryByRole("button", { name: "web.action.toggle_menu" })).toBeNull();
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(screen.getByRole("link", { name: /Гэрээ/ }).getAttribute("href")).toBe("/documents/contracts");
});
