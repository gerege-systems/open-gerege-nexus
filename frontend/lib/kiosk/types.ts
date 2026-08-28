/**
 * Declarative description of a kiosk screen.
 *
 * The kiosk's 26 modules were written by copying one handler into the next, so
 * they all speak the same dialect: a list is a paginated GET, a create or edit
 * puts the whole record in the body, and a delete carries just {id}. That
 * uniformity is the reason a screen here is a declaration rather than a
 * hand-written form — and the reason a module that breaks the pattern (file
 * upload, QR issuing, the report pages) gets a purpose-built page instead of
 * being forced into this shape.
 */

import type { ReactNode } from "react";
import type { DictionaryKey } from "@/lib/i18n";

export type FieldType = "text" | "number" | "boolean" | "textarea" | "json" | "date";

export interface KioskField {
  key: string;
  /** Checked against the dictionary, so a typo is a build error. */
  label: DictionaryKey;
  type?: FieldType;
  required?: boolean;
  /** Server-assigned values are shown when editing but never submitted. */
  readOnly?: boolean;
  placeholder?: string;
}

export interface KioskColumn {
  key: string;
  label: DictionaryKey;
  /** Renders the raw value; used for badges, dates and nested objects. */
  render?: (row: Record<string, any>) => ReactNode;
}

/**
 * Which of the two kiosk apps serves a screen. They mount at different
 * prefixes and are installed independently, so a screen has to say which one
 * it belongs to rather than assuming.
 */
export type KioskApp = "kiosk" | "kiosk-directory";

export interface KioskResource {
  /** URL segment under /module/<app>/. */
  slug: string;
  /** Defaults to the operations app. */
  app?: KioskApp;
  /** i18n keys. */
  title: DictionaryKey;
  description: DictionaryKey;
  /** lucide icon name, as mapped in components/Layout.tsx. */
  icon: string;

  /** GET path under /api/v1/kiosk. */
  list: string;
  /** Extra query parameters the list endpoint requires. */
  listParams?: Record<string, string | number>;

  /** Present only when the module actually exposes the verb. */
  create?: string;
  update?: string;
  remove?: string;

  /** Primary key. Most modules use a numeric id; the lookup tables use code. */
  idKey?: string;

  columns: KioskColumn[];
  fields?: KioskField[];

  /**
   * Turns the rows into something you can look at. The list gets a preview
   * cell and clicking it opens the file — an image is shown, a video plays.
   *
   * Without it a media table is a column of URLs, which is the one thing an
   * operator checking what a kiosk is displaying cannot use.
   */
  media?: {
    /** Row key holding the file URL. */
    urlKey: string;
    /** Row key holding a display name, for the viewer caption. */
    nameKey?: string;
  };

  /**
   * Why a screen is read-only, when the reason is not obvious. Shown to the
   * operator so an absent New button does not read as a missing feature.
   */
  readOnlyReason?: DictionaryKey;
}
