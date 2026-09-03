/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

import { reports } from "../addons/reports";
import { registerDictionary, source } from "../registry";

// The unified report runner's words. The reports themselves come from each
// app's module; the runner screen is shared, so its dictionary registers once.
registerDictionary("reports", source(reports));
