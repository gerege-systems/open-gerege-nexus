// @vitest-environment node
//
// Two files describe who may do what. This one reads both and refuses to let
// them disagree.

import { readFileSync } from "node:fs";
import { join } from "node:path";

import { expect, test } from "vitest";

/**
 * The console hides buttons a role may not press, from a copy of the server's
 * capability table.
 *
 * The copy has to exist — the server answers 403 whatever the browser thinks,
 * and a console that offers an action the server will refuse is a console that
 * teaches its operators to expect failure. What the copy must not do is drift:
 * a capability moved between roles in Go and left alone here shows `support`
 * a Delete button that cannot work, or — the direction that matters — hides
 * from `operator` a button they are entitled to and nobody notices for months,
 * because a missing button looks exactly like a feature that does not exist.
 *
 * Both sides are read as text. The Go map is the authority; the TypeScript
 * copy names a subset of the capabilities, and every one it names has to agree.
 */

const root = join(import.meta.dirname, "..", "..");
const goSource = readFileSync(
  join(root, "..", "backend", "internal", "operator", "operator", "operator.go"),
  "utf8",
);
const consoleSource = readFileSync(
  join(root, "app", "cp", "tenants", "[id]", "page.tsx"),
  "utf8",
);

/** `CapTenantSuspend Capability = "tenant.suspend"` → the dotted verb. */
function goConstants(pattern: RegExp): Record<string, string> {
  const found: Record<string, string> = {};
  for (const [, name, value] of goSource.matchAll(pattern)) found[name] = value;
  return found;
}

function goCapabilities(): Record<string, Set<string>> {
  const caps = goConstants(/(Cap\w+)\s+Capability\s*=\s*"([^"]+)"/g);
  const roles = goConstants(/(Role\w+)\s+Role\s*=\s*"([^"]+)"/g);

  const table = goSource.slice(goSource.indexOf("var capabilities = map[Role]map[Capability]bool{"));
  const held: Record<string, Set<string>> = {};
  // Each role's block runs from `RoleX: {` to the closing brace of that block.
  for (const [, roleConst, body] of table.matchAll(/(Role\w+):\s*\{([\s\S]*?)\n\t\},/g)) {
    const role = roles[roleConst];
    expect(role, `${roleConst} is not one of the four roles`).toBeTruthy();
    held[role] = new Set(
      [...body.matchAll(/(Cap\w+):\s*true/g)].map(([, capConst]) => {
        expect(caps[capConst], `${capConst} has no string value in Go`).toBeTruthy();
        return caps[capConst];
      }),
    );
  }
  return held;
}

function consoleCapabilities(): Record<string, Set<string>> {
  const start = consoleSource.indexOf("const CAPABILITIES");
  expect(start, "the console no longer carries a capability table").not.toBe(-1);
  const block = consoleSource.slice(start, consoleSource.indexOf("};", start));
  const held: Record<string, Set<string>> = {};
  for (const [, role, body] of block.matchAll(/(\w+):\s*\[([^\]]*)\]/g)) {
    held[role] = new Set([...body.matchAll(/"([^"]+)"/g)].map(([, capability]) => capability));
  }
  return held;
}

test("the console's capability table says what the server's says", () => {
  const server = goCapabilities();
  const console_ = consoleCapabilities();

  expect(Object.keys(server).sort()).toEqual(["auditor", "operator", "superadmin", "support"]);
  // The console lists all four roles, even the one that holds nothing: an
  // absent role reads as "not written yet" and would let `auditor` fall
  // through to whatever a missing entry does.
  expect(Object.keys(console_).sort()).toEqual(Object.keys(server).sort());

  // Only the capabilities the console actually branches on. It does not draw a
  // button for `deploy.trigger`, and requiring it to name one would make this
  // test an argument about scope rather than about agreement.
  const named = new Set(Object.values(console_).flatMap((set) => [...set]));

  for (const role of Object.keys(server)) {
    for (const capability of named) {
      expect(
        console_[role].has(capability),
        `${role} × ${capability}: the console says ${console_[role].has(capability)}, ` +
          `the server says ${server[role].has(capability)}`,
      ).toBe(server[role].has(capability));
    }
  }
});

test("every capability the console names is one the server has heard of", () => {
  const known = new Set(Object.values(goCapabilities()).flatMap((set) => [...set]));
  const named = new Set(Object.values(consoleCapabilities()).flatMap((set) => [...set]));

  // A misspelt capability in the console is a button that never appears for
  // anybody, which is a silent failure rather than a visible one.
  for (const capability of named) {
    expect(known.has(capability), `${capability} is not a capability the server defines`).toBe(true);
  }
});
