#!/usr/bin/env node
/**
 * Holds the core API client to the core's own endpoints.
 *
 *   npm run api:check
 *
 * lib/api.ts was 1831 lines and held every endpoint of every app in one object.
 * That is why it is the first file the core boundary work reached for: of
 * the last 133 commits that touched an app, 40 changed it. Not because anybody
 * wanted a shared file — because it was the only file there was, so an app's
 * endpoint had nowhere else to go.
 *
 * It is split now, one file per app. The split holds only while somebody is
 * checking: the next app endpoint written into lib/api/client.ts would compile,
 * pass tsc, ship, and put the file back on the same road. So the core client's
 * paths are checked against the list of prefixes the platform itself owns.
 *
 * Adding a prefix here is allowed and sometimes right — a genuinely new
 * platform surface is a platform surface. It is a visible line in a review
 * rather than a side effect of where an endpoint was written.
 *
 * Exits non-zero on any path outside the list.
 */
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const here = dirname(fileURLToPath(import.meta.url));
const clientPath = join(here, "..", "lib", "api", "client.ts");

/**
 * What the platform itself serves. Each entry matches a path that equals it or
 * continues with "/" — "/store" admits "/store/apps" and not "/storefront".
 *
 * The App Store entries are here on purpose and are not a leak: the store
 * screens are how a deployment installs anything at all, so every deployment
 * has them. backend/pkg/host/server.go serves them from the core for
 * the same reason. Same for /esign, which the documents app absorbed in
 * migration 00058 but whose rails are still platform code — and which is
 * therefore *not* in lib/api/client.ts either, but in lib/api/esign.ts beside
 * the app that drives it.
 */
const CORE_PREFIXES = [
  "/auth",
  "/profile",
  "/tenant",
  "/menus",
  "/store",
  "/installed-apps",
  "/admin/access",
  "/admin/devices",
  "/admin/store",
  "/admin/email-verification",
  "/verify",
  "/push-tokens",
  "/oauth2",
  "/ai",
  "/admin/ai",
  "/integrations",
  // The first-run wizard. It is the platform by definition: it runs before
  // there is an organisation for any app to belong to.
  "/setup",
  // A person's own workspace. Platform by the same test as /profile: the rows
  // behind it are written by whichever modules a deployment happens to carry,
  // but the endpoint and the table are the core's (migration 00086), and a
  // deployment with no such module answers with an empty list rather than a
  // 404. An app's endpoint is one that stops existing when the app is removed.
  "/me",
];

const source = readFileSync(clientPath, "utf8");

// Every path literal, from a plain string or a template. Comments are stripped
// first so that a prose mention of an app's endpoint is not read as one.
const code = source
  .replace(/\/\*[\s\S]*?\*\//g, "")
  .replace(/^\s*\/\/.*$/gm, "");

const found = new Set();
for (const match of code.matchAll(/["'`](\/[a-zA-Z0-9_/-]*)/g)) {
  const path = match[1];
  if (path === "/" || path.length < 2) continue;
  found.add(path);
}

const owned = (path) =>
  CORE_PREFIXES.some((prefix) => path === prefix || path.startsWith(prefix + "/"));

const strays = [...found].filter((path) => !owned(path)).sort();

if (strays.length > 0) {
  console.error(`lib/api/client.ts reaches ${strays.length} path(s) the core does not own:\n`);
  for (const path of strays) console.error(`  ${path}`);
  console.error(`
An app's endpoint belongs in lib/api/<app>.ts, next to the app it serves. The
core client is the file every distribution imports, so what is written here is
carried by deployments that do not have the app and cannot use the endpoint.

If this really is a platform endpoint, add its prefix to CORE_PREFIXES in
${"scripts/check-api-boundaries.mjs"} — deliberately, which is the point.`);
  process.exit(1);
}

console.log(`lib/api/client.ts: ${found.size} paths, all within the core's own prefixes.`);
