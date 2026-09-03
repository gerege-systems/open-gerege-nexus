/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Reports — the unified runner every app's reports appear in. The engine and
// the report definitions live in client-gerege-nexus (modules/reports); these
// are the platform-side calls its screens make.

import { apiBase } from "@/lib/apiBase";
import { request } from "./client";

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

export interface ReportAccessEntry {
  at: string;
  report_key: string;
  by: string;
  details: Record<string, unknown>;
}

export const reportsApi = {
  getReports: () => request<{ groups: ReportGroup[] }>("/reports"),

  getReport: (key: string) => request<ReportMetadata>(`/reports/${encodeURIComponent(key)}`),

  runReport: (key: string, params: Record<string, string>) =>
    request<{ key: string; title: string; result: ReportResult }>(
      `/reports/${encodeURIComponent(key)}/run`,
      { method: "POST", body: JSON.stringify({ params }) },
    ),

  // Not request: the answer is a spreadsheet, not JSON. The blob is handed
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
    request<{ schedules: ReportSchedule[]; delivery_configured: boolean }>("/reports/schedules"),

  createReportSchedule: (input: ReportScheduleInput) =>
    request<{ id: string }>("/reports/schedules", { method: "POST", body: JSON.stringify(input) }),

  updateReportSchedule: (id: string, input: ReportScheduleInput) =>
    request<{ id: string }>(`/reports/schedules/${id}`, { method: "PUT", body: JSON.stringify(input) }),

  deleteReportSchedule: (id: string) =>
    request<void>(`/reports/schedules/${id}`, { method: "DELETE" }),

  // Cross-tenant sharing (§3.5 of the monitoring and reporting proposal).
  getReportGrants: () => request<{ grants: ReportGrant[] }>("/reports/grants"),

  getReportAccessHistory: () =>
    request<{ history: ReportAccessEntry[] }>("/reports/grants/history"),

  requestReportGrant: (input: ReportGrantRequest) =>
    request<{ id: string }>("/reports/grants", { method: "POST", body: JSON.stringify(input) }),

  acceptReportGrant: (id: string) =>
    request<{ id: string }>(`/reports/grants/${id}/accept`, { method: "POST" }),

  revokeReportGrant: (id: string) =>
    request<{ id: string }>(`/reports/grants/${id}/revoke`, { method: "POST" }),

  runConsolidatedReport: (key: string, params: Record<string, string>) =>
    request<{ key: string; title: string; result: ReportResult }>(
      `/reports/${encodeURIComponent(key)}/run-consolidated`,
      { method: "POST", body: JSON.stringify({ params }) },
    ),

};
