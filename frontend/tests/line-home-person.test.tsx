/**
 * Шугамын нүүр дэлгэц дээрх «Нэвтэрсэн хүн».
 *
 * Бүрхүүл нэвтрэлтээ өөрөө эзэмшдэг тул native талд нэвтэрсэн эсэхийг web
 * талаас нотлох цорын ганц газар нь энэ хэсэг. Хоёр зүйл эвдэрч болно:
 * бүртгэл огт уншигдахгүй байх, эсвэл олон нийтийн терминал дээр уншигдах —
 * kiosk-ийн дэлгэцийг дараагийн хүн хардаг.
 */
import { expect, test, vi } from "vitest";
import { render, screen } from "@testing-library/react";

const stub = vi.hoisted(() => ({ line: "desktop", profile: vi.fn(), getMe: vi.fn() }));

vi.mock("next/navigation", () => ({ useParams: () => ({ line: stub.line }) }));
vi.mock("@/lib/shell", () => ({
  useShell: () => ({ shell: null }),
  invokeShell: vi.fn(),
  SHELL_METHODS: { DEVICE_IDENTITY: "device.identity" },
}));
vi.mock("@/lib/api", () => ({ api: { profile: stub.profile, getMe: stub.getMe } }));

import LineHomePage from "@/app/line/[line]/page";

const PERSON = {
  id: "u-1", name: "Батболд", email: "batbold@example.mn", created_at: "2026-01-05T00:00:00Z",
  is_admin: true, organisations: [{ id: "t-1", name: "Гэрэгэ", slug: "gerege" }],
  home: null, active_sessions: 2,
  identities: [{ kind: "eid", provider: "ДАН", subject: "УБ99", linked_at: "2026-02-01T00:00:00Z", last_seen_at: "2026-03-01T00:00:00Z" }],
};
const SESSION = { id: "u-1", tenant_id: "t-1", tenant_name: "Гэрэгэ", name: "Батболд", email: "batbold@example.mn", is_admin: true, permissions: ["apps.read"] };

test("ширээний шугам нэвтэрсэн хүнийг бүтнээр нь харуулна", async () => {
  stub.line = "desktop";
  stub.profile.mockResolvedValue(PERSON);
  stub.getMe.mockResolvedValue(SESSION);

  render(<LineHomePage />);

  expect(await screen.findByText("batbold@example.mn")).toBeDefined();
  expect(screen.getByText("УБ99")).toBeDefined();
  expect(screen.getByText("apps.read")).toBeDefined();
});

test("олон нийтийн терминал дээр хүний бүртгэлийг уншихгүй", () => {
  stub.line = "kiosk";
  stub.profile.mockResolvedValue(PERSON);
  stub.getMe.mockResolvedValue(SESSION);

  render(<LineHomePage />);

  expect(stub.profile).not.toHaveBeenCalled();
  expect(screen.queryByText("Нэвтэрсэн хүн")).toBeNull();
});

test("нэвтрээгүй үед дэлгэц зогсож, шалтгааныг хэлнэ", async () => {
  stub.line = "desktop";
  const denied = Object.assign(new Error("unauthorized"), { status: 401 });
  stub.profile.mockRejectedValue(denied);
  stub.getMe.mockRejectedValue(denied);

  render(<LineHomePage />);

  expect(await screen.findByText(/Нэвтрээгүй байна/)).toBeDefined();
});
