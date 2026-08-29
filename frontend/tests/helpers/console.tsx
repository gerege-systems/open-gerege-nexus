"use client";

/**
 * The signed-in operator, for a screen rendered without its frame.
 *
 * Every /cp page reads `useConsole()` for who is looking at it, and half of
 * what those pages decide — whether the role dropdown is drawn at all, whose
 * row says "you" instead of offering a disable button — is decided from it.
 * The frame itself is `components/cp/Console`, which signs in against the API;
 * a page test that had to go through it would be testing the sign-in form for
 * the fifteenth time.
 *
 * `vi.mock("@/components/cp/Console", () => import("../helpers/console"))`, then
 * `signedInAs({ role: "auditor" })` before the render.
 */

import type { Operator, OperatorRole } from "@/lib/cp";

const DEFAULT: Operator = {
  id: "op-me",
  email: "me@example.test",
  name: "Мөнх Оператор",
  role: "superadmin",
};

let current: Operator = DEFAULT;

/** Who the next render is looking at. Anything unsaid stays the default. */
export function signedInAs(operator: Partial<Operator> & { role?: OperatorRole } = {}): Operator {
  current = { ...DEFAULT, ...operator };
  return current;
}

export function signedOut(): void {
  current = DEFAULT;
}

export const signOut = () => Promise.resolve();

export function useConsole() {
  return { operator: current, signOut };
}

/** The frame itself, so a page that renders inside it in production still can. */
export default function Console({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}
