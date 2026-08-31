/**
 * The frontend's test runner.
 *
 * Vitest rather than `node --test`, which ran the four files this suite grew
 * from: Node cannot transform TSX, so every rule that lives inside a component
 * — who may see the button, what happens when the second password differs —
 * was untestable and therefore untested. The pure-logic files that were
 * testable are still here and still pass; they only changed their import.
 *
 * jsdom is the environment because these are the console's screens, and a
 * screen is a DOM. Anything that needs a real browser (the service worker, the
 * host gate, a page that has to be built and served) is Playwright's, in
 * tests/e2e.
 */
import { defineConfig } from "vitest/config";
import { fileURLToPath } from "node:url";

export default defineConfig({
  test: {
    environment: "jsdom",
    // Only tests/. app/ and components/ hold the product; a test file beside a
    // page would be shipped to the browser by Next's file-system router.
    include: ["tests/**/*.test.{ts,tsx,mts,mjs}"],
    exclude: ["tests/e2e/**"],
    setupFiles: ["tests/setup.ts"],
    globals: false,
    unstubGlobals: true,
    restoreMocks: true,
    // Vitest 3-т `restoreMocks` нь дуудлагын түүхийг ч цэвэрлэдэг байсан;
    // 4-т тэр нь зөвхөн `vi.spyOn`-ийг эх хэлбэрт нь буцаадаг болсон тул
    // `vi.fn()`-ийн тоолуур файл дотор хуримтлагдаж, «нэг удаа дуудагдсан
    // байх ёстой» гэсэн зургаан шалгалт «тав удаа» гэж уначихсан. Түүхийг
    // цэвэрлэх нь одоо тусдаа тохиргоо.
    clearMocks: true,
  },
  resolve: {
    alias: { "@": fileURLToPath(new URL(".", import.meta.url)) },
  },
});
