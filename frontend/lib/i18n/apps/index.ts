/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

/**
 * The apps this build carries, registering their own words.
 *
 * Imported for the side effect: each module calls registerDictionary at the top
 * level, so by the time anything renders, every app UI bundled in this
 * frontend has handed its strings over. This is not a claim that its backend
 * module is in the base Go binary: Documents is an optional UI consumed by a
 * product distribution. A distribution's own frontend entry can register more
 * dictionaries without changing the platform dictionary.
 *
 * Why eager, and not from each app's route.
 *
 * The obvious arrangement is for an app to register in its own
 * `app/<slug>/layout.tsx`, so that its translations are loaded only where they
 * are read. That is wrong here, and measurably so: an app's keys are not only
 * read on the app's own routes.
 *
 *   ai            components/AICopilot.tsx, mounted in the shell header on
 *                 every page (components/Layout.tsx:359, :399)
 *   documents     contract/inbox screens bundled for the client distribution
 *   urtuu         app/module/urtuu/links — the channel's own screen
 *   integrations  app/module/integrations/connectors, and app/line's tile
 *   storefront    components/landing/Storefront.tsx — the signed-out page
 *
 * Registering from the route would leave each of those rendering English while
 * the rest of the page is in Mongolian, and nothing would fail: `t()` falls
 * back to English by design, which is right for a term nobody has translated
 * and wrong for one that is sitting in a file that has not been imported. A
 * silent half-translated screen is exactly the failure this split must not
 * introduce.
 *
 * So the list is here rather than nowhere. What it is not is the old
 * arrangement under a new name: index.tsx no longer imports any app, no app
 * key is part of TranslationKey, and a distribution adds an app without
 * touching either this file or that one. Adding an in-repo app is one line
 * here, in a file that exists for that line.
 *
 * Contacts, products, inventory, billing, gov and appstore_modules were
 * removed on 2026-08-21. Documents later returned only as a gated frontend
 * bundle; its module, migrations and API still live in client-gerege-nexus.
 */

import "./sso_clients";
import "./integrations";
import "./documents";
import "./storefront";
import "./urtuu";
import "./kiosk";
