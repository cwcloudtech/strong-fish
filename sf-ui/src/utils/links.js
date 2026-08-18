// The same URL detection the API applies when it stores a post (see the Go
// utils.FirstURL). It is duplicated rather than fetched so the composer can
// preview the embed as the author types; the API's copy is the authority, and
// the two patterns are kept identical on purpose.
const URL_PATTERN = /https?:\/\/[^\s<>"'`]+[^\s<>"'`.,;:!?)\]}]/;
const BARE_URL_PATTERN = /^\s*(https?:\/\/\S+)\s*$/;

/** The first http(s) URL in text, or "" when there is none. */
export function firstUrl(text) {
  const found = (text || "").match(URL_PATTERN);
  if (found) return found[0];
  // A URL that is the whole text has no trailing character for the pattern's
  // final class to match.
  const bare = (text || "").match(BARE_URL_PATTERN);
  return bare ? bare[1] : "";
}

export default firstUrl;
