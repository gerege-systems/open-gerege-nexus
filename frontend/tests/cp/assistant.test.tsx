/**
 * The assistant every organisation on the deployment meets.
 *
 * These prompts and this corpus used to be edited one organisation at a time,
 * which meant one tenant administrator editing what every other tenant would
 * be answered with. Moving them here made every write a platform write: with a
 * reason, in the audit trail. What the screen owes is that no write leaves
 * without one, and that a half-filled form cannot be submitted as an entry the
 * assistant will then quote at somebody.
 */

import { expect, test, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { aKnowledge, aPrompt } from "../helpers/fixtures";

const api = vi.hoisted(() => ({
  prompts: vi.fn(),
  knowledge: vi.fn(),
  savePrompt: vi.fn(),
  addKnowledge: vi.fn(),
  removeKnowledge: vi.fn(),
  stepUp: vi.fn(),
}));

vi.mock("@/lib/i18n", () => import("../helpers/i18n"));
vi.mock("@/lib/cp", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/cp")>()),
  cp: api,
}));

import Assistant from "@/app/cp/assistant/page";

function loaded({ prompts = [aPrompt()], knowledge = [aKnowledge()] } = {}) {
  api.prompts.mockResolvedValue({ prompts });
  api.knowledge.mockResolvedValue({ knowledge });
}

test("a prompt is saved with a reason, and with the operator's edit", async () => {
  loaded({ prompts: [aPrompt({ key: "assistant.system", content: "Хуучин заавар." })] });
  api.savePrompt.mockResolvedValue({ status: "ok" });
  const person = userEvent.setup();

  render(<Assistant />);
  const editor = await screen.findByDisplayValue("Хуучин заавар.");
  await person.clear(editor);
  await person.type(editor, "Шинэ заавар.");
  await person.click(screen.getByRole("button", { name: "base.action.save" }));

  const dialog = await screen.findByRole("dialog", { name: "base.action.save" });
  expect(api.savePrompt).not.toHaveBeenCalled();
  await person.type(within(dialog).getByRole("textbox"), "бодлого шинэчлэв");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  await waitFor(() =>
    expect(api.savePrompt).toHaveBeenCalledWith("assistant.system", "Шинэ заавар.", true, "бодлого шинэчлэв"),
  );
});

test("an empty prompt cannot be saved over a working one", async () => {
  loaded({ prompts: [aPrompt({ content: "Заавар." })] });
  const person = userEvent.setup();

  render(<Assistant />);
  const editor = await screen.findByDisplayValue("Заавар.");
  await person.clear(editor);

  // Every organisation without a prompt of its own falls back to this one.
  // Saving it blank is not an edit, it is turning the assistant off for the
  // whole deployment by accident.
  expect(screen.getByRole("button", { name: "base.action.save" })).toHaveProperty("disabled", true);
});

test("a prompt the deployment has never written is still offered", async () => {
  loaded({ prompts: [aPrompt({ key: "assistant.greeting", content: "", updated_at: null })] });

  render(<Assistant />);

  expect(await screen.findByText("assistant.greeting")).toBeTruthy();
  expect(screen.getByText("cp.state.never")).toBeTruthy();
});

test("knowledge needs a title and a body before it can be added", async () => {
  loaded({ knowledge: [] });
  const person = userEvent.setup();

  render(<Assistant />);
  const add = await screen.findByRole("button", { name: "ai.action.add_knowledge" });
  expect(add).toHaveProperty("disabled", true);

  await person.type(screen.getByPlaceholderText("ai.field.knowledge_title"), "Журам");
  expect(add).toHaveProperty("disabled", true);

  // A source URL is optional; a body is not. An entry with a title and nothing
  // under it is a heading the assistant will quote as an answer.
  await person.type(screen.getByPlaceholderText("ai.field.knowledge_content"), "Агуулга");
  expect(add).toHaveProperty("disabled", false);
});

test("added knowledge is sent with its reason and the form is emptied", async () => {
  loaded({ knowledge: [] });
  api.addKnowledge.mockResolvedValue({ status: "ok" });
  const person = userEvent.setup();

  render(<Assistant />);
  await person.type(await screen.findByPlaceholderText("ai.field.knowledge_title"), "Журам");
  await person.type(screen.getByPlaceholderText("ai.field.source_url"), "https://example.test/rules");
  await person.type(screen.getByPlaceholderText("ai.field.knowledge_content"), "Агуулга");
  await person.click(screen.getByRole("button", { name: "ai.action.add_knowledge" }));

  const dialog = await screen.findByRole("dialog", { name: "ai.action.add_knowledge" });
  await person.type(within(dialog).getByRole("textbox"), "шинэ журам");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  await waitFor(() =>
    expect(api.addKnowledge).toHaveBeenCalledWith(
      { title: "Журам", content: "Агуулга", source_url: "https://example.test/rules" },
      "шинэ журам",
    ),
  );
  // Left filled, the next operator adds the same entry twice.
  await waitFor(() =>
    expect(screen.getByPlaceholderText("ai.field.knowledge_title")).toHaveProperty("value", ""),
  );
});

test("removing knowledge is a destructive action and asks first", async () => {
  loaded({ knowledge: [aKnowledge({ id: "kn-7", title: "Хуучирсан журам" })] });
  api.removeKnowledge.mockResolvedValue({ status: "ok" });
  const person = userEvent.setup();

  render(<Assistant />);
  await screen.findByText("Хуучирсан журам");
  await person.click(screen.getByRole("button", { name: "base.action.delete" }));

  const dialog = await screen.findByRole("dialog", { name: "base.action.delete" });
  expect(within(dialog).getByText("Хуучирсан журам")).toBeTruthy();
  await person.type(within(dialog).getByRole("textbox"), "давхардсан");
  await person.click(within(dialog).getByRole("button", { name: "cp.action.confirm" }));

  await waitFor(() => expect(api.removeKnowledge).toHaveBeenCalledWith("kn-7", "давхардсан"));
});

test("either half failing to load says so once, and does not half-draw the screen", async () => {
  api.prompts.mockResolvedValue({ prompts: [aPrompt()] });
  api.knowledge.mockRejectedValue(new Error("assistant tables are not readable"));

  render(<Assistant />);

  expect(await screen.findByText("assistant tables are not readable")).toBeTruthy();
  // The prompts arrived, but the screen is one answer: showing half of it as
  // if it were the whole would say the corpus is empty.
  expect(screen.queryByDisplayValue("Чи Гэрэгэ Нексүсийн туслах.")).toBeNull();
});
