"use client";

/**
 * The address that led nowhere.
 *
 * Until this file existed the answer was Next's built-in 404: an English
 * sentence on a bare white page, in a product whose every other screen is in
 * the reader's language and inside the workspace chrome. Rendering it here puts
 * it back in both — this page is a child of the root layout, so the shell draws
 * its navigation around it and the reader can leave without the Back button.
 *
 * A client component because the copy comes from the i18n provider, which is
 * where the reader's language lives.
 */

import Link from "next/link";
import { FileQuestion } from "lucide-react";

import { useI18n } from "@/lib/i18n";

export default function NotFound() {
  const { t } = useI18n();
  return (
    <main className="mx-auto flex max-w-[46rem] flex-col items-start gap-4 py-16">
      <FileQuestion className="h-8 w-8 text-muted" aria-hidden="true" />
      <h1 className="text-2xl font-semibold text-foreground">{t("base.error.not_found_title")}</h1>
      <p className="text-sm text-muted">{t("base.error.not_found_body")}</p>
      <Link
        href="/"
        className="mt-2 inline-flex items-center rounded-lg bg-accent px-4 py-2 text-sm font-medium text-on-accent hover:brightness-105"
      >
        {t("base.error.home")}
      </Link>
    </main>
  );
}
