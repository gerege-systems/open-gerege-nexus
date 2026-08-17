import { Fragment, type ReactNode } from "react";

import Applications from "@/components/landing/Applications";
import Architecture from "@/components/landing/Architecture";
import Capabilities from "@/components/landing/Capabilities";
import Hero from "@/components/landing/Hero";
import PlatformDepth from "@/components/landing/PlatformDepth";
import SiteFooter from "@/components/landing/SiteFooter";
import SiteHeader from "@/components/landing/SiteHeader";
import Storefront from "@/components/landing/Storefront";
import Technology from "@/components/landing/Technology";
import Trust from "@/components/landing/Trust";
import { firstLinkedSection, landingSectionsFromEnv, type LandingSection } from "@/lib/landing";
import { localSignInEnabledOnServer } from "@/lib/signIn";
import { fetchStorefrontOnServer } from "@/lib/storefront";

/**
 * The public landing page — the only screen a visitor sees before signing in.
 *
 * It is composed rather than written out: each section is a self-contained
 * piece of the argument the page makes, and the ones carrying anchors
 * (`#features`, `#trust`, `#technology`) own those ids themselves, so this
 * file keeps working without knowing what is inside them.
 *
 * The default order answers questions in the order they are asked. What is
 * this, and how is it built. What do I get. What is underneath it. Only then
 * how identity works — which is why the page closes on the claim that signing
 * in is not a screen but the floor everything above it stands on. Put first,
 * that claim is a detail about a login box; put last, it is the point.
 *
 * Which sections a deployment shows, and in what order, is its own
 * (`LANDING_SECTIONS` — see lib/landing.ts). The reasoning above is the
 * default's, not a rule: a deployment that is only an identity provider is
 * making a shorter argument and should be allowed to make it.
 *
 * # The store
 *
 * A deployment carrying the app-store modules answers a different question. Its
 * visitor is not asking what the platform is; they are asking what is in the
 * catalogue. So that deployment gets the catalogue, and the platform's argument
 * gives way to it.
 *
 * Which page that is gets decided here, on the server, before anything is sent.
 * It was briefly decided in the browser instead, and that was wrong twice over:
 * a visitor saw the platform page and then watched it turn into a shop, and
 * anything that does not run JavaScript — every crawler — only ever saw the
 * first of those. A catalogue nobody can find is not much of a shop.
 *
 * It stays a run-time question rather than a build-time one, though: one image
 * serves every deployment, so the image cannot know which one it is. The
 * deployment says where its API is (`API_INTERNAL_URL`) and this asks.
 */

/**
 * Every section, by name.
 *
 * A `Record` over the union rather than a lookup that might miss: adding a
 * section to `LANDING_SECTIONS` without giving it a component fails the
 * typecheck here, which is the whole check this pairing needs.
 *
 * A function of the chosen list because the hero's second button points at
 * whatever comes after it, which is not knowable until the list is read — and
 * of `localSignIn`, because a deployment that hands sign-in to somebody else
 * must not draw a sign-in card that answers 403.
 */
function sectionNodes(sections: LandingSection[], localSignIn: boolean): Record<LandingSection, ReactNode> {
  return {
    hero: <Hero seeMoreAnchor={firstLinkedSection(sections)} localSignIn={localSignIn} />,
    architecture: <Architecture />,
    applications: <Applications />,
    platform: <PlatformDepth />,
    trust: <Trust />,
    technology: <Technology />,
    capabilities: <Capabilities />,
  };
}

// Rendered per request, not prerendered at build.
//
// Which product this deployment is depends on an environment variable, and a
// build has no environment: prerendering baked the platform page into the image
// and served it to the first visitor after every deploy — for a full minute,
// until the first revalidation replaced it with the shop. A page that is wrong
// exactly when somebody first looks at it is wrong.
//
// The render is cheap and the fetch behind it is not repeated: it carries its
// own 60-second cache, so the API is asked once a minute however many people
// arrive.
export const dynamic = "force-dynamic";

export default async function LandingPage() {
  // Two questions of the same API, asked together: one page render, one wait.
  const [apps, localSignIn] = await Promise.all([
    fetchStorefrontOnServer(),
    localSignInEnabledOnServer(),
  ]);
  // Read on the server and handed down, for the reason app/layout.tsx reads the
  // brand there: `process.env` in the browser holds only what the build inlined.
  const sections = landingSectionsFromEnv();
  const nodes = sectionNodes(sections, localSignIn);

  return (
    <div className="gp-landing" id="top">
      <SiteHeader sections={sections} />
      <main>
        {apps ? (
          <Storefront apps={apps} />
        ) : (
          sections.map((section) => <Fragment key={section}>{nodes[section]}</Fragment>)
        )}
      </main>
      <SiteFooter />
    </div>
  );
}
