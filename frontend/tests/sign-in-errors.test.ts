// @vitest-environment node
//
// Both sides of the sign-in failure vocabulary, read as text and compared.

import { readFileSync } from "node:fs";
import { join } from "node:path";

import { expect, test } from "vitest";

/**
 * Every reason the server can send back to the sign-in screen.
 *
 * A failed sign-in comes back as `/login?sso_error=<reason>`, and the screen
 * turns that word into a sentence. A reason with no entry falls to the generic
 * message, which is the right default and a poor place to end up by accident:
 * `binding_failed` spent its life as `no_account`, so somebody three clicks
 * from opening an account was told to ask their administrator for one.
 *
 * So the vocabulary is asserted rather than assumed. A new reason on the Go
 * side has to be either given a sentence or listed below as deliberately
 * generic — which is a decision, made once, in the open.
 */

const root = join(import.meta.dirname, "..");
const goSources = ["google.go", "sso.go"].map((name) =>
  readFileSync(join(root, "..", "backend", "internal", "workspace", "identity", name), "utf8"),
);
const loginScreen = readFileSync(join(root, "app", "login", "page.tsx"), "utf8");

/** `h.failGoogle(w, r, "stale_request")` → the reason. */
function reasonsFromGo(): Set<string> {
  const found = new Set<string>();
  for (const source of goSources) {
    for (const [, reason] of source.matchAll(/fail(?:Google|SSO)\(w, r, "([a-z_]+)"\)/g)) {
      found.add(reason);
    }
  }
  return found;
}

function mappedOnTheScreen(): Set<string> {
  const line = loginScreen.slice(loginScreen.indexOf("const SSO_ERRORS"));
  const block = line.slice(0, line.indexOf("};"));
  return new Set([...block.matchAll(/(\w+):"auth\.sso\./g)].map(([, reason]) => reason));
}

/**
 * Reasons that are meant to read as "we could not finish it; try again".
 *
 * Each is a fault on this side with nothing the person can do differently, and
 * four sentences saying so in four ways would be four translations of one
 * fact. Named here so that the choice is visible and a fifth cannot be added
 * by omission.
 */
const DELIBERATELY_GENERIC = new Set([
  "no_code",
  "exchange_failed",
  "provisioning_failed",
  "session_failed",
]);

test("every reason the server sends is either said or deliberately generic", () => {
  const fromServer = reasonsFromGo();
  const said = mappedOnTheScreen();

  // The parsing is the part that rots silently, so it is asserted too.
  expect(fromServer.size).toBeGreaterThan(5);
  expect(said.size).toBeGreaterThan(5);

  for (const reason of fromServer) {
    expect(
      said.has(reason) || DELIBERATELY_GENERIC.has(reason),
      `${reason} reaches /login with no sentence and no decision to leave it generic`,
    ).toBe(true);
  }
});

test("the screen says nothing the server cannot send", () => {
  const fromServer = reasonsFromGo();
  // access_denied is the provider's own word, passed through when Google
  // refuses; it is the one reason that does not originate here.
  const fromProvider = new Set(["access_denied"]);

  for (const reason of mappedOnTheScreen()) {
    expect(
      fromServer.has(reason) || fromProvider.has(reason),
      `${reason} is a message for a failure nothing can produce any more`,
    ).toBe(true);
  }
});

test("a first sign-in that could not be started is not reported as having no account", () => {
  // The two are opposite advice: one says an administrator must act, the other
  // says try again. Google's binding path used to send the first.
  expect(loginScreen).toContain('binding_failed:"auth.sso.error_binding"');
  const google = readFileSync(
    join(root, "..", "backend", "internal", "workspace", "identity", "google.go"),
    "utf8",
  );
  expect(google).toContain('h.failGoogle(w, r, "binding_failed")');
  expect(google).not.toContain('h.failGoogle(w, r, "no_account")');
});
