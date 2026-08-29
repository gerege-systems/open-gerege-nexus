/**
 * The console's client, and the four answers it has to tell apart.
 *
 * Every screen in the console decides what to draw from the *type* of the
 * failure rather than from a status code compared at each call site — the
 * frame shows the sign-in form for `Unauthorized`, the action dialog asks for
 * the authenticator again for `StepUpRequired`, and everything else is a
 * message. Get the mapping wrong in one place and a step-up reads as an error
 * the operator cannot do anything about.
 */

import { afterEach, expect, test, vi } from "vitest";

import { cp, StepUpRequired, Unauthorized } from "@/lib/cp";

/** The API's answer, in the shape fetch returns it. */
function answers(status: number, body: string, ok = status < 400) {
  // Typed by its signature rather than by its implementation, so that a test
  // can read back the path and the init it was called with — which is half of
  // what this file checks.
  const fetch = vi.fn<(path: string, init?: RequestInit) => Promise<unknown>>(async () => ({
    status,
    ok,
    text: async () => body,
  }));
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

afterEach(() => vi.unstubAllGlobals());

test("a session that has gone is Unauthorized, whatever the body says", async () => {
  answers(401, JSON.stringify({ error: "session expired" }));

  await expect(cp.me()).rejects.toBeInstanceOf(Unauthorized);
});

test("a step-up is its own kind of refusal, not an error message", async () => {
  answers(403, JSON.stringify({ code: "step_up_required", error: "step up required" }));

  await expect(cp.setOperatorRole("op-1", "auditor", "why")).rejects.toBeInstanceOf(StepUpRequired);
});

test("what the API said is what the operator reads", async () => {
  answers(409, JSON.stringify({ error: "the last superadmin cannot be disabled" }));

  await expect(cp.setOperatorEnabled("op-1", false, "why")).rejects.toThrow(
    "the last superadmin cannot be disabled",
  );
});

test("a proxy's error page is reported as its status, not as its HTML", async () => {
  answers(502, "<html><head><title>502 Bad Gateway</title></head><body>nginx</body></html>");

  // A response that is not JSON did not come from the API. Putting its body on
  // the screen is how a console ends up rendering somebody else's markup at an
  // operator, and it says nothing they can act on either way.
  await expect(cp.operators()).rejects.toThrow("request failed with 502");
});

test("an empty success is an empty object, not a parse error", async () => {
  answers(204, "");

  await expect(cp.signOut()).resolves.toEqual({});
});

test("the session cookie is sent, which fetch does not do by itself", async () => {
  const fetch = answers(200, JSON.stringify({ operators: [] }));

  await cp.operators();

  const [path, init] = fetch.mock.calls[0];
  expect(path).toBe("/api/platform/v1/operators");
  // Same-origin is not enough: a custom method with no credentials sends no
  // cookie, and every call here is a session call.
  expect(init?.credentials).toBe("include");
  expect((init?.headers as Record<string, string>)["Content-Type"]).toBe("application/json");
});

test("an audit search leaves out the fields nobody filled in", async () => {
  const fetch = answers(200, JSON.stringify({ entries: [] }));

  await cp.audit({ action: "operator.add", target_type: "", target_id: undefined });

  expect(fetch.mock.calls[0][0]).toBe("/api/platform/v1/audit?action=operator.add");
});

test("an identifier with a slash in it cannot walk out of its path", async () => {
  const fetch = answers(200, JSON.stringify({ status: "ok" }));

  await cp.confirmEnrolment("../../session", "123456", "why");

  expect(fetch.mock.calls[0][0]).toBe("/api/platform/v1/operators/..%2F..%2Fsession/enrolment");
});
