/**
 * The five optional languages must be fetched, not shipped.
 *
 * They are off unless a device switches one on (Settings → Appearance), and yet
 * every page — the signed-out landing page included — used to carry all of
 * them: five core overlays imported by lib/i18n/locales/index.ts and five more
 * files per app imported by lib/i18n/apps/*.ts. Measured on the deployment,
 * that was one 434 KB chunk (121 KB over the wire) of Arabic, Chinese, French,
 * Russian and Spanish in front of the first paint, for a reader who had asked
 * for none of it.
 *
 * Two assertions, because the bug can come back in two different ways.
 *
 *   1. The words still arrive. A lazy translation that never loads is a worse
 *      bug than a slow page, and it fails silently — t() falls back to English.
 *   2. Nothing imports them eagerly again — that half is in
 *      tests/first-load-weight.test.ts, with the same guard for the other two
 *      things the landing page used to download and not read.
 */

import { expect, test } from "vitest";
import { render, screen, act, waitFor } from "@testing-library/react";

import { I18nProvider, useI18n } from "@/lib/i18n";

function Probe({ locale, translationKey }: { locale: string; translationKey: string }) {
  const { t, setLocale, locale: current } = useI18n();
  // Rendered as a button rather than driven from outside, because setLocale is
  // only reachable through the context this is testing.
  return (
    <button type="button" onClick={() => setLocale(locale as never)}>
      {current}:{t(translationKey)}
    </button>
  );
}

test("an optional language's words arrive after it is chosen", async () => {
  // Any translated key will do, and picking one from the overlay itself keeps
  // this test from failing the day somebody renames the key it named.
  const { ru } = await import("@/lib/i18n/locales/ru/core");
  const [key, translated] = Object.entries(ru)[0];
  expect(translated, "the overlay must carry something").toBeTruthy();

  render(
    <I18nProvider>
      <Probe locale="ru" translationKey={key} />
    </I18nProvider>,
  );

  const button = screen.getByRole("button");
  // Mongolian first: the chunk has not been asked for yet.
  expect(button.textContent).toMatch(/^mn:/);
  expect(button.textContent).not.toBe(`mn:${translated}`);

  await act(async () => {
    button.click();
  });

  // waitFor rather than a bare assertion: the chunk is a real fetch, so the
  // language changes first and its words land a tick later. That gap is the
  // behaviour being bought here, not an accident of the test.
  await waitFor(() => expect(button.textContent).toBe(`ru:${translated}`));
});
