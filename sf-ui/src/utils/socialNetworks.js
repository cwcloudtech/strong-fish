import { FaBluesky, FaFacebookF, FaXTwitter } from "react-icons/fa6";

/**
 * The networks a profile or a post can be shared to.
 *
 * A list of entries rather than branching code, so adding one is adding an
 * object: an id, a label, an icon, and how it takes a share. Nothing else in
 * the app knows the names of these networks.
 *
 * Every entry here can actually take a share: `share(url, text)` returns the
 * address to open, and a network that has no web share intent does not belong
 * in this list at all.
 *
 * Instagram, TikTok and Discord are the ones deliberately absent. None of them
 * accepts a link from a web page - the first two compose from media on the
 * device, and a Discord link goes to whichever server and channel the person
 * picks, which only their client knows. A button for those could only copy the
 * link, which is what the copy button beside this row already does; offering it
 * twice under a logo suggests something is happening that is not.
 *
 * The phone app is where those networks are reachable, through the system's
 * own share sheet, which hands them the link directly.
 */
const encode = encodeURIComponent;

export const SOCIAL_NETWORKS = [
  {
    id: "facebook",
    label: "Facebook",
    Icon: FaFacebookF,
    // Facebook takes the URL only; it reads the title and image from the
    // page's own Open Graph tags rather than from a parameter.
    share: (url) => `https://www.facebook.com/sharer/sharer.php?u=${encode(url)}`,
  },
  {
    id: "x",
    label: "X",
    Icon: FaXTwitter,
    share: (url, text) => `https://twitter.com/intent/tweet?url=${encode(url)}&text=${encode(text)}`,
  },
  {
    id: "bluesky",
    label: "Bluesky",
    Icon: FaBluesky,
    // Bluesky's compose intent takes one text field, so the link goes inside
    // it; the client turns it into a card on posting.
    share: (url, text) => `https://bsky.app/intent/compose?text=${encode(`${text} ${url}`)}`,
  },
];

export default SOCIAL_NETWORKS;
