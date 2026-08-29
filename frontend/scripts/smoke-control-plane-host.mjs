#!/usr/bin/env node
/** Exercise the built Next server, including proxy and server-component gates. */

import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import http from "node:http";
import { once } from "node:events";

const port = 3137;
const server = spawn(
  process.execPath,
  ["node_modules/next/dist/bin/next", "start", "-p", String(port)],
  {
    env: { ...process.env, CONTROL_PLANE_HOST: "admin.localhost" },
    stdio: ["ignore", "pipe", "pipe"],
  },
);

let output = "";
server.stdout.on("data", (chunk) => { output += chunk; });
server.stderr.on("data", (chunk) => { output += chunk; });

function request(host, path) {
  return new Promise((resolve, reject) => {
    const call = http.get({
      hostname: "127.0.0.1",
      port,
      path,
      headers: { Host: `${host}:${port}` },
    }, (response) => {
      response.resume();
      response.on("end", () => resolve({
        status: response.statusCode,
        location: response.headers.location || "",
        vary: response.headers.vary || "",
      }));
    });
    call.on("error", reject);
  });
}

async function waitUntilReady() {
  const deadline = Date.now() + 15_000;
  while (Date.now() < deadline) {
    if (server.exitCode !== null) throw new Error(`Next exited before startup:\n${output}`);
    try {
      await request("nexus.localhost", "/");
      return;
    } catch {
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
  }
  throw new Error(`Next did not start within 15 seconds:\n${output}`);
}

try {
  await waitUntilReady();

  const root = await request("admin.localhost", "/");
  assert.equal(root.status, 308);
  assert.equal(root.location, "/cp");
  assert.match(root.vary, /(?:^|,\s*)Host(?:,|$)/i);

  assert.equal((await request("admin.localhost", "/cp")).status, 200);
  assert.equal((await request("admin.localhost", "/cp/audit")).status, 200);
  assert.equal((await request("admin.localhost", "/login")).status, 404);
  assert.equal((await request("admin.localhost", "/api/v1/auth/login")).status, 404);

  assert.equal((await request("nexus.localhost", "/")).status, 200);
  assert.equal((await request("nexus.localhost", "/cp")).status, 404);

  console.log("built control-plane host: redirect, allow and deny boundaries are correct.");
} finally {
  if (server.exitCode === null) {
    server.kill("SIGTERM");
    await Promise.race([
      once(server, "exit"),
      new Promise((resolve) => setTimeout(resolve, 2_000)),
    ]);
    if (server.exitCode === null) server.kill("SIGKILL");
  }
}
