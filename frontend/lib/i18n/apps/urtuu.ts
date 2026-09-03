/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

import { registerDictionary, source } from "../registry";
import { urtuu } from "../addons/urtuu";

// mn and en only. The five optional languages live in
// lib/i18n/locales/<language>/index.ts and arrive through addLocale when a
// reader picks one — importing them here put every language in every page's
// first chunk, which is what nobody was reading.
registerDictionary("urtuu", source(urtuu));
