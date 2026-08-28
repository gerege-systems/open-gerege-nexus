"use client";

import React from "react";
import ResourceScreen from "@/components/kiosk/ResourceScreen";
import { KIOSK_BY_SLUG } from "@/lib/kiosk/resources";

/** The directory app's landing screen: the citizens the kiosks serve. */
export default function KioskDirectoryLandingPage() {
  return <ResourceScreen resource={KIOSK_BY_SLUG.users} />;
}
