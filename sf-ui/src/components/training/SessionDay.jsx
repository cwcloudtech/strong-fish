import { Fragment, useState } from "react";
import { FiCheck, FiChevronDown, FiChevronRight, FiEdit2, FiX } from "react-icons/fi";

import SetLogForm from "./SetLogForm";
import Select from "../common/Select";
import { formatReps } from "../../utils/setFormat";
import { useI18n } from "../../i18n/I18nContext";
import { sessionTitle } from "../../utils/sessionTitle";
import ExportMenu from "../programs/ExportMenu";

/**
 * The perceived-RPE values on offer, as the chart itself is written: half
 * points from 6 up, and nothing below - "it moved" is not an RPE, and the
 * chart has no row for one.
 */
const PERCEIVED_RPE = [6, 6.5, 7, 7.5, 8, 8.5, 9, 9.5, 10];

/**
 * Done, or not done.
 *
 * Green when the set (or the session) is finished, red when it is not, and it
 * flips on click - the two states a member is ever in while running a program.
 * There is no third state to clear back to: a set they have not done yet is
 * simply not done.
 */
function DoneButton({ done, label, onClick }) {
  return (
    <button
      type="button"
      className={`sf-done-toggle ${done ? "is-done" : "is-todo"}`}
      onClick={onClick}
      aria-pressed={done}
      aria-label={label}
      title={label}
    >
      {done ? <FiCheck /> : <FiX />}
    </button>
  );
}

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
export default function SessionDay({
  day,
  locale,
  defaultOpen = false,
  editable = false,
  onLog,
  onDayDone,
  onExport,
  exporting = false,
}) {
  const { t } = useI18n();
  const [open, setOpen] = useState(defaultOpen);
  const [logging, setLogging] = useState(null);

  // The API replaces a set's log wholesale, so each of these has to carry
  // whatever else was already logged with it - otherwise picking an RPE would
  // quietly delete the note the member left, and ticking a set off would
  // delete the RPE.
  const logWith = (set, changes) =>
    onLog(set, {
      actualReps: set.log?.actualReps ?? null,
      actualLoad: set.log?.actualLoad ?? null,
      actualRpe: set.log?.actualRpe ?? null,
      beltless: set.log?.beltless ?? false,
      comment: set.log?.comment || "",
      done: set.log?.done ?? false,
      ...changes,
    });

  // Picking how a set felt is also saying you did it: nobody rates a set they
  // have not run.
  const onPerceivedRpe = (set, value) => logWith(set, { actualRpe: Number(value), done: true });

  const sets = day.sets || [];
  const done = sets.filter((set) => set.log?.done).length;
  const allDone = sets.length > 0 && done === sets.length;

  return (
    <div className="sf-card sf-session-day">
      <div className="sf-session-day-header" onClick={() => setOpen((current) => !current)}>
        <div className="sf-row">
          {open ? <FiChevronDown /> : <FiChevronRight />}
          <h3 style={{ margin: 0 }}>
            {t("programs.week", { week: day.week })} · {t("programs.day", { day: day.day })}
            {/* The name the coach gave this session, when they gave it one:
                it was saved and then shown nowhere, so a titled session was
                indistinguishable from an untitled one. */}
            {sessionTitle(day) ? <span className="sf-session-name"> · {sessionTitle(day)}</span> : null}
          </h3>
          <span className="sf-badge sf-badge-muted">{t("programs.setCount", { count: sets.length })}</span>
        </div>
        <div className="sf-row" style={{ gap: "0.5rem" }}>
          {editable ? (
            <span className={`sf-badge ${allDone ? "sf-badge-success" : "sf-badge-muted"}`}>
              {t("session.progress", { done, total: sets.length })}
            </span>
          ) : null}
          {/* One session on its own, beside the week's own export: a session is
              what gets discussed the evening it was trained, and sending a
              twelve-page block to say one thing about one day is why nobody
              sends it. Offered to whoever may read the session, like the
              week's - a coach exporting their athlete's day is not editing it.
              The click is stopped short of the header, which would otherwise
              collapse the panel underneath the menu. */}
          {onExport ? (
            <span
              onClick={(event) => event.stopPropagation()}
              onKeyDown={(event) => event.stopPropagation()}
              role="presentation"
            >
              <ExportMenu
                onExport={onExport}
                busy={exporting}
                label={t("session.exportSession")}
                size="sf-button-sm"
              />
            </span>
          ) : null}
          {/* The whole session in one tap. It sits in the header rather than
              among the sets because it is about the session, and it stops the
              click from opening the panel underneath it. */}
          {editable ? (
            <DoneButton
              done={allDone}
              label={allDone ? t("session.markDayUndone") : t("session.markDayDone")}
              onClick={(event) => {
                event.stopPropagation();
                onDayDone(day, !allDone);
              }}
            />
          ) : null}
        </div>
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
                        {/* A sign on the set, not a control: it is switched in
                            the log form. Here it says how the set was run, to
                            the athlete and to the coach reading it back. */}
                        {set.log?.beltless ? (
                          <span className="sf-badge sf-badge-warning" style={{ marginLeft: "0.4rem" }}>
                            {t("session.beltless")}
                          </span>
                        ) : null}
                        {set.notes ? <div className="sf-muted">{set.notes}</div> : null}
                      </td>
                      <td className="sf-table-num">
                        {/* What the athlete actually got, for the same reason
                            the load reads that way: a set logged at 3 reps has
                            no business still saying 5. */}
                        {set.log?.actualReps != null ? (
                          <span
                            className="sf-logged"
                            title={
                              set.reps > 0
                                ? t("session.loggedRepsPlanned", { value: set.reps })
                                : t("session.loggedReps")
                            }
                          >
                            {set.log.actualReps}
                          </span>
                        ) : (
                          formatReps(set)
                        )}
                      </td>
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
                        {/* The used load belongs here too: it is a real bar
                            weight, and a row reading 130 used next to 122.5 on
                            the bar describes a set nobody did. */}
                        {set.log?.actualLoad != null ? (
                          <span
                            className="sf-logged"
                            title={
                              set.loadKnown && set.roundedLoad
                                ? t("session.loggedLoadPlanned", {
                                    value: `${set.roundedLoad} ${t("common.kg")}`,
                                  })
                                : t("session.loggedLoad")
                            }
                          >
                            {set.log.actualLoad} {t("common.kg")}
                          </span>
                        ) : set.loadKnown && set.roundedLoad ? (
                          `${set.roundedLoad} ${t("common.kg")}`
                        ) : (
                          "—"
                        )}
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
                          />
                        </td>
                      ) : null}
                      {editable ? (
                        <td className="sf-table-num">
                          <div className="sf-row" style={{ gap: "0.3rem", justifyContent: "flex-end" }}>
                            {/* Done or not done, and nothing else: the two
                                states a set is in while a session is being
                                run. */}
                            <DoneButton
                              done={Boolean(set.log?.done)}
                              label={set.log?.done ? t("session.markUndone") : t("session.markDone")}
                              onClick={() => logWith(set, { done: !set.log?.done })}
                            />
                            {/* Everything the two controls above do not carry:
                                the reps and the weight when they differed from
                                the prescription, and a note for the coach. */}
                            <button
                              type="button"
                              className="sf-button sf-button-ghost sf-button-sm"
                              onClick={() => setLogging(isLogging ? null : set.id)}
                              aria-label={t("session.log")}
                              title={t("session.log")}
                            >
                              <FiEdit2 />
                            </button>
                          </div>
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
