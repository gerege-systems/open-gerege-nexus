import assert from "node:assert/strict";
import { test } from "vitest";

import { normaliseSlugInput, slugFromName } from "../lib/slug.mjs";

// Богино нэр нь URL, OAuth audience, токены `tenant_slug` claim-д ордог тул
// галиглал нь маягтын тав тух биш, тархдаг утга юм.
test("кирилл нэр латин богино нэр болно", () => {
  assert.equal(slugFromName("Герэгэ Системс ХХК"), "gerege-sistems");
  assert.equal(slugFromName("Өнөр-Үйлс ХХК"), "onor-uils");
  assert.equal(slugFromName("Монгол шуудан ХК"), "mongol-shuudan");
});

// «Систем» бол sistem; үгийн эхний `е` л `ye` болно.
test("е үгийн эхэнд ye, дотор нь e", () => {
  assert.equal(slugFromName("Ерөнхий сайд"), "yeronkhii-said");
  assert.equal(slugFromName("Системс"), "sistems");
});

// Хуулийн хэлбэр галиглахад `khkhk` болох тул URL-д ордоггүй.
test("хуулийн хэлбэр богино нэрэнд ордоггүй", () => {
  assert.equal(slugFromName("Gerege Systems LLC"), "gerege-systems");
  assert.equal(slugFromName("ХХК"), "");
});

test("латин нэр хэвээрээ", () => {
  assert.equal(slugFromName("Gerege Systems"), "gerege-systems");
  assert.equal(slugFromName("ISEE.mn мэдээ"), "isee-mn-medee");
});

// Хүн `gerege-nexus` бичиж байхад зураас нь тэмдэгт болгонд алга болвол дунд
// нь зураас оруулах аргагүй болно.
test("бичиж байхад төгсгөлийн зураас үлдэнэ, эхнийх нь хасагдана", () => {
  assert.equal(normaliseSlugInput("gerege-"), "gerege-");
  assert.equal(normaliseSlugInput("-gerege"), "gerege");
  assert.equal(normaliseSlugInput("gerege--nexus"), "gerege-nexus");
});

test("талбарт бичсэн кирилл, том үсэг, зай цэгцэрнэ", () => {
  assert.equal(normaliseSlugInput("ГЕРЭГЭ"), "gerege");
  assert.equal(normaliseSlugInput("Gerege Systems"), "gerege-systems");
});

// URL-д ордог утга тул уртад нь хязгаар бий (сервер 64 тэмдэгтээр татгалздаг).
test("64 тэмдэгтээс урт болохгүй", () => {
  assert.equal(normaliseSlugInput("a".repeat(200)).length, 64);
});
