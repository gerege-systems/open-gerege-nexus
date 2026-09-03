/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Documents (io.gerege.nexus.documents) — official documents, templates,
// approval chains, signature policies and retention. The contract screens use
// lib/contracts.ts; these are the document-ledger calls.

import { request } from "./client";

export const documentsApi = {
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
    /** "exclude" drops contracts — they live on their own screen. */
    contracts?: "exclude";
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
    if (params?.contracts) query.set("contracts", params.contracts);
    if (params?.limit) query.set("limit", String(params.limit));
    if (params?.offset) query.set("offset", String(params.offset));
    if (params?.after_at && params?.after_id) {
      query.set("after_at", params.after_at);
      query.set("after_id", params.after_id);
    }
    const suffix = query.toString() ? `?${query}` : "";
    return request<{
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
    request(`/documents/${id}/title`, { method: "PUT", body: JSON.stringify({ title }) }),

  createDocument: (data: { title: string; doc_type: string }) =>
    request("/documents", { method: "POST", body: JSON.stringify(data) }),

  // E-ID signing is an approval the citizen gives on their own device: start
  // pushes the request — naming the document — and poll waits for them to answer.
  // eID has no document-signing endpoint; that approval is the signature.
  startEIDSignature: (id: string, regNumber: string) =>
    request<{
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
    request<{ state: string; document?: any }>(`/documents/${id}/sign/eid/poll`, {
      method: "POST",
      body: JSON.stringify({ session_id: sessionId }),
      signal,
    }),

  // DAN exposes no approval push, so it stays a registration number and a code.
  signDocumentWithDAN: (id: string, data: { reg_number: string; otp_code: string }) =>
    request(`/documents/${id}/sign/dan`, { method: "POST", body: JSON.stringify(data) }),

  // Send a draft for approval.
  routeDocument: (id: string) => request(`/documents/${id}/route`, { method: "POST" }),

  // A document's signature ledger, oldest first.
  getDocumentSignatures: (id: string) =>
    request<
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
    request<Array<{ order: number; name: string; signer_reg_number: string }>>(`/documents/${id}/steps`),

  // Templates a document is started from
  getDocumentTemplates: () =>
    request<
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
    request("/documents/templates", { method: "POST", body: JSON.stringify(data) }),

  updateDocumentTemplate: (
    id: string,
    data: { name: string; doc_type: string; title_pattern: string; active: boolean }
  ) => request(`/documents/templates/${id}`, { method: "PUT", body: JSON.stringify(data) }),

  deleteDocumentTemplate: (id: string) => request<void>(`/documents/templates/${id}`, { method: "DELETE" }),

  useDocumentTemplate: (id: string) => request(`/documents/templates/${id}/use`, { method: "POST" }),

  // How each document type may be signed
  getSignaturePolicies: () =>
    request<
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
  ) => request(`/documents/policies/${docType}`, { method: "PUT", body: JSON.stringify(data) }),

  // Who must sign each document type, in order
  getDocumentWorkflows: () =>
    request<
      Array<{
        doc_type: string;
        steps: Array<{ order: number; name: string; signer_reg_number: string }>;
      }>
    >("/documents/workflows"),

  saveDocumentWorkflow: (docType: string, steps: Array<{ name: string; signer_reg_number: string }>) =>
    request(`/documents/workflows/${docType}`, { method: "PUT", body: JSON.stringify({ steps }) }),

  // How long each document type is kept
  getRetentionRules: () =>
    request<
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
    request(`/documents/retention/${docType}`, { method: "PUT", body: JSON.stringify(data) }),

  // Reject a pending document — moves it to REJECTED.
  rejectDocument: (id: string) =>
    request(`/documents/${id}/reject`, { method: "POST" }),

};
