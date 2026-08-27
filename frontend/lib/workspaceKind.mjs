/**
 * Which screens a workspace has, decided by what kind of workspace it is.
 *
 * Since migration 00085 a workspace is one of two things. An organisation is a
 * company: it installs apps, it has a legal identity, somebody administers it.
 * A home is one person's own space, made for them the first time they sign in
 * belonging to no organisation. Both are workspaces by mechanism — same schema,
 * same row-level policies, same session — and that is exactly why the shell has
 * to be told them apart: nothing else in the request says which one you are in.
 *
 * Written as a module rather than a condition inside the shell so the rule can
 * be read and tested in one place. The alternative that was proposed first was
 * a second shell under /me, and it was worse: a home's other screens — the
 * profile, the devices, the appearance — already live in this one, so a person
 * would have crossed between two chromes on the first click.
 *
 * `undefined` is "not answered yet", and it answers false. The shell holds a
 * loading screen until /api/v1/me returns, so nothing is drawn from this value
 * before it is known; false is the reading that stays correct if that ever
 * stops being true, because a link that appears late is better than one that
 * appears wrongly and is clicked.
 */

/** @type {"personal"} */
export const PERSONAL = "personal";

/**
 * Whether this workspace has the screens that only make sense for a company.
 *
 * The app store: a home installs nothing — apps are bought and enabled for an
 * organisation, by somebody with the right to spend its money. The legal
 * identity: a home has no registration number, no legal name and nobody to
 * be an organisation to.
 *
 * @param {string | undefined | null} workspaceKind
 * @returns {boolean}
 */
export function organisationScreensVisible(workspaceKind) {
  return typeof workspaceKind === "string" && workspaceKind !== "" && workspaceKind !== PERSONAL;
}

/**
 * Whether this row in the workspace switcher is the person's own home.
 *
 * @param {{ kind?: string } | null | undefined} option
 * @returns {boolean}
 */
export function isHome(option) {
  return option?.kind === PERSONAL;
}
