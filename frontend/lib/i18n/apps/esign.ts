/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

import { esign } from "../addons/esign";
import { registerDictionary, source } from "../registry";

// The PDF signing screens travel inside the Documents app (chronicle 2.0.0:
// one app, one slug). Their words register under their own id so the two
// dictionaries can move independently while the screens share routes.
registerDictionary("esign", source(esign));
