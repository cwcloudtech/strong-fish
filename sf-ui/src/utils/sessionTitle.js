/**
 * The name a coach gave a session, or "" when it has none of its own.
 *
 * A session with no title is stored with one the API generates - the English
 * "Week 2 Day 3" - so that exports and other clients have something to print.
 * Showing that back would be both redundant next to the week and day already
 * on the header and, for a reader in French or Arabic, in the wrong language.
 * Only a title somebody actually typed is theirs to see.
 */
export function sessionTitle(day) {
  const title = (day?.title || "").trim();
  if (!title) return "";
  return title === `Week ${day.week} Day ${day.day}` ? "" : title;
}

export default sessionTitle;
