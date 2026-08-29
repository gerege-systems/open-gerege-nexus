import assert from "node:assert/strict";
import { test } from "vitest";

import { homeScreensVisible, isHome, organisationScreensVisible, NO_WORKSPACE } from "../lib/workspaceKind.mjs";

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

// The two questions are asked separately, and both answer "no" when the kind is
// not known. Written as its own test because the tempting shape — one predicate
// and its negation — is wrong in exactly one case, and it is the case that
// happens on every page load: for the moment before /api/v1/me answers, a
// negated predicate says "this is a home" about every workspace there is.
test("an unanswered kind is neither a company nor a home", () => {
  for (const unknown of [undefined, null, ""]) {
    assert.equal(organisationScreensVisible(unknown), false, String(unknown));
    assert.equal(homeScreensVisible(unknown), false, String(unknown));
  }
});

test("a home has its own screen and a company does not", () => {
  assert.equal(homeScreensVisible("personal"), true);
  assert.equal(homeScreensVisible("organisation"), false);
  // An organisation member asks through the organisation, so the screen would
  // be permanently empty for them — see the rail entry in Layout.tsx.
  assert.equal(homeScreensVisible("federation"), false);
});

test("a session in no workspace shows the person's screens and not a company's", () => {
  assert.equal(homeScreensVisible(NO_WORKSPACE), true);
  assert.equal(organisationScreensVisible(NO_WORKSPACE), false);
});

test("the three states stay three: loading is not the same as no workspace", () => {
  // The failure this catches is the cheap version of NO_WORKSPACE — reporting
  // an empty string — which makes "still loading" and "no organisation"
  // indistinguishable, and draws a citizen's rail at everybody for the moment
  // before /api/v1/me returns.
  assert.equal(homeScreensVisible(undefined), false);
  assert.equal(homeScreensVisible(""), false);
  assert.equal(homeScreensVisible(NO_WORKSPACE), true);
});

test("a deployment that has not run 00094 yet still shows a citizen their screens", () => {
  // Personal workspaces still exist there, and are still reported. Answering
  // "no" would hide the personal side from every citizen on that deployment
  // until the migration landed.
  assert.equal(homeScreensVisible("personal"), true);
  assert.equal(organisationScreensVisible("personal"), false);
});
