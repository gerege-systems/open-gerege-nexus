/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// The API client for the core platform and in-repo modules.
//
// The methods and types live in lib/api/, one file per in-repo module and one
// for the platform itself.

export * from "./api/client";
export * from "./api/integrations";
export * from "./api/store";
export * from "./api/ai";
export * from "./api/sso-clients";
export * from "./api/urtuu";
export * from "./api/kiosk";
export * from "./api/documents";
export * from "./api/reports";

import { coreApi } from "./api/client";
import { integrationsApi } from "./api/integrations";
import { storeApi } from "./api/store";
import { aiApi } from "./api/ai";
import { ssoClientsApi } from "./api/sso-clients";
import { urtuuApi } from "./api/urtuu";
import { documentsApi } from "./api/documents";
import { reportsApi } from "./api/reports";

export const api = {
  ...coreApi,
  ...integrationsApi,
  ...storeApi,
  ...aiApi,
  ...ssoClientsApi,
  ...urtuuApi,
  ...documentsApi,
  ...reportsApi,
};
