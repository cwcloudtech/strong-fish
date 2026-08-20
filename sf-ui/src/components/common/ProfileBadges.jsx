import { useI18n } from "../../i18n/I18nContext";

/**
 * What somebody is on StrongFish, and what they call themselves as a lifter.
 *
 * Both are read from the same profile object and both render as badges, but
 * they answer different questions: the role is the account's standing here and
 * is granted, while the specialty is a claim its owner makes about themselves
 * (see the API's models/specialty.go). Giving them separate colours is the
 * whole point - a coach who deadlifts should read as two facts at a glance,
 * not as one grey line of text.
 */

// Roles that carry a badge. "confirmed" is a plain athlete: the account state
// is what the API calls it, and what a member reads is "Athlete".
const ROLES = {
  superadmin: { key: "profile.superadmin", className: "sf-badge-role-superadmin" },
  coach: { key: "profile.coach", className: "sf-badge-role-coach" },
  confirmed: { key: "profile.athlete", className: "sf-badge-role-confirmed" },
};

// The lifts somebody may claim. Anything else - including the empty string an
// account that never picked one carries - shows no badge at all.
const SPECIALTIES = {
  squat: "sf-badge-lift-squat",
  bench: "sf-badge-lift-bench",
  deadlift: "sf-badge-lift-deadlift",
  total: "sf-badge-lift-total",
};

export function RoleBadge({ role }) {
  const { t } = useI18n();
  // An account still waiting on its coach confirmation, or a disabled one,
  // reads as an athlete: that is what it can do here.
  const badge = ROLES[role] || ROLES.confirmed;
  return <span className={`sf-badge sf-badge-role ${badge.className}`}>{t(badge.key)}</span>;
}

export function SpecialtyBadge({ specialty }) {
  const { t } = useI18n();
  const className = SPECIALTIES[specialty];
  if (!className) return null;
  return <span className={`sf-badge sf-badge-lift ${className}`}>{t(`profile.specialties.${specialty}`)}</span>;
}

/** Both badges in the row they share, under a profile's name. */
export default function ProfileBadges({ role, specialty, className = "" }) {
  return (
    <div className={`sf-profile-badges ${className}`.trim()}>
      <RoleBadge role={role} />
      <SpecialtyBadge specialty={specialty} />
    </div>
  );
}
