/**
 * A user's avatar, falling back to their initials. Pictures are stored as base64
 * data URIs with an x/y focal point, so the fallback covers both "no picture" and
 * "picture that failed to decode".
 */
export default function Avatar({ user, size = "" }) {
  const name = `${user?.name || ""} ${user?.surname || ""}`.trim();
  const initials =
    name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0].toUpperCase())
      .join("") || "?";

  return (
    <span className={`sf-avatar ${size}`} title={name}>
      {user?.picture ? (
        <img
          src={user.picture}
          alt={name}
          style={{ objectPosition: `${user.pictureX ?? 50}% ${user.pictureY ?? 50}%` }}
        />
      ) : (
        initials
      )}
    </span>
  );
}
