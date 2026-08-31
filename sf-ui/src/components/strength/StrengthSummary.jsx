import StrengthTier from "./StrengthTier";
import { useI18n } from "../../i18n/I18nContext";

/**
 * What a total is worth, for the calculator: the tier it falls in and how it
 * compares with the lifters here.
 *
 * The percentage is spelled out and drawn, because on the calculator it is the
 * answer to the question being asked. On a profile the same number is the tier
 * badge's tooltip instead (see StrengthTier), where the tier is the headline.
 */
export default function StrengthSummary({ result }) {
  const { t } = useI18n();
  if (!result) return null;

  return (
    <>
      <StrengthTier result={result} />

      {/* Against the members of this deployment, not a published curve fitted
          on international meets - which would tell a beginners' gym that
          everybody in it is bottom decile. The sample is printed with it: a
          percentile among four people is a fact about four people. */}
      {result.percentile?.sample ? (
        <div className="sf-percentile">
          <div className="sf-percentile-head">
            <span className="sf-label">{t("strength.strongerThan")}</span>
            <strong className="sf-percentile-value">{result.percentile.value}%</strong>
          </div>
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
      ) : (
        <p className="sf-muted" style={{ marginBottom: 0 }}>{t("strength.noPopulation")}</p>
      )}
    </>
  );
}
