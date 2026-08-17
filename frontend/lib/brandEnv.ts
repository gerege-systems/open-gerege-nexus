import { DEFAULT_BRAND, type Brand } from "./brand";

/**
 * Reads the brand from the environment. Server-side only — `process.env` on the
 * client holds nothing but what the build inlined, which is the whole point.
 */
export function brandFromEnv(env: NodeJS.ProcessEnv = process.env): Brand {
  return {
    name: text(env.BRAND_NAME) || DEFAULT_BRAND.name,
    // A short name nobody supplied is the full name, not "Nexus": the default
    // pair belongs to the default brand, and inheriting half of it would put
    // this product's abbreviation under somebody else's icon.
    shortName: text(env.BRAND_SHORT_NAME) || text(env.BRAND_NAME) || DEFAULT_BRAND.shortName,
    description: text(env.BRAND_DESCRIPTION) || DEFAULT_BRAND.description,
    logoUrl: assetURL(text(env.BRAND_LOGO_URL)) || DEFAULT_BRAND.logoUrl,
    themeColor: hexColour(text(env.BRAND_THEME_COLOR)) || DEFAULT_BRAND.themeColor,
    iconUrl: assetURL(text(env.BRAND_ICON_URL)),
    maskableIconUrl: assetURL(text(env.BRAND_MASKABLE_ICON_URL)),
    // Falls back to the platform's manual rather than to nothing: a menu item
    // that leads nowhere is worse than one that leads to the general book.
    docsUrl: assetURL(text(env.BRAND_DOCS_URL)) || DEFAULT_BRAND.docsUrl,
  };
}

function text(value: string | undefined): string {
  return (value ?? "").trim();
}

/**
 * A logo address, or nothing.
 *
 * The value is written into `src` on an image tag, so the shapes it may take
 * are named rather than trusted: a path on this host, or an absolute http(s)
 * URL. Everything else — `javascript:` first among them — is not a location a
 * deployment file has any reason to hold, and refusing it here means a
 * mistyped or tampered environment costs a logo rather than a session.
 */
function assetURL(value: string): string {
  if (value.startsWith("/") && !value.startsWith("//")) return value;
  if (/^https?:\/\/\S+$/i.test(value)) return value;
  return "";
}

/**
 * A CSS hex colour, or nothing. It is emitted in a meta tag and read by the
 * launcher; anything else there is a colour the browser silently ignores, and
 * finding that out from a screenshot is worse than falling back visibly.
 */
function hexColour(value: string): string {
  return /^#(?:[0-9a-f]{3}|[0-9a-f]{4}|[0-9a-f]{6}|[0-9a-f]{8})$/i.test(value) ? value : "";
}
