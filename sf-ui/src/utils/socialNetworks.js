import {
  FaBluesky,
  FaDiscord,
  FaFacebookF,
  FaInstagram,
  FaTiktok,
  FaXTwitter,
} from "react-icons/fa6";

/**
 * The networks a profile or a post can be shared to.
 *
 * A list of entries rather than branching code, so adding one is adding an
 * object: an id, a label, an icon, and how it takes a share. Nothing else in
 * the app knows the names of these networks.
 *
 * `share(url, text)` returns the address to open, or **null** when the network
 * has no web share intent at all. That is not an oversight to be filled in
 * later - Instagram and TikTok deliberately offer no way for a web page to
 * hand them a link, because both compose from media on the device. For those
 * the button copies the link instead and says so, which is the honest version
 * of the feature rather than a button that silently does nothing.
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
  {
    id: "discord",
    label: "Discord",
    Icon: FaDiscord,
    // No web intent: Discord has no "share to" URL, because a link goes into
    // whichever server and channel the person picks - something only their
    // client knows. Copying is how everyone shares to Discord anyway.
    share: null,
  },
  {
    id: "instagram",
    label: "Instagram",
    Icon: FaInstagram,
    // No web intent: Instagram composes from the camera roll.
    share: null,
  },
  {
    id: "tiktok",
    label: "TikTok",
    Icon: FaTiktok,
    // No web intent either, for the same reason.
    share: null,
  },
];

export default SOCIAL_NETWORKS;
