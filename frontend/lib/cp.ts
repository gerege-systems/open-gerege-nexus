/**
 * The operator console's client.
 *
 * Deliberately not part of lib/api.ts. That module speaks to the platform on
 * behalf of a tenant user: it knows about the session cookie, the tenant
 * header, the re-login dance the shell performs on a 401. None of that applies
 * here, and sharing the module would mean every screen in the product carrying
 * the console's calls in its bundle.
 *
 * Addresses are relative. The console is served on its own hostname and its
 * API is /api/platform/v1 on that same hostname, so a relative path is always
 * right and an absolute one — the pattern lib/apiBase.ts has to unpick for the
 * device lines — would be a way to get it wrong.
 */

// Production stays same-origin. Development deliberately uses two hostnames
// and two ports, so the optional value lets admin.localhost:3000 call the API at
// admin.localhost:8080 without weakening either host gate.
const BASE = process.env.NEXT_PUBLIC_CONTROL_PLANE_API_URL || "/api/platform/v1";

export type OperatorRole = "superadmin" | "operator" | "support" | "auditor";

export interface Operator {
  id: string;
  email: string;
  name: string;
  role: OperatorRole;
}

export interface Me {
  operator: Operator;
  expires_at: string;
  stepped_up: boolean;
}

export interface TenantSummary {
  id: string;
  slug: string;
  name: string;
  registration_number: string;
  created_at: string;
  user_count: number;
  app_count: number;
  last_activity_at: string | null;
  suspended_at: string | null;
  suspension_reason: string;
  deletion_scheduled_at: string | null;
  maintenance_at?: string | null;
}

export interface Quota {
  tenant_id: string;
  max_users: number | null;
  max_storage_mb: number | null;
  max_ai_calls_monthly: number | null;
  enforcement: "soft" | "hard";
  users: number;
  /** Which limits this build actually applies; the rest are recorded only. */
  enforced: string[];
}

export interface Impersonation {
  id: string;
  operator_email: string;
  user_email: string;
  reason: string;
  redeemed_at: string | null;
  ends_at: string;
  created_at: string;
}

export interface Approval {
  id: string;
  action: string;
  target_type: string;
  target_id: string;
  target_name: string;
  requested_by: string;
  requested_by_name: string;
  requested_reason: string;
  requested_at: string;
  expires_at: string;
}

export interface PersonMembership {
  tenant_id: string;
  tenant_name: string;
  tenant_slug: string;
  roles: string[];
  suspended: boolean;
}

export interface Person {
  id: string;
  email: string;
  name: string;
  locked_until: string | null;
  failed_logins: number;
  sessions: number;
  memberships: PersonMembership[];
}

export interface CreatedTenant {
  id: string;
  slug: string;
  name: string;
  installed: string[];
  failed: string[];
  invited: boolean;
  invite_error?: string;
}

export interface TenantApp {
  id: string;
  name: string;
  version: string;
  status: string;
  enabled: boolean;
  installed_at: string;
}

export interface TenantMember {
  user_id: string;
  email: string;
  name: string;
  roles: string[];
}

export interface TenantActivity {
  action: string;
  resource: string;
  user_id: string;
  created_at: string;
}

export interface AuditEntry {
  id: string;
  operator_id: string;
  operator_email: string;
  action: string;
  target_type: string;
  target_id: string;
  reason: string;
  before: unknown;
  after: unknown;
  ip: string;
  created_at: string;
}

export interface TenantDetail extends TenantSummary {
  legal_name: string;
  tax_number: string;
  apps: TenantApp[];
  members: TenantMember[];
  activity: TenantActivity[];
  operator_actions: AuditEntry[];
  quota: Quota;
  impersonations: Impersonation[];
}

/**
 * Unauthorized is what every screen checks for to decide whether to show the
 * sign-in form. A distinct class rather than a status code compared at each
 * call site, so that a screen cannot forget which number meant what.
 */
export class Unauthorized extends Error {
  constructor() {
    super("unauthorized");
    this.name = "Unauthorized";
  }
}

/** StepUpRequired mirrors the API's `step_up_required` code. */
export class StepUpRequired extends Error {
  constructor() {
    super("step up required");
    this.name = "StepUpRequired";
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(BASE + path, {
    ...init,
    // The session is a cookie, and fetch does not send cookies unless told to
    // even on same-origin requests with a custom method.
    credentials: "include",
    headers: { "Content-Type": "application/json", ...(init.headers || {}) },
  });

  if (response.status === 401) throw new Unauthorized();

  let body: { error?: string; code?: string } | null = null;
  const text = await response.text();
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      // A response that is not JSON is a proxy's error page, not the API's.
      // The status is the only thing worth reporting from it.
    }
  }

  if (!response.ok) {
    if (body?.code === "step_up_required") throw new StepUpRequired();
    throw new Error(body?.error || `request failed with ${response.status}`);
  }
  return (body ?? {}) as T;
}

export const cp = {
  me: () => request<Me>("/me"),

  signIn: (email: string, password: string, code: string) =>
    request<{ operator: Operator; expires_at: string }>("/session", {
      method: "POST",
      body: JSON.stringify({ email, password, code }),
    }),

  signOut: () => request<{ status: string }>("/session", { method: "DELETE" }),

  stepUp: (code: string) =>
    request<{ stepped_up_until: string }>("/step-up", {
      method: "POST",
      body: JSON.stringify({ code }),
    }),

  tenants: (search: string) =>
    request<{ tenants: TenantSummary[] }>(`/tenants?q=${encodeURIComponent(search)}`),

  tenant: (id: string) => request<TenantDetail>(`/tenants/${encodeURIComponent(id)}`),

  audit: (params: { action?: string; target_type?: string; target_id?: string } = {}) => {
    const query = new URLSearchParams(
      Object.entries(params).filter(([, value]) => value) as [string, string][],
    );
    return request<{ entries: AuditEntry[] }>(`/audit?${query.toString()}`);
  },

  operators: () => request<{ operators: (Operator & { disabled_at: string | null; last_login_at: string | null; created_at: string })[] }>("/operators"),

  createTenant: (body: {
    name: string; slug: string; legal_name?: string; registration_number?: string;
    apps?: string[]; admin_email: string; admin_name?: string; reason: string;
  }) => request<CreatedTenant>("/tenants", { method: "POST", body: JSON.stringify(body) }),

  suspend: (id: string, reason: string) =>
    request<{ status: string }>(`/tenants/${id}/suspend`, { method: "POST", body: JSON.stringify({ reason }) }),
  resume: (id: string, reason: string) =>
    request<{ status: string }>(`/tenants/${id}/resume`, { method: "POST", body: JSON.stringify({ reason }) }),
  requestDeletion: (id: string, reason: string) =>
    request<{ status: string; approval_id: string; grace_days: number }>(
      `/tenants/${id}/deletion`, { method: "POST", body: JSON.stringify({ reason }) }),
  cancelDeletion: (id: string, reason: string) =>
    request<{ status: string }>(`/tenants/${id}/deletion`, { method: "DELETE", body: JSON.stringify({ reason }) }),
  deletions: () => request<{ tenants: TenantSummary[] }>("/deletions"),

  setQuota: (id: string, body: Partial<Quota> & { reason: string }) =>
    request<{ status: string }>(`/tenants/${id}/quota`, { method: "PUT", body: JSON.stringify(body) }),

  approvals: () => request<{ approvals: Approval[] }>("/approvals"),
  approve: (id: string, reason: string) =>
    request<{ status: string }>(`/approvals/${id}/approve`, { method: "POST", body: JSON.stringify({ reason }) }),
  reject: (id: string, reason: string) =>
    request<{ status: string }>(`/approvals/${id}/reject`, { method: "POST", body: JSON.stringify({ reason }) }),

  people: (query: string) => request<{ people: Person[] }>(`/people?q=${encodeURIComponent(query)}`),
  unlock: (id: string, reason: string) =>
    request<{ status: string }>(`/people/${id}/unlock`, { method: "POST", body: JSON.stringify({ reason }) }),
  revokeSessions: (id: string, reason: string) =>
    request<{ status: string; sessions: number }>(`/people/${id}/sessions/revoke`,
      { method: "POST", body: JSON.stringify({ reason }) }),
  credentialLink: (id: string, body: { tenant_id: string; purpose: "invite" | "reset"; reason: string }) =>
    request<{ status: string }>(`/people/${id}/credential-link`, { method: "POST", body: JSON.stringify(body) }),

  impersonate: (tenantID: string, userID: string, reason: string) =>
    request<{ url: string; minutes: number }>(`/tenants/${tenantID}/impersonate`,
      { method: "POST", body: JSON.stringify({ user_id: userID, reason }) }),

  /** The export is a download rather than a fetch: it is a file. */
  exportURL: (id: string) => `${BASE}/tenants/${id}/export`,

  usage: (tenantID: string) => request<Usage>(`/tenants/${tenantID}/usage`),
  usageCSVURL: (tenantID: string) => `${BASE}/tenants/${tenantID}/usage.csv`,

  health: () => request<Overview>("/health"),
  catalogStatus: () => request<Overview["catalog"]>("/catalog/status"),
  catalogOverview: () => request<{ catalog: Overview["catalog"]; platform: Overview["version"] }>("/catalog/overview"),
  syncCatalog: (reason: string) =>
    request<{ status: string; changed: boolean }>("/catalog/sync",
      { method: "POST", body: JSON.stringify({ reason }) }),
  deploy: (ref: string, reason: string) =>
    request<{ status: string; url: string }>("/deploy",
      { method: "POST", body: JSON.stringify({ ref, reason }) }),
  recordRestoreTest: (detail: string, reason: string) =>
    request<{ status: string }>("/backups/restore-test",
      { method: "POST", body: JSON.stringify({ detail, reason }) }),

  settings: () => request<{ settings: Setting[]; warnings: string[] }>("/settings"),
  settingHistory: (key: string) =>
    request<{ changes: SettingChange[] }>(`/settings/history?key=${encodeURIComponent(key)}`),
  setSetting: (key: string, value: string, reason: string) =>
    request<{ status: string }>(`/settings/${encodeURIComponent(key)}`,
      { method: "PUT", body: JSON.stringify({ value, reason }) }),
  rollbackSetting: (changeID: string, reason: string) =>
    request<{ status: string }>(`/settings/rollback/${changeID}`,
      { method: "POST", body: JSON.stringify({ reason }) }),

  credentials: () =>
    request<{ credentials: Credential[]; sealing_configured: boolean }>("/credentials"),
  setCredential: (name: string, value: string, reason: string) =>
    request<{ status: string }>(`/credentials/${encodeURIComponent(name)}`,
      { method: "PUT", body: JSON.stringify({ value, reason }) }),
  clearCredential: (name: string, reason: string) =>
    request<{ status: string }>(`/credentials/${encodeURIComponent(name)}`,
      { method: "DELETE", body: JSON.stringify({ reason }) }),

  flags: () => request<{ flags: Flag[] }>("/flags"),
  saveFlag: (flag: Partial<Flag> & { key: string; reason: string }) =>
    request<{ status: string }>("/flags", { method: "POST", body: JSON.stringify(flag) }),
  deleteFlag: (key: string, reason: string) =>
    request<{ status: string }>(`/flags/${encodeURIComponent(key)}`,
      { method: "DELETE", body: JSON.stringify({ reason }) }),
  flagOverride: (key: string, tenantID: string, enabled: boolean | null, reason: string) =>
    request<{ status: string }>(`/flags/${encodeURIComponent(key)}/override`,
      { method: "PUT", body: JSON.stringify({ tenant_id: tenantID, enabled, reason }) }),

  maintenance: (tenantID: string, on: boolean, message: string, reason: string) =>
    request<{ status: string }>(`/tenants/${tenantID}/maintenance`,
      { method: "POST", body: JSON.stringify({ on, message, reason }) }),

  announcements: () => request<{ announcements: Announcement[] }>("/announcements"),
  announce: (body: Partial<Announcement> & { title: string; reason: string }) =>
    request<{ status: string }>("/announcements", { method: "POST", body: JSON.stringify(body) }),
  withdraw: (id: string, reason: string) =>
    request<{ status: string }>(`/announcements/${id}`,
      { method: "DELETE", body: JSON.stringify({ reason }) }),
};

export interface Setting {
  key: string;
  kind: "bool" | "int" | "duration" | "string" | "enum";
  default: string;
  env?: string;
  options?: string[];
  description: string;
  current: string;
  /** Where the current value came from: the console, the environment, or the code. */
  source: "database" | "environment" | "default";
  updated_at: string | null;
}

/**
 * A credential as the console may see it.
 *
 * There is no field here that holds the value, and that is the point of the
 * type: the platform has no route that returns one. `hint` is the last four
 * characters of a value stored in the database — enough to tell two keys apart
 * and to see that a rotation landed, and not enough to use.
 */
export interface Credential {
  name: string;
  env: string;
  description: string;
  docs?: string;
  source: "database" | "environment" | "unset";
  hint?: string;
  updated_at: string | null;
  updated_by: string | null;
}

export interface SettingChange {
  id: string;
  key: string;
  previous_value: string | null;
  new_value: string;
  reason: string;
  changed_by: string;
  changed_at: string;
}

export interface Flag {
  key: string;
  description: string;
  owner: string;
  kind: "release" | "kill_switch" | "experiment";
  enabled: boolean;
  rollout: number;
  expires_at: string | null;
  updated_at: string;
  overrides?: Record<string, boolean>;
}

export interface Announcement {
  id: string;
  tenant_id: string | null;
  kind: "info" | "warning" | "maintenance";
  title: string;
  body: string;
  starts_at: string;
  ends_at: string | null;
  created_at: string;
}

export interface Overview {
  /** False when this deployment has no Prometheus: the screen says so. */
  monitoring: boolean;
  grafana_url: string;
  api: { requests_per_second: number; error_rate: number; p95_seconds: number; read: boolean };
  external: Array<{ system: string; error_rate: number; p95_seconds: number; state: string }>;
  infra: Array<{ name: string; value: number; unit: string; warning: number; state: string }>;
  alerts: Array<{
    name: string; severity: string; summary: string;
    starts_at: string; runbook: string; silenced: boolean;
  }>;
  background: Array<{ name: string; last_run: string | null; ok: boolean; detail: string; pending: number }>;
  tenant_trouble: Array<{ tenant_id: string; name: string; failures: number; sample: string }>;
  backups: {
    configured: boolean;
    last_backup_at: string | null;
    last_size_mb: number;
    last_ok: boolean;
    last_detail: string;
    last_restore_test_at: string | null;
  };
  catalog: {
    last_sync_at: string | null;
    ok: boolean;
    detail: string;
    apps: Array<{ app_id: string; name: string; versions: Record<string, number>; total: number }>;
  };
  version: { platform: string; release: string; migration: number; migration_applied_at: string | null };
  warnings: string[];
}

export interface UsageSeries {
  metric: string;
  points: Array<{ day: string; value: number }>;
  /** A sum for counted metrics, the latest reading for storage, the peak for people. */
  total: number;
  month_to_date: number;
  limit: number | null;
  /** Whether crossing the limit actually stops anything today. */
  enforced: boolean;
}

export interface Usage {
  tenant_id: string;
  series: UsageSeries[];
  /** Null when nothing has ever been counted, which the screen says. */
  collected: string | null;
}
