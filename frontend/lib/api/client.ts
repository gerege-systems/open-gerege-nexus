/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// The core API client: the request every other client is built on, and the
// endpoints that belong to the platform itself rather than to an app.
//
// Every distribution imports this file. An app's endpoint added here is an
// endpoint every deployment carries whether or not it has the app — which is
// how lib/api.ts reached 1831 lines and became the file app work changed most
// often (40 of the last 133 commits that touched an app; see
// docs/CORE_BOUNDARY_PLAN.md §2.1). check-api-boundaries.mjs is what says so
// now.

import { apiBase } from "@/lib/apiBase";

export { apiBase };

export async function request<T>(url: string, options: RequestInit = {}): Promise<T> {
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

export type OAuth2Scope = {
  name: string;
  description: string;
  description_mn: string;
  sensitive?: boolean;
};

export type ConsentPrompt = {
  client_id: string;
  client_name: string;
  client_uri?: string;
  logo_uri?: string;
  redirect_uri: string;
  scopes: OAuth2Scope[];
  // Nullable because a server that has recorded no grant sends `null`, and the
  // screens this type feeds are rendered against deployments of several ages.
  already_granted: string[] | null;
};

export interface SetupStatus {
  /** True while the deployment has no organisation and nobody can sign in. */
  required: boolean;
  /** True when the wizard still holds the token minted at boot. */
  armed: boolean;
  /** True when GEREGE_CORE_TOKEN is set, so the register can be searched. */
  core: boolean;
}

export interface SetupOrganisation {
  core_id: number;
  name: string;
  legal_name: string;
  registration_number: string;
  suggested_slug: string;
  email: string;
  phone: string;
  address: string;
}

export interface SetupPerson {
  core_id: number;
  name: string;
  email: string;
  phone: string;
  registration_number: string;
}

/**
 * The first-run wizard.
 *
 * Every call but the status carries the setup token from the address bar, which
 * the platform wrote to its log at boot. The browser never stores it: a token
 * in localStorage would outlive the one act it authorises.
 */
const setupHeaders = (token: string) => ({ "X-Setup-Token": token });

export const coreApi = {
  setupStatus: () => request<SetupStatus>("/setup/status"),

  setupFindOrganisation: (token: string, registrationNumber: string) =>
    request<SetupOrganisation>("/setup/organisation", {
      method: "POST",
      headers: setupHeaders(token),
      body: JSON.stringify({ registration_number: registrationNumber }),
    }),

  setupFindPerson: (token: string, registrationNumber: string) =>
    request<SetupPerson>("/setup/person", {
      method: "POST",
      headers: setupHeaders(token),
      body: JSON.stringify({ registration_number: registrationNumber }),
    }),

  setupComplete: (
    token: string,
    organisation: { name: string; slug: string; legal_name: string; registration_number: string },
    admin: { email: string; name: string },
    password: string,
  ) =>
    request<{ tenant_id: string; slug: string }>("/setup/complete", {
      method: "POST",
      headers: setupHeaders(token),
      body: JSON.stringify({ organisation, admin, password }),
    }),

  login: (email: string, password: string) =>
    request<{ expires_at: string; user: any }>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),

  loginWithEID: (code?: string, redirectURI?: string, regNumber?: string, otpCode?: string, authMethod?: string) =>
    request<{ expires_at: string; user: any; identity: any }>("/auth/eid/login", {
      method: "POST",
      body: JSON.stringify({ code, redirect_uri: redirectURI, reg_number: regNumber, otp_code: otpCode, auth_method: authMethod }),
    }),

  startEID: (callbackUrl = "") => request<{session_id:string;device_link_url?:string;verification_code:string;expires_at:string}>("/auth/eid/start",{method:"POST",body:JSON.stringify({callbackUrl})}),
  startEIDByNationalID: (nationalId:string,callbackUrl = "") => request<{session_id:string;device_link_url?:string;verification_code:string;expires_at:string}>("/auth/eid/start-id",{method:"POST",body:JSON.stringify({national_id:nationalId,callbackUrl})}),
  // The poll is a long poll the API holds open for up to 25s, so the caller
  // passes a signal to drop it the moment the citizen cancels or leaves.
  pollEID: (sessionId:string,signal?:AbortSignal) => request<{state:string;expires_at?:string;identity?:any}>("/auth/eid/poll",{method:"POST",body:JSON.stringify({session_id:sessionId}),signal}),

  loginWithDAN: (danToken?: string, regNumber?: string, otpCode?: string) =>
    request<{ expires_at: string; user: any; dan_profile: any }>("/auth/dan/login", {
      method: "POST",
      body: JSON.stringify({ dan_token: danToken, reg_number: regNumber, otp_code: otpCode }),
    }),

  // end_session_url is set only on a deployment that signs people in through
  // an SSO provider. The session here is already gone by the time it is
  // returned; what is left is to send the browser there so the provider ends
  // its own, and returns the person to this deployment afterwards.
  logout: () => request<{ status: string; end_session_url?: string }>("/auth/logout", { method: "POST" }),

  // The caller's own record. There is no id parameter: the session decides
  // whose profile is read, so this cannot be pointed at somebody else.
  profile: () =>
    request<{
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
    request<{
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
    request<{
      provider: string; email: string; name: string; consented: boolean;
      claims: Record<string, unknown>; eid_claims: string[];
    }>(`/auth/bind/session?b=${encodeURIComponent(binding)}`),

  bindingConsent: (binding: string) =>
    request<{ consented: boolean }>("/auth/bind/consent", {
      method: "POST", body: JSON.stringify({ binding }),
    }),

  bindingEIDStart: (binding: string, nationalId?: string) =>
    request<{session_id:string;device_link_url?:string;verification_code:string;expires_at?:string}>(
      "/auth/bind/eid/start",
      { method: "POST", body: JSON.stringify({ binding, national_id: nationalId }) },
    ),

  bindingEIDPoll: (binding: string, sessionId: string, signal?: AbortSignal) =>
    request<{state:string;expires_at?:string;bound?:boolean}>("/auth/bind/eid/poll", {
      method: "POST", body: JSON.stringify({ binding, session_id: sessionId }), signal,
    }),

  // Who is asking, for the sign-in screen to name while nobody is signed in.
  // Resolved by the server rather than read out of the URL: a name taken from a
  // query parameter is a name anybody can write, which is how a convincing
  // "sign in to <your bank>" screen gets built.
  oauthClientInfo: (clientId: string) =>
    request<{ client_id: string; client_name: string; logo_uri?: string }>(
      `/oauth2/client-info?client_id=${encodeURIComponent(clientId)}`,
    ),

  // How this deployment signs people in. `enabled` false is the ordinary case:
  // the login screen shows its own eID and password forms. `enabled` true means
  // identity belongs to `provider_name`, and the screen's job is to hand the
  // browser to `start_url` rather than to ask for anything.
  ssoConfig: () =>
    request<{
      enabled: boolean;
      provider_name?: string;
      start_url?: string;
      local_login: boolean;
      // Reported available only when it can actually be used: a deployment that
      // federates has closed its own sign-in paths, and Google is one of them.
      google: { enabled: boolean; start_url?: string };
      // "private" means this deployment provisions nobody: somebody who has
      // never been invited cannot get in however they authenticate, so the
      // screen says so instead of letting them find out by failing.
      access_mode?: "public" | "private";
    }>("/auth/sso/config"),

  // permissions carries the effective grant of every role the member holds; it
  // is empty for administrators, who bypass the check.
  getMe: () => request<{ id: string; tenant_id: string; tenant_name: string; workspace_kind?: string; name: string; email: string; is_admin: boolean; permissions?: string[]; impersonated?: boolean; notices?: Array<{ kind: string; title: string; body: string }> }>("/auth/me"),

  // The organisations the signed-in person may act for. A membership in one is
  // the common case, so callers should expect a list of one rather than treat
  // it as an error.
  // What this person asked other organisations for. Empty on a workspace
  // nothing has published into, which is every organisation and a home whose
  // requests have not started yet — see backend/db/migrations/00086.
  getMyItems: () =>
    request<{
      items: Array<{
        id: string; source_app: string; source_ref: string; provider: string;
        code: string; status: string; answer: string;
        opened_at: string; updated_at: string;
      }>;
    }>("/me/items"),
  getTenants: () =>
    request<{
      current: string;
      tenants: Array<{ id: string; name: string; slug: string; kind?: string }>;
      // Which of them this session is reading across. Always contains current.
      active: string[];
    }>("/auth/tenants"),

  // Read across several organisations at once, the way Odoo's allowed companies
  // work. New rows still go to the one being acted in — that is decided by the
  // row-level policies, not here.
  setActiveTenants: (tenantIds: string[]) =>
    request<{ active: string[] }>("/auth/tenants/active", {
      method: "POST",
      body: JSON.stringify({ tenant_ids: tenantIds }),
    }),

  // Moves the session to another of them. The server rotates the token and
  // re-sets the cookie, so everything fetched before this call belongs to the
  // tenant just left — the caller reloads rather than patching state.
  switchTenant: (tenantId: string) =>
    request<{ tenant_id: string; switched: boolean; expires_at?: string }>("/auth/switch-tenant", {
      method: "POST",
      body: JSON.stringify({ tenant_id: tenantId }),
    }),

  getMenus: () => request<Array<{ id: string; app_id?: string; app_name?: string; parent_id?: string; label: string; path?: string; icon: string; order: number }>>("/menus"),

  // Odoo-style tenant access control
  getAccessOverview: () => request<{
    roles: Array<{ id:string; code:string; name:string; description:string; active:boolean; system:boolean; permissions:string[] }>;
    permissions: Array<{ code:string; name:string; description:string; app:string }>;
    members: Array<{ membership_id:string; user_id:string; name:string; email:string; is_admin:boolean; roles:string[] }>;
  }>("/admin/access/overview"),
  createRole: (data:{code:string;name:string;description:string}) => request<{id:string}>("/admin/access/roles",{method:"POST",body:JSON.stringify(data)}),
  updateRole: (id:string,data:{name:string;description:string;active:boolean}) => request(`/admin/access/roles/${id}`,{method:"PUT",body:JSON.stringify(data)}),
  deleteRole: (id:string) => request<void>(`/admin/access/roles/${id}`,{method:"DELETE"}),
  setRolePermissions: (id:string,permissions:string[]) => request(`/admin/access/roles/${id}/permissions`,{method:"PUT",body:JSON.stringify({permissions})}),
  setMembershipRoles: (id:string,roles:string[]) => request(`/admin/access/memberships/${id}/roles`,{method:"PUT",body:JSON.stringify({roles})}),
  getDevices: () => request<{devices:Array<{id:string;name:string;platform:string;form_factor:string;site:string;status:string;app_version:string;os_version:string;last_seen_at?:string;enrolled_at:string}>}>("/admin/devices"),
  createDeviceEnrollmentCode: () => request<{code:string;expires_at:string}>("/admin/devices/enrollment-codes",{method:"POST"}),
  setStaffPin: (membershipId:string,pin:string) => request<{status:string}>("/admin/devices/staff-pin",{method:"PUT",body:JSON.stringify({membership_id:membershipId,pin})}),
  setDeviceStatus: (id:string,status:"ACTIVE"|"DISABLED"|"RETIRED") => request<{status:string}>("/admin/devices/status",{method:"PUT",body:JSON.stringify({id,status})}),
  registerPushToken: (provider:"APNS"|"FCM",token:string,appId:string) => request<void>("/push-tokens",{method:"POST",body:JSON.stringify({provider,token,app_id:appId})}),
  getOrganisation: () =>
    request<{
      tenant_id: string; slug: string; name: string; legal_name: string;
      registration_number: string; tax_number: string; country_code: string;
      province: string; district: string; khoroo: string; address_line: string;
      postal_code: string; phone: string; email: string; website: string;
      logo_url: string; timezone: string; locale: string; currency: string;
      // The organisation this one is a subsidiary of. A branch or an office is
      // a department; this is another legal entity, and so another tenant.
      parent_tenant_id?: string; parent_name?: string;
    }>("/tenant/profile"),

  // Partial by design: a form that sends the fields it changed must not blank
  // the ones it did not mention.
  updateOrganisation: (patch: Record<string, string>) =>
    request("/tenant/profile", { method: "PUT", body: JSON.stringify(patch) }),

  /**
   * Refresh the organisation's legal identity from the Gerege Core register.
   *
   * Returns the profile as it stands afterwards, so the screen redraws from
   * the server's answer rather than from what it hoped the register said.
   */
  syncOrganisationFromCore: () =>
    request("/tenant/profile/sync-core", { method: "POST" }),

  getPreferences: () =>
    request<{
      name: string; email: string; phone: string; locale: string; timezone: string;
      organisation_locale: string; organisation_timezone: string;
    }>("/profile/preferences"),

  updatePreferences: (patch: { name?: string; phone?: string; locale?: string; timezone?: string }) =>
    request("/profile/preferences", { method: "PUT", body: JSON.stringify(patch) }),

  // Contacts App
  getEmailVerifyOverview: (limit = 25) =>
    request<EmailVerifyOverview>(`/admin/email-verification/overview?limit=${limit}`),

  // Ask the service for a link. App modules call the Go service directly; this
  // is for the product's own screens.
  sendEmailVerification: (data: { email: string; redirect_url?: string; purpose?: string }) =>
    request<EmailVerification>("/verify/send", { method: "POST", body: JSON.stringify(data) }),

  // SSO clients — the OAuth2 / OIDC applications registered against this
  // platform's own provider.
  //
  // client_secret comes back only from create and rotate-secret; every other
  // read omits it, because the server keeps a digest and cannot reproduce it.
  getConsentPrompt: (query: string) => request<ConsentPrompt>(`/oauth2/consent?${query}`),
  decideConsent: (query: string, approved: boolean) => {
    const form = new URLSearchParams(query);
    form.set("approved", String(approved));
    return request<{ redirect_to: string }>("/oauth2/consent", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: form.toString(),
    });
  },
};
