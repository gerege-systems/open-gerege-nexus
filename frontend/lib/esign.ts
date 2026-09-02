/**
 * Typed client for the PDF e-signature app (io.gerege.nexus.esign). Every DTO here
 * mirrors the Go structs in backend/internal/apps/esign; nothing on these
 * screens uses `any`.
 *
 * Two signing rails share one document store:
 *
 *   EID — eID Mongolia qualified remote signing. Asynchronous: the citizen's
 *         own device holds the key and approves with PIN2, so the browser
 *         starts a ceremony, shows a verification code and polls.
 *   HSM — Gerege eSign hardware module. Synchronous: prove a certificate,
 *         draw a signature, the service stamps it.
 */

import { apiBase } from "@/lib/apiBase";

export type Provider = "EID" | "HSM";
export type DocumentStatus = "PENDING" | "SIGNED";

/** The vocabulary the signing view polls for. */
export type SessionState = "pending" | "completed" | "failed" | "expired" | "rejected";

export type BatchStatus = "DRAFT" | "RUNNING" | "COMPLETED" | "FAILED" | "CANCELLED";
export type BatchItemStatus = "PENDING" | "RUNNING" | "SIGNED" | "FAILED" | "SKIPPED";

export type LogAction = "CERT_CHECK" | "SIGN" | "SIGN_START" | "BATCH_SIGN" | "DOWNLOAD";
export type LogOutcome = "OK" | "FAILED" | "REJECTED" | "EXPIRED" | "CANCELLED" | "UNVERIFIED";

export interface EsignDocument {
  id: string;
  tenant_id: string;
  title: string;
  file_name: string;
  status: DocumentStatus;
  provider: Provider;
  page_count: number;
  byte_size: number;
  checksum?: string;
  signer_name?: string;
  signer_reg_no?: string;
  signer_phone?: string;
  signer_etsi?: string;
  on_behalf_of_name?: string;
  certificate_level?: string;
  signed_at: string | null;
  created_at: string;
}

export interface SignSession {
  session_id: string;
  document_id?: string;
  provider: Provider;
  state: SessionState;
  failure_reason?: string;
  filename: string;
  document_hash: string;
  verification_code?: string;
  signer_name?: string;
  signer_etsi?: string;
  on_behalf_of_etsi?: string;
  on_behalf_of_name?: string;
  certificate_level?: string;
  created_at: string;
  completed_at?: string;
  expires_at: string;
}

/** An organisation the signer may act for, read live from the registry. */
export interface Representation {
  org_etsi: string;
  org_register: string;
  org_name: string;
  org_name_en?: string;
  role?: string;
  right_type?: string;
  source?: string;
}

export interface SignatureLogEntry {
  id: string;
  document_id?: string;
  document_title?: string;
  session_id?: string;
  provider: Provider;
  action: LogAction;
  outcome: LogOutcome;
  reg_no?: string;
  phone_no?: string;
  first_name?: string;
  last_name?: string;
  detail?: string;
  created_at: string;
}

export interface Batch {
  id: string;
  name: string;
  provider: Provider;
  status: BatchStatus;
  total: number;
  signed: number;
  failed: number;
  created_at: string;
  started_at?: string;
  finished_at?: string;
  items?: BatchItem[];
}

export interface BatchItem {
  id: string;
  document_id: string;
  document_title: string;
  file_name: string;
  position: number;
  status: BatchItemStatus;
  session_id?: string;
  error?: string;
  signed_at?: string;
}

export interface Placement {
  x: number;
  y: number;
  width: number;
  height: number;
  /** 0 places the stamp on the last page, whatever its number. */
  page_number: number;
  text: string;
}

export interface Probe {
  ok: boolean;
  message: string;
  latency_ms: number;
  checked_at: string;
  checked_by?: string;
}

export interface HSMSettings {
  login_url: string;
  sign_url: string;
  enabled: boolean;
  mock_mode: boolean;
  has_token: boolean;
  last_probe?: Probe;
}

export interface Policy {
  default_provider: Provider;
  require_eid: boolean;
  min_certificate_level: "ADVANCED" | "QUALIFIED" | "QSCD";
  allow_on_behalf_of: boolean;
  allow_self_sign: boolean;
  retention_days: number;
  max_upload_mb: number;
}

export interface Settings {
  placement: Placement;
  hsm: HSMSettings;
  policy: Policy;
  updated_at?: string;
}

export interface Page<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

/** Carries the backend's machine code so a screen can branch without parsing prose. */
export class EsignApiError extends Error {
  readonly code: string;
  readonly status: number;

  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = "EsignApiError";
    this.code = code;
    this.status = status;
  }
}

function authHeaders(extra: Record<string, string> = {}): Record<string, string> {
  const locale = typeof window !== "undefined" ? window.localStorage.getItem("locale") || "mn" : "mn";
  return { "Accept-Language": locale, ...extra };
}

/**
 * Reads a response without assuming it is JSON.
 *
 * An edge proxy answers 413, 502 and 504 with its own HTML page, so calling
 * res.json() straight would surface a raw `Unexpected token '<'` SyntaxError
 * to the user instead of a message about their file being too large.
 */
async function readJSON<T>(res: Response): Promise<T | null> {
  const text = await res.text();
  if (!text) return null;
  try {
    return JSON.parse(text) as T;
  } catch {
    return null;
  }
}

/** Turns a status with no usable body into something a person can act on. */
function httpErrorMessage(status: number): string {
  if (status === 413) return "Файл хэт том байна.";
  if (status === 401 || status === 403) return "Нэвтрэлт дууссан эсвэл эрх хүрэхгүй байна.";
  if (status === 429) return "Хэт олон хүсэлт илгээлээ. Түр хүлээгээд дахин оролдоно уу.";
  if (status >= 500) return "Үйлчилгээ түр саатлаа. Дахин оролдоно уу.";
  return "Хүсэлт амжилтгүй боллоо.";
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(`${apiBase()}${path}`, {
    ...init,
    headers: authHeaders({ "Content-Type": "application/json", ...(init.headers as Record<string, string>) }),
    credentials: "include",
  });

  if (!res.ok) {
    const body = await readJSON<{ error?: string; code?: string }>(res);
    throw new EsignApiError(body?.error || httpErrorMessage(res.status), body?.code || "UNKNOWN", res.status);
  }
  if (res.status === 204) return undefined as T;
  return ((await readJSON<T>(res)) ?? (undefined as T));
}

const toQuery = (params: Record<string, string | number | boolean | undefined>) => {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "" && value !== false) search.set(key, String(value));
  }
  const qs = search.toString();
  return qs ? `?${qs}` : "";
};

async function download(path: string): Promise<Blob> {
  const res = await fetch(`${apiBase()}${path}`, { headers: authHeaders(), credentials: "include" });
  if (!res.ok) {
    const body = await readJSON<{ error?: string; code?: string }>(res);
    throw new EsignApiError(body?.error || httpErrorMessage(res.status), body?.code || "UNKNOWN", res.status);
  }
  return res.blob();
}

/** Saves a blob under a filename without leaking the object URL. */
export function saveBlob(blob: Blob, fileName: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = fileName;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export interface LogFilter {
  action?: string;
  outcome?: string;
  provider?: string;
  document_id?: string;
  q?: string;
  from?: string;
  to?: string;
  limit?: number;
  offset?: number;
}

export const esign = {
  // ─── Documents ────────────────────────────────────────────────────────────
  documents: (params: { status?: string; q?: string; limit?: number; offset?: number } = {}) =>
    request<Page<EsignDocument>>(`/esign/documents${toQuery({ ...params, paginated: true })}`),

  document: (id: string) => request<EsignDocument>(`/esign/documents/${id}`),

  /**
   * Uploads through multipart rather than base64. The older JSON route still
   * exists for API clients, but sending a 25MB PDF as base64 costs a third
   * more bytes and forces the whole file through a string in the browser.
   */
  upload: async (file: File, title: string): Promise<EsignDocument> => {
    const form = new FormData();
    form.set("file", file, file.name);
    if (title) form.set("title", title);

    const res = await fetch(`${apiBase()}/esign/documents/upload`, {
      method: "POST",
      headers: authHeaders(),
      body: form,
      credentials: "include",
    });
    if (!res.ok) {
      const body = await readJSON<{ error?: string; code?: string }>(res);
      throw new EsignApiError(body?.error || httpErrorMessage(res.status), body?.code || "UNKNOWN", res.status);
    }
    return (await readJSON<EsignDocument>(res)) as EsignDocument;
  },

  remove: (id: string) => request<void>(`/esign/documents/${id}`, { method: "DELETE" }),

  downloadDocument: (id: string, variant: "original" | "signed") =>
    download(`/esign/documents/${id}/download?variant=${variant}`),

  // ─── eID Mongolia rail ────────────────────────────────────────────────────

  /** Starts a ceremony for a freshly picked file. */
  signFile: async (file: File, onBehalfOf?: string, signerId?: string): Promise<SignSession> => {
    const form = new FormData();
    form.set("file", file, file.name);
    if (onBehalfOf) form.set("onBehalfOf", onBehalfOf);
    if (signerId) form.set("signer_id", signerId);

    const res = await fetch(`${apiBase()}/esign/sign/init`, {
      method: "POST",
      headers: authHeaders(),
      body: form,
      credentials: "include",
    });
    if (!res.ok) {
      const body = await readJSON<{ error?: string; code?: string }>(res);
      throw new EsignApiError(body?.error || httpErrorMessage(res.status), body?.code || "UNKNOWN", res.status);
    }
    return (await readJSON<SignSession>(res)) as SignSession;
  },

  /**
   * Starts a ceremony for a document already in the store.
   *
   * signerId names the citizen when the account is not linked to eID. It goes
   * in the body rather than a form field because this route is JSON — the
   * multipart form value the upload path uses is not readable here.
   */
  signDocument: (documentId: string, onBehalfOf?: string, signerId?: string) =>
    request<SignSession>("/esign/sign/init", {
      method: "POST",
      body: JSON.stringify({ document_id: documentId, on_behalf_of: onBehalfOf, signer_id: signerId }),
    }),

  session: (id: string) => request<SignSession>(`/esign/sign/${id}`),
  cancelSession: (id: string) => request<SignSession>(`/esign/sign/${id}/cancel`, { method: "POST" }),
  downloadSigned: (id: string) => download(`/esign/sign/${id}/download`),

  organizations: () => request<Representation[]>("/esign/organizations"),

  // ─── HSM rail ─────────────────────────────────────────────────────────────
  checkCertificate: (body: { phone_no: string; civil_id: string; data?: string }) =>
    request<{ is_valid: boolean; given_name: string; surname: string; common_name: string; uid: string }>(
      "/esign/cert/check",
      { method: "POST", body: JSON.stringify(body) },
    ),

  signWithHSM: (
    id: string,
    body: {
      phone_no: string;
      signer_name: string;
      signer_reg_no: string;
      signature_image64: string;
      signature_text?: string;
      x?: number;
      y?: number;
      width?: number;
      height?: number;
      page_number?: number;
    },
  ) =>
    request<{ status: string; document_id: string; signed_at: string; page_number: number; provider: Provider }>(
      `/esign/documents/${id}/sign`,
      { method: "POST", body: JSON.stringify(body) },
    ),

  // ─── Signature log ────────────────────────────────────────────────────────
  logs: (filter: LogFilter = {}) =>
    request<Page<SignatureLogEntry>>(`/esign/logs${toQuery({ ...filter, paginated: true })}`),

  exportLogs: (filter: LogFilter = {}) => download(`/esign/logs/export${toQuery({ ...filter })}`),

  // ─── Batches ──────────────────────────────────────────────────────────────
  batches: (params: { limit?: number; offset?: number } = {}) =>
    request<Page<Batch>>(`/esign/batches${toQuery(params)}`),

  batch: (id: string) => request<Batch>(`/esign/batches/${id}`),

  createBatch: (body: { name: string; provider?: Provider; document_ids: string[] }) =>
    request<Batch>("/esign/batches", { method: "POST", body: JSON.stringify(body) }),

  /** Advances the batch by one document and returns the ceremony to confirm. */
  runBatch: (id: string) =>
    request<{ batch: Batch; session: SignSession | null; error?: string }>(`/esign/batches/${id}/run`, {
      method: "POST",
    }),

  cancelBatch: (id: string) => request<Batch>(`/esign/batches/${id}/cancel`, { method: "POST" }),

  // ─── Settings ─────────────────────────────────────────────────────────────
  settings: () => request<Settings>("/esign/settings"),

  savePlacement: (placement: Placement) =>
    request<Placement>("/esign/settings/placement", { method: "PUT", body: JSON.stringify(placement) }),

  savePolicy: (policy: Policy) =>
    request<Policy>("/esign/settings/policy", { method: "PUT", body: JSON.stringify(policy) }),

  testHSM: () => request<Probe>("/esign/settings/hsm/test", { method: "POST" }),
};
