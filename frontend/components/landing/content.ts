import {
  Activity,
  Boxes,
  BrainCircuit,
  Fingerprint,
  Gauge,
  KeyRound,
  Landmark,
  Languages,
  Layers,
  Lock,
  Network,
  PackageCheck,
  ShieldCheck,
  Store,
  Waypoints,
  Workflow,
} from "lucide-react";

import type {TranslationKey} from "@/lib/i18n";

// The documentation address moved to `lib/brand.ts` as `DEFAULT_BRAND.docsUrl`:
// a deployment carrying its own name carries its own manual, so the address is
// part of its identity rather than a constant of this page.

type Icon = typeof Fingerprint;

/**
 * What the platform does, as four claims. The order is the order a reader
 * meets them: identity first, because nothing else in the list is reachable
 * before someone has signed in.
 */
export const CAPABILITIES: {icon: Icon; title: TranslationKey; body: TranslationKey}[] = [
  {icon: Fingerprint, title: "website.feature.instant_title", body: "website.feature.instant_body"},
  {icon: Network, title: "website.feature.sso_title", body: "website.feature.sso_body"},
  {icon: ShieldCheck, title: "website.feature.passwordless_title", body: "website.feature.passwordless_body"},
  {icon: Waypoints, title: "website.feature.channels_title", body: "website.feature.channels_body"},
];

/** The guarantees behind the identity claim, as a checklist. */
export const TRUST_POINTS: TranslationKey[] = [
  "website.trust.cookie",
  "website.trust.rbac",
  "website.trust.allowlist",
  "website.trust.audit",
];

/**
 * The three parts a sign-in passes through, drawn left to right in the order
 * a request actually travels them.
 */
export const TECHNOLOGY: {icon: Icon; name: string; body: TranslationKey}[] = [
  // The other two name products that are not ours, so this field is a
  // literal. This one names the deployment, and takes the same {brand} the
  // dictionary uses — Technology.tsx resolves it.
  {icon: Layers, name: "{brand}", body: "website.tech.erp_body"},
  {icon: Fingerprint, name: "eID Mongolia", body: "website.tech.eid_body"},
  {icon: KeyRound, name: "OIDC / SSO", body: "website.tech.sso_body"},
];

/**
 * How the platform is put together, for the reader who has decided the sign-in
 * works and now wants to know what they would be adopting.
 */
export const ARCHITECTURE: {icon: Icon; title: TranslationKey; body: TranslationKey}[] = [
  {icon: Boxes, title: "website.arch.modules_title", body: "website.arch.modules_body"},
  {icon: Store, title: "website.arch.store_title", body: "website.arch.store_body"},
  {icon: Workflow, title: "website.arch.dag_title", body: "website.arch.dag_body"},
  {icon: PackageCheck, title: "website.arch.catalog_title", body: "website.arch.catalog_body"},
];

/**
 * The nine applications in the catalogue, in the order the catalogue lists
 * them. Written out rather than fetched: this section is part of the argument
 * the page makes, and a marketing claim that renders empty when an API is slow
 * is worse than one that cannot change without a deploy.
 */
export const APPLICATIONS: TranslationKey[] = [
  "website.apps.core",
  "website.apps.contacts",
  "website.apps.products",
  "website.apps.inventory",
  "website.apps.billing",
  "website.apps.documents",
  "website.apps.gov",
  "website.apps.esign",
  "website.apps.developer",
];

/** What every application inherits from the core, whether or not it asks. */
export const PLATFORM_DEPTH: {icon: Icon; title: TranslationKey; body: TranslationKey}[] = [
  {icon: Activity, title: "website.depth.resilience_title", body: "website.depth.resilience_body"},
  {icon: Landmark, title: "website.depth.gov_title", body: "website.depth.gov_body"},
  {icon: Lock, title: "website.depth.security_title", body: "website.depth.security_body"},
  {icon: BrainCircuit, title: "website.depth.ai_title", body: "website.depth.ai_body"},
  {icon: Languages, title: "website.depth.i18n_title", body: "website.depth.i18n_body"},
  {icon: Gauge, title: "website.depth.observability_title", body: "website.depth.observability_body"},
];
