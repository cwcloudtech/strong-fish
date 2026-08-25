import { useEffect, useState } from "react";

import { media as mediaApi } from "../../api/services";
import { apiUrl } from "../../utils/apiUrl";

/**
 * <media-player> for a URL that may point at a private bucket.
 *
 * Most links are unchanged: a public bucket's object, a Drive /preview page, a
 * YouTube video, a plain image. Those go straight to the player, which is what
 * has always happened and still does.
 *
 * The exception is an object in a bucket its owner marked private. It has no
 * address a browser could fetch, so what the post carries is an address on this
 * API - and the API will not serve it without knowing who is asking. A media
 * element cannot send an Authorization header, so the grant has to be in the
 * URL: this asks the API for a short-lived signed link and hands *that* to the
 * player.
 *
 * While the link is being fetched the player is simply not rendered, rather
 * than being rendered with a URL that would 401 and leave a broken frame on
 * screen for a second.
 */
export default function MediaLink({ url, kind }) {
  const [resolved, setResolved] = useState(() => (isPrivateMedia(url) ? null : url));

  useEffect(() => {
    if (!isPrivateMedia(url)) {
      setResolved(url);
      return undefined;
    }

    let cancelled = false;
    setResolved(null);
    mediaApi
      .link(url)
      .then((signed) => !cancelled && setResolved(signed))
      // A link this member may not read is not an error to shout about: the
      // post simply shows nothing where the video would be, which is what a
      // private profile's video should look like to a stranger.
      .catch(() => !cancelled && setResolved(""));

    return () => {
      cancelled = true;
    };
  }, [url]);

  if (!resolved) return null;
  return <media-player url={resolved} kind={kind} />;
}

/**
 * Whether this URL is an object this API serves itself.
 *
 * Matched against the configured API address rather than by a path pattern
 * alone: a post can carry any link at all, and asking this API to sign
 * somebody else's URL would be both useless and a small open redirect.
 */
export function isPrivateMedia(url) {
  if (typeof url !== "string") return false;
  return url.startsWith(`${apiUrl()}/v1/media/`);
}
