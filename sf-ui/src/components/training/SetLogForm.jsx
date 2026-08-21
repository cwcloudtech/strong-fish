import { useState } from "react";

import { ErrorMessage } from "../common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

/**
 * What the member actually did on one set: the reps and weight, the RPE they
 * perceived, and a comment for their coach.
 *
 * The fields pre-fill from the prescription (and from any earlier log), because
 * the common case is "did exactly what was asked" - the member should only have
 * to change what differed.
 */
export default function SetLogForm({ set, onSubmit, onCancel }) {
  const { t } = useI18n();
  const log = set.log;

  const [form, setForm] = useState({
    actualReps: log?.actualReps ?? set.reps,
    actualRpe: log?.actualRpe ?? set.rpe ?? "",
    actualLoad: log?.actualLoad ?? (set.loadKnown ? set.roundedLoad : ""),
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
              value={form.actualLoad}
              onChange={set_("actualLoad")}
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
