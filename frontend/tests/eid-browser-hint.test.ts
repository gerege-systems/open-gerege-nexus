// @vitest-environment node
//
// The App2App return hint: which browser the citizen started in.
//
// The eID app opens the callback in the system default browser unless the URL
// says otherwise, so a citizen who signed in from Chrome lands in Safari and
// loses the tab they came from. `browserHint` is the half of that fix which
// lives here: it turns a user agent into the parameter the app reads
// (`retScheme` on iOS, `retPkg` on Android). The app only honours values it
// knows, so a typo here is a silent fall back to the default browser — which
// is exactly the bug — hence the table below.

import { expect, test } from "vitest";

import { browserHint } from "@/components/EIDLogin";

const ios = (token: string) =>
  `Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 ${token}`;
const android = (token: string) =>
  `Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 ${token}`;

const cases: Array<[string, string]> = [
  [ios("CriOS/120"), "?retScheme=googlechromes"],
  [ios("EdgiOS/120"), "?retScheme=microsoft-edge-https"],
  [ios("FxiOS/120"), "?retScheme=firefox"],
  [ios("Focus/120"), "?retScheme=firefox-focus"],
  [ios("OPT/4.0"), "?retScheme=touch-https"],
  [ios("OPiOS/16"), "?retScheme=opera-https"],
  [ios("Mobile DuckDuckGo/7"), "?retScheme=ddgQuickLink"],
  [ios("YaBrowser/24.1"), "?retScheme=yandexbrowser-open-url"],
  [android("Chrome/120 Mobile"), "?retPkg=com.android.chrome"],
  [android("Chrome/120 Mobile EdgA/120"), "?retPkg=com.microsoft.emmx"],
  [android("Chrome/120 Mobile OPR/80"), "?retPkg=com.opera.browser"],
  [android("SamsungBrowser/23 Chrome/115"), "?retPkg=com.sec.android.app.sbrowser"],
  [android("YaBrowser/24.1 Chrome/120"), "?retPkg=com.yandex.browser"],
  [android("Firefox/120"), "?retPkg=org.mozilla.firefox"],
  [android("DuckDuckGo/5 Chrome/120"), "?retPkg=com.duckduckgo.mobile.android"],
];

test.each(cases)("%s → %s", (ua, expected) => {
  expect(browserHint(ua)).toBe(expected);
});

// A desktop browser never gets here (the callback is same-device only), and a
// phone browser nobody has a scheme for — Brave, whose user agent is a copy of
// Safari's — must not be guessed at: no hint means the app opens the default
// browser, which is the old behaviour and still correct.
test("no hint for a desktop or unrecognised browser", () => {
  expect(browserHint("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/120")).toBe("");
  expect(browserHint(ios("Version/17.0 Mobile/15E148 Safari/604.1"))).toBe("");
});
