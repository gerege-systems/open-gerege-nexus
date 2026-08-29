import { expect, test, type Page, type Route } from "@playwright/test";

/**
 * The operator console, in a browser, from the door inwards.
 *
 * What is asserted here is what only a browser can answer: that the console's
 * hostname serves the console and nothing else, that signing in leads
 * somewhere, and that walking between screens does not tear the frame down and
 * build it again — the bug fixed on 2026-08-29, which no component test can
 * see because it is a fact about routing.
 *
 * The API is stubbed in the page. These tests are about the frontend; what the
 * API does with an operator is the backend suite's subject, and is tested
 * there against a real database.
 */

const OPERATOR = {
  id: "op-me",
  email: "operator@example.test",
  name: "Мөнх Оператор",
  role: "superadmin",
};

const CREATED = {
  id: "op-new",
  email: "shine@example.test",
  name: "Шинэ Оператор",
  role: "operator",
  secret: "JBSWY3DPEHPK3PXP",
  uri: "otpauth://totp/Nexus:shine@example.test?secret=JBSWY3DPEHPK3PXP",
  password: "correct-horse-battery-staple",
};

const json = (route: Route, body: unknown, status = 200) =>
  route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });

/**
 * The console's API, as far as these journeys need it.
 *
 * `signedIn` starts false so the first screen is the door; the sign-in call
 * flips it, which is what makes the form's success worth asserting rather than
 * assumed. `seen` counts what was asked for, so a test can say "asked once".
 */
async function stubApi(page: Page, { signedIn = false } = {}) {
  const state = { signedIn, seen: [] as string[] };
  // The list the console reads back, which a POST appends to. Typed loosely on
  // purpose: it is an API answer, not a value this suite computes with.
  let operators: Record<string, unknown>[] = [
    {
      ...OPERATOR,
      disabled_at: null,
      last_login_at: "2026-08-28T09:15:00+08:00",
      created_at: "2026-08-01T09:00:00+08:00",
      enrolled: true,
    },
  ];

  await page.route("**/api/platform/v1/**", async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace("/api/platform/v1", "");
    const method = route.request().method();
    state.seen.push(`${method} ${path}`);

    if (path === "/me" && method === "GET") {
      if (!state.signedIn) return json(route, { error: "unauthorized" }, 401);
      return json(route, { operator: OPERATOR, expires_at: "2026-08-29T20:00:00+08:00", stepped_up: true });
    }
    if (path === "/session" && method === "POST") {
      state.signedIn = true;
      return json(route, { operator: OPERATOR, expires_at: "2026-08-29T20:00:00+08:00" });
    }
    if (path === "/operators" && method === "GET") return json(route, { operators });
    if (path === "/operators" && method === "POST") {
      operators = [
        ...operators,
        {
          ...CREATED,
          disabled_at: null,
          last_login_at: null,
          created_at: "2026-08-29T10:00:00+08:00",
          enrolled: false,
        },
      ];
      return json(route, CREATED);
    }
    if (path.endsWith("/enrolment") && method === "POST") return json(route, { status: "ok" });

    // Every other screen the frame may reach, in the shape it expects. A
    // screen handed a half-answer throws, and the failure then reads as
    // "the button was not found" three tests away from the cause.
    return json(route, {
      operators: [],
      tenants: [],
      entries: [],
      schedules: [],
      prompts: [],
      knowledge: [],
      settings: [],
      announcements: [],
      approvals: [],
      flags: [],
      monitoring: false,
      grafana_url: "",
      api: { requests_per_second: 0, error_rate: 0, p95_seconds: 0, read: false },
      external: [],
      infra: [],
      alerts: [],
      background: [],
      tenant_trouble: [],
      backups: {
        configured: false,
        last_backup_at: null,
        last_size_mb: 0,
        last_ok: false,
        last_detail: "",
        last_restore_test_at: null,
      },
      catalog: { last_sync_at: null, ok: true, detail: "", apps: [] },
      version: { platform: "test", release: "test", migration: 0, migration_applied_at: null },
      warnings: [],
    });
  });

  return state;
}

/** The door: three fields, and the only submit button on the screen. */
async function signIn(page: Page) {
  await page.goto("/cp");
  await page.locator('input[autocomplete="username"]').fill(OPERATOR.email);
  await page.locator('input[autocomplete="current-password"]').fill("hunter2");
  await page.locator('input[autocomplete="one-time-code"]').fill("123456");
  await page.locator("form button[type=submit]").click();
  await expect(page.getByText(OPERATOR.name).first()).toBeVisible();
}

test("the console's front door leads to the console", async ({ page }) => {
  await stubApi(page);

  await page.goto("/");

  await expect(page).toHaveURL(/\/cp$/);
});

test("a tenant screen is not served on the console's hostname", async ({ page }) => {
  await stubApi(page);

  const answer = await page.goto("/login");

  // The console is reached over a restricted route; anything else served there
  // is a tenant screen behind that restriction, which is not what it is for.
  expect(answer?.status()).toBe(404);
});

test("nobody signed in is shown the door, and signing in opens it", async ({ page }) => {
  const api = await stubApi(page);

  await page.goto("/cp");
  await expect(page.locator('input[autocomplete="one-time-code"]')).toBeVisible();

  await signIn(page);

  expect(api.seen).toContain("POST /session");
});

test("walking between console screens does not rebuild the frame", async ({ page }) => {
  const api = await stubApi(page, { signedIn: true });

  await page.goto("/cp");
  await expect(page.getByText(OPERATOR.name).first()).toBeVisible();
  const asked = api.seen.filter((call) => call === "GET /me").length;

  // Three screens, two of them in the other console app, all reached the way
  // an operator reaches them: from the menu.
  // Located by address rather than by label: these are the menu's own links,
  // and which words are on them is the translators' business.
  await page.locator('aside a[href="/cp/operators"]').click();
  await expect(page).toHaveURL(/\/cp\/operators$/);
  await page.locator('aside a[href="/cp/audit"]').click();
  await expect(page).toHaveURL(/\/cp\/audit$/);

  // The frame is a layout, so it survives every route beneath it: the session
  // is read once, not once per screen. When it lived inside each page this
  // count went up with every click and the shell flashed its loading state.
  expect(api.seen.filter((call) => call === "GET /me")).toHaveLength(asked);
});

test("an operator is added, and the handover is shown exactly once", async ({ page }) => {
  await stubApi(page, { signedIn: true });

  await page.goto("/cp/operators");
  await expect(page.getByText(OPERATOR.email)).toBeVisible();

  // cp.action.add_operator. The two buttons named by their words are the two
  // this journey is about; everything else is reached by address or by role.
  await page.getByRole("button", { name: "Оператор нэмэх" }).click();
  // The three fields this journey types into, by their labels. Filled by
  // position at first, which broke the day the dialog grew a registration
  // search above them: a position is a claim about every field before the one
  // meant, and this dialog gains fields. The label is the thing being typed
  // into.
  const form = page.getByRole("dialog");
  await form.getByLabel("И-мэйл").fill(CREATED.email);
  await form.getByLabel("Нэр").fill(CREATED.name);
  await form.getByLabel("Шалтгаан").fill("шинэ ажилтан");
  await form.getByRole("button", { name: "Оператор нэмэх" }).click();

  // The one moment these values exist: nothing on the server can show them
  // again, and the QR code is what the new operator's authenticator reads.
  await expect(page.getByText(CREATED.password)).toBeVisible();
  await expect(page.getByText(CREATED.secret)).toBeVisible();
  await expect(page.locator("svg[height='148']")).toBeVisible();

  await page.getByRole("dialog").locator('input[inputmode="numeric"]').fill("123456");
  await page.getByRole("dialog").locator("form button[type=submit]").click();

  // Closed, and not offered again: the new account is in the list, unable to
  // sign in until its enrolment is confirmed.
  await page.getByRole("dialog").getByRole("button").last().click();
  await expect(page.getByText(CREATED.password)).toHaveCount(0);
  await expect(page.getByText(CREATED.email)).toBeVisible();
});
