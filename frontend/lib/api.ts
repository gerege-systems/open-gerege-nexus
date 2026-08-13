import { apiBase } from "@/lib/apiBase";

export { apiBase };

async function fetcher<T>(url: string, options: RequestInit = {}): Promise<T> {
  // Server-owned content (menu labels, app store copy) is translated by the
  // API, so every request carries the locale the user picked.
  const locale = typeof window !== "undefined" ? window.localStorage.getItem("locale") || "mn" : "mn";
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "Accept-Language": locale,
    ...(options.headers as Record<string, string>),
  };
  const res = await fetch(`${apiBase()}${url}`, {
    ...options,
    headers,
    credentials: "include",
  });

  if (!res.ok) {
    let errMessage = "Request failed";
    try {
      const errData = await res.json();
      errMessage = errData.error || errMessage;
    } catch {
      // ignore
    }
    // The status rides along so a caller can tell a transient failure from an
    // answer: a polling loop should retry a dropped connection and stop on a 409.
    const failure = new Error(errMessage) as Error & { status?: number };
    failure.status = res.status;
    throw failure;
  }

  // 204 carries no body by definition, so parsing one would throw on success.
  if (res.status === 204) {
    return undefined as T;
  }

  return res.json();
}

export const APP_MENU_CHANGED_EVENT = "gerege:app-menu-changed";

/** What kind of change a release was. The store colours by it. */
export type ReleaseKind = "feature" | "fix" | "security" | "breaking" | "docs";

/**
 * One line of an app's timeline.
 *
 * `type` is "release" for the publisher's own record — the chronicle entry,
 * already reduced to one language by the server — and the installation event
 * type ("installed", "upgraded", "held", …) for everything this organisation
 * did. `system` marks the lines the auto-update sweep is responsible for
 * rather than a person.
 */
export interface AppHistoryEntry {
  at: string;
  type: string;
  version?: string;
  from?: string;
  kind?: ReleaseKind;
  summary?: string;
  details?: string;
  authors?: string[];
  refs?: string[];
  actor_id?: string;
  actor_name?: string;
  system?: boolean;
  reason?: string;
  added?: string;
}

/** One row of the administrator's store overview. */
export interface StoreOverviewApp {
  app_id: string;
  slug: string;
  name: string;
  binary_version?: string;
  catalog_version: string;
  installed_version?: string;
  installed: boolean;
  enabled: boolean;
  update_available: boolean;
  auto_update: boolean;
  held?: boolean;
  pinned_version?: string;
  /** The compiled module and the catalogue disagree. Always a fault. */
  drifted?: boolean;
  release_kind?: ReleaseKind;
  release_summary?: string;
}

/** An organisation's publishing identity. One per tenant. */
export interface Publisher {
  id: string;
  slug: string;
  name: string;
  contact_email: string;
  verified: boolean;
  verified_at?: string;
  created_at: string;
}

/** An app as the registry holds it — what is *offered*, not what is installed. */
export interface StoreApp {
  id: string;
  slug: string;
  type: "module" | "external";
  name: string;
  description: string;
  category: string;
  visibility: string;
  publisher_name?: string;
  latest_version?: string;
  license?: string;
  repository?: string;
}

/** One submitted or published release. */
export interface StoreVersion {
  id: string;
  app_id: string;
  version: string;
  channel: string;
  min_platform: string;
  status: "draft" | "in_review" | "published" | "rejected" | "yanked";
  submitted_by?: string;
  review_note?: string;
  published_at?: string;
  created_at: string;
  manifest?: { release_notes?: ManifestReleaseNotes };
}

/** A manifest's release notes, as they arrive inside a catalogue entry. */
export interface ManifestReleaseNotes {
  kind?: ReleaseKind;
  summary?: Record<string, string>;
  details?: Record<string, string>;
  authors?: string[];
  refs?: string[];
}

async function mutateApp(url: string) {
  const result = await fetcher<{ status: string; app: string }>(url, { method: "POST" });
  // Layout lives above the App Store pages, so a route refresh does not
  // recreate it. Notify the mounted shell to refetch tenant menus immediately.
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent(APP_MENU_CHANGED_EVENT, { detail: result }));
  }
  return result;
}

export type IntegrationProvider =
  | "webhook"
  | "government"
  | "payment"
  | "custom_rest"
  | "google_drive"
  | "dropbox"
  | "google_meet";

export interface Integration {
  id: string;
  provider: IntegrationProvider;
  name: string;
  target_url: string;
  /** The administrator's intent. A failure is reported in last_error and does
   *  not switch the connector off. */
  status: "ACTIVE" | "INACTIVE";
  config: Record<string, string>;
  account_label: string;
  /** True once an OAuth grant is stored. The token itself never comes back. */
  connected: boolean;
  connected_at?: string;
  last_ping_at?: string;
  last_error?: string;
  capabilities: string[];
  created_at: string;
  updated_at: string;
}

/**
 * Email verification — proving an address, through the hosted service.
 *
 * The platform holds no mailbox credential and issues no keys of its own: it
 * asks the verification service for a link and finds out when the person came
 * back. The service key is a server-side secret and never reaches this code.
 */
export interface EmailVerification {
  id: string;
  /** Who asked: an app module id, or "portal". */
  source: string;
  purpose?: string;
  email: string;
  redirect_url?: string;
  status: "PENDING" | "VERIFIED" | "EXPIRED";
  expires_at: string;
  verified_at?: string;
  created_at: string;
}

export interface EmailVerifyOverview {
  stats: {
    total: number;
    verified: number;
    pending: number;
    expired: number;
    last_24h: number;
    verified_pct: number;
  };
  recent: EmailVerification[];
  /** Whether a service key is present at all. The key itself never comes back. */
  configured: boolean;
  /** The service's own health check, and what it said when it failed. */
  reachable: boolean;
  health?: string;
  provider_url: string;
  admin_url: string;
  return_url: string;
}

export interface IntegrationInput {
  provider: IntegrationProvider;
  name: string;
  target_url?: string;
  /** Write-only. Left blank on an update it means "unchanged", not "clear it". */
  secret_key?: string;
  status?: string;
  config?: Record<string, string>;
}

/** One report, as the list shows it. */
export interface ReportSummary {
  key: string;
  app: string;
  title: string;
  titles: Record<string, string>;
}

export interface ReportGroup {
  app: string;
  reports: ReportSummary[];
}

export interface ReportParamOption {
  value: string;
  titles: Record<string, string>;
}

export interface ReportParam {
  key: string;
  kind: "date_range" | "uuid" | "select" | "text" | "bool";
  titles: Record<string, string>;
  required: boolean;
  options?: ReportParamOption[];
  default?: unknown;
}

export interface ReportColumn {
  key: string;
  titles: Record<string, string>;
  kind: "text" | "number" | "money" | "date" | "month" | "percent";
  /** Which axis this column belongs on, when the report is drawn. */
  chart?: "category" | "value";
  total?: boolean;
}

export interface ReportMetadata {
  key: string;
  app: string;
  titles: Record<string, string>;
  params: ReportParam[];
  columns: ReportColumn[];
}

export interface ReportResult {
  columns: ReportColumn[];
  rows: Array<Record<string, unknown>>;
  totals?: Record<string, number>;
  notes?: Array<{ level: string; message: string }>;
}

export interface ReportSchedule {
  id: string;
  report_key: string;
  name: string;
  params: Record<string, unknown>;
  cron: string;
  format: string;
  recipients: string[];
  active: boolean;
  last_run_at?: string;
  last_status?: string;
  last_error?: string;
  created_at: string;
  titles?: Record<string, string>;
}

export interface ReportScheduleInput {
  report_key: string;
  name: string;
  params: Record<string, string>;
  cron: string;
  format: string;
  recipients: string[];
  active?: boolean;
}

export interface ReportGrant {
  id: string;
  report_key: string;
  grantor_tenant_id: string;
  grantor_name: string;
  grantee_tenant_id: string;
  grantee_name: string;
  scope: "counterparty" | "full";
  counterparty_ref?: string;
  valid_from: string;
  valid_until?: string;
  revoked_at?: string;
  accepted_at?: string;
  note?: string;
  created_at: string;
  /** Which side of the agreement this organisation is on. */
  direction: "given" | "received";
  titles?: Record<string, string>;
}

export interface ReportGrantRequest {
  grantor_registration_number: string;
  report_key: string;
  scope: "counterparty" | "full";
  valid_until?: string;
  note?: string;
}

/** One line of "who read our data". */
export interface ReportAccessEntry {
  at: string;
  report_key: string;
  by: string;
  details: Record<string, unknown>;
}

export const api = {
  // Auth
  login: (email: string, password: string) =>
    fetcher<{ expires_at: string; user: any }>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),

  loginWithEID: (code?: string, redirectURI?: string, regNumber?: string, otpCode?: string, authMethod?: string) =>
    fetcher<{ expires_at: string; user: any; identity: any }>("/auth/eid/login", {
      method: "POST",
      body: JSON.stringify({ code, redirect_uri: redirectURI, reg_number: regNumber, otp_code: otpCode, auth_method: authMethod }),
    }),

  startEID: (callbackUrl = "") => fetcher<{session_id:string;device_link_url?:string;verification_code:string;expires_at:string}>("/auth/eid/start",{method:"POST",body:JSON.stringify({callbackUrl})}),
  startEIDByNationalID: (nationalId:string,callbackUrl = "") => fetcher<{session_id:string;device_link_url?:string;verification_code:string;expires_at:string}>("/auth/eid/start-id",{method:"POST",body:JSON.stringify({national_id:nationalId,callbackUrl})}),
  // The poll is a long poll the API holds open for up to 25s, so the caller
  // passes a signal to drop it the moment the citizen cancels or leaves.
  pollEID: (sessionId:string,signal?:AbortSignal) => fetcher<{state:string;expires_at?:string;identity?:any}>("/auth/eid/poll",{method:"POST",body:JSON.stringify({session_id:sessionId}),signal}),

  loginWithDAN: (danToken?: string, regNumber?: string, otpCode?: string) =>
    fetcher<{ expires_at: string; user: any; dan_profile: any }>("/auth/dan/login", {
      method: "POST",
      body: JSON.stringify({ dan_token: danToken, reg_number: regNumber, otp_code: otpCode }),
    }),

  // end_session_url is set only on a deployment that signs people in through
  // an SSO provider. The session here is already gone by the time it is
  // returned; what is left is to send the browser there so the provider ends
  // its own, and returns the person to this deployment afterwards.
  logout: () => fetcher<{ status: string; end_session_url?: string }>("/auth/logout", { method: "POST" }),

  // The caller's own record. There is no id parameter: the session decides
  // whose profile is read, so this cannot be pointed at somebody else.
  profile: () =>
    fetcher<{
      id: string; name: string; email: string; created_at: string; is_admin: boolean;
      organisations: Array<{ id: string; name: string; slug: string }>;
      identities: Array<{
        kind: string; provider: string; subject: string;
        email?: string; name?: string; surname?: string;
        linked_at: string; last_seen_at: string;
        claims?: Record<string, unknown>;
        issuer?: string; removable?: boolean;
      }>;
      active_sessions: number;
    }>("/profile"),

  // Detach one linked identity. Scoped to the caller for the same reason
  // profile() is: the body names a provider and an account there, never a
  // person. The answer is the list that remains, so the screen never has to
  // guess which buttons should still exist.
  unlinkIdentity: (body: { kind: string; issuer?: string; subject: string }) =>
    fetcher<{
      identities: Array<{
        kind: string; provider: string; subject: string;
        email?: string; name?: string; surname?: string;
        linked_at: string; last_seen_at: string;
        claims?: Record<string, unknown>;
        issuer?: string; removable?: boolean;
      }>;
    }>("/profile/identities/unlink", { method: "POST", body: JSON.stringify(body) }),

  // A first sign-in from an external provider, waiting on eID. The binding
  // token names it; nobody is signed in yet, so it is the whole authority.
  bindingSession: (binding: string) =>
    fetcher<{
      provider: string; email: string; name: string; consented: boolean;
      claims: Record<string, unknown>; eid_claims: string[];
    }>(`/auth/bind/session?b=${encodeURIComponent(binding)}`),

  bindingConsent: (binding: string) =>
    fetcher<{ consented: boolean }>("/auth/bind/consent", {
      method: "POST", body: JSON.stringify({ binding }),
    }),

  bindingEIDStart: (binding: string, nationalId?: string) =>
    fetcher<{session_id:string;device_link_url?:string;verification_code:string;expires_at?:string}>(
      "/auth/bind/eid/start",
      { method: "POST", body: JSON.stringify({ binding, national_id: nationalId }) },
    ),

  bindingEIDPoll: (binding: string, sessionId: string, signal?: AbortSignal) =>
    fetcher<{state:string;expires_at?:string;bound?:boolean}>("/auth/bind/eid/poll", {
      method: "POST", body: JSON.stringify({ binding, session_id: sessionId }), signal,
    }),

  // Who is asking, for the sign-in screen to name while nobody is signed in.
  // Resolved by the server rather than read out of the URL: a name taken from a
  // query parameter is a name anybody can write, which is how a convincing
  // "sign in to <your bank>" screen gets built.
  oauthClientInfo: (clientId: string) =>
    fetcher<{ client_id: string; client_name: string; logo_uri?: string }>(
      `/oauth2/client-info?client_id=${encodeURIComponent(clientId)}`,
    ),

  // How this deployment signs people in. `enabled` false is the ordinary case:
  // the login screen shows its own eID and password forms. `enabled` true means
  // identity belongs to `provider_name`, and the screen's job is to hand the
  // browser to `start_url` rather than to ask for anything.
  ssoConfig: () =>
    fetcher<{
      enabled: boolean;
      provider_name?: string;
      start_url?: string;
      local_login: boolean;
      // Reported available only when it can actually be used: a deployment that
      // federates has closed its own sign-in paths, and Google is one of them.
      google: { enabled: boolean; start_url?: string };
    }>("/auth/sso/config"),

  // permissions carries the effective grant of every role the member holds; it
  // is empty for administrators, who bypass the check.
  getMe: () => fetcher<{ id: string; tenant_id: string; tenant_name: string; name: string; email: string; is_admin: boolean; permissions?: string[] }>("/auth/me"),

  // The organisations the signed-in person may act for. A membership in one is
  // the common case, so callers should expect a list of one rather than treat
  // it as an error.
  getTenants: () =>
    fetcher<{
      current: string;
      tenants: Array<{ id: string; name: string; slug: string }>;
      // Which of them this session is reading across. Always contains current.
      active: string[];
    }>("/auth/tenants"),

  // Read across several organisations at once, the way Odoo's allowed companies
  // work. New rows still go to the one being acted in — that is decided by the
  // row-level policies, not here.
  setActiveTenants: (tenantIds: string[]) =>
    fetcher<{ active: string[] }>("/auth/tenants/active", {
      method: "POST",
      body: JSON.stringify({ tenant_ids: tenantIds }),
    }),

  // Moves the session to another of them. The server rotates the token and
  // re-sets the cookie, so everything fetched before this call belongs to the
  // tenant just left — the caller reloads rather than patching state.
  switchTenant: (tenantId: string) =>
    fetcher<{ tenant_id: string; switched: boolean; expires_at?: string }>("/auth/switch-tenant", {
      method: "POST",
      body: JSON.stringify({ tenant_id: tenantId }),
    }),

  getMenus: () => fetcher<Array<{ id: string; app_id?: string; app_name?: string; parent_id?: string; label: string; path?: string; icon: string; order: number }>>("/menus"),

  // Odoo-style tenant access control
  getAccessOverview: () => fetcher<{
    roles: Array<{ id:string; code:string; name:string; description:string; active:boolean; system:boolean; permissions:string[] }>;
    permissions: Array<{ code:string; name:string; description:string; app:string }>;
    members: Array<{ membership_id:string; user_id:string; name:string; email:string; is_admin:boolean; roles:string[] }>;
  }>("/admin/access/overview"),
  createRole: (data:{code:string;name:string;description:string}) => fetcher<{id:string}>("/admin/access/roles",{method:"POST",body:JSON.stringify(data)}),
  updateRole: (id:string,data:{name:string;description:string;active:boolean}) => fetcher(`/admin/access/roles/${id}`,{method:"PUT",body:JSON.stringify(data)}),
  deleteRole: (id:string) => fetcher<void>(`/admin/access/roles/${id}`,{method:"DELETE"}),
  setRolePermissions: (id:string,permissions:string[]) => fetcher(`/admin/access/roles/${id}/permissions`,{method:"PUT",body:JSON.stringify({permissions})}),
  setMembershipRoles: (id:string,roles:string[]) => fetcher(`/admin/access/memberships/${id}/roles`,{method:"PUT",body:JSON.stringify({roles})}),
  getDevices: () => fetcher<{devices:Array<{id:string;name:string;platform:string;form_factor:string;site:string;status:string;app_version:string;os_version:string;last_seen_at?:string;enrolled_at:string}>}>("/admin/devices"),
  createDeviceEnrollmentCode: () => fetcher<{code:string;expires_at:string}>("/admin/devices/enrollment-codes",{method:"POST"}),
  setStaffPin: (membershipId:string,pin:string) => fetcher<{status:string}>("/admin/devices/staff-pin",{method:"PUT",body:JSON.stringify({membership_id:membershipId,pin})}),
  setDeviceStatus: (id:string,status:"ACTIVE"|"DISABLED"|"RETIRED") => fetcher<{status:string}>("/admin/devices/status",{method:"PUT",body:JSON.stringify({id,status})}),
  registerPushToken: (provider:"APNS"|"FCM",token:string,appId:string) => fetcher<void>("/push-tokens",{method:"POST",body:JSON.stringify({provider,token,app_id:appId})}),
  getCurrentShift: () => fetcher<{shift:null|{id:string;membership_id:string;opened_at:string;opening_float:number} }>("/devices/shifts/current"),
  openShift: (openingFloat:number,notes="") => fetcher<{id:string;opened_at:string}>("/devices/shifts/open",{method:"POST",body:JSON.stringify({opening_float:openingFloat,notes})}),
  closeShift: (closingTotal:number,notes="") => fetcher<{id:string;status:string}>("/devices/shifts/close",{method:"POST",body:JSON.stringify({closing_total:closingTotal,notes})}),

  // Store
  //
  // A manifest carries release notes since the chronicle: one sentence saying
  // what changed in the version being offered, already resolved to the
  // caller's language by the server. It is what turns "an update is available"
  // into something an administrator can decide about.
  getStoreApps: () =>
    fetcher<
      Array<{
        id: string;
        slug: string;
        name: string;
        description: string;
        icon_url: string;
        category: string;
        version: string;
        installed: boolean;
        enabled: boolean;
        installed_version?: string;
        latest_version: string;
        update_available: boolean;
        manifest: any;
      }>
    >("/store/apps"),

  getInstalledApps: () =>
    fetcher<
      Array<{
        id: string;
        app_id: string;
        slug: string;
        name: string;
        installed_version: string;
        status: string;
        enabled: boolean;
        installed_at: string;
        auto_update: boolean;
        pinned_version?: string;
        latest_version?: string;
        update_available: boolean;
        // What a waiting version asks for that the installed one did not.
        // Non-empty means the update is being held for an administrator to
        // approve rather than offered as an ordinary one.
        held_for?: string[];
        held_reason?: string;
        // Part of the platform. Such an app has no Disable button, because
        // disabling it is refused server-side and a button that only ever
        // fails is worse than no button.
        core: boolean;
      }>
    >("/installed-apps"),

  // What changed in an app, and what this organisation did about it, merged
  // into one timeline newest first. A "release" line is the publisher's; every
  // other kind is this tenant's own installation history.
  getAppHistory: (slug: string) =>
    fetcher<{
      app_id: string;
      slug: string;
      name: string;
      installed_version: string;
      latest_version: string;
      timeline: AppHistoryEntry[];
    }>(`/store/apps/${slug}/history`),

  // The administrator's single view of the store: which versions the binary,
  // the catalogue and this tenant each hold, and where they disagree.
  getStoreOverview: () =>
    fetcher<{
      platform_version: string;
      sync: {
        source: "file" | "registry";
        sync_interval: string;
        last_sync_at?: string;
        last_sync_ok?: boolean;
        last_sync_error?: string;
      };
      apps: StoreOverviewApp[];
      summary: { catalog: number; installed: number; updates: number; held: number; drifted: number };
    }>("/admin/store/overview"),

  // --- App Store: the three module surfaces ---------------------------------
  //
  // Only reachable on the instance that *is* the store; every other deployment
  // has these apps uninstalled and the routes gated off.
  getPublisherProfile: () => fetcher<Publisher>("/publisher"),
  savePublisherProfile: (data: { slug: string; name: string; contact_email: string }) =>
    fetcher<Publisher>("/publisher", { method: "PUT", body: JSON.stringify(data) }),
  getPublisherApps: () => fetcher<StoreApp[]>("/publisher/apps"),
  getPublisherVersions: (slug: string) =>
    fetcher<StoreVersion[]>(`/publisher/apps/${slug}/versions`),
  submitVersion: (slug: string, manifest: unknown, channel = "stable") =>
    fetcher<StoreVersion>(`/publisher/apps/${slug}/versions`, {
      method: "POST",
      body: JSON.stringify({ channel, manifest }),
    }),

  getReviewQueue: () => fetcher<StoreVersion[]>("/store-review/queue"),
  decideVersion: (id: string, action: "publish" | "reject" | "yank", note = "") =>
    fetcher<{ status: string }>(`/store-review/versions/${id}`, {
      method: "POST",
      body: JSON.stringify({ action, note }),
    }),
  getReviewPublishers: () => fetcher<Publisher[]>("/store-review/publishers"),
  verifyPublisher: (id: string, verified: boolean) =>
    fetcher<{ verified: boolean }>(
      `/store-review/publishers/${id}/verify?verified=${verified}`, { method: "POST" }),

  getRegistryState: () =>
    fetcher<{ revision: number; key_id: string; public_key: string }>("/appstore/registry/state"),
  rebuildCatalogue: () =>
    fetcher<{ discarded: number }>("/appstore/registry/rebuild", { method: "POST" }),

  // Whether an app follows the catalogue on its own. Turning it on also clears
  // a hold, which is why this refreshes the menus like the other store
  // mutations: an app held back can start contributing menus again.
  setAutoUpdate: (slug: string, enabled: boolean) =>
    fetcher<{ app: string; auto_update: boolean }>(`/store/apps/${slug}/auto-update`, {
      method: "POST",
      body: JSON.stringify({ enabled }),
    }),

  installApp: (slug: string) => mutateApp(`/store/apps/${slug}/install`),

  // An upgrade changes which menus an app contributes just as an install does,
  // so it goes through the same notification rather than beside it.
  upgradeApp: (slug: string) => mutateApp(`/store/apps/${slug}/upgrade`),

  // Ask the registry for a catalog now rather than at the next scheduled sync.
  // Answers 501 on a deployment that reads its catalog from a file, which is
  // every self-hosted one — the button is hidden there rather than failing.
  // Where the catalogue comes from and how the last refresh went. The hourly
  // sync leaves only a log line, so this is the one place a registry that has
  // been failing for a week is distinguishable from one that has published
  // nothing.
  getCatalogStatus: () =>
    fetcher<{
      source: "file" | "registry";
      apps: number;
      sync_interval: string;
      last_sync_at?: string;
      last_sync_ok?: boolean;
      last_sync_error?: string;
    }>("/admin/store/status"),

  syncStore: () =>
    fetcher<{ status: "updated" | "unchanged"; apps: number }>("/admin/store/sync", { method: "POST" }),

  enableApp: (slug: string) => mutateApp(`/store/apps/${slug}/enable`),

  disableApp: (slug: string) => mutateApp(`/store/apps/${slug}/disable`),

  // Organisation & People — the platform's own core app. What the organisation
  // is, how it is arranged, and who works in it.
  getOrganisation: () =>
    fetcher<{
      tenant_id: string; slug: string; name: string; legal_name: string;
      registration_number: string; tax_number: string; country_code: string;
      province: string; district: string; khoroo: string; address_line: string;
      postal_code: string; phone: string; email: string; website: string;
      logo_url: string; timezone: string; locale: string; currency: string;
      // The organisation this one is a subsidiary of. A branch or an office is
      // a department; this is another legal entity, and so another tenant.
      parent_tenant_id?: string; parent_name?: string;
    }>("/core/organisation"),

  // Partial by design: a form that sends the fields it changed must not blank
  // the ones it did not mention.
  updateOrganisation: (patch: Record<string, string>) =>
    fetcher("/core/organisation", { method: "PUT", body: JSON.stringify(patch) }),

  getDepartments: () =>
    fetcher<Array<{
      id: string; code: string; name: string; parent_id?: string;
      manager_membership_id?: string; manager_name?: string;
      active: boolean; people_count: number;
      tenant_id: string; tenant_name: string;
    }>>("/core/departments"),

  createDepartment: (body: { code: string; name: string; parent_id?: string; manager_membership_id?: string }) =>
    fetcher<{ id: string }>("/core/departments", { method: "POST", body: JSON.stringify(body) }),

  updateDepartment: (id: string, body: { name: string; parent_id?: string; manager_membership_id?: string }) =>
    fetcher(`/core/departments/${id}`, { method: "PUT", body: JSON.stringify(body) }),

  // Archiving keeps the row, because people and documents point at it.
  archiveDepartment: (id: string) =>
    fetcher(`/core/departments/${id}/archive`, { method: "POST" }),

  // Deleting removes it, and the server refuses the moment anything does point
  // at it — this is for the unit created by mistake, not the one that was used.
  deleteDepartment: (id: string) => fetcher(`/core/departments/${id}`, { method: "DELETE" }),

  // The other half of archiving. It is reversible by design, so the screen that
  // lists what it archived can put one back.
  restoreDepartment: (id: string) =>
    fetcher(`/core/departments/${id}/restore`, { method: "POST" }),

  getPeople: () =>
    fetcher<Array<{
      membership_id: string; user_id: string; name: string; email: string;
      phone: string; job_title: string; department_id?: string;
      department_name?: string; active: boolean; is_admin: boolean;
      roles: string[]; joined_at: string;
      // Which organisation this membership is in. The list spans every
      // organisation the session is reading across, so a row can belong to one
      // other than the one being acted in.
      tenant_id: string; tenant_name: string;
    }>>("/core/people"),

  updatePerson: (id: string, body: { job_title?: string; department_id?: string }) =>
    fetcher(`/core/people/${id}`, { method: "PUT", body: JSON.stringify(body) }),

  setPersonActive: (id: string, active: boolean) =>
    fetcher(`/core/people/${id}/${active ? "reactivate" : "deactivate"}`, { method: "POST" }),

  getPreferences: () =>
    fetcher<{
      name: string; email: string; phone: string; locale: string; timezone: string;
      organisation_locale: string; organisation_timezone: string;
    }>("/core/me/preferences"),

  updatePreferences: (patch: { name?: string; phone?: string; locale?: string; timezone?: string }) =>
    fetcher("/core/me/preferences", { method: "PUT", body: JSON.stringify(patch) }),

  // Contacts App
  getContacts: () =>
    fetcher<
      Array<{
        id: string;
        name: string;
        email: string;
        phone: string;
        company: string;
        active: boolean;
        created_at: string;
      }>
    >("/contacts"),

  createContact: (data: { name: string; email: string; phone: string; company: string; active: boolean }) =>
    fetcher("/contacts", { method: "POST", body: JSON.stringify(data) }),

  updateContact: (id: string, data: { name: string; email: string; phone: string; company: string; active: boolean }) =>
    fetcher(`/contacts/${id}`, { method: "PUT", body: JSON.stringify(data) }),

  // Products App
  getProducts: () =>
    fetcher<
      Array<{
        id: string;
        sku: string;
        name: string;
        price: number;
        active: boolean;
        created_at: string;
      }>
    >("/products"),

  createProduct: (data: { sku: string; name: string; price: number; active: boolean }) =>
    fetcher("/products", { method: "POST", body: JSON.stringify(data) }),

  updateProduct: (id: string, data: { sku: string; name: string; price: number; active: boolean }) =>
    fetcher(`/products/${id}`, { method: "PUT", body: JSON.stringify(data) }),

  // Inventory App
  getWarehouses: () =>
    fetcher<
      Array<{
        id: string;
        code: string;
        name: string;
        address: string;
        created_at: string;
      }>
    >("/inventory/warehouses"),

  createWarehouse: (data: { code: string; name: string; address: string }) =>
    fetcher("/inventory/warehouses", { method: "POST", body: JSON.stringify(data) }),

  getStockLevels: () =>
    fetcher<
      Array<{
        id: string;
        warehouse_id: string;
        product_id: string;
        quantity: number;
        updated_at: string;
      }>
    >("/inventory/stock-levels"),

  getStockMovements: () =>
    fetcher<
      Array<{
        id: string;
        warehouse_id: string;
        product_id: string;
        quantity_change: number;
        reference: string;
        created_at: string;
      }>
    >("/inventory/movements"),

  adjustStock: (data: { warehouse_id: string; product_id: string; quantity_change: number; reference: string }) =>
    fetcher("/inventory/adjustments", { method: "POST", body: JSON.stringify(data) }),

  // AI Assistant & Forecasting
  queryAICopilot: (prompt: string) =>
    fetcher<{ answer: string; intent: string; data?: any; actionable?: string[] }>("/ai/copilot", {
      method: "POST",
      body: JSON.stringify({ prompt }),
    }),

  chatAI: (data: {
    prompt?: string;
    lang?: string;
    history?: Array<{ role: "user" | "model"; text: string }>;
    audio?: { mime: string; data: string };
  }) => fetcher<{ answer: string; reply: string; steps?: Array<{ tool: string }>; degraded?: boolean }>("/ai/chat", {
    method: "POST", body: JSON.stringify(data),
  }),

  speakAI: (text: string) => fetcher<{ mime: string; data: string }>("/ai/tts", {
    method: "POST", body: JSON.stringify({ text }),
  }),

  translateAI: (data: { text?: string; audio?: { mime: string; data: string }; target_lang: string; speak?: boolean }) =>
    fetcher<{ source_text: string; translated: string; audio?: { mime: string; data: string } }>("/ai/translate", {
      method: "POST", body: JSON.stringify(data),
    }),

  getAIPrompts: () => fetcher<Array<{key:string;content:string;active:boolean;global:boolean}>>("/admin/ai/prompts"),
  updateAIPrompt: (key:string, content:string, active=true) => fetcher(`/admin/ai/prompts/${key}`, {method:"PUT",body:JSON.stringify({content,active})}),
  getAIKnowledge: () => fetcher<Array<{id:string;title:string;content:string;source_url:string;updated_at:string}>>("/admin/ai/knowledge"),
  createAIKnowledge: (data:{title:string;content:string;source_url:string}) => fetcher<{id:string}>("/admin/ai/knowledge",{method:"POST",body:JSON.stringify(data)}),

  getAIForecast: () =>
    fetcher<
      Array<{
        product_id: string;
        sku: string;
        product_name: string;
        current_stock: number;
        recommended_min: number;
        reorder_alert: boolean;
        suggested_reorder: number;
      }>
    >("/ai/stock-forecast"),

  // XYP State Data Exchange (xyp.gerege.mn)
  queryXYPCitizen: (regNumber: string) =>
    fetcher<{
      reg_number: string;
      civil_id: string;
      last_name: string;
      first_name: string;
      gender: string;
      address: string;
      passport_status: string;
      verified: boolean;
    }>("/xyp/citizen", {
      method: "POST",
      body: JSON.stringify({ reg_number: regNumber }),
    }),

  queryXYPCompany: (companyReg: string) =>
    fetcher<{
      company_reg: string;
      name: string;
      executive: string;
      address: string;
      vat_payer: boolean;
      status: string;
      founding_date: string;
    }>("/xyp/company", {
      method: "POST",
      body: JSON.stringify({ company_reg: companyReg }),
    }),

  // External Integrations Manager.
  //
  // Connectors are per tenant and stored server-side; the secret and any OAuth
  // grant are write-only, so nothing here ever reads a credential back.
  getIntegrations: () => fetcher<Integration[]>("/integrations"),

  // Which providers this deployment can actually offer. A provider whose OAuth
  // client was never configured comes back unavailable with the reason, so the
  // screen can say why instead of showing a form that cannot work.
  getIntegrationProviders: () =>
    fetcher<{
      providers: Array<{
        provider: IntegrationProvider;
        oauth: boolean;
        capabilities: string[];
        available: boolean;
        reason?: string;
      }>;
      encryption_configured: boolean;
      redirect_uri: string;
    }>("/integrations/providers"),

  registerIntegration: (data: IntegrationInput) =>
    fetcher<Integration>("/integrations", { method: "POST", body: JSON.stringify(data) }),

  updateIntegration: (id: string, data: IntegrationInput) =>
    fetcher<Integration>(`/integrations/${id}`, { method: "PUT", body: JSON.stringify(data) }),

  deleteIntegration: (id: string) =>
    fetcher<{ status: string }>(`/integrations/${id}`, { method: "DELETE" }),

  // Starts the OAuth grant. The answer is the provider URL to send the
  // administrator to; the callback lands back on the settings screen.
  connectIntegration: (id: string) =>
    fetcher<{ authorization_url: string }>(`/integrations/${id}/connect`, { method: "POST" }),

  disconnectIntegration: (id: string) =>
    fetcher<{ status: string }>(`/integrations/${id}/disconnect`, { method: "POST" }),

  // What has recently left the platform. A signed document reaching an outside
  // account is a disclosure, and this is the record of it.
  getIntegrationDeliveries: (limit = 50) =>
    fetcher<
      Array<{
        id: string;
        integration_id: string;
        kind: string;
        reference: string;
        outcome: "OK" | "FAILED";
        detail?: string;
        external_id?: string;
        external_url?: string;
        created_at: string;
      }>
    >(`/integrations/deliveries?limit=${limit}`),

  // Send an already-signed document to a storage connector. Automatic export
  // covers documents signed after a connector was set up; this covers the ones
  // signed before it, and the retry after a destination was unreachable.
  exportEsignDocument: (id: string, integrationId?: string) =>
    fetcher<{ exported: Array<{ integration_name: string; provider: string; url?: string }> }>(
      `/esign/documents/${id}/export`,
      { method: "POST", body: JSON.stringify(integrationId ? { integration_id: integrationId } : {}) }
    ),

  // Reports (io.gerege.nexus.reports)
  //
  // The engine is generic, so the client is too: nothing here names a report.
  // A screen lists what the tenant may run, asks for one report's declaration,
  // and posts parameters back against it.
  getReports: () => fetcher<{ groups: ReportGroup[] }>("/reports"),

  getReport: (key: string) => fetcher<ReportMetadata>(`/reports/${encodeURIComponent(key)}`),

  runReport: (key: string, params: Record<string, string>) =>
    fetcher<{ key: string; title: string; result: ReportResult }>(
      `/reports/${encodeURIComponent(key)}/run`,
      { method: "POST", body: JSON.stringify({ params }) },
    ),

  // Not fetcher: the answer is a spreadsheet, not JSON. The blob is handed
  // back so the caller can name the download from the Content-Disposition the
  // server set rather than guessing an extension.
  exportReport: async (key: string, params: Record<string, string>, format: "xlsx" | "csv") => {
    const locale = typeof window !== "undefined" ? window.localStorage.getItem("locale") || "mn" : "mn";
    const res = await fetch(
      `${apiBase()}/reports/${encodeURIComponent(key)}/export?format=${format}`,
      {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json", "Accept-Language": locale },
        body: JSON.stringify({ params }),
      },
    );
    if (!res.ok) {
      throw new Error("Export failed");
    }
    const disposition = res.headers.get("Content-Disposition") || "";
    const match = /filename="?([^"]+)"?/.exec(disposition);
    return { blob: await res.blob(), filename: match?.[1] || `report.${format}` };
  },

  getReportSchedules: () =>
    fetcher<{ schedules: ReportSchedule[]; delivery_configured: boolean }>("/reports/schedules"),

  createReportSchedule: (input: ReportScheduleInput) =>
    fetcher<{ id: string }>("/reports/schedules", { method: "POST", body: JSON.stringify(input) }),

  updateReportSchedule: (id: string, input: ReportScheduleInput) =>
    fetcher<{ id: string }>(`/reports/schedules/${id}`, { method: "PUT", body: JSON.stringify(input) }),

  deleteReportSchedule: (id: string) =>
    fetcher<void>(`/reports/schedules/${id}`, { method: "DELETE" }),

  // Cross-tenant sharing (§3.5 of the monitoring and reporting proposal).
  getReportGrants: () => fetcher<{ grants: ReportGrant[] }>("/reports/grants"),

  getReportAccessHistory: () =>
    fetcher<{ history: ReportAccessEntry[] }>("/reports/grants/history"),

  requestReportGrant: (input: ReportGrantRequest) =>
    fetcher<{ id: string }>("/reports/grants", { method: "POST", body: JSON.stringify(input) }),

  acceptReportGrant: (id: string) =>
    fetcher<{ id: string }>(`/reports/grants/${id}/accept`, { method: "POST" }),

  revokeReportGrant: (id: string) =>
    fetcher<{ id: string }>(`/reports/grants/${id}/revoke`, { method: "POST" }),

  runConsolidatedReport: (key: string, params: Record<string, string>) =>
    fetcher<{ key: string; title: string; result: ReportResult }>(
      `/reports/${encodeURIComponent(key)}/run-consolidated`,
      { method: "POST", body: JSON.stringify({ params }) },
    ),

  // Billing App (io.gerege.nexus.billing)
  getInvoices: () =>
    fetcher<
      Array<{
        id: string;
        invoice_number: string;
        contact_name: string;
        amount: number;
        vat_amount: number;
        ebarimt_status: string;
        status: string;
        created_at: string;
      }>
    >("/billing/invoices"),

  createInvoice: (data: { contact_name: string; amount: number }) =>
    fetcher("/billing/invoices", { method: "POST", body: JSON.stringify(data) }),

  // Documents App (io.gerege.nexus.documents)
  // One page of a tenant's documents, newest first, with how many there are in total —
  // each row counts its own signatures and outstanding steps, so the list cannot be
  // unbounded, and a screen showing part of it has to be able to say so.
  getDocuments: (params?: {
    status?: string;
    doc_type?: string;
    q?: string;
    order?: "oldest";
    limit?: number;
    offset?: number;
    // Continue after a row already seen. Prefer this to offset on a list other people
    // are changing: offset counts from the start of a set that can shift, so a document
    // approved between two requests makes the next one skip a row — and a skipped row is
    // on no screen at all. Both halves together, or neither.
    after_at?: string;
    after_id?: string;
  }) => {
    const query = new URLSearchParams();
    if (params?.status) query.set("status", params.status);
    if (params?.doc_type) query.set("doc_type", params.doc_type);
    if (params?.q) query.set("q", params.q);
    if (params?.order) query.set("order", params.order);
    if (params?.limit) query.set("limit", String(params.limit));
    if (params?.offset) query.set("offset", String(params.offset));
    if (params?.after_at && params?.after_id) {
      query.set("after_at", params.after_at);
      query.set("after_id", params.after_id);
    }
    const suffix = query.toString() ? `?${query}` : "";
    return fetcher<{
      documents: Array<{
        id: string;
        title: string;
        doc_type: string;
        status: string;
        signed_by?: string;
        signature_hash?: string;
        signer_reg_number?: string;
        signer_method?: string;
        signed_at?: string;
        signature_count: number;
        required_signatures: number;
        outstanding_steps: number;
        created_at: string;
      }>;
      total: number;
      limit: number;
      offset: number;
    }>(`/documents${suffix}`);
  },

  // A title can be corrected until the first signature; after that it is what the
  // citizen read on their own device before approving.
  renameDocument: (id: string, title: string) =>
    fetcher(`/documents/${id}/title`, { method: "PUT", body: JSON.stringify({ title }) }),

  createDocument: (data: { title: string; doc_type: string }) =>
    fetcher("/documents", { method: "POST", body: JSON.stringify(data) }),

  // E-ID signing is an approval the citizen gives on their own device: start
  // pushes the request — naming the document — and poll waits for them to answer.
  // eID has no document-signing endpoint; that approval is the signature.
  startEIDSignature: (id: string, regNumber: string) =>
    fetcher<{
      session_id: string;
      verification_code: string;
      // Absent when eID states no deadline — the normal case for a push session.
      // Absent is not "expired"; it means nobody has said when this one dies.
      expires_at?: string;
      device_link_url?: string;
      display_text: string;
    }>(`/documents/${id}/sign/eid/start`, { method: "POST", body: JSON.stringify({ reg_number: regNumber }) }),

  // The API holds this open for up to 25s, so the caller passes a signal to drop
  // it the moment the operator closes the dialog.
  pollEIDSignature: (id: string, sessionId: string, signal?: AbortSignal) =>
    fetcher<{ state: string; document?: any }>(`/documents/${id}/sign/eid/poll`, {
      method: "POST",
      body: JSON.stringify({ session_id: sessionId }),
      signal,
    }),

  // DAN exposes no approval push, so it stays a registration number and a code.
  signDocumentWithDAN: (id: string, data: { reg_number: string; otp_code: string }) =>
    fetcher(`/documents/${id}/sign/dan`, { method: "POST", body: JSON.stringify(data) }),

  // Send a draft for approval.
  routeDocument: (id: string) => fetcher(`/documents/${id}/route`, { method: "POST" }),

  // A document's signature ledger, oldest first.
  getDocumentSignatures: (id: string) =>
    fetcher<
      Array<{
        signer_name: string;
        signer_reg_number: string;
        signer_method: string;
        signature_hash: string;
        signed_at: string;
        step_order: number;
        certificate_serial?: string;
        certificate_issuer?: string;
      }>
    >(`/documents/${id}/signatures`),

  // The document's OWN approval chain — the copy taken when it started waiting,
  // which a later configuration change does not touch.
  getDocumentSteps: (id: string) =>
    fetcher<Array<{ order: number; name: string; signer_reg_number: string }>>(`/documents/${id}/steps`),

  // Templates a document is started from
  getDocumentTemplates: () =>
    fetcher<
      Array<{
        id: string;
        name: string;
        doc_type: string;
        title_pattern: string;
        active: boolean;
        created_at: string;
      }>
    >("/documents/templates"),

  createDocumentTemplate: (data: { name: string; doc_type: string; title_pattern: string }) =>
    fetcher("/documents/templates", { method: "POST", body: JSON.stringify(data) }),

  updateDocumentTemplate: (
    id: string,
    data: { name: string; doc_type: string; title_pattern: string; active: boolean }
  ) => fetcher(`/documents/templates/${id}`, { method: "PUT", body: JSON.stringify(data) }),

  deleteDocumentTemplate: (id: string) => fetcher<void>(`/documents/templates/${id}`, { method: "DELETE" }),

  useDocumentTemplate: (id: string) => fetcher(`/documents/templates/${id}/use`, { method: "POST" }),

  // How each document type may be signed
  getSignaturePolicies: () =>
    fetcher<
      Array<{
        doc_type: string;
        allow_eid: boolean;
        allow_dan: boolean;
        require_named_signer: boolean;
        configured: boolean;
        updated_at?: string;
      }>
    >("/documents/policies"),

  saveSignaturePolicy: (
    docType: string,
    data: { allow_eid: boolean; allow_dan: boolean; require_named_signer: boolean }
  ) => fetcher(`/documents/policies/${docType}`, { method: "PUT", body: JSON.stringify(data) }),

  // Who must sign each document type, in order
  getDocumentWorkflows: () =>
    fetcher<
      Array<{
        doc_type: string;
        steps: Array<{ order: number; name: string; signer_reg_number: string }>;
      }>
    >("/documents/workflows"),

  saveDocumentWorkflow: (docType: string, steps: Array<{ name: string; signer_reg_number: string }>) =>
    fetcher(`/documents/workflows/${docType}`, { method: "PUT", body: JSON.stringify({ steps }) }),

  // How long each document type is kept
  getRetentionRules: () =>
    fetcher<
      Array<{
        doc_type: string;
        retain_years: number;
        note: string;
        configured: boolean;
        updated_at?: string;
        // Absent when the server could not count them; a save treats that as
        // non-fatal, so the caller must not read absence as zero.
        expired?: number;
        total?: number;
      }>
    >("/documents/retention"),

  saveRetentionRule: (docType: string, data: { retain_years: number; note: string }) =>
    fetcher(`/documents/retention/${docType}`, { method: "PUT", body: JSON.stringify(data) }),

  // Reject a pending document — moves it to REJECTED.
  rejectDocument: (id: string) =>
    fetcher(`/documents/${id}/reject`, { method: "POST" }),

  // PDF E-Sign App (io.gerege.nexus.esign)
  getEsignDocuments: () =>
    fetcher<
      Array<{
        id: string;
        title: string;
        file_name: string;
        status: string;
        page_count: number;
        signer_name: string;
        signer_reg_no: string;
        signer_phone: string;
        signed_at: string | null;
        created_at: string;
      }>
    >("/esign/documents"),

  uploadEsignDocument: (data: { title: string; file_name: string; pdf_base64: string }) =>
    fetcher("/esign/documents", { method: "POST", body: JSON.stringify(data) }),

  checkEsignCert: (data: { phone_no: string; civil_id: string; data?: string }) =>
    fetcher<{ is_valid: boolean; given_name: string; surname: string; common_name: string; uid: string }>(
      "/esign/cert/check",
      { method: "POST", body: JSON.stringify(data) }
    ),

  signEsignDocument: (
    id: string,
    data: { phone_no: string; signer_name: string; signer_reg_no: string; signature_image64: string }
  ) => fetcher<{ status: string; document_id: string; signed_at: string }>(`/esign/documents/${id}/sign`, {
    method: "POST",
    body: JSON.stringify(data),
  }),

  getEsignLogs: () =>
    fetcher<
      Array<{
        id: string;
        document_id: string;
        reg_no: string;
        phone_no: string;
        first_name: string;
        last_name: string;
        action: string;
        created_at: string;
      }>
    >("/esign/logs"),

  downloadEsignDocument: async (id: string, variant: "original" | "signed"): Promise<Blob> => {
    const res = await fetch(`${apiBase()}/esign/documents/${id}/download?variant=${variant}`, {
      credentials: "include",
    });
    if (!res.ok) throw new Error("Download failed");
    return res.blob();
  },

  // Email verification.
  //
  // There is no key management here any more: keys belong to the sending
  // service and are administered there. What this platform keeps is the record
  // of what it asked for.
  getEmailVerifyOverview: (limit = 25) =>
    fetcher<EmailVerifyOverview>(`/admin/email-verification/overview?limit=${limit}`),

  // Ask the service for a link. App modules call the Go service directly; this
  // is for the product's own screens.
  sendEmailVerification: (data: { email: string; redirect_url?: string; purpose?: string }) =>
    fetcher<EmailVerification>("/verify/send", { method: "POST", body: JSON.stringify(data) }),

  // Developer Portal & OAuth2 SSO Apps
  //
  // client_secret comes back only from create and rotate-secret; every other
  // read omits it, because the server keeps a digest and cannot reproduce it.
  getDeveloperApps: () => fetcher<OAuth2Client[]>("/developer/apps"),
  getDeveloperApp: (clientID: string) =>
    fetcher<OAuth2Client>(`/developer/apps/${encodeURIComponent(clientID)}`),
  createDeveloperApp: (app: OAuth2ClientDraft) =>
    fetcher<OAuth2Client>("/developer/apps", { method: "POST", body: JSON.stringify(app) }),
  updateDeveloperApp: (clientID: string, app: OAuth2ClientDraft) =>
    fetcher<OAuth2Client>(`/developer/apps/${encodeURIComponent(clientID)}`, {
      method: "PUT",
      body: JSON.stringify(app),
    }),
  deleteDeveloperApp: (clientID: string) =>
    fetcher<void>(`/developer/apps/${encodeURIComponent(clientID)}`, { method: "DELETE" }),
  rotateDeveloperAppSecret: (clientID: string) =>
    fetcher<OAuth2Client>(`/developer/apps/${encodeURIComponent(clientID)}/rotate-secret`, {
      method: "POST",
    }),
  getDeveloperScopes: () =>
    fetcher<{ scopes: OAuth2Scope[]; grant_types: string[] }>("/developer/scopes"),
  getDeveloperEndpoints: () => fetcher<Record<string, string>>("/developer/endpoints"),
  getDeveloperSigningKeys: () =>
    fetcher<{ keys: SigningKey[]; jwks_uri: string }>("/developer/signing-keys"),
  getDeveloperAudit: () =>
    fetcher<{ clients: ClientActivity[]; consents: ConsentRecord[] }>("/developer/audit"),
  revokeDeveloperAppTokens: (clientID: string) =>
    fetcher<{ revoked: number }>(`/developer/apps/${encodeURIComponent(clientID)}/tokens`, {
      method: "DELETE",
    }),
  withdrawDeveloperConsent: (clientID: string, userID: string) =>
    fetcher<void>(
      `/developer/apps/${encodeURIComponent(clientID)}/consents/${encodeURIComponent(userID)}`,
      { method: "DELETE" },
    ),

  // OAuth2 consent screen. The query string is the authorization request the
  // browser arrived with; the server re-validates all of it rather than
  // trusting what the page echoes back.
  getConsentPrompt: (query: string) => fetcher<ConsentPrompt>(`/oauth2/consent?${query}`),
  decideConsent: (query: string, approved: boolean) => {
    const form = new URLSearchParams(query);
    form.set("approved", String(approved));
    return fetcher<{ redirect_to: string }>("/oauth2/consent", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: form.toString(),
    });
  },
};

export type OAuth2Scope = {
  name: string;
  description: string;
  description_mn: string;
  sensitive?: boolean;
};

export type OAuth2ClientDraft = {
  client_name: string;
  client_uri?: string;
  client_type?: "confidential" | "public";
  redirect_uris: string[];
  /**
   * Where the platform may return somebody after this application signs them
   * out of it, matched exactly like redirect_uris. Optional: an application
   * that never ends a session here needs none.
   */
  post_logout_redirect_uris?: string[];
  grant_types: string[];
  scopes: string[];
  disabled?: boolean;
};

export type OAuth2Client = {
  id: string;
  client_id: string;
  client_name: string;
  client_uri?: string;
  client_type: "confidential" | "public";
  redirect_uris: string[];
  post_logout_redirect_uris: string[];
  grant_types: string[];
  scopes: string[];
  disabled: boolean;
  created_at: string;
  updated_at: string;
  secret_rotated_at?: string;
  last_used_at?: string;
  /** Present only in the response that created or rotated it. */
  client_secret?: string;
};

export type SigningKey = {
  kid: string;
  algorithm: string;
  active: boolean;
  created_at: string;
  retired_at?: string;
};

export type ClientActivity = {
  client_id: string;
  client_name: string;
  client_type: "confidential" | "public";
  disabled: boolean;
  active_access_tokens: number;
  active_refresh_tokens: number;
  consented_users: number;
  last_used_at?: string;
};

export type ConsentRecord = {
  client_id: string;
  client_name: string;
  user_id: string;
  user_email: string;
  user_name: string;
  scopes: string[];
  granted_at: string;
};

export type ConsentPrompt = {
  client_id: string;
  client_name: string;
  client_uri?: string;
  logo_uri?: string;
  redirect_uri: string;
  scopes: OAuth2Scope[];
  already_granted: string[];
};
