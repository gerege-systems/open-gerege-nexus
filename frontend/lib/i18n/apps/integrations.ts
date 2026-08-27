/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

import { registerDictionary, source } from "../registry";
import { integrations } from "../addons/integrations";
import { ar } from "../locales/ar/integrations";
import { zh } from "../locales/zh/integrations";
import { fr } from "../locales/fr/integrations";
import { ru } from "../locales/ru/integrations";
import { es } from "../locales/es/integrations";

registerDictionary("integrations", { ...source(integrations), ar, zh, fr, ru, es });
