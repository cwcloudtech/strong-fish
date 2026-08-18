/**
 * Turning a post into something worth sharing.
 *
 * A post's text carries its own link inline - that is how the embed is detected
 * in the first place (see the API's utils.FirstURL). Sharing the text verbatim
 * would send the reader to the YouTube video rather than to the post, and would
 * leave a bare URL sitting in the middle of the shared copy. So the links come
 * out of the text, and the link to the post goes in the share's own url field
 * where every network expects it.
 */

// Matches what the API detects, so text and embed can't disagree about what
// counts as a link.
const URL_PATTERN = /https?:\/\/\S+/g;

/** How much of a post travels with a share before it is cut. */
const MAX_LENGTH = 180;

/**
 * The post's text with its URLs removed and the leftover whitespace tidied, cut
 * to a length every network will accept.
 *
 * Cut on a word boundary where there is one nearby: a share ending mid-word
 * reads as broken rather than as abridged.
 */
export function shareTextFor(content, fallback = "") {
  const stripped = (content || "").replace(URL_PATTERN, " ").replace(/\s+/g, " ").trim();
  if (!stripped) return fallback;
  if (stripped.length <= MAX_LENGTH) return stripped;

  const cut = stripped.slice(0, MAX_LENGTH);
  const lastSpace = cut.lastIndexOf(" ");
  return `${(lastSpace > MAX_LENGTH - 30 ? cut.slice(0, lastSpace) : cut).trimEnd()}…`;
}

/** Where a shared post points: the post itself, readable without an account. */
export function postShareUrl(postId) {
  return `${window.location.origin}/posts/${postId}`;
}
