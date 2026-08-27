"use client";

/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

import React from "react";
import { Package } from "lucide-react";

import { ICONS } from "./icons.generated";

/**
 * The icon a menu entry asked for, by name.
 *
 * The map behind this used to live in components/Layout.tsx, written by hand:
 * about sixty static imports and sixty entries, added by whoever noticed their
 * menu was rendering an empty square. Two things were wrong with it. A name
 * missing from it failed silently, and because it sat in the core's shell, an
 * app outside this repository could not use an icon the core had not already
 * thought of. Twenty-two of its entries turned out to be for apps that had
 * already left; nothing said so, because nothing could.
 *
 * The map is generated now — see scripts/generate-icon-map.mjs — from the icons
 * the platform actually names, in Go menu definitions, catalogue manifests and
 * the shell's own search index. A distribution runs the same generator in its
 * own build and gets its own apps' icons. Nobody edits a list.
 *
 * The fallback is a real icon rather than nothing: "we did not recognise this
 * name" should look like something, not like a gap. It is also not the whole
 * safety net — backend/internal/workspace/menu/icons_test.go fails the build when
 * Go names an icon lucide does not have, and CI regenerates this map and fails
 * on a diff.
 */
export function MenuIcon({ name, className = "w-5 h-5" }: { name?: string; className?: string }) {
  const Icon = (name && ICONS[name]) || Package;
  return <Icon className={className} />;
}
