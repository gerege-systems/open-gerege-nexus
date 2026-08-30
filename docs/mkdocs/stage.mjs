/**
 * Assemble an MkDocs tree from the repository's own Markdown.
 *
 * docs.gerege.mn is built with MkDocs and Material for MkDocs, and this site is
 * built the same way so the two read as one set of documentation rather than
 * two products that happen to share a company.
 *
 * The page list is NOT duplicated here. It is imported from ../site/pages.mjs,
 * which already decides what is publishable, what it is called, and which group
 * it belongs to. Two lists would drift, and the one that drifts is always the
 * one nobody is looking at.
 *
 * MkDocs reads a single docs_dir, and half of these files live at the
 * repository root (README, CHANGELOG, SECURITY…). So the tree is staged into
 * build/docs rather than pointed at in place, and the links between pages are
 * rewritten to match.
 */
import {mkdir, readFile, writeFile, rm, cp} from "node:fs/promises";
import {existsSync} from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";
import {PAGES} from "../site/pages.mjs";

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, "../..");
const out = path.join(here, "build");
const docsDir = path.join(out, "docs");

// A page's source path → the slug it is published under. Used to rewrite the
// links between documents: `docs/MONITORING.md` in the source becomes
// `monitoring.md` here, and a link that still points at the old path would 404
// on a site whose files have been renamed.
const bySource = new Map(PAGES.map((p) => [p.src, p]));

function rewriteLinks(markdown, fromSrc) {
  const fromDir = path.dirname(fromSrc);
  return markdown.replace(/\]\(([^)\s]+?)(#[^)]*)?\)/g, (whole, target, anchor = "") => {
    if (/^(https?:|mailto:|#|\/)/.test(target)) return whole;
    // Resolve the link the way it reads in the repository, then look it up.
    const resolved = path.normalize(path.join(fromDir, decodeURIComponent(target)));
    const page = bySource.get(resolved);
    // `index.md`, not `.`: MkDocs treats a bare dot as an unrecognised link and
    // --strict turns that into a failed build.
    if (page) return `](${page.slug}.md${anchor})`;
    // Anything the site does not publish keeps working by pointing at GitHub,
    // which is where that file still is.
    return `](https://github.com/gerege-systems/open-gerege-nexus/blob/main/${resolved}${anchor})`;
  });
}

await rm(out, {recursive: true, force: true});
await mkdir(docsDir, {recursive: true});

const groups = new Map();
for (const page of PAGES) {
  const source = path.join(repo, page.src);
  if (!existsSync(source)) {
    console.error(`missing: ${page.src}`);
    process.exitCode = 1;
    continue;
  }
  const body = rewriteLinks(await readFile(source, "utf8"), page.src);
  const name = page.slug === "index" ? "index.md" : `${page.slug}.md`;
  await writeFile(path.join(docsDir, name), body);
  if (!groups.has(page.group)) groups.set(page.group, []);
  groups.get(page.group).push({title: page.title, file: name});
}

// Brand and assets travel with the tree.
await cp(path.join(here, "stylesheets"), path.join(docsDir, "stylesheets"), {recursive: true});
await cp(path.join(here, "assets"), path.join(docsDir, "assets"), {recursive: true});
if (existsSync(path.join(repo, "docs/assets"))) {
  // Images only. A stray .md under assets/ is a page MkDocs would build and
  // then complain is missing from the nav — and --strict makes that fatal.
  await cp(path.join(repo, "docs/assets"), path.join(docsDir, "assets"), {
    recursive: true,
    filter: (src) => !src.endsWith(".md"),
  });
}

const nav = [...groups].map(([group, items]) => {
  const lines = items.map((i) => `      - ${JSON.stringify(i.title)}: ${i.file}`);
  return `  - ${JSON.stringify(group)}:\n${lines.join("\n")}`;
}).join("\n");

const template = await readFile(path.join(here, "mkdocs.template.yml"), "utf8");
await writeFile(path.join(out, "mkdocs.yml"), template.replace("# {{NAV}}", `nav:\n${nav}`));

console.log(`staged ${PAGES.length} pages into ${path.relative(repo, docsDir)}`);
