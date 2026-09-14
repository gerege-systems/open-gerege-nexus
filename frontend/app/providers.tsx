"use client";

import React from "react";

import Layout from "@/components/Layout";
import InstallApp from "@/components/InstallApp";
import { DesignSystemProvider, Toaster, TooltipProvider, mnStrings } from "@gerege-systems/ui";
import { I18nProvider } from "@/lib/i18n";
import { ThemeProvider } from "@/lib/theme";
import { BrandProvider } from "@/lib/brandContext";
import type { Brand } from "@/lib/brand";
import type { BrandCopy } from "@/lib/brandCopy";

/**
 * Everything under the document that has to run in the browser.
 *
 * This used to be the root layout itself, which was a client component so the
 * providers could live in it. The brand is what split them: it is read from the
 * environment at request time, and only a server component can read it, so the
 * root became one and this file took the providers. The file comment there
 * once called that split larger than the metadata warranted — it was, for
 * metadata; a name the image is not allowed to know is a different bargain.
 *
 * The brand is passed down rather than fetched: it is in the first byte of HTML
 * the server sends, so no screen shows one product's name and then swaps it.
 * The deployment's wording travels the same way and for the same reason.
 */
export default function Providers({
  brand,
  copy,
  children,
}: {
  brand: Brand;
  copy: BrandCopy;
  children: React.ReactNode;
}) {
  // The brand name reaches every translation as {brand}; see lib/i18n. I18n is
  // the outer provider because the name itself is translatable — BrandProvider
  // resolves it against the language being read.
  return (
    <I18nProvider brand={brand.name} copy={copy}>
      <BrandProvider brand={brand} copy={copy}>
        <ThemeProvider>
          {/* The design system's own strings (aria-labels, "Close", pagination
              words) in Mongolian; the platform's copy stays in lib/i18n. */}
          <DesignSystemProvider strings={mnStrings}>
            <TooltipProvider>
              <Layout>{children}</Layout>
              <InstallApp />
              <Toaster />
            </TooltipProvider>
          </DesignSystemProvider>
        </ThemeProvider>
      </BrandProvider>
    </I18nProvider>
  );
}
