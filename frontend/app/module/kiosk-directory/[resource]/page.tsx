"use client";

import React from "react";
import { useParams } from "next/navigation";
import { useI18n } from "@/lib/i18n";
import ResourceScreen from "@/components/kiosk/ResourceScreen";
import { KIOSK_BY_SLUG } from "@/lib/kiosk/resources";
import { FileQuestion } from "lucide-react";

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
