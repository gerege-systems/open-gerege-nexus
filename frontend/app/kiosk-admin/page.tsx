"use client";

import React from "react";
import ResourceScreen from "@/components/kiosk/ResourceScreen";
import { KIOSK_BY_SLUG } from "@/lib/kiosk/resources";

/**
 * The app's landing screen. Terminals are what the kiosk estate is about.
 *
 * It sits at /kiosk-admin rather than /kiosk because /kiosk is the terminal's
 * own public screen — the one a citizen sees standing in front of the machine,
 * and the only kiosk route in PUBLIC_ROUTES. This is the console that manages
 * that estate, and it is not public.
 */
export default function KioskLandingPage() {
  return <ResourceScreen resource={KIOSK_BY_SLUG.terminals} />;
}
