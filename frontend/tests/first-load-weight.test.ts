// @vitest-environment node
//
// What the landing page is allowed to download before it can paint.
//
// Measured on nexus.gerege.mn: the signed-out page pulled 441 KB (gzipped) of
// JavaScript, of which three things were read by almost nobody who paid for
// them —
//
//   the five optional languages   121 KB  off unless a device switches one on
//   the Sentry SDK                 77 KB  no deploy has ever set a DSN
//   the QR code drawer             15 KB  drawn after choosing phone sign-in
//
// — and all three were static imports, which is the only reason they were in
// the first chunk. They are fetched on demand now, and the page is 292 KB.
//
// This is a source test rather than a bundle test on purpose. A bundle test
// needs a production build, which CI does elsewhere and slowly; the mistake
// worth catching is one line, it is always the same line, and it reads as
// completely ordinary in review: `import { X } from "..."` at the top of a file
// that the landing page reaches. Nothing breaks when it comes back. The page
// just gets slow again, on the connections least able to afford it.

import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";

import { expect, test } from "vitest";

const FRONTEND = path.join(__dirname, "..");
const I18N = path.join(FRONTEND, "lib", "i18n");

/** `import ... from "<module>"` — the eager form. `import("<module>")` is fine. */
function importsStatically(file: string, pattern: string): boolean {
  const source = readFileSync(file, "utf8");
  return new RegExp(`^import[^;]*from\\s*['"][^'"]*${pattern}`, "m").test(source);
}

test("no module puts an optional language in the first chunk", () => {
  const reachable = [
    ...readdirSync(path.join(I18N, "apps")).map((f) => path.join(I18N, "apps", f)),
    path.join(I18N, "index.tsx"),
    path.join(I18N, "locales", "index.ts"),
  ].filter((f) => f.endsWith(".ts") || f.endsWith(".tsx"));

  const offenders = reachable.flatMap((file) =>
    ["ar", "zh", "fr", "ru", "es"]
      .filter((locale) => importsStatically(file, `locales/${locale}/`))
      .map((locale) => `${path.relative(FRONTEND, file)} imports locales/${locale}`),
  );

  expect(offenders).toEqual([]);
});

test("the Sentry SDK is loaded only where a DSN asks for it", () => {
  const file = path.join(FRONTEND, "instrumentation-client.ts");
  // `import type` is erased at build and costs nothing; a value import is the
  // whole SDK.
  const source = readFileSync(file, "utf8");
  expect(/^import\s+(?!type\b)[^;]*from\s*['"]@sentry\/nextjs['"]/m.test(source)).toBe(false);
  expect(source).toContain("import('@sentry/nextjs')");
});

test("the QR code drawer is fetched when a QR code is drawn", () => {
  expect(importsStatically(path.join(FRONTEND, "components", "EIDLogin.tsx"), "qrcode.react")).toBe(
    false,
  );
});
