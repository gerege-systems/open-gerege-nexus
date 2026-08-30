/**
 * Assemble an MkDocs tree from the repository's own Markdown.
 *
 * docs.gerege.mn is built with MkDocs and Material for MkDocs, and this site is
 * built the same way so the two read as one set of documentation rather than
 * two products that happen to share a company.
 *
 * The page list is NOT duplicated here. It is imported from ./pages.mjs,
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
import {PAGES} from "./pages.mjs";

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, "../..");
const out = path.join(here, "build");
const docsDir = path.join(out, "docs");

// A page's source path → the slug it is published under. Used to rewrite the
// links between documents: `docs/OPERATIONS.md` in the source becomes
// `operations.md` here, and a link that still points at the old path would 404
// on a site whose files have been renamed.
const bySource = new Map(PAGES.map((p) => [p.src, p]));

/**
 * Which files are translations of which page.
 *
 * The repository keeps its translated documents as siblings — README_EN.md,
 * README_AR.md, SECURITY_EN.md — and pages.mjs lists each one, because on
 * GitHub they really are separate files. On a site with a language switcher
 * they are not separate pages: seven "Overview" rows stacked in one sidebar is
 * the switcher's job done badly by the navigation.
 *
 * So they are staged as MkDocs translations (`index.en.md` beside `index.md`)
 * and dropped from the nav. Every other page has no translation and falls back
 * to the Mongolian original, which is what fallback_to_default is for.
 */
const TRANSLATION_OF = new Map(Object.entries({
  "docs/README_EN.md":                     ["index", "en"],
  "docs/README_AR.md":                     ["index", "ar"],
  "docs/README_ZH.md":                     ["index", "zh"],
  "docs/README_FR.md":                     ["index", "fr"],
  "docs/README_RU.md":                     ["index", "ru"],
  "docs/README_ES.md":                     ["index", "es"],
  "docs/CONTRIBUTING_EN.md":               ["contributing", "en"],
  "docs/SECURITY_EN.md":                   ["security", "en"],
  "docs/CODE_OF_CONDUCT_EN.md":            ["code-of-conduct", "en"],
}));

function rewriteLinks(markdown, fromSrc, fromSlug) {
  const fromDir = path.dirname(fromSrc);

  // `asUrl` splits the two kinds of link to a translated file.
  //
  // A Markdown link is validated by MkDocs against the files it knows, so it has
  // to name one: prose that says "see the English contributing guide" resolves
  // to the contributing page, which the reader gets in whatever language they
  // are already in. A URL like ../en/contributing/ is not a file and --strict
  // rejects it.
  //
  // The HTML pass is the language row, and there switching language IS the
  // point. MkDocs does not validate raw HTML, so a locale URL is fine there.
  const retarget = (asUrl) => (whole, target, anchor = "") => {
    if (/^(https?:|mailto:|#|\/)/.test(target)) return whole;
    // Resolve the link the way it reads in the repository, then look it up.
    const resolved = path.normalize(path.join(fromDir, decodeURIComponent(target)));
    // A link to a translated file goes to that language's copy of the page.
    // README.md carries a row of them — Монгол · العربية · 中文 · … — and
    // pointing all six at the page the reader is already on would make the row
    // decorative. `use_directory_urls` puts a locale at /<locale>/<slug>/.
    //
    // Site-absolute, because the row travels: the same seven links are rendered
    // at `/`, at `/documents/`, and again under six locale prefixes. A relative
    // `../` count that is right at one of those depths is wrong at the others,
    // and the row is the first thing on the page — nobody misses it being
    // broken.
    const translated = TRANSLATION_OF.get(resolved);
    if (translated) {
      const [base, locale] = translated;
      if (!asUrl) return `](${base}.md${anchor})`;
      return `](/${locale}/${base === "index" ? "" : `${base}/`}${anchor})`;
    }
    // The flag row's images live in docs/assets, and that directory is copied
    // into the staged tree below — so they are files this site serves, not
    // files it lacks. Without this branch they fell through to the GitHub
    // fallback and became `github.com/.../blob/...`, a URL that serves an HTML
    // page: seven broken icons at the top of every page, in every language.
    //
    // Site-absolute rather than relative, because the same row is rendered at
    // `/`, at `/architecture/` and again under every locale prefix, and one
    // `../` count cannot be right for all of them.
    //
    // Images only, and only ones that are really there. `docs/assets` also
    // holds ATTRIBUTION.md, which the site does not publish: sending that to an
    // absolute path would make a link --strict rejects, where the GitHub
    // fallback below is the right answer for it.
    if (/^docs\/assets\/.+\.(png|jpe?g|svg|webp|gif)$/i.test(resolved)
        && existsSync(path.join(repo, resolved))) {
      return `](/${resolved.slice("docs/".length)}${anchor})`;
    }
    const page = bySource.get(resolved);
    // In Markdown, `index.md` rather than `.`: MkDocs treats a bare dot as an
    // unrecognised link and --strict turns that into a failed build.
    //
    // In HTML it has to be the published URL instead. MkDocs rewrites `.md` in
    // Markdown links and leaves raw HTML alone, so `href="index.md"` reached
    // the browser verbatim — the Монгол entry of the language row 404ed from
    // every translated page while its six neighbours worked.
    if (page) {
      if (!asUrl) return `](${page.slug}.md${anchor})`;
      return `](/${page.slug === "index" ? "" : `${page.slug}/`}${anchor})`;
    }
    // Anything the site does not publish keeps working by pointing at GitHub,
    // which is where that file still is.
    return `](https://github.com/gerege-systems/open-gerege-nexus/blob/main/${resolved}${anchor})`;
  };

  // Markdown links first, then the raw HTML ones. README.md writes its language
  // row as <a href="docs/README_AR.md">, and a rewriter that only understands
  // `](…)` leaves those pointing at a path the site does not serve — six 404s
  // in the most visited row of the front page. The HTML pass reuses the same
  // resolution and unwraps the `](…)` the helper returns.
  const asMarkdown = markdown.replace(/\]\(([^)\s]+?)(#[^)]*)?\)/g, retarget(false));

  return asMarkdown.replace(/(href|src)="([^"]+?)(#[^"]*)?"/g, (whole, attr, target, anchor = "") => {
    const rewritten = retarget(true)(`](${target}${anchor})`, target, anchor);
    if (rewritten.startsWith("](")) return `${attr}="${rewritten.slice(2, -1)}"`;
    return whole;
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
  const body = rewriteLinks(await readFile(source, "utf8"), page.src, page.slug);
  const translation = TRANSLATION_OF.get(page.src);
  if (translation) {
    const [base, locale] = translation;
    await writeFile(path.join(docsDir, `${base}.${locale}.md`), body);
    continue; // a translation, not a row in the navigation
  }

  const name = `${page.slug}.md`;
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

const inNav = [...groups.values()].reduce((n, g) => n + g.length, 0);
console.log(`staged ${inNav} pages + ${PAGES.length - inNav} translations`);
