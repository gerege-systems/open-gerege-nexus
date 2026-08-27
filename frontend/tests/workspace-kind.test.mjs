import assert from "node:assert/strict";
import test from "node:test";

import { isHome, organisationScreensVisible } from "../lib/workspaceKind.mjs";

test("a home has no app store and no legal identity", () => {
  assert.equal(organisationScreensVisible("personal"), false);
});

test("an organisation keeps every screen it had", () => {
  assert.equal(organisationScreensVisible("organisation"), true);
});

// The value the shell holds before /api/v1/me answers, and the value a
// deployment sends from a version of the API that predates the column.
test("an unanswered kind shows nothing rather than guessing", () => {
  for (const unknown of [undefined, null, ""]) {
    assert.equal(organisationScreensVisible(unknown), false, String(unknown));
  }
});

// A kind nobody has heard of is somebody else's future, not this build's. It is
// not a home, so it is not hidden — the alternative is a deployment upgrading
// its API and watching every organisation's rail lose two links.
test("an unknown kind is treated as an organisation", () => {
  assert.equal(organisationScreensVisible("federation"), true);
});

test("the switcher can tell the home apart", () => {
  assert.equal(isHome({ kind: "personal" }), true);
  assert.equal(isHome({ kind: "organisation" }), false);
  assert.equal(isHome(undefined), false);
});
