// @vitest-environment node
//
// The link a phone follows into the eID app. eID 2.2.2 no longer registers
// `geregesmartid://`: the official App2App link is `{deviceLinkBase}?sessionId=…&vc=…`
// from the RP-API device-link answer, and `eidmongolia://approve` when no base
// came with the session.

import { expect, test } from "vitest";

import { appLink } from "@/components/EIDLogin";

test("uses the https device link when eID gave a base", () => {
  expect(appLink({ session_id: "s 1", verification_code: "48213", device_link_base: "https://ca.eidmongolia.mn/dl" }))
    .toBe("https://ca.eidmongolia.mn/dl?sessionId=s+1&vc=48213");
});

test("falls back to the eidmongolia scheme without a base", () => {
  expect(appLink({ session_id: "s-1", verification_code: "48213" }))
    .toBe("eidmongolia://approve?sessionId=s-1&vc=48213");
});

test("leaves vc out when there is none", () => {
  expect(appLink({ session_id: "s-1", verification_code: "" })).toBe("eidmongolia://approve?sessionId=s-1");
});

test("never navigates to a base that is not a plain https URL", () => {
  for (const base of ["javascript:alert(1)", "http://ca.eidmongolia.mn/dl", "https://ca.eidmongolia.mn/dl?x=1"]) {
    expect(appLink({ session_id: "s-1", verification_code: "1", device_link_base: base }))
      .toBe("eidmongolia://approve?sessionId=s-1&vc=1");
  }
});
