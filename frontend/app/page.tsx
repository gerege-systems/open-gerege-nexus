import { Fragment } from "react";
import { redirect } from "next/navigation";

import SiteFooter from "@/components/landing/SiteFooter";
import SiteHeader from "@/components/landing/SiteHeader";
import Storefront from "@/components/landing/Storefront";
import { sectionNodes } from "@/components/landing/sections";
import { SECTION_LINKS, landingSectionsFromEnv } from "@/lib/landing";
import { setupRequiredOnServer } from "@/lib/setup";
import { localSignInEnabledOnServer } from "@/lib/signIn";
import { fetchStorefrontOnServer } from "@/lib/storefront";

/**
 * The public landing page — the first screen a visitor sees before signing in.
 *
 * It is composed rather than written out: each section is a self-contained
 * piece of the argument the site makes, so this file keeps working without
 * knowing what is inside them.
 *
 * It no longer carries all of them. A section the header menu names now has a
 * page of its own (app/architecture, /platform, /trust — see
 * components/landing/SectionScreen.tsx), so this page renders only what is
 * left: the hero, and the sections that were always read on the way down
 * rather than jumped to. The menu decides the split — `SECTION_LINKS` is the
 * one list, so linking a section moves it off this page and unlinking it
 * brings it back, and no section is ever printed twice on the site.
 *
 * The default order answers questions in the order they are asked. What is
 * this. What do I get. What is underneath it.
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
  // Three questions of the same API, asked together: one page render, one wait.
  const [apps, localSignIn, setupRequired] = await Promise.all([
    fetchStorefrontOnServer(),
    localSignInEnabledOnServer(),
    setupRequiredOnServer(),
  ]);
  // A deployment with no organisation has no visitors yet — only the person who
  // installed it, and the one thing they need is the wizard. Everything this
  // page would otherwise say is false there: nobody can sign in, the store
  // cannot be installed from, and the argument the sections make is about a
  // platform that is not running yet.
  //
  // Sent rather than linked, because the state ends the moment the wizard is
  // finished: a link would be a permanent piece of furniture answering a
  // question that is asked once. The wizard itself refuses without the token
  // the operator was given in the log, so this discloses nothing a stranger
  // could not learn by trying to sign in.
  if (setupRequired) redirect("/setup");
  // Read on the server and handed down, for the reason app/layout.tsx reads the
  // brand there: `process.env` in the browser holds only what the build inlined.
  const sections = landingSectionsFromEnv();
  const nodes = sectionNodes(sections, localSignIn);
  const here = sections.filter((section) => !SECTION_LINKS[section]);

  return (
    <div className="gp-landing" id="top">
      <SiteHeader sections={sections} />
      <main>
        {apps ? (
          <Storefront apps={apps} />
        ) : (
          here.map((section) => <Fragment key={section}>{nodes[section]}</Fragment>)
        )}
      </main>
      <SiteFooter />
    </div>
  );
}
