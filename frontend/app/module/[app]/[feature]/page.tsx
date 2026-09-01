"use client";

import { useParams } from "next/navigation";
import { useI18n } from "@/lib/i18n";
import { Clock3, Construction, Sparkles } from "lucide-react";

function title(value: string) { return decodeURIComponent(value).split("-").map((word) => word.charAt(0).toUpperCase()+word.slice(1)).join(" "); }

export default function ComingSoonModulePage() {
  const params=useParams<{app:string;feature:string}>();
  const {t}=useI18n();
  return <div className="w-full min-h-[calc(100vh-8rem)] grid place-items-center">
    <section className="w-full max-w-3xl bg-surface border border-line rounded-xl p-8 sm:p-12 text-center relative overflow-hidden">
      <div className="absolute inset-x-0 top-0 h-1 bg-accent"/>
      <div className="mx-auto w-16 h-16 rounded-xl bg-accent-soft text-accent grid place-items-center mb-6"><Construction className="w-8 h-8"/></div>
      <p className="text-xs font-semibold uppercase tracking-[0.2em] text-accent">{title(params.app)} · roadmap</p>
      <h1 className="mt-2 text-3xl sm:text-4xl font-semibold text-foreground">{title(params.feature)}</h1>
      <p className="mt-4 text-muted max-w-xl mx-auto">{t("web.view.coming_soon_body")}</p>
      <div className="mt-8 inline-flex items-center gap-2 rounded-full bg-surface-2 px-4 py-2 text-sm font-medium text-muted"><Clock3 className="w-4 h-4"/>{t("web.view.coming_soon")}<Sparkles className="w-4 h-4 text-accent"/></div>
    </section>
  </div>;
}
