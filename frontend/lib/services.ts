import type {TranslationKey} from "@/lib/i18n";

/**
 * The services that stand beside the platform on their own addresses.
 *
 * A Nexus deployment is not one hostname. The console answers on one, the
 * documentation on another, and an installation that has grown a warehouse, a
 * backup store or a monitoring stack has one more for each. A visitor who
 * only ever sees the front door has no way to learn that any of them exist.
 *
 * The first of them is not ours. eID Mongolia is national infrastructure and
 * the rail this platform authenticates citizens against; it stands at the head
 * of the list because it is the first thing a visitor meets — the sign-in card
 * on the front page is already talking to it — and because a reader who wants
 * to know what is behind that card should not have to look for it.
 *
 * What is written here is what every deployment shares — the name of each
 * service, what it is for, and the drawing that goes with it. What is *not*
 * written here is where they live: `admin.nexus.gerege.mn` is this
 * installation's address and nobody else's, so the addresses come from the
 * deployment's own environment. A service with no address configured is not
 * drawn, which is the honest default: most installations have a console and
 * documentation, some have all six, and a card leading to a hostname that
 * does not resolve is worse than no card.
 */
export const SERVICE_IDS = ["eid", "admin", "dwh", "backups", "monitor", "docs"] as const;

export type ServiceId = (typeof SERVICE_IDS)[number];

export type Service = {
  id: ServiceId;
  href: string;
  title: TranslationKey;
  body: TranslationKey;
};

/**
 * The environment variable each address is read from.
 *
 * Named per service rather than parsed out of one packed string: an operator
 * setting up a warehouse edits one line, a deployment that never gets one
 * never mentions it, and a typo in one address cannot take the other five
 * down with it.
 */
const ENV_VARS: Record<ServiceId, string> = {
  eid: "SERVICE_URL_EID",
  admin: "SERVICE_URL_ADMIN",
  dwh: "SERVICE_URL_DWH",
  backups: "SERVICE_URL_BACKUPS",
  monitor: "SERVICE_URL_MONITOR",
  docs: "SERVICE_URL_DOCS",
};

const COPY: Record<ServiceId, {title: TranslationKey; body: TranslationKey}> = {
  eid:     {title: "website.service.eid_title",     body: "website.service.eid_body"},
  admin:   {title: "website.service.admin_title",   body: "website.service.admin_body"},
  dwh:     {title: "website.service.dwh_title",     body: "website.service.dwh_body"},
  backups: {title: "website.service.backups_title", body: "website.service.backups_body"},
  monitor: {title: "website.service.monitor_title", body: "website.service.monitor_body"},
  docs:    {title: "website.service.docs_title",    body: "website.service.docs_body"},
};

/**
 * Reads the configured services, in the order above.
 *
 * Server-side only, like `landingSectionsFromEnv`: `process.env` in the
 * browser holds only what the build inlined, and these addresses are a
 * property of the running deployment rather than of the image.
 *
 * An address that is not an absolute http(s) URL is dropped with a warning
 * rather than rendered. A card is a link somebody will press; one pointing at
 * `admin.example` because a scheme was forgotten is a broken promise, and the
 * warning is what tells the operator which line to fix.
 */
export function servicesFromEnv(env: NodeJS.ProcessEnv = process.env): Service[] {
  const services: Service[] = [];
  for (const id of SERVICE_IDS) {
    const raw = (env[ENV_VARS[id]] ?? "").trim();
    if (!raw) continue;
    if (!/^https?:\/\/\S+$/.test(raw)) {
      console.warn(`${ENV_VARS[id]}: "${raw}" is not an http(s) address; the ${id} card is not drawn`);
      continue;
    }
    services.push({id, href: raw, ...COPY[id]});
  }
  return services;
}
