/**
 * Builds the published documentation site from the Markdown already in this
 * repository.
 *
 *   node build.mjs        → docs/site/dist
 *
 * The documents are the source of truth and stay readable on GitHub; this
 * script only wraps them in a shell and rewrites the links. Nothing here should
 * ever require editing a document to keep the site working — a document that
 * renders on GitHub renders here.
 *
 * Link rewriting is the whole trick. A link in a Markdown file is relative to
 * that file, and the site is flat, so every href is resolved against its source
 * file and then re-pointed at one of three places:
 *
 *   - another published page  → its slug on this site
 *   - an asset under docs/    → the copied asset
 *   - anything else in the repo (LICENSE, .env.example, Go source) → GitHub
 *
 * That last case is why the site can link to code it does not publish.
 */
import {cpSync, existsSync, mkdirSync, readFileSync, rmSync, statSync, writeFileSync} from "node:fs";
import {createHash} from "node:crypto";
import {dirname, join, posix, relative, resolve} from "node:path";
import {fileURLToPath} from "node:url";

import {Marked} from "marked";
import {BLOB, GITHUB, LANGUAGES, PAGES, TREE} from "./pages.mjs";

const HERE = dirname(fileURLToPath(import.meta.url));
const REPO = resolve(HERE, "..", "..");
const OUT = join(HERE, "dist");
const THEME = readFileSync(join(HERE, "theme.css"));
const THEME_FILE = `theme.${createHash("sha256").update(THEME).digest("hex").slice(0, 12)}.css`;

const bySrc = new Map(PAGES.map((p) => [p.src, p]));
const esc = (value) =>
  String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");

/* ── Markdown → HTML ──────────────────────────────────────────────────────── */

const slugged = new Map();

/** A stable, collision-free id for a heading, so the sidebar can link into it. */
function headingId(text) {
  const decoded = text
    .replace(/<[^>]+>/g, "")
    .replace(/&#x([\da-f]+);/gi, (_, value) => String.fromCodePoint(Number.parseInt(value, 16)))
    .replace(/&#(\d+);/g, (_, value) => String.fromCodePoint(Number.parseInt(value, 10)))
    .replace(/&(amp|lt|gt|quot|apos|nbsp);/gi, (_, entity) => {
      return ({amp: "&", lt: "<", gt: ">", quot: '"', apos: "'", nbsp: " "})[entity.toLowerCase()];
    });
  const base =
    decoded
      .toLowerCase()
      .replace(/['’]/g, "")
      .replace(/[^\p{L}\p{N}]+/gu, "-")
      .replace(/^-+|-+$/g, "") || "section";
  const seen = slugged.get(base) ?? 0;
  slugged.set(base, seen + 1);
  return seen ? `${base}-${seen}` : base;
}

/**
 * Re-points one URL from "relative to this Markdown file" to "correct on this
 * site". Published Markdown becomes a local page, assets stay local, source
 * files open on GitHub, and repository directories use GitHub's tree view.
 */
function rewrite(href, srcPath) {
  if (!href || /^(?:[a-z][a-z\d+.-]*:|#)/i.test(href)) return href;

  const match = href.match(/^([^?#]*)(\?[^#]*)?(#.*)?$/);
  const [, pathPart, query = "", hash = ""] = match ?? [];
  if (!pathPart) return href;

  const repoRel = posix.normalize(posix.join(posix.dirname(srcPath), pathPart)).replace(/^\.\//, "");
  const suffix = `${query}${hash}`;

  const page = bySrc.get(repoRel);
  if (page) return `${page.slug}.html${suffix}`;
  // Assets are copied and served; a Markdown file sitting among them is prose,
  // not an asset, and a browser would download it rather than render it — so it
  // goes to GitHub with the rest of the unpublished files.
  if (repoRel.startsWith("docs/assets/") && !repoRel.endsWith(".md")) {
    return repoRel.slice("docs/".length) + suffix;
  }

  const repositoryTarget = resolve(REPO, repoRel);
  const repositoryRoot = `${REPO}/`;
  const isDirectory =
    repositoryTarget.startsWith(repositoryRoot) &&
    existsSync(repositoryTarget) &&
    statSync(repositoryTarget).isDirectory();
  return `${isDirectory ? TREE : BLOB}/${repoRel}${suffix}`;
}

/** Rewrite only attributes that came from raw HTML in the Markdown source. */
function rewriteRawHtml(html, srcPath) {
  return html.replace(/\b(href|src)=(['"])(.*?)\2/gi, (attribute, name, quote, value) => {
    return `${name}=${quote}${esc(rewrite(value, srcPath))}${quote}`;
  });
}

/** The site supplies its own language bar; the GitHub-only source row is kept out. */
function withoutEmbeddedLanguageRow(markdown, page) {
  if (!page.lang) return markdown;
  return markdown.replace(/<p>[\s\S]*?<\/p>/i, (block) => {
    return /README_(?:AR|ZH|EN|FR|RU|ES)\.md/.test(block) ? "" : block;
  });
}

function render(markdown, page) {
  slugged.clear();
  const headings = [];
  const srcPath = page.src;
  const md = new Marked({gfm: true, breaks: false});

  md.use({
    renderer: {
      heading({tokens, depth}) {
        const text = this.parser.parseInline(tokens);
        const id = headingId(text);
        if (depth === 2 || depth === 3) headings.push({id, text, depth});
        return `<h${depth} id="${id}"><a class="anchor" href="#${id}" aria-hidden="true"></a>${text}</h${depth}>\n`;
      },
      link({href, title, tokens}) {
        const text = this.parser.parseInline(tokens);
        const target = rewrite(href, srcPath);
        const external = /^https?:/.test(target);
        return `<a href="${esc(target)}"${title ? ` title="${esc(title)}"` : ""}${
          external ? ' target="_blank" rel="noopener"' : ""
        }>${text}</a>`;
      },
      image({href, title, text}) {
        return `<img src="${esc(rewrite(href, srcPath))}" alt="${esc(text ?? "")}"${
          title ? ` title="${esc(title)}"` : ""
        }>`;
      },
      html({text}) {
        return rewriteRawHtml(text, srcPath);
      },
      table(token) {
        // Wrapped so a wide table scrolls inside the column instead of pushing
        // the whole page sideways on a phone.
        const rendered = md.Renderer.prototype.table.call(this, token);
        return `<div class="table-scroll">${rendered}</div>`;
      },
    },
  });

  return {html: md.parse(withoutEmbeddedLanguageRow(markdown, page)), headings};
}

/* ── Page shell ───────────────────────────────────────────────────────────── */

function navigationSections(current) {
  const groups = new Map();
  for (const p of PAGES) {
    // The seven overview translations collapse to one entry; the rest of the
    // languages are reachable from the language row on the page itself.
    if (p.lang && p.lang !== "mn") continue;
    if (!groups.has(p.group)) groups.set(p.group, []);
    groups.get(p.group).push(p);
  }
  return [...groups]
    .map(([group, pages]) => {
      const items = pages
        .map((p) => {
          const active = p.slug === current || (p.lang && PAGES.find((q) => q.slug === current)?.lang && p.lang === "mn");
          return `<li><a href="${p.slug}.html"${active ? ' class="active" aria-current="page"' : ""}>${esc(p.title)}</a></li>`;
        })
        .join("");
      return `<div class="nav-group"><h4>${esc(group)}</h4><ul>${items}</ul></div>`;
    })
    .join("");
}

function sidebar(current, className = "sidebar", label = "Баримтын цэс") {
  return `<nav class="${className}" aria-label="${label}">${navigationSections(current)}</nav>`;
}

function mobileNavigation(current, title) {
  return `<details class="mobile-nav">
  <summary><span>Баримтын цэс</span><strong>${esc(title)}</strong></summary>
  ${sidebar(current, "mobile-sidebar", "Мобайл баримтын цэс")}
</details>`;
}

function languageRow(page) {
  if (!page?.lang) return "";
  const links = LANGUAGES.map(({lang, label, flag}) => {
    const target = PAGES.find((p) => p.lang === lang);
    if (!target) return "";
    const img = `<img src="assets/icons/${flag}" width="18" height="18" alt="">`;
    return lang === page.lang
      ? `<b>${img} ${esc(label)}</b>`
      : `<a href="${target.slug}.html">${img} ${esc(label)}</a>`;
  }).join("");
  return `<div class="lang-row">${links}</div>`;
}

function shell({title, slug, body, toc = "", page}) {
  const rtl = page?.rtl ? ' dir="rtl"' : "";
  return `<!doctype html>
<html lang="${page?.lang ?? "mn"}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${esc(title)} · Gerege Nexus</title>
<meta name="description" content="Gerege Nexus — үйлчилгээ, үйл ажиллагаа, системийн нэгдсэн нээлттэй эхийн платформ.">
<meta name="theme-color" content="#06336e">
<link rel="icon" href="assets/icons/flag-mn.png">
<link rel="stylesheet" href="assets/${THEME_FILE}">
</head>
<body>
<a class="skip" href="#content">Агуулга руу шилжих</a>
<header class="topbar">
  <a class="brand" href="index.html"><span class="mark">GN</span><span class="brand-name">Gerege Nexus</span></a>
  <nav class="topnav" aria-label="Үндсэн цэс">
    <a href="index.html">Тойм</a>
    <a href="architecture.html">Архитектур</a>
    <a href="module-authoring.html">Хөгжүүлэлт</a>
    <a href="documents.html">Баримтын индекс</a>
  </nav>
  <div class="topactions">
    <a class="ghost" href="${GITHUB}" target="_blank" rel="noopener">GitHub</a>
    <a class="gold" href="https://nexus.gerege.mn" target="_blank" rel="noopener">Нэвтрэх</a>
  </div>
</header>
<div class="layout">
${mobileNavigation(slug, title)}
${sidebar(slug)}
<main id="content"${rtl}>
${page ? languageRow(page) : ""}
${body}
</main>
${toc}
</div>
<footer class="sitefoot">
  <span>© 2026 Gerege Systems · Apache 2.0</span>
  <span><a href="${GITHUB}" target="_blank" rel="noopener">Эх код</a> · <a href="changelog.html">Өөрчлөлтийн түүх</a> · <a href="security.html">Аюулгүй байдал</a></span>
</footer>
</body>
</html>
`;
}

function tocFor(headings) {
  if (headings.length < 3) return "";
  const items = headings
    .map((h) => `<li class="d${h.depth}"><a href="#${h.id}">${h.text}</a></li>`)
    .join("");
  return `<aside class="toc" aria-label="Энэ хуудсанд"><h4>Энэ хуудсанд</h4><ul>${items}</ul></aside>`;
}

/* ── Build ────────────────────────────────────────────────────────────────── */

function validateManifest() {
  const sources = new Set();
  const slugs = new Set();
  for (const page of PAGES) {
    if (!existsSync(join(REPO, page.src))) throw new Error(`Missing documentation source: ${page.src}`);
    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(page.slug)) throw new Error(`Invalid page slug: ${page.slug}`);
    if (sources.has(page.src)) throw new Error(`Duplicate documentation source: ${page.src}`);
    if (slugs.has(page.slug)) throw new Error(`Duplicate page slug: ${page.slug}`);
    sources.add(page.src);
    slugs.add(page.slug);
  }
}

validateManifest();

rmSync(OUT, {recursive: true, force: true});
mkdirSync(OUT, {recursive: true});

for (const page of PAGES) {
  const markdown = readFileSync(join(REPO, page.src), "utf8");
  const {html, headings} = render(markdown, page);
  writeFileSync(
    join(OUT, `${page.slug}.html`),
    shell({title: page.title, slug: page.slug, body: html, toc: tocFor(headings), page}),
  );
}

mkdirSync(join(OUT, "assets"), {recursive: true});
cpSync(join(REPO, "docs/assets"), join(OUT, "assets"), {
  recursive: true,
  filter: (src) => !src.endsWith(".md"),
});
// The hashed filename makes the HTML and CSS an atomic pair at the CDN edge.
// Keep the stable name too, so an older cached HTML document never gets a 404.
writeFileSync(join(OUT, `assets/${THEME_FILE}`), THEME);
writeFileSync(join(OUT, "assets/theme.css"), THEME);
// Tells GitHub Pages not to run the output through Jekyll, which would drop
// any file or directory whose name begins with an underscore.
writeFileSync(join(OUT, ".nojekyll"), "");

console.log(`built ${PAGES.length} pages → ${relative(process.cwd(), OUT)}`);
