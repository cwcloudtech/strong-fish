import { useEffect, useMemo, useState } from "react";

import { exercises as exercisesApi, programs as programsApi } from "../../api/services";
import Modal from "../common/Modal";
import { ErrorMessage } from "../common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

/**
 * Writes one prescribed set.
 *
 * The load mode is asked for explicitly rather than inferred: a coach editing a
 * set is stating how it should be loaded, unlike the importer, which has to
 * guess from a spreadsheet's columns. Which fields are shown follows from it,
 * so an RPE set never collects a weight that would be ignored.
 */
export default function SetFormModal({ clubId, programId, day, set, onClose, onSaved }) {
  const { t, locale } = useI18n();
  const [catalog, setCatalog] = useState([]);
  const [form, setForm] = useState({
    exerciseId: set?.exerciseId || "",
    reps: set?.reps ?? 5,
    rpe: set?.rpe ?? 8,
    percentage: set?.percentage ?? 75,
    absoluteLoad: set?.absoluteLoad ?? 20,
    loadMode: set?.loadMode || "rpe",
    notes: set?.notes || "",
  });
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    exercisesApi.list().then(setCatalog).catch(() => setCatalog([]));
  }, []);

  const setField = (field) => (event) => setForm((current) => ({ ...current, [field]: event.target.value }));

  // Picking a bodyweight movement (pull-ups, dips) settles the load mode: there
  // is no weight to prescribe, so the other modes would be meaningless.
  const selected = useMemo(
    () => catalog.find((exercise) => exercise.id === form.exerciseId),
    [catalog, form.exerciseId]
  );

  useEffect(() => {
    if (selected?.bodyweight && form.loadMode !== "bodyweight") {
      setForm((current) => ({ ...current, loadMode: "bodyweight" }));
    }
  }, [selected, form.loadMode]);

  const submit = async () => {
    setBusy(true);
    setError(null);

    const payload = {
      exerciseId: form.exerciseId,
      reps: Number(form.reps),
      loadMode: form.loadMode,
      notes: form.notes,
      // Only the field the mode actually uses is sent; the rest stay null so a
      // set never carries a stale number from a mode it isn't in.
      rpe: form.loadMode === "rpe" ? Number(form.rpe) : null,
      percentage: form.loadMode === "percentage" ? Number(form.percentage) : null,
      absoluteLoad: form.loadMode === "absolute" ? Number(form.absoluteLoad) : null,
    };

    try {
      if (set) await programsApi.updateSet(clubId, programId, set.id, payload);
      else await programsApi.addSet(clubId, programId, day.id, payload);
      onSaved();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  const grouped = useMemo(() => {
    const byCategory = {};
    for (const exercise of catalog) {
      (byCategory[exercise.category] ||= []).push(exercise);
    }
    return byCategory;
  }, [catalog]);

  return (
    <Modal
      title={set ? t("programs.editSet") : t("programs.addSet")}
      onClose={onClose}
      actions={
        <>
          <button className="sf-button sf-button-secondary" onClick={onClose} disabled={busy}>
            {t("common.cancel")}
          </button>
          <button className="sf-button" onClick={submit} disabled={busy || !form.exerciseId || Number(form.reps) <= 0}>
            {t("common.save")}
          </button>
        </>
      }
    >
      <div className="sf-field">
        <label className="sf-label">{t("session.exercise")}</label>
        <select className="sf-select" value={form.exerciseId} onChange={setField("exerciseId")} autoFocus>
          <option value="">{t("common.none")}</option>
          {Object.entries(grouped).map(([category, list]) => (
            <optgroup
              key={category}
              label={t(`exercises.category${category[0].toUpperCase()}${category.slice(1)}`)}
            >
              {list.map((exercise) => (
                <option key={exercise.id} value={exercise.id}>
                  {exercise.labels?.[locale] || exercise.labels?.en || exercise.slug}
                </option>
              ))}
            </optgroup>
          ))}
        </select>
      </div>

      <div className="sf-field">
        <label className="sf-label">{t("session.loadMode")}</label>
        <select
          className="sf-select"
          value={form.loadMode}
          onChange={setField("loadMode")}
          disabled={selected?.bodyweight}
        >
          <option value="rpe">{t("session.loadModeRpe")}</option>
          <option value="percentage">{t("session.loadModePercentage")}</option>
          <option value="absolute">{t("session.loadModeAbsolute")}</option>
          <option value="bodyweight">{t("session.loadModeBodyweight")}</option>
        </select>
        <p className="sf-muted">{t("session.loadModeHelp")}</p>
      </div>

      <div className="sf-row" style={{ gap: "0.6rem", alignItems: "flex-start" }}>
        <div className="sf-field" style={{ flex: 1, minWidth: 110 }}>
          <label className="sf-label">{t("session.reps")}</label>
          <input className="sf-input" type="number" min="1" value={form.reps} onChange={setField("reps")} />
        </div>

        {form.loadMode === "rpe" ? (
          <div className="sf-field" style={{ flex: 1, minWidth: 110 }}>
            <label className="sf-label">{t("session.rpe")}</label>
            <input
              className="sf-input"
              type="number"
              min="1"
              max="10"
              step="0.5"
              value={form.rpe}
              onChange={setField("rpe")}
            />
          </div>
        ) : null}

        {form.loadMode === "percentage" ? (
          <div className="sf-field" style={{ flex: 1, minWidth: 110 }}>
            <label className="sf-label">{t("session.percentage")}</label>
            <input
              className="sf-input"
              type="number"
              min="1"
              max="150"
              value={form.percentage}
              onChange={setField("percentage")}
            />
          </div>
        ) : null}

        {form.loadMode === "absolute" ? (
          <div className="sf-field" style={{ flex: 1, minWidth: 110 }}>
            <label className="sf-label">
              {t("session.absoluteLoad")} ({t("common.kg")})
            </label>
            <input
              className="sf-input"
              type="number"
              min="0"
              step="0.5"
              value={form.absoluteLoad}
              onChange={setField("absoluteLoad")}
            />
          </div>
        ) : null}
      </div>

      <div className="sf-field">
        <label className="sf-label">
          {t("session.notes")} <span className="sf-muted">({t("common.optional")})</span>
        </label>
        <input className="sf-input" value={form.notes} onChange={setField("notes")} />
      </div>

      <ErrorMessage error={error} />
    </Modal>
  );
}
