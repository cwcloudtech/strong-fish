import { useState } from "react";
import { FiEdit2, FiPlus, FiTrash2 } from "react-icons/fi";

import Modal, { ConfirmModal } from "../common/Modal";
import SetFormModal from "./SetFormModal";
import { ErrorMessage } from "../common/Feedback";
import { programs as programsApi } from "../../api/services";
import { useI18n } from "../../i18n/I18nContext";

const exerciseLabel = (set, locale) =>
  set.exerciseLabels?.[locale] || set.exerciseLabels?.en || set.exerciseSlug;

/**
 * One session in edit mode: its sets as an editable list, plus the actions that
 * change the session itself.
 *
 * This is the authoring counterpart of SessionDay (which renders a session for
 * training, with computed loads). They're kept apart on purpose - a coach
 * writing a program cares about the prescription, an athlete running it cares
 * about the weight, and merging both into one component would mean every row
 * carrying two sets of controls.
 */
export default function SessionEditor({ clubId, programId, day, locale, onChanged }) {
  const { t } = useI18n();
  const [editingSet, setEditingSet] = useState(null);
  const [deletingSet, setDeletingSet] = useState(null);
  const [editingDay, setEditingDay] = useState(false);
  const [deletingDay, setDeletingDay] = useState(false);
  const [error, setError] = useState(null);

  const removeSet = async () => {
    try {
      await programsApi.removeSet(clubId, programId, deletingSet.id);
      setDeletingSet(null);
      onChanged();
    } catch (err) {
      setError(err);
      setDeletingSet(null);
    }
  };

  const removeDay = async () => {
    try {
      await programsApi.removeDay(clubId, programId, day.id);
      setDeletingDay(false);
      onChanged();
    } catch (err) {
      setError(err);
      setDeletingDay(false);
    }
  };

  const sets = day.sets || [];

  return (
    <div className="sf-card">
      <div className="sf-row-between">
        <h3 style={{ margin: 0 }}>
          {t("programs.week", { week: day.week })} · {t("programs.day", { day: day.day })}
        </h3>
        <div className="sf-row" style={{ gap: "0.25rem" }}>
          <button className="sf-button sf-button-ghost sf-button-sm" onClick={() => setEditingDay(true)}>
            <FiEdit2 /> {t("common.edit")}
          </button>
          <button className="sf-button sf-button-ghost sf-button-sm" onClick={() => setDeletingDay(true)}>
            <FiTrash2 /> {t("common.delete")}
          </button>
        </div>
      </div>

      <ErrorMessage error={error} />

      <div className="sf-table-wrapper" style={{ marginTop: "0.6rem" }}>
        <table className="sf-table">
          <thead>
            <tr>
              <th>{t("session.exercise")}</th>
              <th className="sf-table-num">{t("session.reps")}</th>
              <th>{t("session.loadMode")}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {sets.map((set) => (
              <tr key={set.id}>
                <td>
                  {exerciseLabel(set, locale)}
                  {set.notes ? <div className="sf-muted">{set.notes}</div> : null}
                </td>
                <td className="sf-table-num">{set.reps}</td>
                <td className="sf-muted">{describeLoad(t, set)}</td>
                <td className="sf-table-num">
                  <div className="sf-row" style={{ justifyContent: "flex-end", gap: "0.25rem" }}>
                    <button
                      className="sf-button sf-button-ghost sf-button-sm"
                      onClick={() => setEditingSet(set)}
                      aria-label={t("common.edit")}
                    >
                      <FiEdit2 />
                    </button>
                    <button
                      className="sf-button sf-button-ghost sf-button-sm"
                      onClick={() => setDeletingSet(set)}
                      aria-label={t("common.delete")}
                    >
                      <FiTrash2 />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <button
        className="sf-button sf-button-secondary sf-button-sm"
        style={{ marginTop: "0.6rem" }}
        onClick={() => setEditingSet({})}
      >
        <FiPlus /> {t("programs.addSet")}
      </button>

      {editingSet ? (
        <SetFormModal
          clubId={clubId}
          programId={programId}
          day={day}
          set={editingSet.id ? editingSet : null}
          onClose={() => setEditingSet(null)}
          onSaved={() => {
            setEditingSet(null);
            onChanged();
          }}
        />
      ) : null}

      {deletingSet ? (
        <ConfirmModal
          title={t("common.delete")}
          message={t("programs.confirmDeleteSet")}
          onConfirm={removeSet}
          onClose={() => setDeletingSet(null)}
        />
      ) : null}

      {editingDay ? (
        <SessionFormModal
          clubId={clubId}
          programId={programId}
          day={day}
          onClose={() => setEditingDay(false)}
          onSaved={() => {
            setEditingDay(false);
            onChanged();
          }}
        />
      ) : null}

      {deletingDay ? (
        <ConfirmModal
          title={t("common.delete")}
          message={t("programs.confirmDeleteSession", { week: day.week, day: day.day })}
          onConfirm={removeDay}
          onClose={() => setDeletingDay(false)}
        />
      ) : null}
    </div>
  );
}

/** A one-line summary of how a set is loaded, for the authoring table. */
function describeLoad(t, set) {
  switch (set.loadMode) {
    case "rpe":
      return `${t("session.rpe")} ${set.rpe ?? "—"}`;
    case "percentage":
      return `${set.percentage ?? "—"}%`;
    case "absolute":
      return `${set.absoluteLoad ?? "—"} ${t("common.kg")}`;
    default:
      return t("session.bodyweight");
  }
}

/** Creates or renumbers a session. */
export function SessionFormModal({ clubId, programId, day, onClose, onSaved }) {
  const { t } = useI18n();
  const [form, setForm] = useState({
    week: day?.week ?? "",
    day: day?.day ?? "",
    title: day?.title || "",
  });
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const set = (field) => (event) => setForm((current) => ({ ...current, [field]: event.target.value }));

  const submit = async () => {
    setBusy(true);
    setError(null);
    const payload = {
      week: form.week === "" ? 0 : Number(form.week),
      day: form.day === "" ? 0 : Number(form.day),
      title: form.title,
    };
    try {
      if (day) await programsApi.updateDay(clubId, programId, day.id, payload);
      else await programsApi.addDay(clubId, programId, payload);
      onSaved();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  // Adding a session with the numbers left blank continues the program's own
  // numbering server-side, which is what a coach filling a block wants.
  const valid = day ? Number(form.week) > 0 && Number(form.day) > 0 : true;

  return (
    <Modal
      title={day ? t("programs.editSession") : t("programs.addSession")}
      onClose={onClose}
      actions={
        <>
          <button className="sf-button sf-button-secondary" onClick={onClose} disabled={busy}>
            {t("common.cancel")}
          </button>
          <button className="sf-button" onClick={submit} disabled={busy || !valid}>
            {t("common.save")}
          </button>
        </>
      }
    >
      <div className="sf-row" style={{ gap: "0.6rem", alignItems: "flex-start" }}>
        <div className="sf-field" style={{ flex: 1, minWidth: 110 }}>
          <label className="sf-label">
            {t("session.week")}
            {day ? "" : ` (${t("common.optional")})`}
          </label>
          <input className="sf-input" type="number" min="1" value={form.week} onChange={set("week")} />
        </div>
        <div className="sf-field" style={{ flex: 1, minWidth: 110 }}>
          <label className="sf-label">
            {t("session.day")}
            {day ? "" : ` (${t("common.optional")})`}
          </label>
          <input className="sf-input" type="number" min="1" value={form.day} onChange={set("day")} />
        </div>
      </div>
      <div className="sf-field">
        <label className="sf-label">
          {t("session.title")} <span className="sf-muted">({t("common.optional")})</span>
        </label>
        <input className="sf-input" value={form.title} onChange={set("title")} />
      </div>
      <ErrorMessage error={error} />
    </Modal>
  );
}
