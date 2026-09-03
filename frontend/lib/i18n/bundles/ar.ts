/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

/**
 * Everything this build knows how to say in ar, in one chunk.
 *
 * The five optional languages are opt-in per device (see DEFAULT_LOCALES in
 * ../../index.tsx), so on almost every visit none of these words are read. They
 * used to be imported statically anyway — the core overlay from ../index.ts and
 * one file per app from lib/i18n/apps/*.ts — which put 262 KB of Arabic,
 * Chinese, French, Russian and Spanish into the first chunk of every page,
 * including the signed-out landing page. Gathered here instead, the bundler can
 * give the language a chunk of its own and fetch it when somebody asks for it.
 *
 * An app's file is listed here rather than in lib/i18n/apps/<app>.ts, which now
 * registers only the mn/en source. The app still owns its words; what moved is
 * when they load, not who wrote them.
 *
 * It lives beside the locale directories rather than inside one because
 * scripts/i18n-layout.mjs merges every .ts in `locales/<language>/` into that
 * language's overlay: a file exporting a bundle rather than a flat key map
 * would be read as two nonsense keys and reported as orphans by
 * `npm run i18n:check`.
 */
import { ar as core } from "../locales/ar/core";
import { ar as integrations } from "../locales/ar/integrations";
import { ar as sso_clients } from "../locales/ar/sso_clients";
import { ar as storefront } from "../locales/ar/storefront";
import { ar as urtuu } from "../locales/ar/urtuu";

// ai.ts sits beside these and is deliberately absent: its keys are part of the
// core dictionary (lib/i18n/addons/ai.ts, spread in core.ts), not an app's, and
// nothing has ever registered them under an app id. Adding it here would invent
// a dictionary that did not exist before this file did.
const ar = {
  core,
  apps: { integrations, sso_clients, storefront, urtuu },
};

export default ar;
