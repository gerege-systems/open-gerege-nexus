/**
 * Fails the build if the generated site links to something that is not there.
 *
 *   node check-links.mjs        (run after build.mjs)
 *
 * Only internal links are checked. Whether an external URL is alive is not
 * something this repository can hold true, and a link checker that reaches the
 * network turns an unrelated outage into a failed documentation build.
 *
 * The checks cover defects that can look fine in the Markdown source:
 *
 *   1. A link to a page that was never published — usually a document added to
 *      docs/ and not to the PAGES list in build.mjs.
 *   2. A `.md` href that survived rewriting, which a browser downloads instead
 *      of rendering.
 *   3. A `#fragment` naming a heading that no longer exists, which lands the
 *      reader silently at the top of the page.
 *   4. A published page accidentally rewritten as a GitHub source-code link.
 *   5. Duplicate ids or root-relative URLs that break on the Pages subpath.
 */
import {existsSync, readFileSync, readdirSync} from "node:fs";
import {dirname, relative, resolve} from "node:path";
import {fileURLToPath} from "node:url";

import {BLOB, PAGES} from "./pages.mjs";

const DIST = resolve(dirname(fileURLToPath(import.meta.url)), "dist");
const publishedFiles = new Set(PAGES.map((page) => `${page.slug}.html`));

if (!existsSync(DIST)) {
  console.error("dist/ is missing — run `npm run build` first.");
  process.exit(1);
}

const pages = readdirSync(DIST).filter((f) => f.endsWith(".html"));
const problems = [];
const idsByPage = new Map();

for (const publishedFile of publishedFiles) {
  if (!pages.includes(publishedFile)) problems.push(`${publishedFile} (manifest page was not built)`);
}

function decode(value) {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

for (const page of pages) {
  const html = readFileSync(resolve(DIST, page), "utf8");
  const ids = [...html.matchAll(/\bid=(['"])(.*?)\1/g)].map((match) => match[2]);
  const seenIds = new Set();
  for (const id of ids) {
    if (seenIds.has(id)) problems.push(`${page} → #${id} (duplicate id)`);
    seenIds.add(id);
  }
  idsByPage.set(page, new Set(ids));
}

for (const page of pages) {
  const html = readFileSync(resolve(DIST, page), "utf8");
  for (const [, , href] of html.matchAll(/\b(?:href|src)=(['"])(.*?)\1/g)) {
    if (href.startsWith(`${BLOB}/`)) {
      const linkedFile = new URL(href).pathname.split("/").at(-1);
      if (publishedFiles.has(linkedFile)) {
        problems.push(`${page} → ${href} (published page was misrouted to GitHub)`);
      }
    }
    if (/^(?:[a-z][a-z\d+.-]*:|\/\/)/i.test(href)) continue;

    const match = href.match(/^([^?#]*)(?:\?[^#]*)?(?:#(.*))?$/);
    const [, rawPath = "", rawFragment] = match ?? [];

    if (rawPath.startsWith("/")) {
      problems.push(`${page} → ${href} (root-relative URL breaks on the GitHub Pages project path)`);
      continue;
    }

    const path = decode(rawPath);
    const fragment = rawFragment === undefined ? undefined : decode(rawFragment);
    const targetPage = path || page;

    if (/\.md$/.test(path)) {
      problems.push(`${page} → ${href} (Markdown link was not rewritten)`);
      continue;
    }

    if (path) {
      const target = resolve(DIST, path);
      const outsideDist = relative(DIST, target).startsWith("..");
      if (outsideDist || !existsSync(target)) {
        problems.push(`${page} → ${href} (no such file in dist)`);
        continue;
      }
    }

    if (fragment && idsByPage.has(targetPage) && !idsByPage.get(targetPage).has(fragment)) {
      problems.push(`${page} → ${href} (no heading with that id)`);
    }
  }
}

if (problems.length) {
  console.error(`${problems.length} broken link(s):\n`);
  for (const p of problems) console.error(`  ${p}`);
  process.exit(1);
}

console.log(`${pages.length} pages, no broken internal links.`);
