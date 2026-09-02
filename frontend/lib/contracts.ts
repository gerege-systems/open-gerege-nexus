/**
 * Typed client for the contract half of the documents app
 * (io.gerege.nexus.documents, client-gerege-nexus).
 *
 * A contract is a document with PARTIES: legal entities the text is issued to,
 * each with the PEOPLE allowed to sign for it. The issuer freezes one PDF per
 * party at send time; each party signs exactly the bytes it was shown, with a
 * qualified eID signature (PIN2 on the citizen's own phone).
 *
 * Two surfaces, deliberately separate on the server and mirrored here:
 *
 *   /documents/...          the issuer's view — addressed by document id.
 *   /documents/inbox/...    the recipient's view — addressed by PARTY id,
 *                           because the recipient does not own the document
 *                           and the server never shows it to them.
 */

import { apiBase } from "@/lib/apiBase";

// ─────────────────────────────────────────────────────────────────── shapes

export type ContractState =
  | "NONE" | "DRAFT" | "SENT" | "PARTIALLY_SIGNED"
  | "EXECUTED" | "DECLINED" | "WITHDRAWN" | "EXPIRED" | "TERMINATED";

export type PartyState =
  | "draft" | "invited" | "viewed" | "signed" | "declined" | "withdrawn" | "expired";

export type PartyRole = "issuer" | "counterparty" | "witness" | "guarantor";
export type PartyKind = "member" | "tenant" | "peer" | "person" | "organisation";

export interface ContractRow {
  id: string;
  parent_document_id?: string | null;
  issued_count?: number;
  issued_executed?: number;
  contract_number?: string;
  title: string;
  doc_type: string;
  contract_state: ContractState;
  signing_mode: string;
  counterparties: string;
  party_count: number;
  signed_count: number;
  required_count: number;
  declined_count: number;
  amount?: number;
  currency?: string;
  effective_from?: string;
  sent_at?: string;
  executed_at?: string;
  due_at?: string;
  effective_to?: string;
  created_at: string;
}

export interface Signatory {
  id: string;
  party_id: string;
  full_name: string;
  position?: string;
  reg_number?: string;
  user_id?: string;
  signed_at?: string;
}

export interface Party {
  id: string;
  ordinal: number;
  party_role: PartyRole;
  party_kind: PartyKind;
  display_name: string;
  legal_name?: string;
  registration_number?: string;
  address_line?: string;
  contact_email?: string;
  contact_phone?: string;
  member_user_id?: string;
  counterparty_tenant_id?: string;
  required: boolean;
  sign_order?: number;
  state: PartyState;
  invited_at?: string;
  viewed_at?: string;
  signed_at?: string;
  declined_at?: string;
  decline_reason?: string;
  signatories?: Signatory[];
  has_copy: boolean;
  has_signed_copy: boolean;
}

/** The document's attached PDF, when one exists. */
export interface Attachment {
  file_name: string;
  size_bytes: number;
  /** The issuer has already signed the file with PIN2 — parties will sign bytes that cover that signature. */
  master_signed: boolean;
}

/** The parties answer carries the contract's own facts too — one load per screen. */
export interface ContractShape {
  parties: Party[];
  mode: string;
  contract_state: ContractState;
  title: string;
  doc_type: string;
  contract_number: string;
  amount?: number;
  currency?: string;
  effective_from?: string;
  effective_to?: string;
  due_at?: string;
  attachment?: Attachment | null;
}

export interface InboxItem {
  party_id: string;
  document_id: string;
  title: string;
  doc_type: string;
  party_role: string;
  issuer_name: string;
  issuer_registration?: string;
  state: PartyState;
  invited_at?: string;
  due_at?: string;
  has_copy: boolean;
  has_signed_copy: boolean;
}

export interface InboxBrief {
  party_role: string;
  display_name: string;
  state: PartyState;
  mine: boolean;
}

export interface InboxDetail {
  party_id: string;
  document_id: string;
  title: string;
  doc_type: string;
  state: PartyState;
  required: boolean;
  body_text: string;
  sha256: string;
  frozen_at?: string;
  has_copy: boolean;
  parties: InboxBrief[];
  my_signatories: Signatory[];
}

export interface CeremonySession {
  session_id: string;
  verification_code: string;
  display_text?: string;
}

/** COMPLETE / PENDING / REFUSED / EXPIRED — the eID rail's own vocabulary. */
export interface CeremonyProgress {
  state: string;
}

export interface Invitation {
  id: string;
  token: string;
  path: string;
  expires_at: string;
  channel: string;
}

export interface SendResult {
  sent: number;
  skipped: Array<{ party_id: string; name: string; reason: string }>;
}

// ───────────────────────────────────────────────────────────────── plumbing

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(`${apiBase()}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...(init.headers as Record<string, string>) },
    credentials: "include",
  });
  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    try {
      const body = (await res.json()) as { error?: string };
      if (body?.error) message = body.error;
    } catch {
      // a plain-text or empty error body keeps the status message
    }
    const failure = new Error(message) as Error & { status?: number };
    failure.status = res.status;
    throw failure;
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

// ─────────────────────────────────────────────────────────────────── client

export const contracts = {
  // ── the issuer's side, addressed by document id
  list: () => request<{ contracts: ContractRow[] }>("/documents/contracts"),
  // POST /documents/contracts, ЭНГИЙН /documents биш: гэрээ төрөхдөө
  // contract_state='DRAFT' авч, Гэрээний жагсаалтад ШУУД гарна. Ердийн
  // create-ээр үүсгэвэл NONE төрж, жагсаалтаас алга болдог байсан.
  create: (title: string) =>
    request<{ id: string }>("/documents/contracts", {
      method: "POST",
      body: JSON.stringify({ title }),
    }),
  parties: (id: string) => request<ContractShape>(`/documents/${id}/parties`),
  saveFacts: (
    id: string,
    facts: {
      contract_number: string;
      amount: number | null;
      currency: string;
      effective_from: string;
      effective_to: string;
      due_at: string;
    },
  ) => request<void>(`/documents/${id}/contract`, { method: "PUT", body: JSON.stringify(facts) }),
  body: (id: string) => request<{ body: string }>(`/documents/${id}/body`),
  saveBody: (id: string, body: string) =>
    request<{ body: string }>(`/documents/${id}/body`, { method: "PUT", body: JSON.stringify({ body }) }),
  addParty: (id: string, party: Record<string, unknown>) =>
    request<Party>(`/documents/${id}/parties`, { method: "POST", body: JSON.stringify(party) }),
  removeParty: (id: string, pid: string) =>
    request<void>(`/documents/${id}/parties/${pid}`, { method: "DELETE" }),
  addSignatory: (
    id: string,
    pid: string,
    signatory: { full_name: string; position: string; reg_number: string },
  ) =>
    request<Signatory>(`/documents/${id}/parties/${pid}/signatories`, {
      method: "POST",
      body: JSON.stringify(signatory),
    }),
  removeSignatory: (id: string, pid: string, sid: string) =>
    request<void>(`/documents/${id}/parties/${pid}/signatories/${sid}`, { method: "DELETE" }),
  invite: (id: string, pid: string) =>
    request<Invitation>(`/documents/${id}/parties/${pid}/invite`, {
      method: "POST",
      body: JSON.stringify({ channel: "link" }),
    }),
  send: (id: string, mode: "counterpart" | "joint") =>
    request<SendResult>(`/documents/${id}/send`, { method: "POST", body: JSON.stringify({ mode }) }),
  withdraw: (id: string, reason: string) =>
    request<{ contract_state: ContractState }>(`/documents/${id}/withdraw`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    }),
  reopen: (id: string) =>
    request<{ contract_state: ContractState }>(`/documents/${id}/reopen`, { method: "POST", body: "{}" }),
  signStart: (id: string, pid: string) =>
    request<CeremonySession>(`/documents/${id}/parties/${pid}/sign/start`, { method: "POST", body: "{}" }),
  signPoll: (id: string, pid: string) =>
    request<CeremonyProgress>(`/documents/${id}/parties/${pid}/sign/poll`, { method: "POST", body: "{}" }),
  // ── the master PDF: upload, download, and the issuer's own PIN2
  attach: async (id: string, file: File) => {
    const form = new FormData();
    form.append("file", file);
    const res = await fetch(`${apiBase()}/documents/${id}/file`, {
      method: "POST",
      body: form,
      credentials: "include",
    });
    if (!res.ok) {
      let message = `HTTP ${res.status}`;
      try {
        const body = (await res.json()) as { error?: string };
        if (body?.error) message = body.error;
      } catch { /* keep the status message */ }
      throw new Error(message);
    }
    return res.json() as Promise<{ file_name: string; size_bytes: number; sha256: string }>;
  },
  fileUrl: (id: string) => `${apiBase()}/documents/${id}/file`,
  masterSignStart: (id: string, regNumber: string) =>
    request<CeremonySession>(`/documents/${id}/sign/eid/start`, {
      method: "POST",
      body: JSON.stringify({ reg_number: regNumber }),
    }),
  masterSignPoll: (id: string, sessionID: string) =>
    request<CeremonyProgress>(`/documents/${id}/sign/eid/poll`, {
      method: "POST",
      body: JSON.stringify({ session_id: sessionID }),
    }),
  /** The signed-in user's own eID registration number, or "" when none is linked. */
  myEidReg: async (): Promise<string> => {
    const profile = await request<{
      identities?: Array<{ kind: string; claims?: { reg_number?: string } }>;
    }>("/profile");
    const eid = profile.identities?.find((identity) => identity.kind === "eid");
    return eid?.claims?.reg_number ?? "";
  },

  // ── many recipients from one file
  importParties: async (id: string, file: File) => {
    const form = new FormData();
    form.append("file", file);
    const res = await fetch(`${apiBase()}/documents/${id}/parties/import`, {
      method: "POST",
      body: form,
      credentials: "include",
    });
    if (!res.ok) {
      let message = `HTTP ${res.status}`;
      try {
        const body = (await res.json()) as { error?: string };
        if (body?.error) message = body.error;
      } catch { /* keep the status message */ }
      throw new Error(message);
    }
    return res.json() as Promise<{ added: number; skipped: Array<{ row: number; name?: string; reason: string }> }>;
  },
  importTemplateUrl: () => `${apiBase()}/documents/parties/import-template.xlsx`,
  wordTemplateUrl: () => `${apiBase()}/documents/contract-template.docx`,

  // ── issue preview: parse the file server-side, create NOTHING.
  // Excel goes up once; what comes back is JSON the UI can chunk — so a
  // 500-row list never has to fit one request, and the admin sees who is
  // already issued BEFORE pressing the button.
  issuePreview: async (id: string, file: File) => {
    const form = new FormData();
    form.append("file", file);
    const res = await fetch(`${apiBase()}/documents/${id}/issue/preview`, {
      method: "POST", body: form, credentials: "include",
    });
    if (!res.ok) {
      let message = `HTTP ${res.status}`;
      try {
        const body = (await res.json()) as { error?: string };
        if (body?.error) message = body.error;
      } catch { /* keep the status message */ }
      throw new Error(message);
    }
    return res.json() as Promise<{
      recipients: Array<{
        name: string; org_reg?: string; signer_name?: string; signer_reg: string;
        position?: string; line?: number; already_issued?: boolean; problem?: string;
      }>;
    }>;
  },

  // ── issue chunked: sequential requests of ≤10, so a Word master (each
  // recipient costs a LibreOffice conversion) stays inside the server's
  // per-request cap and the admin watches real progress instead of a spinner.
  issueChunked: async (
    id: string,
    recipients: Array<{ name: string; org_reg?: string; signer_name?: string; signer_reg: string; position?: string }>,
    onProgress?: (done: number, total: number) => void,
  ) => {
    const total = recipients.length;
    const out = {
      issued: 0,
      children: [] as Array<{ document_id: string; name: string }>,
      skipped: [] as Array<{ row?: number; name?: string; reason: string }>,
    };
    for (let at = 0; at < total; at += 10) {
      const part = await contracts.issue(id, recipients.slice(at, at + 10));
      out.issued += part.issued;
      out.children.push(...part.children);
      out.skipped.push(...part.skipped);
      onProgress?.(Math.min(at + 10, total), total);
    }
    return out;
  },

  // ── issue: one template, a SEPARATE bilateral contract per recipient.
  // A borrower must never become a co-party of the other borrowers' loans.
  issue: async (id: string, payload: File | Array<{ name: string; org_reg?: string; signer_name?: string; signer_reg: string; position?: string }>) => {
    let res: Response;
    if (payload instanceof File) {
      const form = new FormData();
      form.append("file", payload);
      res = await fetch(`${apiBase()}/documents/${id}/issue`, {
        method: "POST", body: form, credentials: "include",
      });
    } else {
      res = await fetch(`${apiBase()}/documents/${id}/issue`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ recipients: payload }),
        credentials: "include",
      });
    }
    if (!res.ok) {
      let message = `HTTP ${res.status}`;
      try {
        const body = (await res.json()) as { error?: string };
        if (body?.error) message = body.error;
      } catch { /* keep the status message */ }
      throw new Error(message);
    }
    return res.json() as Promise<{
      issued: number;
      children: Array<{ document_id: string; name: string }>;
      skipped: Array<{ row?: number; name?: string; reason: string }>;
    }>;
  },

  // Frozen and signed copies are links, not fetches: the PDF opens in a tab
  // and the cookie rides along on its own.
  copyUrl: (id: string, pid: string) => `${apiBase()}/documents/${id}/parties/${pid}/copy`,
  signedUrl: (id: string, pid: string) => `${apiBase()}/documents/${id}/parties/${pid}/signed.pdf`,

  // ── the recipient's side, addressed by party id
  inbox: (all: boolean) =>
    request<{ items: InboxItem[] }>(`/documents/inbox${all ? "?state=all" : ""}`),
  inboxShow: (pid: string) => request<InboxDetail>(`/documents/inbox/${pid}`),
  inboxNominate: (pid: string, signatory: { full_name: string; position: string; reg_number: string }) =>
    request<Signatory>(`/documents/inbox/${pid}/signatories`, {
      method: "POST",
      body: JSON.stringify(signatory),
    }),
  inboxSignStart: (pid: string) =>
    request<CeremonySession>(`/documents/inbox/${pid}/sign/start`, { method: "POST", body: "{}" }),
  inboxSignPoll: (pid: string) =>
    request<CeremonyProgress>(`/documents/inbox/${pid}/sign/poll`, { method: "POST", body: "{}" }),
  inboxDecline: (pid: string, reason: string) =>
    request<{ state: PartyState }>(`/documents/inbox/${pid}/decline`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    }),
  inboxCopyUrl: (pid: string) => `${apiBase()}/documents/inbox/${pid}/copy`,
  inboxSignedUrl: (pid: string) => `${apiBase()}/documents/inbox/${pid}/signed.pdf`,
};
