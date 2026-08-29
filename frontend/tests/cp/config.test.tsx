/**
 * The deployment's own configuration: settings, credentials and flags.
 *
 * The credential half is the reason this file exists. The platform has no
 * route that returns a stored secret, and the console must not become the one
 * place where a borrowed session is worth more than the actions it can take —
 * so the screen shows four characters of a key and the dialog that sets one
 * starts empty. Both are properties of the markup, and both are the kind of
 * thing a well-meaning "prefill the field" change would undo.
 */

import { expect, test, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const api = vi.hoisted(() => ({
  settings: vi.fn(),
  flags: vi.fn(),
  credentials: vi.fn(),
  setSetting: vi.fn(),
  setCredential: vi.fn(),
  clearCredential: vi.fn(),
  saveFlag: vi.fn(),
  deleteFlag: vi.fn(),
  settingHistory: vi.fn(),
  rollbackSetting: vi.fn(),
  stepUp: vi.fn(),
}));

vi.mock("@/lib/i18n", () => import("../helpers/i18n"));
vi.mock("@/lib/cp", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/cp")>()),
  cp: api,
}));

import type { Credential, Flag, Setting } from "@/lib/cp";
import Config from "@/app/cp/config/page";

// Typed as the API's own records so that a row overriding one field — a
// credential that came from the environment, a setting still at its default —
// stays a valid record rather than narrowing to the first literal written.
const SETTING: Setting = {
  key: "session.idle_timeout",
  kind: "duration",
  default: "30m",
  env: "SESSION_IDLE_TIMEOUT",
  description: "Хэр удаан хөдөлгөөнгүй байхад session хаагдах",
  current: "45m",
  source: "database",
  updated_at: "2026-08-20T10:00:00+08:00",
};

const CREDENTIAL: Credential = {
  name: "smtp_password",
  env: "SMTP_PASSWORD",
  description: "И-мэйл илгээх нууц үг",
  source: "database",
  hint: "9f3a",
  updated_at: "2026-08-20T10:00:00+08:00",
  updated_by: "op-1",
};

const FLAG: Flag = {
  key: "documents.signing",
  description: "Гарын үсэг",
  owner: "platform",
  kind: "kill_switch",
  enabled: true,
  rollout: 100,
  expires_at: null,
  updated_at: "2026-08-20T10:00:00+08:00",
};

function loaded({
  settings = [SETTING],
  credentials = [CREDENTIAL],
  flags = [FLAG],
  sealing = true,
  warnings = [] as string[],
} = {}) {
  api.settings.mockResolvedValue({ settings, warnings });
  api.credentials.mockResolvedValue({ credentials, sealing_configured: sealing });
  api.flags.mockResolvedValue({ flags });
}

test("a credential is shown by its last four characters and nothing else", async () => {
  loaded();

  render(<Config />);
  await screen.findByText("smtp_password");

  expect(screen.getByText("…9f3a")).toBeTruthy();
  // Whatever else this screen learns to show, it must never be handed the
  // value: there is no field for one on the wire, and this is what says so.
  expect(JSON.stringify(await api.credentials.mock.results[0].value)).not.toContain("value");
});

test("the dialog that sets a credential starts empty and is write-only", async () => {
  loaded();
  api.setCredential.mockResolvedValue({ status: "ok" });
  const person = userEvent.setup();

  render(<Config />);
  await screen.findByText("smtp_password");
  await person.click(screen.getByRole("button", { name: "cp.action.set_credential" }));

  const dialog = await screen.findByRole("dialog", { name: "smtp_password" });
  const field = within(dialog).getByLabelText("cp.field.value") as HTMLInputElement;
  // Nothing to prefill it with, and a dialog that showed the current key would
  // be the one place a stolen session is worth more than what it can do.
  expect(field.value).toBe("");
  expect(field.type).toBe("password");
  expect(within(dialog).getByText("cp.message.credential_write_only")).toBeTruthy();

  await person.type(field, "шинэ-нууц-үг");
  await person.type(within(dialog).getByLabelText("cp.field.reason"), "ротаци");
  await person.click(
    within(dialog).getByRole("button", { name: "cp.action.set_credential" }),
  );

  await waitFor(() =>
    expect(api.setCredential).toHaveBeenCalledWith("smtp_password", "шинэ-нууц-үг", "ротаци"),
  );
});

test("with no sealing key the console says so and refuses to take a secret", async () => {
  loaded({ sealing: false });

  render(<Config />);
  await screen.findByText("smtp_password");

  // A secret written without a key to seal it with is a secret in the clear.
  expect(screen.getByText("cp.message.sealing_off")).toBeTruthy();
  expect(screen.getByRole("button", { name: "cp.action.set_credential" })).toHaveProperty("disabled", true);
});

test("only a credential this console stored can be cleared from it", async () => {
  loaded({
    credentials: [
      CREDENTIAL,
      { ...CREDENTIAL, name: "sentry_dsn", source: "environment", hint: undefined },
    ],
  });

  render(<Config />);
  await screen.findByText("sentry_dsn");

  // One clear button, on the row the database owns. Clearing an environment
  // value from here would be a button that says it removed something and did
  // not.
  expect(screen.getAllByRole("button", { name: /cp\.action\.clear_credential/ })).toHaveLength(1);
  expect(screen.getByRole("button", { name: "cp.action.clear_credential: smtp_password" })).toBeTruthy();
});

test("where a value came from is on the screen beside it", async () => {
  loaded({
    settings: [
      SETTING,
      { ...SETTING, key: "session.absolute_timeout", source: "environment", current: "8h" },
      { ...SETTING, key: "session.grace", source: "default", current: "5m" },
    ],
  });

  render(<Config />);
  await screen.findByText("session.grace");

  // A setting reading "environment" cannot be changed from here whatever the
  // console writes, so saying which is which is the difference between an
  // edit that lands and one that is silently overwritten on the next deploy.
  expect(screen.getAllByText("cp.source.database").length).toBeGreaterThan(0);
  expect(screen.getByText("cp.source.environment")).toBeTruthy();
  expect(screen.getByText("cp.source.default")).toBeTruthy();
});

test("changing a setting sends the new value with a reason", async () => {
  loaded();
  api.setSetting.mockResolvedValue({ status: "ok" });
  const person = userEvent.setup();

  render(<Config />);
  await screen.findByText("session.idle_timeout");
  await person.click(screen.getByRole("button", { name: "cp.action.change" }));

  const dialog = await screen.findByRole("dialog", { name: "session.idle_timeout" });
  // The default and the environment variable are named inside the same label,
  // so the field's accessible name is all three.
  const value = within(dialog).getByLabelText(/cp\.field\.value/);
  await person.clear(value);
  await person.type(value, "15m");
  await person.type(within(dialog).getByLabelText("cp.field.reason"), "аюулгүй байдлын шаардлага");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  await waitFor(() =>
    expect(api.setSetting).toHaveBeenCalledWith("session.idle_timeout", "15m", "аюулгүй байдлын шаардлага"),
  );
});

test("turning a kill switch off asks why, and keeps the rest of the flag", async () => {
  loaded();
  api.saveFlag.mockResolvedValue({ status: "ok" });
  const person = userEvent.setup();

  render(<Config />);
  await screen.findByText("documents.signing");
  await person.click(screen.getByRole("button", { name: "cp.action.turn_off: documents.signing" }));

  const dialog = await screen.findByRole("dialog", { name: "cp.action.turn_off" });
  await person.type(within(dialog).getByRole("textbox"), "гэмтэлтэй хувилбар");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  // The switch is the only thing that moves: a toggle that rewrote the rollout
  // or dropped the expiry would be a second, silent change.
  await waitFor(() =>
    expect(api.saveFlag).toHaveBeenCalledWith({
      ...FLAG,
      enabled: false,
      reason: "гэмтэлтэй хувилбар",
    }),
  );
});

test("a setting's history offers the way back to what it was", async () => {
  loaded();
  api.settingHistory.mockResolvedValue({
    changes: [
      {
        id: "chg-1",
        key: SETTING.key,
        previous_value: "30m",
        new_value: "45m",
        reason: "хүсэлтээр",
        changed_by: "op-1",
        changed_at: "2026-08-20T10:00:00+08:00",
      },
    ],
  });
  api.rollbackSetting.mockResolvedValue({ status: "ok" });
  const person = userEvent.setup();

  render(<Config />);
  await screen.findByText("session.idle_timeout");
  await person.click(
    screen.getByRole("button", { name: "cp.section.history: session.idle_timeout" }),
  );

  expect(await screen.findByText("хүсэлтээр")).toBeTruthy();
  await person.click(screen.getByRole("button", { name: "cp.action.rollback" }));

  const dialog = await screen.findByRole("dialog", { name: "cp.action.rollback" });
  await person.type(within(dialog).getByRole("textbox"), "буруу байсан");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  // By the row's id rather than by the value: two changes may carry the same
  // number and only one of them is the one being undone.
  await waitFor(() => expect(api.rollbackSetting).toHaveBeenCalledWith("chg-1", "буруу байсан"));
});

test("one half of the configuration failing does not draw the other half as empty", async () => {
  api.settings.mockResolvedValue({ settings: [SETTING], warnings: [] });
  api.flags.mockResolvedValue({ flags: [FLAG] });
  api.credentials.mockRejectedValue(new Error("credential table is not readable"));

  render(<Config />);

  expect(await screen.findByText("credential table is not readable")).toBeTruthy();
  // The three lists are read together and drawn together. The settings arrived
  // and are still not shown, because a screen that renders the half it got
  // says the other half is empty rather than unread.
  expect(screen.queryByText("session.idle_timeout")).toBeNull();
  expect(screen.queryByText("documents.signing")).toBeNull();
});
