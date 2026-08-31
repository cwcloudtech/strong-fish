import { useCallback, useEffect, useState } from "react";

import Select from "../components/common/Select";
import StrengthBadges from "../components/strength/StrengthBadges";
import { ErrorMessage } from "../components/common/Feedback";
import { strength as strengthApi } from "../api/services";
import { useI18n } from "../i18n/I18nContext";

/**
 * The powerlifting calculator: three coefficients on one total.
 *
 * Open with no account, because working out what a total is worth is the
 * question somebody asks before they have one. Signed in, the form arrives
 * filled from the profile and the recorded maxes - the numbers are already in
 * the app, and retyping them is how a calculator goes unused.
 *
 * The scoring is the API's (internal/strength), never repeated here: three
 * fitted polynomials in two languages would be two answers to the same
 * question, and the wrong one would be whichever nobody checked.
 */

/** Pounds to kilograms, for the members who think in the other one. */
const LB_PER_KG = 2.2046226218;

const EMPTY = { gender: "male", division: "raw", bodyweight: "", squat: "", bench: "", deadlift: "" };

export default function Strength() {
  const { t } = useI18n();
  const [form, setForm] = useState(EMPTY);
  const [unit, setUnit] = useState("kg");
  const [result, setResult] = useState(null);
  const [error, setError] = useState(null);

  // Pre-filled from whoever is asking. A logged-out caller gets an empty form
  // back, so there is nothing to branch on here.
  useEffect(() => {
    strengthApi
      .defaults()
      .then((defaults) =>
        setForm((current) => ({
          ...current,
          gender: defaults.gender || "male",
          division: defaults.division || "raw",
          bodyweight: defaults.bodyweight ? String(defaults.bodyweight) : "",
          squat: defaults.squat ? String(defaults.squat) : "",
          bench: defaults.bench ? String(defaults.bench) : "",
          deadlift: defaults.deadlift ? String(defaults.deadlift) : "",
        }))
      )
      .catch(() => setForm(EMPTY));
  }, []);

  const set = (field) => (value) => setForm((current) => ({ ...current, [field]: value }));
  const setInput = (field) => (event) => set(field)(event.target.value);

  const toKg = useCallback(
    (value) => {
      const parsed = Number(String(value).replace(",", "."));
      if (!Number.isFinite(parsed) || parsed <= 0) return 0;
      return unit === "lb" ? parsed / LB_PER_KG : parsed;
    },
    [unit]
  );

  // Scored on every change, debounced: the API is the only place the formulas
  // live, and a calculator that needs a button pressed to answer is a form.
  useEffect(() => {
    const payload = {
      gender: form.gender,
      division: form.division,
      bodyweight: toKg(form.bodyweight),
      squat: toKg(form.squat),
      bench: toKg(form.bench),
      deadlift: toKg(form.deadlift),
    };
    if (!payload.bodyweight || payload.squat + payload.bench + payload.deadlift <= 0) {
      setResult(null);
      return undefined;
    }

    const timer = setTimeout(() => {
      strengthApi.score(payload).then(setResult).catch(setError);
    }, 250);
    return () => clearTimeout(timer);
  }, [form, toKg]);

  /** A weight back in whatever unit is being typed in. */
  const show = (kg) => {
    const value = unit === "lb" ? kg * LB_PER_KG : kg;
    return `${Math.round(value * 10) / 10} ${t(unit === "lb" ? "strength.lb" : "common.kg")}`;
  };

  return (
    <div className="sf-page" style={{ maxWidth: 860 }}>
      <h1 className="sf-title">{t("strength.title")}</h1>
      <p className="sf-subtitle">{t("strength.subtitle")}</p>

      <div className="sf-card">
        <div className="sf-row" style={{ gap: "0.6rem", alignItems: "flex-start" }}>
          <div className="sf-field" style={{ flex: 1, minWidth: 150 }}>
            <label className="sf-label">{t("profile.gender")}</label>
            <Select
              options={[
                { value: "male", label: t("profile.genderMale") },
                { value: "female", label: t("profile.genderFemale") },
              ]}
              value={form.gender}
              onChange={set("gender")}
            />
          </div>
          <div className="sf-field" style={{ flex: 1, minWidth: 150 }}>
            <label className="sf-label">{t("strength.unit")}</label>
            <Select
              options={[
                { value: "kg", label: t("strength.kg") },
                { value: "lb", label: t("strength.lb") },
              ]}
              value={unit}
              onChange={setUnit}
            />
          </div>
          <div className="sf-field" style={{ flex: 1, minWidth: 150 }}>
            <label className="sf-label">{t("strength.division")}</label>
            <Select
              options={[
                { value: "raw", label: t("strength.raw") },
                { value: "equipped", label: t("strength.equipped") },
              ]}
              value={form.division}
              onChange={set("division")}
            />
          </div>
        </div>

        <div className="sf-row" style={{ gap: "0.6rem", alignItems: "flex-start" }}>
          {["bodyweight", "squat", "bench", "deadlift"].map((field) => (
            <div className="sf-field" key={field} style={{ flex: 1, minWidth: 120 }}>
              <label className="sf-label" htmlFor={field}>
                {t(`strength.${field}`)}
              </label>
              <input
                id={field}
                className="sf-input"
                type="number"
                min="0"
                step="0.5"
                inputMode="decimal"
                value={form[field]}
                onChange={setInput(field)}
              />
            </div>
          ))}
        </div>
        <p className="sf-muted" style={{ margin: 0 }}>{t("strength.unitHelp")}</p>
      </div>

      <ErrorMessage error={error} />

      {result ? (
        <>
          <div className="sf-card sf-strength-scores">
            <div className="sf-strength-total">
              <span className="sf-label">{t("strength.total")}</span>
              <strong>{show(result.total)}</strong>
            </div>
            {[
              ["dots", result.scores.dots],
              ["wilks", result.scores.wilks],
              ["ipfGl", result.scores.ipfGl],
            ].map(([key, value]) => (
              <div className="sf-strength-score" key={key}>
                <span className="sf-label">{t(`strength.${key}`)}</span>
                <strong>{value}</strong>
              </div>
            ))}
          </div>

          <StrengthBadges result={result} />
        </>
      ) : (
        <p className="sf-muted">{t("strength.enterLifts")}</p>
      )}
    </div>
  );
}
