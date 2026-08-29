import assert from "node:assert/strict";
import { test } from "vitest";

import { safeReturnPath } from "../lib/safeReturnPath.mjs";

test("authentication accepts same-origin return paths", () => {
  for (const path of [
    "/profile",
    "/oauth/consent?client_id=example&scope=openid",
    "/settings/access#members",
  ]) {
    assert.equal(safeReturnPath(path), path);
  }
});

test("authentication rejects external and malformed return paths", () => {
  for (const path of [
    "https://attacker.example",
    "//attacker.example",
    "/\\attacker.example",
    "/safe\\..\\attacker.example",
    " /profile",
    "/profile\nLocation: https://attacker.example",
    "",
    null,
    undefined,
  ]) {
    assert.equal(safeReturnPath(path, "/safe"), "/safe", String(path));
  }
});
