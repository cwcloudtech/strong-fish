import { useState } from "react";

import { ErrorMessage } from "../common/Feedback";
import Switch from "../common/Switch";
import { useI18n } from "../../i18n/I18nContext";

/**
 * What the member actually did on one set: the reps and weight, the RPE they
 * perceived, and a comment for their coach.
 *
 * Reps and RPE pre-fill from the prescription (and from any earlier log),
 * because the common case is "did exactly what was asked" - the member should
 * only have to change what differed.
 *
 * The load does not. A pre-filled weight is saved as if it had been typed the
 * moment the set is ticked off, so every set came back claiming a load the
 * member never entered - and the row then shows it as the weight they used.
 * The prescription is offered as the field's placeholder instead: visible, and
 * not a value.
 */
export default function SetLogForm({ set, onSubmit, onCancel }) {
  const { t } = useI18n();
  const log = set.log;

  const [form, setForm] = useState({
    actualReps: log?.actualReps ?? set.reps,
    actualRpe: log?.actualRpe ?? set.rpe ?? "",
    actualLoad: log?.actualLoad ?? "",
    beltless: Boolean(log?.beltless),
    comment: log?.comment ?? "",
  });
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const set_ = (field) => (event) => setForm((current) => ({ ...current, [field]: event.target.value }));

  const submit = async (event) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      // Blank fields mean "not recorded" rather than zero, which is why they're
      // sent as null instead of being coerced to a number.
      await onSubmit({
        actualReps: form.actualReps === "" ? null : Number(form.actualReps),
        actualRpe: form.actualRpe === "" ? null : Number(form.actualRpe),
        actualLoad: form.actualLoad === "" ? null : Number(form.actualLoad),
        beltless: Boolean(form.beltless),
        comment: form.comment,
        // Filling this in is saying you did the set; the button in the row
        // says the same thing, and the two must not disagree.
        done: true,
      });
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit}>
      <div className="sf-log-form">
        <div className="sf-field">
          <label className="sf-label">{t("session.actualReps")}</label>
          <input className="sf-input sf-input-sm" type="number" min="0" value={form.actualReps} onChange={set_("actualReps")} />
        </div>
        <div className="sf-field">
          <label className="sf-label">{t("session.actualRpe")}</label>
          <input
            className="sf-input sf-input-sm"
            type="number"
            min="1"
            max="10"
            step="0.5"
            value={form.actualRpe}
            onChange={set_("actualRpe")}
          />
        </div>
        {set.loadMode !== "bodyweight" ? (
          <div className="sf-field">
            <label className="sf-label">{t("session.actualLoad")}</label>
            <input
              className="sf-input sf-input-sm"
              type="number"
              min="0"
              step="0.5"
              placeholder={set.loadKnown && set.roundedLoad ? String(set.roundedLoad) : undefined}
              value={form.actualLoad}
              onChange={set_("actualLoad")}
            />
          </div>
        ) : null}
        {/* Only where a belt is the norm: the squat and the deadlift and
            their variations, which the API reads off the same table the loads
            come from. A bench is never asked. */}
        {set.withBelt ? (
          <div className="sf-field">
            <label className="sf-label" htmlFor={`beltless-${set.id}`}>
              {t("session.beltlessQuestion")}
            </label>
            <Switch
              id={`beltless-${set.id}`}
              checked={Boolean(form.beltless)}
              onChange={(value) => setForm((current) => ({ ...current, beltless: value }))}
            />
          </div>
        ) : null}
        <div className="sf-field sf-log-comment">
          <label className="sf-label">{t("session.comment")}</label>
          <input className="sf-input sf-input-sm" value={form.comment} onChange={set_("comment")} />
        </div>
        <div className="sf-row" style={{ gap: "0.35rem" }}>
          <button className="sf-button sf-button-sm" type="submit" disabled={busy}>
            {t("session.done")}
          </button>
          <button type="button" className="sf-button sf-button-ghost sf-button-sm" onClick={onCancel} disabled={busy}>
            {t("common.cancel")}
          </button>
        </div>
      </div>

      {log?.e1rm ? (
        <p className="sf-muted" style={{ margin: "0 0 0.5rem" }}>
          {t("session.yourE1rm", { value: log.e1rm })}
          {set.oneRm && log.e1rm > set.oneRm ? ` ${t("session.beatsMax")}` : ""}
        </p>
      ) : null}
      <ErrorMessage error={error} />
    </form>
  );
}
