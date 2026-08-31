import { useI18n } from "../../i18n/I18nContext";

/**
 * A lifter's tier, where they sit among the club, and what they have earned.
 *
 * Locked badges are shown alongside the earned ones, with how far along they
 * are: a target you can see is worth more than one that appears out of nowhere
 * the day you hit it. The API decides all of it (internal/strength) - this
 * renders what it was handed and translates the keys.
 */
export default function StrengthBadges({ result, compact = false }) {
  const { t } = useI18n();
  if (!result) return null;

  const badges = result.badges || [];
  const earned = badges.filter((badge) => badge.earned);
  const locked = badges.filter((badge) => !badge.earned && badge.progress > 0);
  const shown = compact ? earned : [...earned, ...locked];

  return (
    <div className="sf-card">
      {result.tier?.key ? (
        <div className="sf-strength-tier">
          <span className={`sf-tier sf-tier-${result.tier.key}`}>{t(`strength.tiers.${result.tier.key}`)}</span>
          <span className="sf-muted">{t("strength.tierFrom", { value: result.scores?.dots ?? 0 })}</span>
        </div>
      ) : null}

      {/* Where this score sits among the members of this deployment - not a
          published curve fitted on international meets, which would tell a
          beginners' gym that everybody in it is bottom decile. The sample is
          printed with it: a percentile among four people is a fact about four
          people. */}
      {result.percentile?.sample ? (
        <div className="sf-percentile">
          <div className="sf-percentile-bar">
            <span style={{ width: `${result.percentile.value}%` }} />
          </div>
          <p className="sf-muted" style={{ margin: "0.3rem 0 0" }}>
            {t("strength.percentile", {
              value: result.percentile.value,
              sample: result.percentile.sample,
            })}
          </p>
        </div>
      ) : null}

      {shown.length ? (
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
      ) : (
        <p className="sf-muted" style={{ marginBottom: 0 }}>{t("strength.noBadges")}</p>
      )}
    </div>
  );
}
