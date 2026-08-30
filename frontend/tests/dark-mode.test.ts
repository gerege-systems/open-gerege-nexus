// @vitest-environment node
//
// Хоёр дүрэм, хоёулаа нэг өдрийн есөн алдаанаас гарсан.

import { readdirSync, readFileSync, statSync } from "node:fs";
import { extname, join, relative } from "node:path";

import { expect, test } from "vitest";

const root = join(import.meta.dirname, "..");

function sources(dir: string): string[] {
  const found: string[] = [];
  for (const entry of readdirSync(dir)) {
    if (entry === "node_modules" || entry === ".next" || entry === "tests") continue;
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) found.push(...sources(path));
    else if (extname(path) === ".tsx") found.push(path);
  }
  return found;
}

const files = [...sources(join(root, "app")), ...sources(join(root, "components"))];

/**
 * Үндсэн товч өөрийн бичгийн өнгийг accent-аас авна.
 *
 * `--gerege-blue` бол accent бөгөөд харанхуй горимын `neutral` — АНХДАГЧ accent —
 * түүнийг цайвар саарал болгодог. `text-white`-ыг шууд бичсэн товч тэнд цагаан
 * дээр цагаан болж уншигдахаа больдог: «Хадгалах», «Шинэ flag», «Зарлал
 * нийтлэх» гурав нэг өдөр ийм байдлаар мэдээлэгдсэн.
 *
 * `--gerege-on-blue` нь тэр асуултын хариу бөгөөд энэ тест түүнийг тойрч
 * гарахыг барина.
 */
test("үндсэн товч бичгийнхээ өнгийг accent-аас авна", () => {
  const offenders: string[] = [];
  for (const file of files) {
    const source = readFileSync(file, "utf8");
    for (const match of source.matchAll(/className=[{"`]([^"`]+)/g)) {
      const classes = match[1];
      if (!classes.includes("bg-[var(--gerege-blue)]")) continue;
      if (/(?<![\w:-])text-white(?![\w-])/.test(classes)) {
        offenders.push(`${relative(root, file)}: ${classes.trim().slice(0, 70)}`);
      }
    }
  }
  expect(offenders, "text-white-ийн оронд text-[var(--gerege-on-blue)]").toEqual([]);
});

/**
 * Харанхуй гадаргуу дээрх гарчиг өнгөө өөрөө хэлнэ.
 *
 * `globals.css`-ийн суурь давхарга `h1..h4 { color: var(--gerege-fg) }` гэж
 * бичдэг ба элементийн сонголт өвлөлтөөс хүчтэй. Тиймээс `text-white` авч яваа
 * харанхуй хэсгийн дотор буй гарчиг тэр өнгийг ӨВЛӨХГҮЙ: гэрэл горимд
 * --gerege-fg нь бараг хар, харанхуй дэвсгэр дээр бичигдээд алга болно.
 *
 * Ингэж AI туслахын толгойн гарчиг, төхөөрөмжийн «Enrollment code», киоскийн
 * хоёр гарчиг алга болсон.
 */
test("text-white авч яваа хэсгийн гарчиг өнгөө өөрөө хэлнэ", () => {
  // `globals.css`-ийн суурь давхарга `h1..h4 { color: var(--gerege-fg) }` гэж
  // бичдэг ба элементийн сонголт өвлөлтөөс хүчтэй. Тиймээс `text-white` авч
  // яваа ХАРАНХУЙ хэсгийн дотор буй гарчиг тэр өнгийг ӨВЛӨХГҮЙ — өөрийн
  // өнгөгүй бол --gerege-fg-ээр буддаг: AI туслахын толгойн гарчиг харанхуй
  // дээр бараан болж алга болсон, төхөөрөмжийн «Enrollment code» ба киоскийн
  // хоёр гарчиг гэрэл горимд ижил замаар алга болсон.
  //
  // Эцэг нь өнгөө зарласан газраас 400 тэмдэгтийн дотор — энэ кодын сангийн
  // толгойнууд нэг мөрөнд бичигддэг — өнгөгүй гарчиг байвал тэмдэглэнэ.
  const statesColour = /text-(white|slate-[1-3]00|cyan-[1-3]00|indigo-[1-3]00|gray-[1-3]00|\[var\()/;
  const offenders: string[] = [];

  for (const file of files) {
    const source = readFileSync(file, "utf8");
    for (const parent of source.matchAll(/className=[{"`][^"`]*(?<![\w:-])text-white(?![\w-])[^"`]*/g)) {
      const window = source.slice(parent.index, parent.index + 400);
      for (const heading of window.matchAll(/<(h[1-6])\s+className="([^"]*)"/g)) {
        if (statesColour.test(heading[2])) continue;
        offenders.push(`${relative(root, file)}: <${heading[1]} class="${heading[2].slice(0, 40)}">`);
      }
    }
  }
  expect(offenders, "гарчигт text-white нэм — эцгийнхээ өнгийг өвлөхгүй").toEqual([]);
});
