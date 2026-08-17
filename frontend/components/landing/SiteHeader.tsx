"use client";

import Link from "next/link";

import LanguageSwitcher from "@/components/LanguageSwitcher";
import { SECTION_LINKS, type LandingSection } from "@/lib/landing";
import {useI18n} from "@/lib/i18n";
import {useBrand} from "@/lib/brandContext";

/**
 * The public header.
 *
 * The section items scroll to a part of this page and are listed in the order
 * those sections are rendered, so the first item is never the furthest away —
 * which is why the list comes from the page rather than being written out
 * here: a deployment that drops a section must not be left with a menu item
 * that scrolls nowhere.
 *
 * The last item leaves for the published documentation, so it opens in a new
 * tab and carries `rel="noopener"` rather than silently replacing the page
 * someone is reading. Its address is the deployment's (`BRAND_DOCS_URL`): a
 * deployment with its own name has its own manual, and sending its readers to
 * the platform's is sending them somewhere that does not describe what they
 * are looking at.
 */
export default function SiteHeader({sections}: {sections: LandingSection[]}) {
  const {t} = useI18n();
  const brand = useBrand();

  return (
    <header className="gp-nav">
      <a href="#top" className="gp-brand">
        <img src={brand.logoUrl} alt="" />
        <span>{brand.name}</span>
      </a>
      <nav>
        {sections.map((section) => {
          const link = SECTION_LINKS[section];
          if (!link) return null;
          return (
            <a key={section} href={`#${link.anchor}`}>
              {t(link.label)}
            </a>
          );
        })}
        <a href={brand.docsUrl} target="_blank" rel="noopener noreferrer">
          {t("website.menu.docs")}
        </a>
      </nav>
      <div className="gp-actions">
        <LanguageSwitcher variant="dark" />
        <Link href="/login" className="gp-gold">
          {t("website.action.sign_in")}
        </Link>
      </div>
    </header>
  );
}
