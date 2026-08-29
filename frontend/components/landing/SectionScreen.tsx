import { notFound, redirect } from "next/navigation";

import SiteFooter from "@/components/landing/SiteFooter";
import SiteHeader from "@/components/landing/SiteHeader";
import { sectionNodes } from "@/components/landing/sections";
import { landingSectionsFromEnv, type LandingSection } from "@/lib/landing";
import { setupRequiredOnServer } from "@/lib/setup";

/**
 * One page per header menu item.
 *
 * The menu used to scroll: every section lived on the landing page and the
 * items were anchors into it. A visitor who followed one had no address to
 * share, no back button that meant anything, and a phone had to download the
 * whole argument to read a third of it. So each linked section is now a page,
 * at the address its anchor already named — every link ever shared still lands
 * on the same words.
 *
 * A page file per section rather than one `app/[section]` route, which is the
 * shorter diff and the wrong one: a dynamic segment at the root matches every
 * unmatched path, and this app's layout streams its shell before the page
 * renders, so the `notFound()` inside arrives after a 200 has been sent. Soft
 * 404s on every mistyped address is a steep price for saving two three-line
 * files.
 *
 * The header still gets the whole section list, so the menu is the same menu on
 * every page — a reader who followed one item can reach the others.
 *
 * `hero` is never rendered here: it has no menu item, and the sign-in card it
 * carries belongs on the front door, which is why `localSignIn` is not read.
 */
export default async function SectionScreen({ section }: { section: LandingSection }) {
  // The same handoff the landing page makes (app/page.tsx): before there is an
  // organisation there is nothing here that is true yet, and the only person
  // who can be reading is the one who should be in the wizard.
  if (await setupRequiredOnServer()) redirect("/setup");
  const sections = landingSectionsFromEnv();
  // A deployment that dropped the section dropped its menu item too; nothing
  // links here, and rendering a section the deployment decided not to have
  // would be showing words it took down.
  if (!sections.includes(section)) notFound();

  return (
    <div className="gp-landing" id="top">
      <SiteHeader sections={sections} />
      <main>{sectionNodes(sections, false)[section]}</main>
      <SiteFooter />
    </div>
  );
}
