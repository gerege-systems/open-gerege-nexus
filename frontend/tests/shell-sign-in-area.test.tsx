/**
 * What the workspace shell does with somebody who has no session.
 *
 * The rule is asserted next door in public-routes.test.ts; this asserts the
 * behaviour it decides, because that is where the failure was visible. The
 * shell asks `/api/v1/me` on mount and sends a 401 to `/login`, so a screen
 * wrongly counted as private is not merely framed oddly — it is never shown at
 * all, and the flow carrying somebody to it ends where it started.
 */

import { expect, test, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

const route = vi.hoisted(() => ({ pathname: "/", push: vi.fn() }));
const api = vi.hoisted(() => ({
  // Nobody is signed in: every screen behind the door answers 401.
  getMe: vi.fn(async () => {
    throw new Error("unauthorized");
  }),
  getMenus: vi.fn(async () => []),
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

import { ThemeProvider } from "@/lib/theme";
import Layout from "@/components/Layout";

function shellAt(pathname: string) {
  route.pathname = pathname;
  api.getMe.mockClear();
  route.push.mockClear();
  return render(
    <ThemeProvider>
      <Layout>
        <p>дэлгэцийн агуулга</p>
      </Layout>
    </ThemeProvider>,
  );
}

/** The shell has settled when it has either drawn the page or navigated away. */
async function settled() {
  await waitFor(() => expect(screen.queryByText("дэлгэцийн агуулга") || route.push.mock.calls.length).toBeTruthy());
}

test("a first sign-in from Google reaches the screen it was sent to", async () => {
  // Google's callback redirects here with the parked identity. This is the
  // regression: the shell used to ask who was signing in, take the 401 and push
  // back to /login — which reads as a Google button that does nothing.
  shellAt("/login/bind");
  await settled();

  expect(route.push).not.toHaveBeenCalled();
  expect(api.getMe).not.toHaveBeenCalled();
  expect(screen.getByText("дэлгэцийн агуулга")).toBeTruthy();
});

test("an invitation link reaches the password screen", async () => {
  shellAt("/login/set-password");
  await settled();

  expect(route.push).not.toHaveBeenCalled();
  expect(api.getMe).not.toHaveBeenCalled();
});

test("a handover is spent rather than sent to the sign-in screen", async () => {
  // The session does not exist yet; spending the token is what creates it.
  shellAt("/impersonate");
  await settled();

  expect(route.push).not.toHaveBeenCalled();
  expect(api.getMe).not.toHaveBeenCalled();
});

test("a screen behind the door still sends a stranger to sign in", async () => {
  shellAt("/profile");

  await waitFor(() => expect(route.push).toHaveBeenCalledWith("/login"));
  expect(api.getMe).toHaveBeenCalled();
});
