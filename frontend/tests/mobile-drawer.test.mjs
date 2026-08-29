// @vitest-environment node
//
// This file reads a stylesheet as text; it needs no document. Saying so keeps
// Node's own `URL` in place — jsdom replaces it with the browser's, and
// `readFileSync(new URL(...))` refuses one of those.
import assert from "node:assert/strict";
import { test } from "vitest";
import { readFileSync } from "node:fs";

// The mobile drawer opens, asserted as a number rather than as a screenshot.
//
// The menu on a phone is one CSS rule: a fixed panel parked at translateX(-100%)
// that comes back to 0 when React puts `is-mobile-open` on it. On 2026-08-10 the
// native shell's workarea exclusions were added to the parked rule and not to
// the open one, which quietly made the parked rule the more specific of the two
// — `:not()` carries the specificity of its argument. The class went on the
// element, the transform did not, and the drawer never moved.
//
// Nothing caught it for a fortnight. There is no test that opens a page in a
// phone-sized browser, and by eye it reads as "this app has no menu" rather
// than as a control that does not work. What follows is the property itself:
// whatever opens the drawer must outweigh whatever parks it.
const css = readFileSync(new URL("../app/globals.css", import.meta.url), "utf8");

// Every selector in this rule is classes and :not(), so the number of dots is
// the specificity — including the dots inside :not(), which is exactly the rule
// that was got wrong. A general specificity parser would be a different program
// and would not be checking anything this file needs.
const weigh = (selectorList) =>
  Math.max(...selectorList.split(",").map((s) => (s.match(/\./g) || []).length));

const ruleFor = (declaration) => {
  const at = css.indexOf(declaration);
  assert.notEqual(at, -1, `no rule declares ${declaration}`);
  const open = css.lastIndexOf("{", at);
  const start = css.lastIndexOf("}", open) + 1;
  // Comments live between the previous rule and this one; the selector list is
  // what is left after they are taken out.
  const selectors = css.slice(start, open).replace(/\/\*[\s\S]*?\*\//g, "").trim();
  return { selectors, at };
};

test("the mobile drawer's open state outweighs its parked state", () => {
  const parked = ruleFor("transform: translateX(-100%)");
  const opened = ruleFor("transform: translateX(0)");

  assert.ok(
    weigh(opened.selectors) > weigh(parked.selectors),
    `the rule that opens the drawer weighs ${weigh(opened.selectors)} and the one that ` +
      `parks it weighs ${weigh(parked.selectors)}, so the drawer stays off screen ` +
      `with its class applied.\n\nparked: ${parked.selectors.trim()}\nopened: ${opened.selectors.trim()}`,
  );
});

test("the drawer's open state is still scoped away from the native shell", () => {
  // The workarea is the native client's own chrome: there the panel is part of
  // the layout and must never be a fixed overlay. Winning the specificity
  // contest by dropping that exclusion would fix the phone and break the app.
  const opened = ruleFor("transform: translateX(0)");
  for (const selector of opened.selectors.split(",")) {
    assert.match(selector, /:not\(\.gerege-workarea \.gerege-sidebar\)/, selector.trim());
  }
});
