"use client";

import {useEffect, useRef, useState} from "react";
import Link from "next/link";

import LanguageSwitcher from "@/components/LanguageSwitcher";
import { SECTION_LINKS, type LandingSection } from "@/lib/landing";
import {useI18n} from "@/lib/i18n";
import {useBrand} from "@/lib/brandContext";

/**
 * The public header.
 *
 * Each section item is a page of its own (`app/[section]`), at the address the
 * item used to scroll to. They are listed in the order the page renders those
 * sections, and the list comes from the page rather than being written out
 * here: a deployment that drops a section must not be left with a menu item
 * pointing at a 404.
 *
 * The last item leaves for the published documentation, so it opens in a new
 * tab and carries `rel="noopener"` rather than silently replacing the page
 * someone is reading. Its address is the deployment's (`BRAND_DOCS_URL`): a
 * deployment with its own name has its own manual, and sending its readers to
 * the platform's is sending them somewhere that does not describe what they
 * are looking at.
 *
 * On a narrow screen the same items are behind a button rather than gone.
 * They used to be gone: the stylesheet hid the nav below 900px and the
 * language switcher below 640px, and nothing replaced either — a phone got a
 * logo and a Sign in button, and no way to reach any section of the page or to
 * read it in another language.
 */
export default function SiteHeader({sections}: {sections: LandingSection[]}) {
  const {t} = useI18n();
  const brand = useBrand();
  const [open, setOpen] = useState(false);
  const header = useRef<HTMLElement>(null);

  // Three ways out, because a panel that covers the page and can only be
  // dismissed by finding the same small button again is a trap on a phone: the
  // button, Escape, and a tap anywhere else.
  //
  // "Anywhere else" is measured against the whole header rather than the panel,
  // so a tap on the button itself is inside and is left to the button — closing
  // here and reopening there would make it look like the button does nothing.
  //
  // pointerdown rather than click: it fires for touch and mouse alike, and it
  // fires before the tap has a chance to activate whatever is underneath, so
  // the first tap outside dismisses instead of also pressing something.
  useEffect(() => {
    if (!open) return;

    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Node && header.current?.contains(target)) return;
      setOpen(false);
    };

    document.addEventListener("keydown", onKey);
    document.addEventListener("pointerdown", onPointerDown);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("pointerdown", onPointerDown);
    };
  }, [open]);

  const items = (
    <>
      {sections.map((section) => {
        const link = SECTION_LINKS[section];
        if (!link) return null;
        return (
          <Link key={section} href={`/${link.anchor}`} onClick={() => setOpen(false)}>
            {t(link.label)}
          </Link>
        );
      })}
      <a
        href={brand.docsUrl}
        target="_blank"
        rel="noopener noreferrer"
        onClick={() => setOpen(false)}
      >
        {t("website.menu.docs")}
      </a>
    </>
  );

  return (
    <header className="gp-nav" ref={header}>
      <Link href="/" className="gp-brand">
        <img src={brand.logoUrl} alt="" />
        <span>{brand.name}</span>
      </Link>
      <nav>{items}</nav>
      <div className="gp-actions">
        <LanguageSwitcher variant="dark" />
        <Link href="/login" className="gp-gold">
          {t("website.action.sign_in")}
        </Link>
        {/* The button is the only part of this header that a wide screen never
            shows: above 900px every item is already on the bar. */}
        <button
          type="button"
          className="gp-nav__toggle"
          aria-expanded={open}
          aria-controls="gp-mobile-menu"
          aria-label={t("website.menu.toggle")}
          onClick={() => setOpen((was) => !was)}
        >
          <span aria-hidden="true">{open ? "✕" : "☰"}</span>
        </button>
      </div>
      {open && (
        <div className="gp-nav__menu" id="gp-mobile-menu">
          <nav>{items}</nav>
          {/* The language switcher lives here too: below 640px the stylesheet
              hides the one on the bar, and a reader who cannot find their own
              language leaves. */}
          <div className="gp-nav__menu-actions">
            <LanguageSwitcher variant="dark" />
          </div>
        </div>
      )}
    </header>
  );
}
