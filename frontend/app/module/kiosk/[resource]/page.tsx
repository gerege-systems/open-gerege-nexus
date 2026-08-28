"use client";

import React from "react";
import { useParams } from "next/navigation";
import { useI18n } from "@/lib/i18n";
import ResourceScreen from "@/components/kiosk/ResourceScreen";
import { KIOSK_BY_SLUG } from "@/lib/kiosk/resources";
import { FileQuestion } from "lucide-react";

/**
 * One route for every kiosk screen.
 *
 * The 26 modules answer a uniform request dialect, so their screens differ only
 * in the resource definition. A file per screen would be forty near-identical
 * files drifting apart; this stays one.
 *
 * It sits at app/module/kiosk/[resource] rather than app/module/[app]/[feature]
 * because a static segment outranks a dynamic one in the router — the kiosk
 * screens resolve here, everything else keeps falling through to the
 * coming-soon page.
 */
export default function KioskResourcePage() {
  const params = useParams<{ resource: string }>();
  const { t } = useI18n();
  const resource = KIOSK_BY_SLUG[params.resource];

  if (!resource || (resource.app ?? "kiosk") !== "kiosk") {
    return (
      <div className="w-full min-h-[calc(100vh-12rem)] grid place-items-center">
        <div className="text-center">
          <FileQuestion className="w-10 h-10 text-slate-300 mx-auto mb-3" />
          <p className="text-slate-600 font-medium">{t("kiosk.message.unknown_screen")}</p>
          <p className="text-sm text-slate-400 mt-1">{params.resource}</p>
        </div>
      </div>
    );
  }

  return <ResourceScreen resource={resource} />;
}
