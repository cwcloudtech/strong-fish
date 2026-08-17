import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "react-toastify";
import { FiTrash2 } from "react-icons/fi";

import { exercises as exercisesApi, oneRms as oneRmsApi } from "../../api/services";
import { ConfirmModal } from "../../components/common/Modal";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

const label = (item, locale) => item.labels?.[locale] || item.labels?.en || item.slug;

/**
 * The member's maxes. This is the single input the whole training side hangs
 * off: every prescribed load is computed from these, so changing one here
 * recalculates every program the member is running.
 */
export default function OneRms() {
  const { t, locale } = useI18n();
  const [catalog, setCatalog] = useState(null);
  const [maxes, setMaxes] = useState(null);
  const [drafts, setDrafts] = useState({});
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(null);
  const [confirming, setConfirming] = useState(null);

  const load = useCallback(async () => {
    try {
      const [catalogResult, maxesResult] = await Promise.all([exercisesApi.list(), oneRmsApi.list()]);
      setCatalog(catalogResult);
      setMaxes(maxesResult);
      setDrafts(Object.fromEntries(maxesResult.map((max) => [max.exerciseId, String(max.value)])));
    } catch (err) {
      setError(err);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const byExercise = useMemo(() => new Map((maxes || []).map((max) => [max.exerciseId, max])), [maxes]);

  // The three competition lifts come first and are always listed, even without a
  // value: they're what most prescriptions resolve against, so an empty one is
  // the thing worth prompting for. Other movements only appear once the member
  // has actually recorded a max for them.
  // `extra` holds movements the member added by hand this session but hasn't
  // saved a value for yet, so the row stays visible while they type.
  const [extra, setExtra] = useState([]);

  const { mainLifts, otherLifts, addable } = useMemo(() => {
    const all = catalog || [];
    const shown = all.filter(
      (exercise) => !exercise.main && (byExercise.has(exercise.id) || extra.includes(exercise.id))
    );
    return {
      mainLifts: all.filter((exercise) => exercise.main),
      otherLifts: shown,
      addable: all.filter(
        (exercise) => !exercise.main && !exercise.bodyweight && !shown.some((item) => item.id === exercise.id)
      ),
    };
  }, [catalog, byExercise, extra]);

  const save = async (exercise) => {
    const value = Number(drafts[exercise.id]);
    if (!value || value <= 0) {
      setError(t("errors.invalidOneRm"));
      return;
    }
    setSaving(exercise.id);
    setError(null);
    try {
      await oneRmsApi.set(exercise.id, value);
      toast.success(t("oneRms.saved"));
      await load();
    } catch (err) {
      setError(err);
    } finally {
      setSaving(null);
    }
  };

  const remove = async (exercise) => {
    setConfirming(null);
    try {
      await oneRmsApi.remove(exercise.id);
      toast.success(t("oneRms.removed"));
      await load();
    } catch (err) {
      setError(err);
    }
  };

  if (!catalog || !maxes) return <Spinner />;

  const renderRow = (exercise) => {
    const current = byExercise.get(exercise.id);
    return (
      <tr key={exercise.id}>
        <td>
          {label(exercise, locale)}
          {exercise.main ? <span className="sf-badge" style={{ marginLeft: "0.4rem" }}>{t(`exercises.category${exercise.category[0].toUpperCase()}${exercise.category.slice(1)}`)}</span> : null}
        </td>
        <td style={{ width: 150 }}>
          <div className="sf-row" style={{ gap: "0.3rem", flexWrap: "nowrap" }}>
            <input
              className="sf-input sf-input-sm"
              type="number"
              min="0"
              step="0.5"
              value={drafts[exercise.id] ?? ""}
              onChange={(event) => setDrafts((current_) => ({ ...current_, [exercise.id]: event.target.value }))}
              onKeyDown={(event) => event.key === "Enter" && save(exercise)}
            />
            <span className="sf-muted">{t("common.kg")}</span>
          </div>
        </td>
        <td className="sf-muted" style={{ whiteSpace: "nowrap" }}>
          {current ? new Date(current.updatedAt).toLocaleDateString(locale) : "—"}
        </td>
        <td className="sf-muted">
          {current?.history?.length ? current.history.slice(-4).map((entry) => entry.value).join(" → ") + " → " + current.value : "—"}
        </td>
        <td className="sf-table-num">
          <div className="sf-row" style={{ justifyContent: "flex-end", gap: "0.3rem", flexWrap: "nowrap" }}>
            <button
              type="button"
              className="sf-button sf-button-sm"
              onClick={() => save(exercise)}
              disabled={saving === exercise.id || !drafts[exercise.id]}
            >
              {t("common.save")}
            </button>
            {current ? (
              <button
                type="button"
                className="sf-button sf-button-ghost sf-button-sm"
                onClick={() => setConfirming(exercise)}
                aria-label={t("common.delete")}
              >
                <FiTrash2 />
              </button>
            ) : null}
          </div>
        </td>
      </tr>
    );
  };

  return (
    <div className="sf-page">
      <div className="sf-page-header">
        <div>
          <h1>{t("oneRms.title")}</h1>
          <p className="sf-subtitle">{t("oneRms.subtitle")}</p>
        </div>
      </div>

      <ErrorMessage error={error} />

      <div className="sf-card">
        <h3>{t("oneRms.mainLifts")}</h3>
        <div className="sf-table-wrapper">
          <table className="sf-table">
            <thead>
              <tr>
                <th>{t("oneRms.lift")}</th>
                <th>{t("oneRms.value")}</th>
                <th>{t("oneRms.updated")}</th>
                <th>{t("oneRms.history")}</th>
                <th />
              </tr>
            </thead>
            <tbody>{mainLifts.map(renderRow)}</tbody>
          </table>
        </div>
      </div>

      <div className="sf-card">
        <h3>{t("oneRms.otherLifts")}</h3>
        <p className="sf-muted">{t("exercises.oneRmRefHelp")}</p>

        {otherLifts.length === 0 ? (
          <EmptyState message={t("oneRms.empty")} />
        ) : (
          <div className="sf-table-wrapper">
            <table className="sf-table">
              <thead>
                <tr>
                  <th>{t("oneRms.lift")}</th>
                  <th>{t("oneRms.value")}</th>
                  <th>{t("oneRms.updated")}</th>
                  <th>{t("oneRms.history")}</th>
                  <th />
                </tr>
              </thead>
              <tbody>{otherLifts.map(renderRow)}</tbody>
            </table>
          </div>
        )}

        {addable.length > 0 ? (
          <div className="sf-field" style={{ maxWidth: 320, marginTop: "0.8rem", marginBottom: 0 }}>
            <label className="sf-label" htmlFor="add-lift">
              {t("oneRms.addLift")}
            </label>
            <select
              id="add-lift"
              className="sf-select"
              value=""
              onChange={(event) => event.target.value && setExtra((current) => [...current, event.target.value])}
            >
              <option value="">{t("common.none")}</option>
              {addable.map((exercise) => (
                <option key={exercise.id} value={exercise.id}>
                  {label(exercise, locale)}
                </option>
              ))}
            </select>
          </div>
        ) : null}
      </div>

      {confirming ? (
        <ConfirmModal
          title={t("common.delete")}
          message={t("oneRms.confirmDelete", { name: label(confirming, locale) })}
          onConfirm={() => remove(confirming)}
          onClose={() => setConfirming(null)}
        />
      ) : null}
    </div>
  );
}
