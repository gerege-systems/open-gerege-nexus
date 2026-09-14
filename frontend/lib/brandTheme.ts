import fs from "node:fs";

/**
 * A deployment's look, the way the design system's theme editor hands it out.
 *
 * ui.gecore.mn/#theme ends in "Get code": a few `<html>` attributes whose
 * values ship with the library, and a `:root` / `.dark` block of the tokens
 * that changed. A distribution built on this core pastes exactly that here —
 * nothing in the core has to be edited, because the core's own look is
 * written the same way (app/globals.css, "Theme editor" block) and every
 * legacy `--gerege-*` name is an alias of the library's tokens.
 *
 *   BRAND_THEME_STYLE     data-style  — nova | vega | maia | lyra | mira |
 *                         luma | sera | rhea | nexus (default: nexus)
 *   BRAND_THEME_DEPTH     data-depth  — flat | soft | raised | deep (soft)
 *   BRAND_THEME_RADIUS    data-radius — "full" for pill controls, else unset
 *   BRAND_THEME_ACCENT    data-accent — a library preset (blue, violet,
 *                         emerald, rose) or one of this core's (cobalt, teal,
 *                         neutral); unset keeps the CSS's accent
 *   BRAND_THEME_CSS_FILE  path to the pasted `:root`/`.dark` CSS (a mounted
 *                         file beside the logo and copy)
 *   BRAND_THEME_CSS       the same CSS inline, for a token or two
 *
 * The CSS lands at the end of <body> as a plain <style>: last in the cascade
 * and unlayered, so it wins over the library's tokens and the core's block
 * exactly as pasting it at the bottom of a stylesheet would.
 */
export interface BrandTheme {
  /** Attributes for <html>, already validated. */
  attributes: Record<string, string>;
  /** The pasted token CSS, or "" when the deployment has none. */
  css: string;
}

const STYLES = new Set(["nova", "vega", "maia", "lyra", "mira", "luma", "sera", "rhea", "nexus"]);
const DEPTHS = new Set(["flat", "soft", "raised", "deep"]);
const ACCENTS = new Set(["blue", "violet", "emerald", "rose", "cobalt", "teal", "neutral"]);

let cached: BrandTheme | null = null;

export function brandThemeFromEnv(env: NodeJS.ProcessEnv = process.env): BrandTheme {
  if (cached && env === process.env) return cached;
  const attributes: Record<string, string> = {};
  const style = pick(env.BRAND_THEME_STYLE, STYLES) || "nexus";
  const depth = pick(env.BRAND_THEME_DEPTH, DEPTHS) || "soft";
  attributes["data-style"] = style;
  attributes["data-depth"] = depth;
  if (text(env.BRAND_THEME_RADIUS) === "full") attributes["data-radius"] = "full";
  const accent = pick(env.BRAND_THEME_ACCENT, ACCENTS);
  if (accent) attributes["data-accent"] = accent;
  const theme = { attributes, css: sanitise(readCss(env)) };
  if (env === process.env) cached = theme;
  return theme;
}

function readCss(env: NodeJS.ProcessEnv): string {
  const path = text(env.BRAND_THEME_CSS_FILE);
  if (path) {
    try {
      return fs.readFileSync(path, "utf8");
    } catch (error) {
      console.warn(`BRAND_THEME_CSS_FILE could not be read (${path}):`, error);
    }
  }
  return env.BRAND_THEME_CSS ?? "";
}

/**
 * The pasted text is a stylesheet, not markup: a `</style>` inside it would
 * end the element early. It is the only sequence that can, so it is the only
 * one removed; everything else CSS can say is left to the browser.
 */
function sanitise(css: string): string {
  return css.replace(/<\/style/gi, "").trim();
}

function pick(value: string | undefined, allowed: Set<string>): string {
  const v = text(value).toLowerCase();
  return allowed.has(v) ? v : "";
}

function text(value: string | undefined): string {
  return (value ?? "").trim();
}
