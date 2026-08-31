import { useI18n } from "../../i18n/I18nContext";

/**
 * What a lifter has earned.
 *
 * Shown on a profile, not on the calculator: a badge is something a member
 * won with their own recorded maxes, and awarding one for a number somebody
 * typed into a form would make it worth nothing. The calculator gets the tier
 * and the percentile instead (see StrengthSummary).
 *
 * Locked badges are shown alongside the earned ones with how far along they
 * are - a target you can see is worth more than one that appears out of
 * nowhere the day you hit it. The API decides all of it (internal/strength);
 * this renders what it was handed and translates the keys.
 */
export default function StrengthBadges({ result, earnedOnly = false }) {
  const { t } = useI18n();
  if (!result) return null;

  const badges = result.badges || [];
  const earned = badges.filter((badge) => badge.earned);
  const locked = badges.filter((badge) => !badge.earned && badge.progress > 0);
  const shown = earnedOnly ? earned : [...earned, ...locked];

  if (!shown.length) {
    return <p className="sf-muted" style={{ marginBottom: 0 }}>{t("strength.noBadges")}</p>;
  }

  return (
    <ul className="sf-badges">
      {shown.map((badge) => (
        <li key={badge.key} className={`sf-badge-card${badge.earned ? " is-earned" : ""}`}>
          <span className="sf-badge-name">{t(`strength.badges.${badge.key}`)}</span>
          {badge.earned ? null : (
            <span className="sf-badge-progress" aria-hidden="true">
              <span style={{ width: `${Math.round((badge.progress || 0) * 100)}%` }} />
            </span>
          )}
        </li>
      ))}
    </ul>
  );
}
