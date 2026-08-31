import Tooltip from "../common/Tooltip";
import { useI18n } from "../../i18n/I18nContext";

/**
 * The tier a DOTS score falls in - "Platform Contender", "Titan" - as the one
 * badge that gets a colour of its own.
 *
 * Where the lifter sits among the members here is its tooltip rather than a
 * line of its own: on a profile the tier is the headline, and a percentile
 * printed beside it competes with the thing it is describing. The calculator
 * shows the same number as a bar, because there it *is* the answer.
 */
export default function StrengthTier({ result, showScore = true }) {
  const { t } = useI18n();
  if (!result?.tier?.key) return null;

  const percentile = result.percentile?.sample
    ? t("strength.percentile", {
        value: result.percentile.value,
        sample: result.percentile.sample,
      })
    : t("strength.noPopulation");

  return (
    <div className="sf-strength-tier">
      <Tooltip label={percentile}>
        <span className={`sf-tier sf-tier-${result.tier.key}`}>{t(`strength.tiers.${result.tier.key}`)}</span>
      </Tooltip>
      {showScore ? <span className="sf-muted">{t("strength.tierFrom", { value: result.scores?.dots ?? 0 })}</span> : null}
    </div>
  );
}
