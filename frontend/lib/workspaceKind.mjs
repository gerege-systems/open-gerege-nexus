/**
 * Which screens a workspace has, decided by what kind of workspace it is.
 *
 * Since migration 00094 there are two answers and they are not two kinds of
 * workspace. An organisation is a company: it installs apps, it has a legal
 * identity, somebody administers it. "none" is a person signed in and standing
 * in no workspace at all, which is what belonging to no organisation looks
 * like — their own record is keyed on them rather than on a room they were put
 * in, so there is no room.
 *
 * This replaced a design where every person was given a workspace of their own.
 * It worked and it put a row in the customer table for every human being who
 * ever authenticated; at a million people that was 3.9 GB, most of it
 * access-control rows for workspaces with one member who owned them.
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
 * What the API reports when a session stands in no workspace.
 *
 * A word rather than an empty string, and the reason is the paragraph below
 * about `undefined`: the shell has three states to tell apart and two of them
 * are falsy. "The answer has not arrived" and "there is no workspace" must not
 * be the same value, or every sign-in draws a citizen's rail for a moment —
 * including an administrator's.
 *
 * @type {"none"}
 */
export const NO_WORKSPACE = "none";

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
  return (
    typeof workspaceKind === "string" &&
    workspaceKind !== "" &&
    workspaceKind !== NO_WORKSPACE &&
    workspaceKind !== PERSONAL
  );
}

/**
 * Whether these are the screens of one person with no organisation.
 *
 * The mirror of the rule above and deliberately not its negation in the shell:
 * writing `!organisationScreensVisible(kind)` there would have made an
 * unanswered kind — the moment before /api/v1/me returns, or a deployment whose
 * API predates the field — draw a citizen's rail at everybody. Both questions
 * default to "not this one", which is the only pair of answers that shows
 * nothing rather than the wrong thing.
 *
 * PERSONAL is still accepted. Deployments that have not yet run 00094 still
 * have personal workspaces and still report them, and a shell that answered
 * "no" to those would hide the personal side from every citizen on them until
 * the migration landed.
 *
 * @param {string | undefined | null} workspaceKind
 * @returns {boolean}
 */
export function homeScreensVisible(workspaceKind) {
  return workspaceKind === NO_WORKSPACE || workspaceKind === PERSONAL;
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
