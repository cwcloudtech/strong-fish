import { Fragment, useState } from "react";
import { FiChevronDown, FiChevronRight } from "react-icons/fi";

import SetLogForm from "./SetLogForm";
import { formatReps } from "../../utils/setFormat";
import { useI18n } from "../../i18n/I18nContext";

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
                              set.oneRm
                                ? t("session.from1rm", {
                                    lift: set.exerciseOneRmRef || set.exerciseSlug,
                                    value: set.oneRm,
                                  })
                                : undefined
                            }
                          >
                            {set.load} {t("common.kg")}
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
                          <button
                            type="button"
                            className="sf-button sf-button-ghost sf-button-sm"
                            onClick={() => setLogging(isLogging ? null : set.id)}
                          >
                            {set.log ? `RPE ${set.log.actualRpe ?? "—"}` : t("session.log")}
                          </button>
                        </td>
                      ) : null}
                    </tr>

                    {set.log?.comment && !isLogging ? (
                      <tr className="sf-set-log-row">
                        <td colSpan={editable ? 7 : 6} className="sf-muted">
                          “{set.log.comment}”
                          {set.log.e1rm ? ` · ${t("session.e1rm")} ${set.log.e1rm} ${t("common.kg")}` : ""}
                        </td>
                      </tr>
                    ) : null}

                    {isLogging ? (
                      <tr className="sf-set-log-row">
                        <td colSpan={editable ? 7 : 6}>
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
