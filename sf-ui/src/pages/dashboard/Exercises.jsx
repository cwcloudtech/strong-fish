import { useCallback, useEffect, useState } from "react";
import { toast } from "react-toastify";
import { FiEdit2, FiTrash2 } from "react-icons/fi";

import { exercises as exercisesApi } from "../../api/services";
import Modal, { ConfirmModal } from "../../components/common/Modal";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

const CATEGORIES = ["squat", "bench", "deadlift", "accessory"];
const categoryLabel = (t, category) => t(`exercises.category${category[0].toUpperCase()}${category.slice(1)}`);

/**
 * The shared exercise catalog. It's global rather than per-club on purpose: a
 * coach adds "larsen press" once and it shows up in every other coach's
 * autocomplete, and in every importer that meets that spelling.
 */
export default function Exercises() {
  const { t, locale } = useI18n();
  const [list, setList] = useState(null);
  const [query, setQuery] = useState("");
  const [editing, setEditing] = useState(null);
  const [confirming, setConfirming] = useState(null);
  const [error, setError] = useState(null);

  const load = useCallback(() => {
    exercisesApi.list(query).then(setList).catch(setError);
  }, [query]);

  useEffect(() => {
    const timer = setTimeout(load, query ? 250 : 0);
    return () => clearTimeout(timer);
  }, [load, query]);

  const remove = async (exercise) => {
    setConfirming(null);
    try {
      await exercisesApi.remove(exercise.id);
      toast.success(t("exercises.deleted"));
      load();
    } catch (err) {
      setError(err);
    }
  };

  return (
    <div className="sf-page">
      <div className="sf-page-header">
        <div>
          <h1>{t("exercises.title")}</h1>
          <p className="sf-subtitle">{t("exercises.subtitle")}</p>
        </div>
        <button className="sf-button" onClick={() => setEditing({})}>
          {t("exercises.create")}
        </button>
      </div>

      <div className="sf-field" style={{ maxWidth: 320 }}>
        <input
          className="sf-input"
          placeholder={t("common.search")}
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
      </div>

      <ErrorMessage error={error} />

      {!list ? (
        <Spinner />
      ) : list.length === 0 ? (
        <EmptyState message={t("exercises.empty")} />
      ) : (
        <div className="sf-card">
          <div className="sf-table-wrapper">
            <table className="sf-table">
              <thead>
                <tr>
                  <th>{t("exercises.name")}</th>
                  <th>{t("exercises.category")}</th>
                  <th>{t("exercises.oneRmRef")}</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {list.map((exercise) => (
                  <tr key={exercise.id}>
                    <td>
                      {exercise.labels?.[locale] || exercise.labels?.en || exercise.slug}
                      <div className="sf-muted">{exercise.slug}</div>
                    </td>
                    <td>
                      <span className="sf-badge sf-badge-muted">{categoryLabel(t, exercise.category)}</span>
                      {exercise.bodyweight ? (
                        <span className="sf-badge sf-badge-muted" style={{ marginLeft: "0.3rem" }}>
                          {t("session.bodyweight")}
                        </span>
                      ) : null}
                    </td>
                    <td className="sf-muted">
                      {exercise.oneRmRef ? categoryLabel(t, exercise.oneRmRef) : t("exercises.oneRmRefNone")}
                    </td>
                    <td className="sf-table-num">
                      <div className="sf-row" style={{ justifyContent: "flex-end", gap: "0.25rem" }}>
                        <button className="sf-button-ghost sf-button-sm" onClick={() => setEditing(exercise)} aria-label={t("common.edit")}>
                          <FiEdit2 />
                        </button>
                        <button className="sf-button-ghost sf-button-sm" onClick={() => setConfirming(exercise)} aria-label={t("common.delete")}>
                          <FiTrash2 />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {editing ? (
        <ExerciseFormModal
          exercise={editing.id ? editing : null}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            toast.success(t("exercises.saved"));
            load();
          }}
        />
      ) : null}

      {confirming ? (
        <ConfirmModal
          title={t("common.delete")}
          message={t("exercises.confirmDelete", {
            name: confirming.labels?.[locale] || confirming.labels?.en || confirming.slug,
          })}
          onConfirm={() => remove(confirming)}
          onClose={() => setConfirming(null)}
        />
      ) : null}
    </div>
  );
}

function ExerciseFormModal({ exercise, onClose, onSaved }) {
  const { t } = useI18n();
  const [form, setForm] = useState({
    name: exercise?.labels?.en || "",
    labelEn: exercise?.labels?.en || "",
    labelFr: exercise?.labels?.fr || "",
    aliases: (exercise?.aliases || []).join(", "),
    category: exercise?.category || "accessory",
    oneRmRef: exercise?.oneRmRef || "",
    bodyweight: exercise?.bodyweight || false,
  });
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const set = (field) => (event) =>
    setForm((current) => ({
      ...current,
      [field]: event.target.type === "checkbox" ? event.target.checked : event.target.value,
    }));

  const submit = async () => {
    setBusy(true);
    setError(null);
    const payload = {
      name: form.name || form.labelEn,
      labels: { en: form.labelEn, fr: form.labelFr },
      aliases: form.aliases.split(",").map((alias) => alias.trim()).filter(Boolean),
      category: form.category,
      oneRmRef: form.oneRmRef,
      bodyweight: form.bodyweight,
    };
    try {
      if (exercise) await exercisesApi.update(exercise.id, payload);
      else await exercisesApi.create(payload);
      onSaved();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  return (
    <Modal
      title={exercise ? t("exercises.edit") : t("exercises.create")}
      onClose={onClose}
      actions={
        <>
          <button className="sf-button sf-button-secondary" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className="sf-button" onClick={submit} disabled={busy || !form.labelEn.trim()}>
            {t("common.save")}
          </button>
        </>
      }
    >
      <div className="sf-row" style={{ gap: "0.6rem" }}>
        <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
          <label className="sf-label">{t("exercises.labelEn")}</label>
          <input className="sf-input" value={form.labelEn} onChange={set("labelEn")} autoFocus />
        </div>
        <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
          <label className="sf-label">{t("exercises.labelFr")}</label>
          <input className="sf-input" value={form.labelFr} onChange={set("labelFr")} />
        </div>
      </div>

      <div className="sf-field">
        <label className="sf-label">{t("exercises.aliases")}</label>
        <input className="sf-input" value={form.aliases} onChange={set("aliases")} />
        <p className="sf-muted" style={{ marginTop: "0.25rem" }}>
          {t("exercises.aliasesHelp")}
        </p>
      </div>

      <div className="sf-row" style={{ gap: "0.6rem" }}>
        <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
          <label className="sf-label">{t("exercises.category")}</label>
          <select className="sf-select" value={form.category} onChange={set("category")}>
            {CATEGORIES.map((category) => (
              <option key={category} value={category}>
                {categoryLabel(t, category)}
              </option>
            ))}
          </select>
        </div>
        <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
          <label className="sf-label">{t("exercises.oneRmRef")}</label>
          <select className="sf-select" value={form.oneRmRef} onChange={set("oneRmRef")}>
            <option value="">{t("exercises.oneRmRefNone")}</option>
            {CATEGORIES.filter((category) => category !== "accessory").map((category) => (
              <option key={category} value={category}>
                {categoryLabel(t, category)}
              </option>
            ))}
          </select>
        </div>
      </div>
      <p className="sf-muted" style={{ marginTop: "-0.5rem" }}>
        {t("exercises.oneRmRefHelp")}
      </p>

      <label className="sf-checkbox">
        <input type="checkbox" checked={form.bodyweight} onChange={set("bodyweight")} />
        {t("exercises.bodyweight")}
      </label>

      <ErrorMessage error={error} />
    </Modal>
  );
}
