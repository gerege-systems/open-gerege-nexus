/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Gerege Kiosk: the request dialect its twenty-six modules share.
//
// Not a method per endpoint, which is what every other client here is. The
// kiosk's modules answer the same four shapes across all of them, so its
// screens are declared as data in lib/kiosk/resources.ts and driven through
// these four helpers — a named method per endpoint would be several hundred of
// them, each one a restatement of the same convention.
//
// The modules themselves live in the Gerege Kiosk distribution, not in this
// repository.

import { request } from "./client";

/**
 * A page of kiosk records, however the module chose to answer.
 *
 * The modules are split between two shapes, depending on which handler predates
 * which: the ones built on go_grc's paginator honour page_size and report
 * total_row, and the rest answer with the entire table as a bare array.
 * Normalising here keeps every screen from having to know which it is talking
 * to.
 */
export interface KioskPage<T> {
  items: T[];
  total: number;
  /**
   * Whether the server did the paging.
   *
   * A screen has to know: page two of a server-paged list is another request,
   * but of a bare array it is a slice of what is already in hand. Getting this
   * wrong is silent — the pager renders either way and simply shows the first
   * page for ever.
   */
  serverPaged: boolean;
}

/**
 * `base` selects which of the two kiosk apps to address. They mount at separate
 * prefixes and are gated independently, so it is not cosmetic: sending a
 * directory request to the operations prefix is a 403 from the app gate rather
 * than a 404.
 */
export async function kioskList<T = Record<string, unknown>>(
  path: string,
  params?: Record<string, string | number | undefined>,
  base = "kiosk",
): Promise<KioskPage<T>> {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params ?? {})) {
    if (v !== undefined && v !== "") qs.set(k, String(v));
  }
  const suffix = qs.toString() ? (path.includes("?") ? "&" : "?") + qs.toString() : "";
  const body = await request<T[] | { items?: T[]; total_row?: number }>(`/${base}${path}${suffix}`);
  if (Array.isArray(body)) return { items: body, total: body.length, serverPaged: false };
  return {
    items: body?.items ?? [],
    total: body?.total_row ?? body?.items?.length ?? 0,
    serverPaged: true,
  };
}

// Create, update and delete speak one dialect across all twenty-six modules:
// the whole record goes in the body, and a delete carries just {id} rather than
// putting it in the path. Encoding that convention once is what lets each
// screen be a declaration instead of a hand-written form.
export function kioskCreate<T>(path: string, body: unknown, base = "kiosk") {
  return request<T>(`/${base}${path}`, { method: "POST", body: JSON.stringify(body) });
}

export function kioskUpdate<T>(path: string, body: unknown, base = "kiosk") {
  return request<T>(`/${base}${path}`, { method: "PUT", body: JSON.stringify(body) });
}

export function kioskRemove<T>(path: string, id: unknown, idKey = "id", base = "kiosk") {
  return request<T>(`/${base}${path}`, { method: "DELETE", body: JSON.stringify({ [idKey]: id }) });
}
