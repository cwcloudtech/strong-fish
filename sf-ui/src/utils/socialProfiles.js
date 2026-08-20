import { FaBluesky, FaDumbbell, FaInstagram, FaTiktok, FaXTwitter } from "react-icons/fa6";

/**
 * The accounts a member can show on their profile.
 *
 * A table rather than five hand-written fields: the settings form and the
 * profile's link row are both generated from it, so adding a network is adding
 * an entry - a key, a label, an icon and the address its accounts live at.
 *
 * The API stores the account's own name ("marie.lifts"), never a whole URL, and
 * hands back whatever a member pasted reduced to that (see the API's
 * models/socials.go). `link` is what turns it back into an address, which is
 * why the base lives here beside the icon rather than in the payload.
 */
export const SOCIAL_PROFILES = [
  {
    key: "instagram",
    label: "Instagram",
    Icon: FaInstagram,
    placeholder: "marie.lifts",
    link: (account) => `https://www.instagram.com/${account}`,
  },
  {
    key: "tiktok",
    label: "TikTok",
    Icon: FaTiktok,
    placeholder: "marie.lifts",
    link: (account) => `https://www.tiktok.com/@${account}`,
  },
  {
    key: "x",
    label: "X",
    Icon: FaXTwitter,
    placeholder: "marielifts",
    link: (account) => `https://x.com/${account}`,
  },
  {
    key: "bluesky",
    label: "Bluesky",
    Icon: FaBluesky,
    // A Bluesky handle is a domain, and the profile path is what it sits under.
    placeholder: "marie.bsky.social",
    link: (account) => `https://bsky.app/profile/${account}`,
  },
  {
    key: "openpowerlifting",
    label: "OpenPowerlifting",
    // The strength icon: this one is not a social network but the federation
    // results database, and a dumbbell says so at a glance.
    Icon: FaDumbbell,
    placeholder: "mariedubois",
    link: (account) => `https://www.openpowerlifting.org/u/${account}`,
    // The only entry with a second field: a placing read off that page, which
    // the app never computes.
    rankKey: "openpowerliftingRank",
  },
];

/** The entries a profile actually filled in, ready to render as links. */
export function filledSocials(socials) {
  if (!socials) return [];
  return SOCIAL_PROFILES.filter((network) => socials[network.key]).map((network) => ({
    ...network,
    account: socials[network.key],
    href: network.link(socials[network.key]),
    rank: network.rankKey ? socials[network.rankKey] : "",
  }));
}

export default SOCIAL_PROFILES;
