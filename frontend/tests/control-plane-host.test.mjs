import assert from "node:assert/strict";
import { test } from "vitest";

import { controlPlaneHostDecision } from "../lib/controlPlaneHost.mjs";

const decide = (host, path, configured = "admin.localhost") =>
  controlPlaneHostDecision(host, configured, path);

test("the console host admits only console-owned paths", () => {
  assert.equal(decide("admin.localhost:3000", "/"), "redirect");

  for (const path of [
    "/cp",
    "/cp/audit",
    "/api/platform/v1",
    "/api/platform/v1/session",
  ]) {
    assert.equal(decide("ADMIN.LOCALHOST.", path), "allow", path);
  }

  for (const path of [
    "/login",
    "/settings",
    "/api/v1/auth/login",
    "/cpx",
    "/api/platform/v10",
    "/robots.txt",
  ]) {
    assert.equal(decide("admin.localhost", path), "not-found", path);
  }
});

test("tenant and look-alike hosts are not treated as the console", () => {
  for (const host of [
    "nexus.localhost:3000",
    "localhost:3000",
    "nexus.gerege.mn",
    "admin.localhost.attacker.example",
    "",
    null,
  ]) {
    assert.equal(decide(host, "/cp"), "other-host", String(host));
  }
  assert.equal(decide("admin.localhost", "/cp", ""), "other-host");
});
