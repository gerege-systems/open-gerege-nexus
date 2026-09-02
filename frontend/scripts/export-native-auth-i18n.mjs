import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";

import { loadModule } from "./i18n-layout.mjs";

const root = process.cwd();
const locales = ["mn", "en", "ar", "zh", "fr", "ru", "es"];
const out = path.resolve(root, "../native-apps/generated-i18n");
await mkdir(out, { recursive: true });
const androidRoot = path.resolve(root, "../native-apps/mobile/android/app/src/main/res");
const windowsRoot = path.resolve(root, "../native-apps/desktop/windows/Resources");
const iosCatalog = path.resolve(root, "../native-apps/mobile/ios/Sources/GeregeShellUI/Resources/Login.xcstrings");
const xml = value => String(value).replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "\\'");
const resourceKey = key => key.replaceAll(".", "_").replace(/[^a-zA-Z0-9_]/g, "_");
const catalog = { sourceLanguage: "mn", strings: {}, version: "1.0" };

// Read as data rather than transpiled, the same as every other script here.
//
// It used to go through ts.transpileModule, which stopped existing: TypeScript 7
// rewrote the compiler and the root namespace no longer carries ModuleKind or
// transpileModule, so this script had been failing with
// `Cannot read properties of undefined (reading 'ESNext')`. Nothing noticed,
// because it is not in CI — it is run by hand when the native shells need new
// strings. The dictionaries are `export const x = {...}` where every value is a
// literal, so the compiler was never needed.
const auth = await loadModule(path.resolve(root, "lib/i18n/addons/auth.ts"));
const overlays = {};
for (const locale of locales.slice(2)) {
  // core.ts, not the directory: auth is one of the platform's own dictionaries,
  // and the files beside it belong to apps this export does not carry.
  overlays[locale] = await loadModule(path.resolve(root, `lib/i18n/locales/${locale}/core.ts`));
}

/**
 * The web dictionary carries the product's name as `{brand}`, substituted at
 * render time from the deployment's environment. Nothing on the other side of
 * this script does that: an Android string resource, a .resx and an .xcstrings
 * are read by a bundle that was signed with one name on it already. So the name
 * is resolved here, at export, and a shell built for another brand exports its
 * own strings with BRAND_NAME set — the same variable the server reads.
 */
const brand = (process.env.BRAND_NAME || "").trim() || "Gerege Nexus";
const resolve = value => String(value).replaceAll("{brand}", brand);

for (const locale of locales) {
  const strings = Object.fromEntries(Object.entries(auth).map(([key, translations]) => [key, resolve(overlays[locale]?.[key] ?? translations[locale] ?? translations.en)]));
  await writeFile(path.join(out, `${locale}.json`), `${JSON.stringify(strings, null, 2)}\n`);
  const qualifier = locale === "mn" ? "values" : `values-${locale}`;
  await mkdir(path.join(androidRoot, qualifier), { recursive: true });
  await writeFile(path.join(androidRoot, qualifier, "auth.xml"), `<?xml version="1.0" encoding="utf-8"?>\n<resources>\n${Object.entries(strings).map(([key,value])=>`    <string name="${resourceKey(key)}">${xml(value)}</string>`).join("\n")}\n</resources>\n`);
  await mkdir(windowsRoot, { recursive: true });
  const suffix = locale === "mn" ? "" : `.${locale}`;
  await writeFile(path.join(windowsRoot, `Login${suffix}.resx`), `<?xml version="1.0" encoding="utf-8"?>\n<root>\n  <resheader name="resmimetype"><value>text/microsoft-resx</value></resheader>\n  <resheader name="version"><value>2.0</value></resheader>\n${Object.entries(strings).map(([key,value])=>`  <data name="${resourceKey(key)}" xml:space="preserve"><value>${xml(value)}</value></data>`).join("\n")}\n</root>\n`);
  for (const [key,value] of Object.entries(strings)) {
    catalog.strings[key] ??= { localizations: {} };
    catalog.strings[key].localizations[locale] = { stringUnit: { state: "translated", value } };
  }
}
await mkdir(path.dirname(iosCatalog), { recursive: true });
await writeFile(iosCatalog, `${JSON.stringify(catalog, null, 2)}\n`);
console.log(`Exported native auth strings to ${out}`);
