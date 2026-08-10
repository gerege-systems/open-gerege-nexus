const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

async function fetcher<T>(url: string, options: RequestInit = {}): Promise<T> {
  // Server-owned content (menu labels, app store copy) is translated by the
  // API, so every request carries the locale the user picked.
  const locale = typeof window !== "undefined" ? window.localStorage.getItem("locale") || "mn" : "mn";
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "Accept-Language": locale,
    ...(options.headers as Record<string, string>),
  };
  const res = await fetch(`${API_BASE}${url}`, {
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

  logout: () => fetcher<{ status: string }>("/auth/logout", { method: "POST" }),

  // permissions carries the effective grant of every role the member holds; it
  // is empty for administrators, who bypass the check.
  getMe: () => fetcher<{ id: string; tenant_id: string; tenant_name: string; name: string; email: string; is_admin: boolean; permissions?: string[] }>("/auth/me"),

  // The organisations the signed-in person may act for. A membership in one is
  // the common case, so callers should expect a list of one rather than treat
  // it as an error.
  getTenants: () => fetcher<{ current: string; tenants: Array<{ id: string; name: string; slug: string }> }>("/auth/tenants"),

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

  // Store
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
      }>
    >("/installed-apps"),

  installApp: (slug: string) => mutateApp(`/store/apps/${slug}/install`),

  enableApp: (slug: string) => mutateApp(`/store/apps/${slug}/enable`),

  disableApp: (slug: string) => mutateApp(`/store/apps/${slug}/disable`),

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

  // Billing App (io.example.billing)
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

  // Documents App (io.example.documents)
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

  // PDF E-Sign App (io.example.esign)
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
    const res = await fetch(`${API_BASE}/esign/documents/${id}/download?variant=${variant}`, {
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
