import { type ReactNode } from "react";

import Applications from "@/components/landing/Applications";
import Architecture from "@/components/landing/Architecture";
import Capabilities from "@/components/landing/Capabilities";
import Hero from "@/components/landing/Hero";
import PlatformDepth from "@/components/landing/PlatformDepth";
import Services from "@/components/landing/Services";
import Technology from "@/components/landing/Technology";
import Trust from "@/components/landing/Trust";
import { firstLinkedSection, type LandingSection } from "@/lib/landing";
import { servicesFromEnv } from "@/lib/services";

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
 *
 * It lives here rather than beside the home page because it is read twice: the
 * home page renders the sections nobody links to, and app/[section] renders the
 * one a menu item names.
 */
export function sectionNodes(
  sections: LandingSection[],
  localSignIn: boolean,
): Record<LandingSection, ReactNode> {
  return {
    hero: <Hero seeMoreAnchor={firstLinkedSection(sections)} localSignIn={localSignIn} />,
    architecture: <Architecture />,
    applications: <Applications />,
    // Хаягуудыг энд уншина: энэ функц сервер дээр ажилладаг бөгөөд браузерын
    // `process.env` нь зөвхөн build-д шигтгэсэн зүйлийг агуулдаг.
    services: <Services services={servicesFromEnv()} />,
    platform: <PlatformDepth />,
    trust: <Trust />,
    technology: <Technology />,
    capabilities: <Capabilities />,
  };
}
