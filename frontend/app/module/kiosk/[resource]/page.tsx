"use client";

import React from "react";
import { useParams } from "next/navigation";
import { useI18n } from "@/lib/i18n";
import ResourceScreen from "@/components/kiosk/ResourceScreen";
import { KIOSK_BY_SLUG } from "@/lib/kiosk/resources";
import { ErrorState } from "@gerege-systems/ui";

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
      <div className="w-full min-h-[calc(100dvh-12rem)] grid place-items-center">
        <ErrorState variant="404" title={t("kiosk.message.unknown_screen")} description={params.resource} />
      </div>
    );
  }

  return <ResourceScreen resource={resource} />;
}
