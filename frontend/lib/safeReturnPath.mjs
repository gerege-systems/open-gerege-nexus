/**
 * Accept a same-origin path to resume after authentication.
 *
 * A leading slash alone is not enough: browsers treat backslashes as slashes
 * in special URLs, so `/\\attacker.example` becomes a protocol-relative
 * navigation. Keep this check next to every authentication method instead of
 * asking each screen to remember that edge case.
 *
 * @param {unknown} raw
 * @param {string} [fallback]
 * @returns {string}
 */
export function safeReturnPath(raw, fallback = "/profile") {
  if (typeof raw !== "string" || !raw.startsWith("/") || raw.startsWith("//")) {
    return fallback;
  }
  if (raw.includes("\\") || /[\u0000-\u001f\u007f]/.test(raw)) return fallback;

  try {
    const base = new URL("https://nexus.invalid");
    const target = new URL(raw, base);
    return target.origin === base.origin ? raw : fallback;
  } catch {
    return fallback;
  }
}
