"use client";

/**
 * The console's small shared pieces: a card, a table, a badge, and the one
 * date formatter every screen uses.
 *
 * They lived in the tenant detail page for a phase, and four other screens
 * imported them from it — a page importing a page, which works and reads as an
 * accident. This is where they belong.
 */

import React from "react";

export function Card({ title, action, children }: { title: string; action?: React.ReactNode; children: React.ReactNode }) {
  return (
    <section className="bg-surface rounded-xl border border-line overflow-hidden">
      <h2 className="px-4 py-3 border-b border-line font-medium text-foreground flex items-center gap-3">
        <span className="flex-1">{title}</span>
        {action}
      </h2>
      {children}
    </section>
  );
}

export function Table({ head, rows, empty }: { head: string[]; rows: React.ReactNode[][]; empty: string }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead className="bg-surface-2 text-muted">
          <tr>
            {head.map((cell, index) => (
              <th key={index} className="text-left font-medium px-4 py-2.5">
                {cell}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-line">
          {rows.map((row, index) => (
            <tr key={index} className="hover:bg-surface-hover">
              {row.map((cell, cellIndex) => (
                <td key={cellIndex} className="px-4 py-2.5 text-foreground">
                  {cell}
                </td>
              ))}
            </tr>
          ))}
          {rows.length === 0 && (
            <tr>
              <td colSpan={head.length} className="px-4 py-8 text-center text-muted">
                {empty}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}

export type Tone = "red" | "amber" | "emerald" | "slate" | "green";

export function Badge({ tone, children }: { tone: Tone; children: React.ReactNode }) {
  const tones: Record<Tone, string> = {
    red: "bg-red-100 text-red-800",
    amber: "bg-amber-100 text-amber-900",
    emerald: "bg-emerald-100 text-emerald-800",
    green: "bg-emerald-100 text-emerald-800",
    slate: "bg-surface-2 text-foreground",
  };
  return <span className={`text-xs font-medium rounded-full px-2 py-0.5 ${tones[tone]}`}>{children}</span>;
}

/**
 * Re-exported so the console's screens keep importing it from here.
 *
 * The implementation moved to lib/datetime.ts when the format did: every
 * timestamp on the platform is now `yyyy-MM-dd HH:mm` in Asia/Ulaanbaatar,
 * not the reader's own locale and zone. The reasoning is written out there.
 */
export { formatMoment } from "@/lib/datetime";
