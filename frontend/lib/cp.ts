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
  /** When the limits were last written; the organisation's creation if never. */
  updated_at: string;
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
  /** True when the administrator was chosen rather than invited. */
  admin_existed?: boolean;
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

  // Across every organisation at once: which limits are set where, and which
  // app is installed where.
  quotas: () => request<{ quotas: QuotaLine[] }>("/tenant-quotas"),
  installations: () => request<{ installations: Installation[] }>("/app-installations"),

  // The assistant, as it stands for every organisation: the prompts it carries
  // into a conversation and the corpus it answers from.
  prompts: () => request<{ prompts: Prompt[] }>("/assistant/prompts"),
  savePrompt: (key: string, content: string, active: boolean, reason: string) =>
    request<{ status: string }>(`/assistant/prompts/${encodeURIComponent(key)}`, {
      method: "PUT",
      body: JSON.stringify({ content, active, reason }),
    }),
  knowledge: () => request<{ knowledge: Knowledge[] }>("/assistant/knowledge"),
  addKnowledge: (entry: { title: string; content: string; source_url: string }, reason: string) =>
    request<{ status: string }>("/assistant/knowledge", {
      method: "POST",
      body: JSON.stringify({ ...entry, reason }),
    }),
  removeKnowledge: (id: string, reason: string) =>
    request<{ status: string }>(`/assistant/knowledge/${encodeURIComponent(id)}`, {
      method: "DELETE",
      body: JSON.stringify({ reason }),
    }),

  // Who the platform has been asked to write to, across every organisation.
  verifications: (limit = 25) => request<VerificationLedger>(`/email-verifications?limit=${limit}`),

  audit: (params: { action?: string; target_type?: string; target_id?: string } = {}) => {
    const query = new URLSearchParams(
      Object.entries(params).filter(([, value]) => value) as [string, string][],
    );
    return request<{ entries: AuditEntry[] }>(`/audit?${query.toString()}`);
  },

  operators: () => request<{ operators: OperatorSummary[] }>("/operators"),

  // Everybody with an account on this deployment, and one of them in full.
  roster: (search = "", filter = "", offset = 0) =>
    request<Roster>(`/people/roster?q=${encodeURIComponent(search)}&filter=${encodeURIComponent(filter)}&offset=${offset}`),
  person: (id: string) => request<PersonDetail>(`/people/${encodeURIComponent(id)}`),

  // Adding an operator. The answer carries the password and the enrolment once
  // and is never repeatable: nothing on the server can show them again.
  addOperator: (body: { email: string; name: string; role: string; reason: string }) =>
    request<CreatedOperator>("/operators", { method: "POST", body: JSON.stringify(body) }),
  confirmEnrolment: (id: string, code: string, reason: string) =>
    request<{ status: string }>(`/operators/${encodeURIComponent(id)}/enrolment`, {
      method: "POST",
      body: JSON.stringify({ code, reason }),
    }),
  setOperatorEnabled: (id: string, enabled: boolean, reason: string) =>
    request<{ status: string }>(`/operators/${encodeURIComponent(id)}/${enabled ? "enable" : "disable"}`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    }),
  setOperatorRole: (id: string, role: string, reason: string) =>
    request<{ status: string }>(`/operators/${encodeURIComponent(id)}/role`, {
      method: "POST",
      body: JSON.stringify({ role, reason }),
    }),
  changePassword: (current: string, next: string) =>
    request<{ status: string }>("/me/password", {
      method: "POST",
      body: JSON.stringify({ current, next }),
    }),

  createTenant: (body: {
    name: string; slug: string; legal_name?: string; registration_number?: string;
    apps?: string[]; admin_user_id?: string; admin_email?: string; admin_name?: string; reason: string;
  }) => request<CreatedTenant>("/tenants", { method: "POST", body: JSON.stringify(body) }),

  // What the register says about a registration number, and who on this
  // deployment has proved who they are with eID.
  findOrganisation: (regNo: string) =>
    request<DirectoryOrganisation>(`/directory/organisation?reg_no=${encodeURIComponent(regNo)}`),
  addMember: (tenantID: string, userID: string, reason: string) =>
    request<{ status: string }>(`/tenants/${encodeURIComponent(tenantID)}/people`, {
      method: "POST",
      body: JSON.stringify({ user_id: userID, reason }),
    }),
  findPerson: (regNo: string) =>
    request<DirectoryPerson>(`/directory/person?reg_no=${encodeURIComponent(regNo)}`),
  verifiedPeople: (search = "") =>
    request<{ people: VerifiedPerson[]; directory: boolean }>(`/directory/people?q=${encodeURIComponent(search)}`),

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

  // System Operations: the three reads its screens stand on. Each one is a
  // list the front page only counts.
  platformUsage: () => request<PlatformUsage>("/usage"),
  reportSchedules: () => request<{ schedules: ReportSchedule[] }>("/report-schedules"),
  backups: (limit = 50) => request<{ backups: BackupEntry[]; status: Overview["backups"] }>(`/backups?limit=${limit}`),
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

/** One instruction the assistant carries into every conversation. */
export interface Prompt {
  key: string;
  content: string;
  active: boolean;
  /** Null for a key the deployment has never written; the screen still offers it. */
  updated_at: string | null;
}

/** One entry in the corpus every organisation's assistant answers from. */
export interface Knowledge {
  id: string;
  title: string;
  content: string;
  source_url: string;
  updated_at: string;
}

export interface Verification {
  id: string;
  tenant_id: string;
  /** Empty when the organisation has since been deleted; the row stays. */
  tenant_name: string;
  source: string;
  purpose: string;
  email: string;
  status: "PENDING" | "VERIFIED" | "EXPIRED";
  created_at: string;
  verified_at: string | null;
}

export interface VerificationLedger {
  stats: {
    total: number;
    verified: number;
    pending: number;
    expired: number;
    last_24h: number;
    verified_pct: number;
    tenants: number;
  };
  recent: Verification[];
  service: {
    /** Whether a key is present at all. The key itself never comes back. */
    configured: boolean;
    reachable: boolean;
    /** What the provider said when it was not reachable. */
    health?: string;
    provider_url: string;
    admin_url: string;
  };
}

/** One organisation's line in the platform usage report. */
export interface TenantUsageLine {
  tenant_id: string;
  tenant_name: string;
  slug: string;
  suspended: boolean;
  /** Only the metrics counted for this organisation this month. */
  metrics: Record<string, number>;
  /** Null is "never counted", which reads differently from a row of zeroes. */
  collected: string | null;
}

export interface PlatformUsage {
  month: string;
  metrics: string[];
  tenants: TenantUsageLine[];
  totals: Record<string, number>;
}

export interface ReportSchedule {
  id: string;
  tenant_id: string;
  tenant_name: string;
  name: string;
  report_key: string;
  cron: string;
  format: string;
  recipients: string[];
  active: boolean;
  last_run_at: string | null;
  last_status: string;
}

export interface BackupEntry {
  id: string;
  kind: "backup" | "restore_test";
  started_at: string;
  finished_at: string | null;
  size_mb: number;
  ok: boolean;
  detail: string;
  /** Empty for the script's own rows; an operator id for a hand-written one. */
  recorded_by: string;
}

export interface OperatorSummary extends Operator {
  disabled_at: string | null;
  last_login_at: string | null;
  created_at: string;
  /** False until somebody proves the authenticator works; such an account cannot sign in. */
  enrolled: boolean;
}

/** Shown once, when an operator is added, and never again. */
export interface CreatedOperator {
  id: string;
  email: string;
  name: string;
  role: string;
  /** The authenticator's secret, and the URI a QR code is drawn from. */
  secret: string;
  uri: string;
  /** Generated here rather than chosen: the first thing it should be used for is changing it. */
  password: string;
}

/** One organisation's limits, with the organisation named. */
export interface QuotaLine extends Quota {
  tenant_name: string;
  slug: string;
  suspended: boolean;
}

/** One app in one organisation. */
export interface Installation {
  tenant_id: string;
  tenant_name: string;
  slug: string;
  app_id: string;
  app_name: string;
  installed_version: string;
  status: string;
  enabled: boolean;
  installed_at: string;
  updated_at: string;
}

/** What the Gerege Core register says about a registration number. */
export interface DirectoryOrganisation {
  core_id: number;
  name: string;
  legal_name: string;
  registration_number: string;
  suggested_slug: string;
  email: string;
  phone: string;
  address: string;
}

/** Somebody who has signed in with eID on this deployment. */
export interface VerifiedPerson {
  user_id: string;
  name: string;
  email: string;
  reg_number: string;
  linked_at: string;
  last_seen_at: string;
  /** How many organisations they already belong to. */
  organisations: number;
}

/** One row of the people roster. */
export interface RosterPerson {
  id: string;
  email: string;
  name: string;
  /** Whether eID has ever vouched for this account. */
  verified: boolean;
  /** How many federated providers it is linked to. */
  providers: number;
  organisations: number;
  sessions: number;
  /** The newest session, which is as close to "last here" as the schema holds. */
  last_seen_at: string | null;
  locked_until: string | null;
  active: boolean;
  created_at: string;
}

export interface Roster {
  people: RosterPerson[];
  total: number;
  counts: { verified: number; locked: number; homeless: number; signed_in: number };
}

/** One way into an account: eID, or a federated provider by its issuer. */
export interface PersonIdentity {
  kind: string;
  subject: string;
  detail: string;
  linked_at: string;
  last_seen_at: string | null;
}

export interface PersonMembership {
  tenant_id: string;
  tenant_name: string;
  slug: string;
  roles: string[];
  joined_at: string;
}

export interface PersonSession {
  id: string;
  tenant_id: string | null;
  created_at: string;
  last_seen_at: string | null;
  expires_at: string;
}

export interface PersonDetail extends RosterPerson {
  identities: PersonIdentity[];
  memberships: PersonMembership[];
  open_sessions: PersonSession[];
  impersonations: Array<{ operator_email: string; reason: string; created_at: string }>;
}

/** What the register says about a person's registration number. */
export interface DirectoryPerson {
  core_id: number;
  name: string;
  email: string;
  phone: string;
  registration_number: string;
}
