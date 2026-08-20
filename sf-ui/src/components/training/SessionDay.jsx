import { Fragment, useState } from "react";
import { FiChevronDown, FiChevronRight, FiEdit2 } from "react-icons/fi";

import SetLogForm from "./SetLogForm";
import Select from "../common/Select";
import { formatReps } from "../../utils/setFormat";
import { useI18n } from "../../i18n/I18nContext";

/**
 * The perceived-RPE values on offer, as the chart itself is written: half
 * points from 6 up, and nothing below - "it moved" is not an RPE, and the
 * chart has no row for one.
 */
const PERCEIVED_RPE = [6, 6.5, 7, 7.5, 8, 8.5, 9, 9.5, 10];

/** The exercise's name in the reader's language, falling back to English. */
export function exerciseLabel(set, locale) {
  return set.exerciseLabels?.[locale] || set.exerciseLabels?.en || set.exerciseSlug;
}

/**
 * One training session: the prescribed sets with the weight this member should
 * actually lift, plus their logged feedback.
 *
 * The load column is the point of the whole app - it's computed server-side from
 * the member's own current 1RM, so it changes the moment they update a max. A
 * "?" means they haven't recorded the max it would come from yet.
 */
export default function SessionDay({ day, locale, defaultOpen = false, editable = false, onLog, onClearLog }) {
  const { t } = useI18n();
  const [open, setOpen] = useState(defaultOpen);
  const [logging, setLogging] = useState(null);

  // The API replaces a set's log wholesale, so picking an RPE has to carry
  // whatever else was already logged with it - otherwise choosing "8" would
  // quietly delete the note the member left and the weight they typed.
  const onPerceivedRpe = (set, value) => {
    if (!value) return onClearLog(set);
    return onLog(set, {
      actualReps: set.log?.actualReps ?? null,
      actualLoad: set.log?.actualLoad ?? null,
      actualRpe: Number(value),
      comment: set.log?.comment || "",
      done: true,
    });
  };

  const sets = day.sets || [];
  const done = sets.filter((set) => set.log?.done).length;

  return (
    <div className="sf-card sf-session-day">
      <div className="sf-session-day-header" onClick={() => setOpen((current) => !current)}>
        <div className="sf-row">
          {open ? <FiChevronDown /> : <FiChevronRight />}
          <h3 style={{ margin: 0 }}>
            {t("programs.week", { week: day.week })} · {t("programs.day", { day: day.day })}
          </h3>
          <span className="sf-badge sf-badge-muted">{t("programs.setCount", { count: sets.length })}</span>
        </div>
        {editable ? (
          <span className={`sf-badge ${done === sets.length && sets.length > 0 ? "sf-badge-success" : "sf-badge-muted"}`}>
            {t("session.progress", { done, total: sets.length })}
          </span>
        ) : null}
      </div>

      {open ? (
        <div className="sf-table-wrapper" style={{ marginTop: "0.8rem" }}>
          <table className="sf-table">
            <thead>
              <tr>
                <th>{t("session.exercise")}</th>
                <th className="sf-table-num">{t("session.reps")}</th>
                <th className="sf-table-num">{t("session.rpe")}</th>
                <th className="sf-table-num">{t("session.percentage")}</th>
                <th className="sf-table-num">{t("session.load")}</th>
                <th className="sf-table-num">{t("session.onTheBar")}</th>
                {editable ? <th className="sf-table-num">{t("session.perceivedRpe")}</th> : null}
                {editable ? <th /> : null}
              </tr>
            </thead>
            <tbody>
              {sets.map((set) => {
                const isLogging = logging === set.id;
                return (
                  <Fragment key={set.id}>
                    <tr className={set.log?.done ? "sf-set-done" : undefined}>
                      <td>
                        {exerciseLabel(set, locale)}
                        {set.loadMode === "bodyweight" ? (
                          <span className="sf-badge sf-badge-muted" style={{ marginLeft: "0.4rem" }}>
                            {t("session.bodyweight")}
                          </span>
                        ) : null}
                        {set.notes ? <div className="sf-muted">{set.notes}</div> : null}
                      </td>
                      <td className="sf-table-num">{formatReps(set)}</td>
                      <td className="sf-table-num">{set.rpe ?? t("common.unknown")}</td>
                      <td className="sf-table-num">
                        {set.loadKnown && set.computedPercentage ? `${set.computedPercentage}%` : "—"}
                      </td>
                      <td className="sf-table-num">
                        {set.loadMode === "bodyweight" ? (
                          <span className="sf-muted">—</span>
                        ) : set.loadKnown ? (
                          <span
                            className="sf-load"
                            title={
                              set.autoregulated
                                ? t("session.fromTodaysE1rm", { value: set.oneRm })
                                : set.oneRm
                                  ? t("session.from1rm", {
                                      lift: set.exerciseOneRmRef || set.exerciseSlug,
                                      value: set.oneRm,
                                    })
                                  : undefined
                            }
                          >
                            {set.load} {t("common.kg")}
                            {/* A load that moved because of what was just
                                lifted has to say so, or the weight looks like
                                it changed on its own. */}
                            {set.autoregulated ? <span className="sf-autoregulated" aria-hidden="true"> ⟳</span> : null}
                          </span>
                        ) : (
                          <span className="sf-load-unknown" title={t("session.missingOneRm")}>
                            {t("common.unknown")}
                          </span>
                        )}
                      </td>
                      <td className="sf-table-num">
                        {set.loadKnown && set.roundedLoad ? `${set.roundedLoad} ${t("common.kg")}` : "—"}
                      </td>
                      {editable ? (
                        <td className="sf-table-num">
                          {/* The whole log, in one control: pick how the set
                              actually felt and it is saved there and then, and
                              the sets after it are re-resolved against the max
                              that effort demonstrates. The form below is still
                              there for the rest - reps, weight, a note. */}
                          <Select
                            className="sf-rpe-select"
                            options={PERCEIVED_RPE.map((value) => ({ value: String(value), label: String(value) }))}
                            value={set.log?.actualRpe != null ? String(set.log.actualRpe) : ""}
                            onChange={(value) => onPerceivedRpe(set, value)}
                            placeholder={t("session.rpePlaceholder")}
                            clearable={set.log?.actualRpe != null}
                          />
                        </td>
                      ) : null}
                      {editable ? (
                        <td className="sf-table-num">
                          {/* Everything the dropdown does not carry: the reps
                              and the weight when they differed from the
                              prescription, and a note for the coach. */}
                          <button
                            type="button"
                            className="sf-button sf-button-ghost sf-button-sm"
                            onClick={() => setLogging(isLogging ? null : set.id)}
                            aria-label={t("session.log")}
                            title={t("session.log")}
                          >
                            <FiEdit2 />
                          </button>
                        </td>
                      ) : null}
                    </tr>

                    {(set.log?.comment || set.log?.e1rm) && !isLogging ? (
                      <tr className="sf-set-log-row">
                        <td colSpan={editable ? 8 : 6} className="sf-muted">
                          {set.log.comment ? `“${set.log.comment}”` : null}
                          {set.log.e1rm
                            ? `${set.log.comment ? " · " : ""}${t("session.e1rm")} ${set.log.e1rm} ${t("common.kg")}`
                            : ""}
                        </td>
                      </tr>
                    ) : null}

                    {isLogging ? (
                      <tr className="sf-set-log-row">
                        <td colSpan={editable ? 8 : 6}>
                          <SetLogForm
                            set={set}
                            onSubmit={async (payload) => {
                              await onLog(set, payload);
                              setLogging(null);
                            }}
                            onClear={
                              set.log
                                ? async () => {
                                    await onClearLog(set);
                                    setLogging(null);
                                  }
                                : null
                            }
                            onCancel={() => setLogging(null)}
                          />
                        </td>
                      </tr>
                    ) : null}
                  </Fragment>
                );
              })}
            </tbody>
          </table>
        </div>
      ) : null}
    </div>
  );
}
