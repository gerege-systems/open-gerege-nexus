/**
 * The theme, decided before the first frame is drawn.
 *
 * ThemeProvider cannot do this: it runs after hydration, so a reader who has
 * chosen dark used to get a full white page first and the dark one a moment
 * later. This file is loaded synchronously from the document head, so the
 * class is on <html> before anything is painted.
 *
 * It is a file rather than an inline script so no `script-src 'unsafe-inline'`
 * has to be opened for it — the backend already serves `script-src 'self'`,
 * and the front door should be able to adopt the same policy without this
 * being the reason it cannot.
 *
 * It reads exactly what lib/theme.tsx writes (one JSON blob under
 * `gerege_theme`) and sets exactly what globals.css reads. Nothing else: if it
 * throws, the page still renders, in light.
 */
(function () {
  try {
    var saved = JSON.parse(localStorage.getItem("gerege_theme") || "null") || {};
    var mode = saved.mode === "light" || saved.mode === "dark" ? saved.mode : "system";
    var dark =
      mode === "dark" ||
      (mode === "system" && matchMedia("(prefers-color-scheme: dark)").matches);
    var root = document.documentElement;
    if (dark) root.classList.add("dark");
    // The three that are not light/dark but do change the first paint: the
    // accent repaints every surface, density sets the root font size, and the
    // design decides whether the top bar is blue.
    root.dataset.accent = saved.accent || "neutral";
    root.dataset.density = saved.density || "comfortable";
    root.dataset.design = saved.design === "gerege" ? "gerege" : "original";
    // Native form controls, scrollbars and <select> popups follow this, not
    // the class — without it dark mode keeps a white scrollbar.
    root.style.colorScheme = dark ? "dark" : "light";
  } catch (e) {
    /* localStorage can be unavailable (private mode, blocked cookies). Light. */
  }
})();
