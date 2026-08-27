#!/usr/bin/env node
/**
 * Writes the icon map the shell draws menus with, from the icons the platform
 * actually names.
 *
 *   npm run icons:generate      # then commit lib/icons.generated.ts
 *   npm run icons:generate -- --check   # CI: fail if it is out of date
 *
 * The map used to be written by hand in frontend/components/Layout.tsx: sixty
 * static imports and sixty entries, added by whoever noticed their menu was
 * rendering an empty square. Two things were wrong with it. A name missing from
 * it failed silently, and an app outside this repository could not use an icon
 * the core had not already thought of.
 *
 * Lucide's own DynamicIcon resolves names at runtime and would have removed the
 * file entirely. It was tried and rejected on measurement: webpack emits one
 * chunk per icon in the library, so `.next/static` went from 3.5 MB in 95 chunks
 * to 11 MB in 1828 for an application that draws about twenty. The initial
 * bundle does not grow — the chunks are lazy — but tripling the deployed asset
 * tree to lazy-load twenty icons is paying for flexibility nobody asked for.
 *
 * So the map stays static and stops being written by hand. What it contains is
 * derived from where icons are declared:
 *
 *   - backend Go menu definitions and blueprints  (`Icon: "..."`)
 *   - the bundled catalogue's manifests           (`"icon": "..."`)
 *   - the shell's own search index                (`icon:"..."`)
 *
 * A distribution runs this in its own frontend build and gets its own apps'
 * icons; nobody edits a list. backend/internal/workspace/menu/icons_test.go is
 * the other half — it holds every name declared in Go to the set lucide can
 * actually draw.
 */
import { readFile, writeFile, readdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import * as lucide from "lucide-react";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(HERE, "..", "..");
const OUT = path.join(HERE, "..", "lib", "icons.generated.ts");
const CHECK = process.argv.includes("--check");

/** Every file under dir matching one of the suffixes. */
async function walk(dir, suffixes) {
  const found = [];
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    // testdata is skipped for the same reason the Go toolchain skips it: what
    // is in there is a fixture, and backend/testdata/canary declares menus of
    // its own. A test distribution must not put an icon in the shipped bundle.
    if (["node_modules", ".next", ".git", "testdata"].includes(entry.name)) continue;
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) found.push(...(await walk(full, suffixes)));
    else if (suffixes.some((s) => entry.name.endsWith(s))) found.push(full);
  }
  return found;
}

async function collect(dir, suffixes, pattern) {
  const names = new Set();
  for (const file of await walk(dir, suffixes)) {
    const source = await readFile(file, "utf8");
    for (const match of source.matchAll(pattern)) names.add(match[1]);
  }
  return names;
}

/**
 * Icons this image must draw for apps that are not in this repository.
 *
 * The generator's premise is that a distribution runs it in its own frontend
 * build. client-gerege-nexus does not have one: it ships the web image this
 * repository publishes and carries three apps that used to be here — the
 * organisation, documents and the e-Government link. Their menus arrive from
 * that deployment's own API with these icon names on them, and nothing in this
 * tree names them any more, so the shell would draw the fallback box on every
 * entry of every screen those apps own.
 *
 * The list shrinks to nothing the day those distributions build their own
 * frontend. Not a general escape hatch: an icon goes here only when this image
 * is the one drawing it for somebody else.
 *
 * The last six arrived on 2026-08-25 with sso_clients, which left for
 * appstore-gerege-nexus. Its case is the strongest of the three: its screens
 * are still in this tree — /sso-clients and /module/sso-clients/* — so this
 * image is drawing the whole app for whoever carries the module, and the menu
 * entries that reach those screens would be the only part of it rendering a
 * fallback box.
 */
const forDistributionsWithoutAFrontend = [
  "file-text", //   documents
  "landmark", //    the e-Government link
  "network", //     the organisation's departments
  "users", //       its people
  "code-2", //      the SSO client register
  "key-round", //   its API keys
  "scroll-text", // its access audit
  "shield-check", //its OAuth scopes
  "route", //       its redirect policies
  "key-square", //  its signing keys
];

const declared = new Set([
  ...forDistributionsWithoutAFrontend,
  ...(await collect(path.join(ROOT, "backend"), [".go"], /Icon:\s*"([a-z0-9-]+)"/g)),
  ...(await collect(path.join(ROOT, "catalog"), [".json"], /"icon"\s*:\s*"([a-z0-9-]+)"/g)),
  ...(await collect(path.join(ROOT, "frontend", "app"), [".tsx"], /icon:\s*"([a-z0-9-]+)"/g)),
  ...(await collect(path.join(ROOT, "frontend", "components"), [".tsx"], /icon:\s*"([a-z0-9-]+)"/g)),
]);

/** building-2 → Building2, bar-chart-3 → BarChart3, file-text → FileText. */
const component = (name) =>
  name
    .split("-")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join("");

const drawable = [];
const unknown = [];
for (const name of [...declared].sort()) {
  const exported = component(name);
  if (typeof lucide[exported] === "function" || typeof lucide[exported] === "object") drawable.push([name, exported]);
  else unknown.push(name);
}

// Not fatal: a name lucide does not have renders the fallback, and the Go test
// is where that is a failure. Said out loud so it is not discovered from a
// screenshot.
for (const name of unknown) console.warn(`  no lucide icon named ${JSON.stringify(name)} — it will render the fallback`);

const body = `/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Generated by frontend/scripts/generate-icon-map.mjs — do not edit.
//
// Every icon named by a Go menu definition, a catalogue manifest or the shell's
// own search index. Run \`npm run icons:generate\` after changing any of those;
// CI runs it with --check and fails on a diff.

import type { LucideIcon } from "lucide-react";
import {
${drawable.map(([, exported]) => `  ${exported},`).join("\n")}
} from "lucide-react";

export const ICONS: Record<string, LucideIcon> = {
${drawable.map(([name, exported]) => `  ${JSON.stringify(name)}: ${exported},`).join("\n")}
};
`;

const current = await readFile(OUT, "utf8").catch(() => null);
if (CHECK) {
  if (current !== body) {
    console.error(
      `lib/icons.generated.ts is out of date.\n\n` +
        `An icon was named in Go, in a manifest or in the shell and the map was not\n` +
        `regenerated, so it renders the fallback glyph. Run:\n\n` +
        `    npm run icons:generate\n\n` +
        `and commit the result.`,
    );
    process.exit(1);
  }
  console.log(`lib/icons.generated.ts: ${drawable.length} icons, up to date.`);
} else {
  await writeFile(OUT, body, "utf8");
  console.log(`Wrote ${drawable.length} icons to lib/icons.generated.ts`);
}
