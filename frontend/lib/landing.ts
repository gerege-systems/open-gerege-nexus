import type { TranslationKey } from "./i18n";

/**
 * Which sections the landing page is built from, and in what order.
 *
 * The page makes an argument, and not every deployment is making the same one.
 * A single sign-on provider has no catalogue of business applications to show;
 * a platform deployment that runs none of the commercial modules should not
 * advertise nine of them. Saying so used to mean forking the frontend — the
 * same fork `lib/brandCopy.ts` exists to prevent, on the same reasoning. That
 * file gave a deployment's *words* a home in configuration; this gives its
 * page *shape* one, because rewriting a section's prose does not help when the
 * honest answer is that the section does not apply.
 *
 * The default is every section in the order this page has always rendered
 * them, so a deployment that names nothing sees exactly what it saw before.
 */
export const LANDING_SECTIONS = [
  "hero",
  "architecture",
  "applications",
  // Директ хаягуудын картууд. Аргумент дуусаад, юу ажиллаж байгааг нэрлэх
  // ээлж ирэх газарт: доор нь платформын гүн, аюулгүй байдал зэрэг нь тэдгээр
  // үйлчилгээ юун дээр зогсож байгааг тайлбарлана.
  "services",
  "platform",
  "trust",
  "technology",
  "capabilities",
] as const;

export type LandingSection = (typeof LANDING_SECTIONS)[number];

/**
 * The anchor a section owns, and the word the menu uses for it.
 *
 * Kept here rather than in the header so that the menu cannot disagree with
 * the page: a deployment that drops `applications` drops its menu item with
 * it, instead of leaving a link that scrolls nowhere. `hero` is absent because
 * it is the top of the page — the brand mark already goes there.
 *
 * A section may also render without a menu item, which is why this is partial:
 * `applications`, `technology` and `capabilities` are read on the way down the
 * page rather than jumped to, and a seven-item menu was crowding out the two
 * controls a visitor actually came for. Their labels stay in the translations
 * (`website.menu.applications`, `.technology`, `.identity`) for a deployment
 * that wants them linked again.
 */
export const SECTION_LINKS: Partial<
  Record<LandingSection, { anchor: string; label: TranslationKey }>
> = {
  architecture: { anchor: "architecture", label: "website.menu.architecture" },
  platform: { anchor: "platform", label: "website.menu.platform" },
  trust: { anchor: "trust", label: "website.menu.trust" },
};

/**
 * Where the hero's "see what it does" button goes: the first section below it
 * that can be linked to.
 *
 * Written out rather than hard-coded as `#architecture`, because a deployment
 * that drops the architecture section would otherwise be left with a button
 * that scrolls nowhere — the same failure the menu avoids, on the one control
 * a visitor is most likely to press. Nothing below the hero means nothing to
 * point at, and the button is not drawn.
 */
export function firstLinkedSection(sections: LandingSection[]): string | undefined {
  for (const section of sections) {
    const link = SECTION_LINKS[section];
    if (link) return link.anchor;
  }
  return undefined;
}

/**
 * Reads the section list from the environment.
 *
 *   LANDING_SECTIONS="hero trust technology capabilities"
 *
 * Comma or whitespace separated, and the order given is the order rendered —
 * a deployment that wants its security section first says so by putting it
 * first, rather than by asking for a second variable.
 *
 * An unknown name is dropped with a warning rather than refused: the landing
 * page is the one screen a visitor sees before signing in, and a typo in a
 * deployment file should cost that deployment a section, not its front door.
 * By the same reasoning a list of nothing but typos falls back to the full
 * page — an operator who wrote `LANDING_SECTIONS=heroo` wanted a hero, not a
 * blank sheet.
 */
export function landingSectionsFromEnv(
  env: NodeJS.ProcessEnv = process.env,
): LandingSection[] {
  const raw = (env.LANDING_SECTIONS ?? "").trim();
  if (!raw) return [...LANDING_SECTIONS];

  const known: readonly string[] = LANDING_SECTIONS;
  const chosen: LandingSection[] = [];
  for (const name of raw.split(/[\s,]+/).filter(Boolean)) {
    const section = name.toLowerCase();
    if (!known.includes(section)) {
      console.warn(`LANDING_SECTIONS: "${name}" is not a landing section; ignored`);
      continue;
    }
    // Listing a section twice would render it twice, duplicating an id.
    if (!chosen.includes(section as LandingSection)) chosen.push(section as LandingSection);
  }
  return chosen.length > 0 ? chosen : [...LANDING_SECTIONS];
}

/**
 * The section a menu item's address names, or nothing.
 *
 * The anchor doubles as the slug: `SECTION_LINKS` already holds the one word
 * the menu, the URL and the section's own `id` all use, so a section page is
 * reachable at exactly the address the menu was scrolling to before it became
 * a page — every link ever shared keeps working.
 *
 * Read against the deployment's own list rather than every section there is:
 * a deployment that dropped a section should answer 404 for it, not render a
 * page it decided not to have.
 */
export function sectionByAnchor(
  sections: LandingSection[],
  anchor: string,
): LandingSection | undefined {
  return sections.find((section) => SECTION_LINKS[section]?.anchor === anchor);
}

/**
 * The addresses the menu points at, as paths.
 *
 * Every linked section there is, not the deployment's own list: a deployment
 * that dropped one answers 404 for it, and a 404 must not be mistaken for a
 * screen that wants a session (components/Layout.tsx reads this to know the
 * page is public) or handed to the signed-in shell.
 */
export const SECTION_PATHS: string[] = LANDING_SECTIONS.flatMap((section) => {
  const link = SECTION_LINKS[section];
  return link ? [`/${link.anchor}`] : [];
});
