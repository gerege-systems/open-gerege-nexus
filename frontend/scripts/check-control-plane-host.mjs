#!/usr/bin/env node
/**
 * The control-plane hostname must never render a tenant screen.
 *
 * This is a behavioural check of the same pure decision function proxy.ts
 * calls. It runs on Node 20 in CI and does not need a Next server.
 */
import assert from "node:assert/strict";
import { controlPlaneHostDecision } from "../lib/controlPlaneHost.mjs";

const decide = (host, path, configured = "admin.localhost") =>
  controlPlaneHostDecision(host, configured, path);

assert.equal(decide("admin.localhost:3000", "/"), "redirect");
assert.equal(decide("ADMIN.LOCALHOST.", "/cp"), "allow");
assert.equal(decide("admin.localhost", "/cp/audit"), "allow");
assert.equal(decide("admin.localhost", "/api/platform/v1/session"), "allow");

for (const path of ["/login", "/settings", "/api/v1/auth/login", "/cpx", "/robots.txt"]) {
  assert.equal(decide("admin.localhost", path), "not-found", `${path} leaked onto the console host`);
}

for (const host of ["nexus.localhost:3000", "localhost:3000", "nexus.gerege.mn"]) {
  assert.equal(decide(host, "/"), "other-host", `${host} was treated as the console host`);
}

assert.equal(decide("localhost:3000", "/cp", ""), "other-host");

console.log("control-plane host: root redirects, console/API pass, tenant paths stay hidden.");
