/**
 * Where the About link and the Documentation link point.
 *
 * Both live in the wiki (sf-wiki) rather than in this app: they are long-form
 * prose that changes on its own schedule, and shipping a second copy here only
 * creates two versions to keep in step.
 *
 * The API reports both, so a deployment can point at its own wiki without
 * rebuilding the frontend; the build-time values are the fallback for a client
 * that rendered before the config arrived.
 */
const ABOUT_FALLBACK =
  process.env.REACT_APP_ABOUT_URL || "https://doc.strong-fish.com/docs/about";
const DOC_FALLBACK = process.env.REACT_APP_DOC_URL || "https://doc.strong-fish.com";

export function aboutUrl(config) {
  return config?.aboutUrl || ABOUT_FALLBACK;
}

export function docUrl(config) {
  return config?.docUrl || DOC_FALLBACK;
}

export default aboutUrl;
