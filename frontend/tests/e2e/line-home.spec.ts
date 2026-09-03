import { expect, test, type Page, type Route } from "@playwright/test";

/**
 * Төхөөрөмжийн шугамын нүүр, жинхэнэ хөтөч дээр.
 *
 * Энд шалгаж байгаа зүйлс нь бүгд jsdom-ийн хариулж ЧАДАХГҮЙ зүйлс бөгөөд
 * гурвуулаа production дээр нэг өдөр зэрэг илэрсэн:
 *
 *   1. `Host` дээр тулгуурласан шилжилт — `desktop.localhost/` нь
 *      `/line/desktop` рүү очих ёстой (`proxy.ts`). Энэ нь ямар ч
 *      компонентоос асууж болдоггүй, routing-ийн баримт.
 *   2. Дүрс ҮНЭХЭЭР зурагдсан эсэх. `ICONS` толинд байхгүй нэр нь алдаа
 *      өгдөггүй — хоосон дөрвөлжин үлдээдэг, тэр нь зөвхөн зурган дээр
 *      харагдана.
 *   3. Хэвтээ халилт. Ажлын муж native хүрээн дотор суудаг тул хажуу тийш
 *      гүйдэг хуудас тэнд гүйлгэх зураасгүйгээр тайрагдана.
 *
 * API нь хуудсан дотор stub-даг (`page.route`) — энэ бол frontend-ийн тест.
 */

const PROFILE = {
  id: "6f1c2d84-3b91-4a7e-9c02-1d5a8e7b4f30",
  name: "Цэнддорж Эрдэнэбат",
  email: "person@example.test",
  created_at: "2026-01-14T09:12:00Z",
  is_admin: true,
  organisations: [{ id: "t1", name: "Гэрэгэ Системс", slug: "gerege-systems" }],
  home: { id: "h1", name: "Миний муж", slug: "person" },
  active_sessions: 3,
  identities: [{
    kind: "eid", provider: "ДАН", subject: "111949212017",
    linked_at: "2026-01-14T09:12:00Z", last_seen_at: "2026-09-04T00:42:00Z",
    issuer: "https://eid.gov.mn",
  }],
};

const ME = {
  id: PROFILE.id, tenant_id: "t1", tenant_name: "Гэрэгэ Системс",
  workspace_kind: "organisation", name: PROFILE.name, email: PROFILE.email,
  is_admin: true, permissions: ["apps.read", "access.manage"],
};

const json = (route: Route, body: unknown, status = 200) =>
  route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });

async function signedIn(page: Page) {
  await page.route("**/api/v1/profile", (route) => json(route, PROFILE));
  await page.route("**/api/v1/auth/me", (route) => json(route, ME));
}

/** Хуудас өөрөөсөө өргөн үү. Ажлын мужид хэвтээ гүйлт бол тайралт. */
async function overflow(page: Page) {
  return page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
}

test("ширээний шугамын үндэс нь нүүр рүү очиж, нэвтэрсэн хүнийг харуулна", async ({ page, baseURL }) => {
  const port = new URL(baseURL!).port;
  await signedIn(page);

  await page.goto(`http://desktop.localhost:${port}/`);

  // 1. Host-оор шилжсэн эсэх.
  await expect(page).toHaveURL(new RegExp("/line/desktop$"));

  // 2. Хуудасны гол агуулга нь хүн.
  await expect(page.getByRole("heading", { level: 2, name: PROFILE.name })).toBeVisible();
  await expect(page.getByText("Гэрэгэ Системс").first()).toBeVisible();
  await expect(page.getByText("111949212017")).toBeVisible();

  // Пайз нь зөвхөн олон нийтийн терминалынх — ширээн дээр дөрвөн зураас
  // болж хоосон зогсдог байсан тул эндээс хасагдсан.
  await expect(page.locator(".paiza")).toHaveCount(0);

  // 3. Үйлдэл бүр дүрстэй. Толинд байхгүй нэр нь хоосон хайрцаг үлдээдэг.
  const actions = page.locator(".line-action");
  const count = await actions.count();
  expect(count).toBeGreaterThan(0);
  for (let i = 0; i < count; i++) {
    await expect(actions.nth(i).locator(".line-action-icon svg")).toHaveCount(1);
  }

  expect(await overflow(page)).toBeLessThanOrEqual(0);
});

test("нэвтрээгүй үед дэлгэц зогсож, шалтгаанаа хэлнэ", async ({ page, baseURL }) => {
  const port = new URL(baseURL!).port;
  await page.route("**/api/v1/profile", (route) => json(route, { error: "unauthorized" }, 401));
  await page.route("**/api/v1/auth/me", (route) => json(route, { error: "unauthorized" }, 401));

  await page.goto(`http://desktop.localhost:${port}/line/desktop`);

  await expect(page.getByText(/Нэвтрээгүй байна/)).toBeVisible();
  // Хуудас өөрөө зогсох ёстой: шугам дээр web-ийн /login хаалттай тул
  // нэвтрэлт рүү түлхэх нь мухардал үүсгэнэ.
  await expect(page).toHaveURL(new RegExp("/line/desktop$"));
});

test("гар утасны өргөнд хэвтээ халилтгүй", async ({ page, baseURL }) => {
  const port = new URL(baseURL!).port;
  await page.setViewportSize({ width: 390, height: 844 });
  await signedIn(page);

  await page.goto(`http://mobile.localhost:${port}/line/mobile`);

  await expect(page.getByRole("heading", { level: 2, name: PROFILE.name })).toBeVisible();
  expect(await overflow(page)).toBeLessThanOrEqual(0);
});

test("олон нийтийн терминал хүний бүртгэлийг харуулахгүй", async ({ page, baseURL }) => {
  const port = new URL(baseURL!).port;
  let asked = 0;
  await page.route("**/api/v1/profile", (route) => { asked += 1; return json(route, PROFILE); });
  await page.route("**/api/v1/auth/me", (route) => { asked += 1; return json(route, ME); });

  await page.goto(`http://kiosk.localhost:${port}/line/kiosk`);

  await expect(page.locator(".paiza")).toHaveCount(1);
  await expect(page.locator(".person")).toHaveCount(0);
  // Дэлгэцийг дараагийн дугаарлаж байгаа хүн хардаг тул хүсэлт нь ЯВАХ ч
  // ёсгүй — хоосон рендер хийх нь хангалтгүй.
  expect(asked).toBe(0);
});
