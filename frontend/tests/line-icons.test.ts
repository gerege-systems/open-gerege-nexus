// @vitest-environment node
//
// Шугамын үйлдэл бүр дүрстэй байх ёстой.
//
// `ICONS` толинд байхгүй нэр нь ЮУ Ч унагадаггүй: React `undefined` рендерлээд
// өнгөрдөг тул дэлгэц дээр хоосон саарал дөрвөлжин үлдэнэ. Яг тэр байдлаар
// `key`, `link`, `settings`, `shield-check`, `monitor-cog` тав нь дөрвөн
// шугам дээр хоосон хайрцаг болж production дээр сууж байсныг хэрэглэгч
// скриншотоор илрүүлэв (2026-09-04).
//
// Эх кодыг нь уншиж тулгаж байгаа нь санаатай: `ICONS` нь хуудасны дотоод
// тогтмол бөгөөд түүнийг зөвхөн энэ шалгалтын төлөө экспортлох нь хуудсыг
// тестэд зориулж өөрчилсөн хэрэг болно.
import { readFileSync } from "node:fs";
import path from "node:path";

import { expect, test } from "vitest";

const LINE_DIR = path.join(__dirname, "..", "app", "line", "[line]");

function iconNamesInUse(): string[] {
  const source = readFileSync(path.join(LINE_DIR, "lines.ts"), "utf8");
  return [...source.matchAll(/icon:\s*"([^"]+)"/g)].map((m) => m[1]);
}

function iconNamesDeclared(): string[] {
  const source = readFileSync(path.join(LINE_DIR, "page.tsx"), "utf8");
  const map = source.slice(source.indexOf("const ICONS"), source.indexOf("};", source.indexOf("const ICONS")));
  // `grid: <LayoutGrid />` ба `"shield-check": <ShieldCheck />` хоёр хэлбэр.
  return [...map.matchAll(/(?:^|[{,]\s*)"?([a-z][a-z0-9-]*)"?\s*:/gm)].map((m) => m[1]);
}

test("шугамын үйлдэл бүрийн дүрс ICONS толинд байна", () => {
  const declared = new Set(iconNamesDeclared());
  const used = iconNamesInUse();

  expect(used.length).toBeGreaterThan(0);
  expect(used.filter((name) => !declared.has(name))).toEqual([]);
});
