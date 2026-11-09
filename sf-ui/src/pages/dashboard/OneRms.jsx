import { Fragment, useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "react-toastify";
import { FiSave, FiTrash2 } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import { devices as devicesApi, exercises as exercisesApi, oneRms as oneRmsApi } from "../../api/services";
import { ConfirmModal } from "../../components/common/Modal";
import Tooltip from "../../components/common/Tooltip";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

const label = (item, locale) => item.labels?.[locale] || item.labels?.en || item.slug;

// Velocity loss across the set, as a whole percent: how much the fastest rep
// slowed to the last - the same figure the export's VBT column shows.
function velocityLoss(velocities) {
  const vs = velocities || [];
  if (vs.length < 2) return 0;
  const best = Math.max(...vs);
  const last = vs[vs.length - 1];
  return best > 0 && last < best ? Math.round(((best - last) / best) * 100) : 0;
}

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
  // The last bar-speed set of each lift, to show its reps and per-rep speeds
  // under the matching max. Best-effort and optional: a member with no tracker
  // just sees the maxes, so a failed read must never blank the page.
  const [barSets, setBarSets] = useState([]);

  const load = useCallback(async () => {
    try {
      const [catalogResult, maxesResult] = await Promise.all([exercisesApi.list(), oneRmsApi.list()]);
      setCatalog(catalogResult);
      setMaxes(maxesResult);
      setDrafts(Object.fromEntries(maxesResult.map((max) => [max.exerciseId, String(max.value)])));
    } catch (err) {
      setError(err);
    }
    try {
      setBarSets(await devicesApi.barSpeedList());
    } catch {
      setBarSets([]);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const byExercise = useMemo(() => new Map((maxes || []).map((max) => [max.exerciseId, max])), [maxes]);
  // Keyed by lift (squat/bench/deadlift), which is a main lift's own category.
  const barByLift = useMemo(
    () => new Map((barSets || []).filter((set) => set.repVelocities?.length).map((set) => [set.lift, set])),
    [barSets],
  );

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
      toast.success(t("oneRms.saved"), toastOptions);
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
      toast.success(t("oneRms.removed"), toastOptions);
      await load();
    } catch (err) {
      setError(err);
    }
  };

  if (!catalog || !maxes) return <Spinner />;

  const renderRow = (exercise) => {
    const current = byExercise.get(exercise.id);
    // A competition lift's recorded set, keyed by its category (squat/bench/
    // deadlift). Shown under the max as its reps and each rep's speed.
    const set = exercise.main ? barByLift.get(exercise.category) : null;
    return (
      <Fragment key={exercise.id}>
      <tr>
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
          {/* Icon actions, like every other row in the app: a table of maxes
              is scanned down the left, and a column of repeated "Save" words
              is noise where an icon is a target. Enter in the field saves too,
              which is what most people will actually use. */}
          <div className="sf-row" style={{ justifyContent: "flex-end", gap: "0.3rem", flexWrap: "nowrap" }}>
            <Tooltip label={t("common.save")}>
              <button
                type="button"
                className="sf-icon-button sf-icon-button-plain"
                onClick={() => save(exercise)}
                disabled={saving === exercise.id || !drafts[exercise.id]}
                aria-label={t("common.save")}
              >
                <FiSave />
              </button>
            </Tooltip>
            {current ? (
              <Tooltip label={t("common.delete")}>
                <button
                  type="button"
                  className="sf-icon-button sf-icon-button-plain"
                  onClick={() => setConfirming(exercise)}
                  aria-label={t("common.delete")}
                >
                  <FiTrash2 />
                </button>
              </Tooltip>
            ) : null}
          </div>
        </td>
      </tr>
      {set ? (
        <tr>
          <td colSpan={5} className="sf-muted" style={{ fontSize: "0.85rem", paddingTop: 0, borderTop: "none" }}>
            <strong>VBT</strong>
            {" - "}
            {t("devices.barSpeed.detectedReps", { count: set.reps })}
            {" - "}
            {velocityLoss(set.repVelocities)}%
          </td>
        </tr>
      ) : null}
      </Fragment>
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
