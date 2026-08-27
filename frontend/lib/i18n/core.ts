/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

import { base } from "./base";
import { web } from "./web";
import { access } from "./addons/access";
import { app_store } from "./addons/app_store";
import { appearance } from "./addons/appearance";
import { auth } from "./addons/auth";
import { core } from "./addons/core";
import { cp } from "./addons/cp";
import { emailverify } from "./addons/emailverify";
import { ai } from "./addons/ai";
import { modules } from "./addons/modules";
import { setup } from "./addons/setup";
import { website } from "./addons/website";

/**
 * The platform's own dictionary, assembled at compile time.
 *
 * `base` and `web` are the shared ones: a term that more than one screen shows
 * belongs there, never duplicated per screen. The rest are the platform's areas
 * — signing in, access control, the app store, the control plane, appearance,
 * email verification, the module chrome and the public site.
 *
 * Keys read `<module>.<kind>.<term>`, where kind classifies the term the way
 * Odoo does — field (a data label), action (a button), menu, state (a selection
 * value), model (a business object), view (screen copy) or message (something
 * said to the user).
 *
 * The apps are not here. They register their own words at runtime through
 * registry.ts, which is what stopped this file from being edited by every app
 * that needed a heading. What is kept from the old arrangement is the part that
 * was worth keeping: this object is `as const`, so TranslationKey below is the
 * exact set of platform keys and a typo in one is a build error rather than a
 * key rendered at somebody.
 */
export const coreDictionary = {
  ...base,
  ...web,
  ...access,
  ...appearance,
  ...app_store,
  ...auth,
  ...cp,
  ...core,
  ...emailverify,
  ...ai,
  ...modules,
  ...setup,
  ...website,
} as const;

/** Every key the platform itself defines. An app's key is an ordinary string. */
export type TranslationKey = keyof typeof coreDictionary;
