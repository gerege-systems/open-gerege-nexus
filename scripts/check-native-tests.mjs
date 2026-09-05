#!/usr/bin/env node
/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

/**
 * A native client that has tests has to run them.
 *
 *   node scripts/check-native-tests.mjs
 *
 * The Windows client carried five test files — an SPKI pin validator, an HMAC
 * signer, a sensitive-action guard, a certificate service, an esign settings
 * reader — from the day it was written until 2026-09-05, and not one of them
 * ever ran. `native-clients.yml` called `dotnet build` and stopped. The suites
 * were in the solution, so they compiled; a green tick meant "this compiles",
 * and nobody had reason to read it as anything else.
 *
 * That is the failure this file exists to make impossible to repeat. `backend`
 * has `planes_test.go` for its architecture and a dozen migration tests for its
 * schema; the native tree, now a third of the repository, had nothing that
 * failed when a rule was broken.
 *
 * Two rules:
 *
 *   1. A client with test files must have a test step in the workflow. This is
 *      the Windows failure exactly, and it is the one worth automating: the
 *      tests exist, somebody wrote them, and only the pipeline forgot.
 *   2. A client with no test files at all must be named below, with the reason.
 *      Zero tests is a position a team may hold; holding it silently is not.
 *
 * Exits non-zero when either rule is broken.
 */
import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const WORKFLOW = ".github/workflows/native-clients.yml";

/** The clients this repository builds, and how a test file looks in each. */
const clients = [
  { dir: "native-apps/desktop/gerege-token-kit", label: "GeregeTokenKit (SPM)", test: /Tests?\.swift$/ },
  { dir: "native-apps/desktop/macos", label: "macOS", test: /Tests?\.swift$/ },
  { dir: "native-apps/desktop/windows", label: "Windows", test: /Tests?\.cs$/ },
  { dir: "native-apps/mobile/ios", label: "iOS", test: /Tests?\.swift$/ },
  { dir: "native-apps/mobile/android", label: "Android", test: /\.kt$/, only: "src/test/" },
];

/**
 * Clients that ship no tests, and why that is where they are today.
 *
 * An entry is a debt written down, not a permission. Removing one means the
 * client grew a test; adding one means somebody decided it would not, and said
 * so in a sentence the next reader can argue with.
 */
const clientsWithoutTests = {
  "native-apps/desktop/macos":
    "Логик нь GeregeTokenKit (тесттэй) ба APIClient/PinnedSessionDelegate хооронд хуваагдсан; " +
    "XCTest target хараахан project.yml-д байхгүй. Хамгийн эхний нэр дэвшигч нь SPKIHash — " +
    "Windows тал яг түүнийг тесттэй (SpkiPinValidatorTests).",
  "native-apps/mobile/ios":
    "Мак-тай эх кодоо хуваалцдаг тул логикийн ихэнх нь тэнд шалгагдана; " +
    "өөрийн гэсэн хэсэг нь бүхэлдээ SwiftUI дэлгэц. macOS-д XCTest орсны дараа хамт үзнэ.",
};

const walk = async (dir) => {
  const out = [];
  let entries;
  try {
    entries = await readdir(path.join(ROOT, dir), { withFileTypes: true });
  } catch {
    return out;
  }
  for (const e of entries) {
    if (e.name === "build" || e.name === "obj" || e.name === "bin" || e.name === ".build") continue;
    const child = path.join(dir, e.name);
    if (e.isDirectory()) out.push(...(await walk(child)));
    else out.push(child);
  }
  return out;
};

const TEST_COMMAND = /\b(swift\s+test|dotnet\s+test|gradlew[^\n]*\btest\b|xcodebuild[^\n]*\btest\b)/;

/** Working directories the workflow runs a test command in. */
async function testedDirectories() {
  const text = await readFile(path.join(ROOT, WORKFLOW), "utf8");
  const lines = text.split("\n");
  const tested = new Set();
  lines.forEach((line, i) => {
    if (!/^\s*-\s*(name:|run:|uses:)/.test(line) && !/^\s*run:/.test(line)) return;
    if (!TEST_COMMAND.test(line)) return;
    // The step's own working-directory, which in this file follows the command.
    for (let j = i + 1; j < lines.length && !/^\s*-\s/.test(lines[j]); j++) {
      const wd = lines[j].match(/^\s*working-directory:\s*(\S+)/);
      if (wd) {
        tested.add(wd[1].replace(/\/$/, ""));
        return;
      }
    }
  });
  return tested;
}

const tested = await testedDirectories();
const problems = [];

for (const client of clients) {
  const files = (await walk(client.dir)).filter(
    (f) => client.test.test(f) && (!client.only || f.includes(client.only)),
  );
  const runs = tested.has(client.dir);

  if (files.length > 0 && !runs) {
    problems.push(
      `${client.label} (${client.dir}) нь ${files.length} тест файлтай атлаа ` +
        `${WORKFLOW} дотор тестийн алхамгүй — тэдгээр нь компайл хийгдээд хэзээ ч гүйхгүй.`,
    );
  }
  if (files.length === 0 && !(client.dir in clientsWithoutTests)) {
    problems.push(
      `${client.label} (${client.dir}) нь нэг ч тест файлгүй бөгөөд ` +
        `scripts/check-native-tests.mjs-ийн clientsWithoutTests-д шалтгаангүй байна.`,
    );
  }
  if (files.length > 0 && client.dir in clientsWithoutTests) {
    problems.push(
      `${client.label} (${client.dir}) нь одоо ${files.length} тест файлтай — ` +
        `clientsWithoutTests-ээс хасаж, тестийн алхмыг нь баталгаажуул.`,
    );
  }
}

if (problems.length) {
  for (const p of problems) console.error(`::error file=${WORKFLOW}::${p}`);
  process.exit(1);
}

console.log(
  `${clients.length} native клиент шалгав: тесттэй нь бүгд гүйдэг, тестгүй нь бүгд нэрлэгдсэн.`,
);
