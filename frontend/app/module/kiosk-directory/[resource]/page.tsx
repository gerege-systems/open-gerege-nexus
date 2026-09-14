"use client";

import React from "react";
import { useParams } from "next/navigation";
import { useI18n } from "@/lib/i18n";
import ResourceScreen from "@/components/kiosk/ResourceScreen";
import { KIOSK_BY_SLUG } from "@/lib/kiosk/resources";
import { ErrorState } from "@gerege-systems/ui";

/**
 * The directory app's screens. Same registry and same component as the
 * operations app — only the mount prefix and the install gate differ, which is
 * exactly what made the split worth doing and this file cheap.
 */
export default function KioskDirectoryResourcePage() {
  const params = useParams<{ resource: string }>();
  const { t } = useI18n();
  const resource = KIOSK_BY_SLUG[params.resource];

  if (!resource || resource.app !== "kiosk-directory") {
    return (
      <div className="w-full min-h-[calc(100dvh-12rem)] grid place-items-center">
        <ErrorState variant="404" title={t("kiosk.message.unknown_screen")} description={params.resource} />
      </div>
    );
  }

  return <ResourceScreen resource={resource} />;
}
