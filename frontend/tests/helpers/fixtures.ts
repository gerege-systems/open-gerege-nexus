/**
 * Answers shaped like the API's, with only the field under test spelled out.
 *
 * Each factory fills in a whole, valid record and takes an override. A test
 * that writes `anOperator({ enrolled: false })` says what it is about in one
 * line, and — the reason these exist rather than object literals — a new
 * required field is added in one place instead of thirty.
 */

import type {
  BackupEntry,
  Knowledge,
  OperatorSummary,
  Overview,
  PlatformUsage,
  Prompt,
  ReportSchedule,
} from "@/lib/cp";

export function anOperator(overrides: Partial<OperatorSummary> = {}): OperatorSummary {
  return {
    id: "op-1",
    email: "bat@example.test",
    name: "Бат Дорж",
    role: "operator",
    disabled_at: null,
    last_login_at: "2026-08-28T09:15:00+08:00",
    created_at: "2026-08-01T09:00:00+08:00",
    enrolled: true,
    ...overrides,
  };
}

export function anOverview(overrides: Partial<Overview> = {}): Overview {
  return {
    monitoring: true,
    grafana_url: "",
    api: { requests_per_second: 12.34, error_rate: 0.004, p95_seconds: 0.128, read: true },
    external: [],
    infra: [],
    alerts: [],
    background: [],
    tenant_trouble: [],
    backups: {
      configured: true,
      last_backup_at: "2026-08-29T02:00:00+08:00",
      last_size_mb: 512,
      last_ok: true,
      last_detail: "",
      last_restore_test_at: null,
    },
    catalog: { last_sync_at: null, ok: true, detail: "", apps: [] },
    version: { platform: "1.11.0", release: "2026.08", migration: 97, migration_applied_at: null },
    warnings: [],
    ...overrides,
  };
}

export function aPlatformUsage(overrides: Partial<PlatformUsage> = {}): PlatformUsage {
  return {
    month: "2026-08",
    metrics: ["storage_mb"],
    tenants: [],
    totals: { storage_mb: 0 },
    ...overrides,
  };
}

export function aSchedule(overrides: Partial<ReportSchedule> = {}): ReportSchedule {
  return {
    id: "sch-1",
    tenant_id: "ten-1",
    tenant_name: "Гэрэгэ ХХК",
    name: "Сарын тайлан",
    report_key: "monthly",
    cron: "0 6 1 * *",
    format: "pdf",
    recipients: ["nyagtlan@example.test"],
    active: true,
    last_run_at: "2026-08-01T06:00:00+08:00",
    last_status: "ok",
    ...overrides,
  };
}

export function aBackup(overrides: Partial<BackupEntry> = {}): BackupEntry {
  return {
    id: "bak-1",
    kind: "backup",
    started_at: "2026-08-29T02:00:00+08:00",
    finished_at: "2026-08-29T02:04:00+08:00",
    size_mb: 512,
    ok: true,
    detail: "",
    recorded_by: "",
    ...overrides,
  };
}

export function aPrompt(overrides: Partial<Prompt> = {}): Prompt {
  return {
    key: "assistant.system",
    content: "Чи Гэрэгэ Нексүсийн туслах.",
    active: true,
    updated_at: "2026-08-20T10:00:00+08:00",
    ...overrides,
  };
}

export function aKnowledge(overrides: Partial<Knowledge> = {}): Knowledge {
  return {
    id: "kn-1",
    title: "Дүрэм",
    content: "Байгууллагын дотоод журам.",
    source_url: "",
    updated_at: "2026-08-20T10:00:00+08:00",
    ...overrides,
  };
}
