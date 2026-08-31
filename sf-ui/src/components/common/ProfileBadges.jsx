import { useI18n } from "../../i18n/I18nContext";

/**
 * What somebody is on StrongFish.
 *
 * The role is the account's standing here and is granted. What a lifter is
 * *good at* used to sit beside it as a badge they picked for themselves; that
 * is now derived from their own maxes instead (see the API's
 * internal/strength), because a claim and a measurement answering the same
 * question meant a profile could contradict the numbers underneath it.
 */

// Roles that carry a badge. "confirmed" is a plain athlete: the account state
// is what the API calls it, and what a member reads is "Athlete".
const ROLES = {
  superadmin: { key: "profile.superadmin", className: "sf-badge-role-superadmin" },
  coach: { key: "profile.coach", className: "sf-badge-role-coach" },
  confirmed: { key: "profile.athlete", className: "sf-badge-role-confirmed" },
};

export function RoleBadge({ role }) {
  const { t } = useI18n();
  // An account still waiting on its coach confirmation, or a disabled one,
  // reads as an athlete: that is what it can do here.
  const badge = ROLES[role] || ROLES.confirmed;
  return <span className={`sf-badge sf-badge-role ${badge.className}`}>{t(badge.key)}</span>;
}

/** The badge under a profile's name. */
export default function ProfileBadges({ role, className = "" }) {
  return (
    <div className={`sf-profile-badges ${className}`.trim()}>
      <RoleBadge role={role} />
    </div>
  );
}
