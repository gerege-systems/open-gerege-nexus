#!/usr/bin/env node
/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

/**
 * Every relative link in a Markdown file has to lead somewhere.
 *
 *   node scripts/check-doc-links.mjs
 *
 * `frontend/scripts/check-shell-links.mjs` does this for the routes a shell
 * offers. Nothing did it for the documents, and the gap cost exactly what that
 * one did: when `docs/adr/`, `docs/RELEASING.md`, `docs/MONITORING.md` and six
 * more were removed, twenty-eight links from CHANGELOG.md kept pointing at
 * them. Every one of them was a 404 on GitHub for weeks, and nothing was red —
 * a path in a link is a string, and a string is always valid.
 *
 * The same is true of every path in `docs/`, `deploy/`, `native-apps/` and the
 * six translated READMEs, which is why this walks the whole repository rather
 * than one directory.
 *
 * What it checks: inline links `[text](target)` and reference definitions
 * `[id]: target` whose target is a relative path. External schemes and bare
 * `#anchor` links are left alone.
 *
 * What it does NOT check: whether a `#fragment` names a heading that exists,
 * and links that wrap across two lines. Both are real gaps; neither is worth a
 * Markdown parser here, and a checker that reports the easy half of the truth
 * beats one nobody writes.
 *
 * Code is skipped, and that part is not incidental. The first attempt at this
 * cleanup ran a regular expression over the whole file and rewrote
 * `nexus.Provide[nexus.SSOClientRegistry](ssoprovider.AsClientRegistry(...))`
 * — Go generics, inside backticks — as though it were a link. Fenced blocks and
 * inline spans are blanked out before anything is matched.
 *
 * Exits non-zero on any link that leads nowhere.
 */
import { readFile, stat } from "node:fs/promises";
import { execFileSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

/**
 * Links that are known to lead nowhere and are staying that way.
 *
 * The list is the point. A target belongs here only with the reason it cannot
 * simply be fixed, so that the next person meets an argument rather than a
 * silent exception. It is empty today; an empty list is not a reason to delete
 * it, because the next document removed without its links needs somewhere to be
 * written down.
 *
 * Shape: "<file>::<target>": "why".
 */
const linksThatLeadNowhere = {};

const exists = (p) => stat(p).then(() => true, () => false);

/** Blank out fenced blocks and inline code, keeping every offset intact. */
function withoutCode(lines) {
  let fence = null;
  return lines.map((line) => {
    const opener = line.match(/^\s*(```+|~~~+)/);
    if (fence) {
      const closed = opener && opener[1].startsWith(fence[0]) && opener[1].length >= fence.length;
      if (closed) fence = null;
      return " ".repeat(line.length);
    }
    if (opener) {
      fence = opener[1];
      return " ".repeat(line.length);
    }
    // Inline spans: keep the backticks, blank what they hold. A link's target
    // is never inside them, so a real link survives this untouched.
    return line.replace(/(`+)([^`]*?)\1/g, (m, ticks, body) => ticks + " ".repeat(body.length) + ticks);
  });
}

const INLINE = /\[[^\]\n]*\]\(\s*(<[^>\n]*>|[^)\s]+)/g;
const REFERENCE = /^\s{0,3}\[[^\]\n]+\]:\s*(\S+)/;

/** The relative targets one file names, as [target, lineNumber] pairs. */
function targetsIn(text) {
  const lines = text.split("\n");
  const scannable = withoutCode(lines);
  const found = [];
  scannable.forEach((line, i) => {
    const push = (raw) => {
      const target = raw.replace(/^<|>$/g, "").trim();
      if (!target) return;
      // Absolute URLs, protocol-relative URLs, in-page anchors, and the
      // template placeholders documents use to show a shape (`<ssh-host>`).
      if (/^([a-z][a-z0-9+.-]*:|\/\/|#|<)/i.test(target)) return;
      found.push([target, i + 1]);
    };
    for (const m of line.matchAll(INLINE)) push(m[1]);
    const ref = line.match(REFERENCE);
    if (ref) push(ref[1]);
  });
  return found;
}

const files = execFileSync("git", ["ls-files", "*.md"], { cwd: ROOT, encoding: "utf8" })
  .split("\n")
  .filter(Boolean);

const broken = [];
for (const file of files) {
  const text = await readFile(path.join(ROOT, file), "utf8");
  for (const [target, line] of targetsIn(text)) {
    const key = `${file}::${target}`;
    if (key in linksThatLeadNowhere) continue;
    // A fragment or query names a place inside the target, not another file.
    const relative = decodeURIComponent(target.split("#")[0].split("?")[0]);
    if (!relative) continue;
    const resolved = relative.startsWith("/")
      ? path.join(ROOT, relative)
      : path.resolve(ROOT, path.dirname(file), relative);
    if (!(await exists(resolved))) broken.push({ file, line, target });
  }
}

if (broken.length) {
  for (const { file, line, target } of broken) {
    console.error(`::error file=${file},line=${line}::${target} — энэ зам байхгүй`);
  }
  console.error(`\n${broken.length} холбоос хаашаа ч хүргэхгүй байна (${files.length} файл шалгав).`);
  process.exit(1);
}

console.log(`${files.length} markdown файлын харьцангуй холбоос бүр байгаа зам руу заасан.`);
