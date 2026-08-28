/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

/**
 * The kiosk estate's words.
 *
 * Two locales rather than seven, and deliberately: Mongolian is the source
 * language for this app — the estate is operated in Mongolian, by people
 * reading the screens in it — and English is what every other locale falls back
 * to. The five overlay files the other apps carry are not here because nothing
 * has been translated into them yet, and an empty overlay would claim otherwise.
 *
 * The modules behind these screens live in the Gerege Kiosk distribution. The
 * screens are here for the reason every product's screens are: the shell is one
 * image serving every deployment, and a page is dead without its module anyway —
 * absent from the menu, refused by the API.
 */

import { registerDictionary, source } from "../registry";
import { kiosk } from "../addons/kiosk";

registerDictionary("kiosk", source(kiosk));
