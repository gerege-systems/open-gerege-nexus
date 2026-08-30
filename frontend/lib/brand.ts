/**
 * The name, the mark and the colour this deployment answers to.
 *
 * `lib/apiBase.ts` took the deployment's *address* out of the build. This takes
 * its *identity* out, and the argument is the same one a step further: an image
 * that knows its own name is an image one customer owns. Level 1 of the
 * ecosystem strategy — one image, a different `.env`, a hundred deployments —
 * is only true when both are read at run time.
 *
 * Read from the process environment rather than from `NEXT_PUBLIC_*`, because a
 * NEXT_PUBLIC value is substituted into the bundle by `next build`: it would be
 * the same variable, baked at exactly the moment this file exists to stop
 * baking it. That is why `app/layout.tsx` reads it on the server and hands the
 * result down, and why it is dynamic — see the note there.
 *
 * The reading itself lives in `lib/brandEnv.ts`, apart from the shape and the
 * defaults here, so that a client component naming the product does not drag a
 * `process.env` reader into every browser bundle.
 *
 * `BRAND_NAME` is also read by the API (backend/internal/kernel/config), for
 * the two sentences it puts in front of a person. They are two halves of one
 * setting; a deployment that renames only one of them has a sign-in screen and
 * an eID prompt that disagree about which product this is.
 *
 * Only the name is expected to be set. The rest exist because a rebrand that
 * changes the word and keeps the blue emblem is not a rebrand, and because the
 * values are all a deployment can supply without a build: an image cannot be
 * given a new logo file after the fact, but it can be pointed at one.
 */
export type Brand = {
  name: string;
  /** What a launcher has room for under an icon. */
  shortName: string;
  description: string;
  /** Where the mark is served from — a path on this host, or an absolute URL. */
  logoUrl: string;
  /** The browser and launcher chrome colour, as a CSS hex colour. */
  themeColor: string;
  /**
   * The icon a launcher, a tab and a home screen use — the tab favicon, the
   * Apple touch icon and the manifest's own entry, all from one address.
   *
   * A separate value from the logo because they are different pictures doing
   * different jobs: the logo sits in a header beside a name, at 36 pixels, on
   * the page's own background; the icon stands alone at 512 on somebody's home
   * screen and needs its own ground under it. A deployment that has only one
   * image can point both here.
   *
   * Empty means the icons this image was built with — which is the honest
   * default: an icon set is files, and a deployment that has not supplied one
   * has not got one.
   */
  iconUrl: string;
  /**
   * The same icon drawn for a launcher that crops it — Android's adaptive
   * icons, which cut a circle or a squircle out of whatever they are given.
   *
   * Separate and optional because a maskable icon is not a resize: the artwork
   * has to bleed to the edge and keep everything that matters inside a safe
   * zone, so declaring an ordinary square icon maskable gets its corners cut
   * off. Unset means no maskable entry is published at all, and the launcher
   * masks the ordinary one itself — which is the same picture, and visibly
   * cropped rather than silently wrong.
   */
  maskableIconUrl: string;
  /**
   * Where this deployment's manual lives — the landing page's last menu item.
   *
   * A full URL rather than a route because the documentation is not part of
   * this application: it ships on its own schedule and a reader following it
   * is leaving the product. Which is exactly why it belongs to the
   * deployment: a distribution with its own name, its own modules and its own
   * screens has its own manual, and sending its readers to the platform's is
   * sending them to a book about a different product.
   *
   * The default is the platform's own, built from the Markdown in this
   * repository by `docs/mkdocs` and served from docs.nexus.gerege.mn.
   */
  docsUrl: string;
};

export const DEFAULT_BRAND: Brand = {
  name: "Gerege Nexus",
  shortName: "Nexus",
  description:
    "Төрийн болон хувийн хэвшлийн байгууллагын үйлчилгээ, үйл ажиллагаа, систем, өгөгдлийг нэгтгэх модульт платформ.",
  logoUrl: "/brand.webp",
  themeColor: "#1869eb",
  // The icons that ship in the image. See app/manifest.ts for why the maskable
  // one is separate artwork rather than the same file relabelled.
  iconUrl: "",
  maskableIconUrl: "",
  docsUrl: "https://docs.nexus.gerege.mn/",
};

/**
 * The copy keys a deployment writes its own name under, per language.
 *
 * `BRAND_NAME` is one string, and a deployment whose name is Mongolian showed
 * that Mongolian name to an English reader — the header said one thing and
 * every sentence around it said another. The name is prose like the rest of a
 * deployment's wording, so it is written where the rest of it is written:
 * `BRAND_COPY_FILE`, matched per locale. Unwritten languages keep the
 * environment's value, which is the honest state for a name nobody translated.
 */
export const BRAND_COPY_KEYS = {
  name: "brand.name",
  shortName: "brand.short_name",
  description: "brand.description",
} as const;

/**
 * The brand as one language sees it.
 *
 * Everything except the three words is the same in every language: a logo, a
 * colour and an address do not translate.
 */
export function localizedBrand(
  brand: Brand,
  copy: Record<string, Partial<Record<string, string>>>,
  locale: string,
): Brand {
  const pick = (key: string, fallback: string) => copy[key]?.[locale]?.trim() || fallback;
  return {
    ...brand,
    name: pick(BRAND_COPY_KEYS.name, brand.name),
    shortName: pick(BRAND_COPY_KEYS.shortName, brand.shortName),
    description: pick(BRAND_COPY_KEYS.description, brand.description),
  };
}
