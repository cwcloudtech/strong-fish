import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { training } from "../../api/services";
import ExportMenu from "../../components/programs/ExportMenu";
import downloadBlob from "../../utils/downloadBlob";
import { ErrorMessage, Spinner } from "../../components/common/Feedback";
import SessionDay, { exerciseLabel } from "../../components/training/SessionDay";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";

/**
 * The sessions grouped by week, in order.
 *
 * The same grouping the exports use server-side (programsheet.Weeks): a week
 * number can be skipped by an imported spreadsheet, so the weeks are read off
 * the sessions rather than assumed contiguous.
 */
function weeksOf(days) {
  const byNumber = new Map();
  for (const day of days) {
    if (!byNumber.has(day.week)) byNumber.set(day.week, []);
    byNumber.get(day.week).push(day);
  }
  return [...byNumber.entries()]
    .sort(([a], [b]) => a - b)
    .map(([number, weekDays]) => ({ number, days: weekDays }));
}

/**
 * One assigned program, session by session. Every load on screen was computed
 * server-side against this member's current 1RMs, so after logging a set (or
 * updating a max elsewhere) the whole thing is simply re-fetched rather than
 * patched locally - there's no derived state to keep in sync.
 */
export default function TrainingSession() {
  const { t, locale } = useI18n();
  const { user } = useAuth();
  const { assignmentId } = useParams();
  const [assignment, setAssignment] = useState(null);
  const [error, setError] = useState(null);
  const [exporting, setExporting] = useState(false);

  const load = useCallback(() => {
    training.get(assignmentId).then(setAssignment).catch(setError);
  }, [assignmentId]);

  useEffect(load, [load]);

  const logSet = async (set, payload) => {
    await training.logSet(assignmentId, set.id, payload);
    load();
  };

  // A whole session at once, in one request: the API writes the flag onto
  // every set of the day, keeping whatever each one already carried.
  const setDayDone = async (day, done) => {
    await training.setDayDone(assignmentId, day.id, done);
    load();
  };

  // The block, one week of it, or one session, with the member's feedback on
  // it. Both narrowings are undefined for the whole thing - which is what the
  // API's ?week and ?day do too, so there is one rule rather than two.
  const exportAs = async (format, week, day) => {
    setExporting(true);
    try {
      const blob = await training.export(assignmentId, { format, week, day: day?.id, locale });
      // A session's file says which one it is: three files called
      // "bloc-force.pdf" in a downloads folder help nobody.
      const name = [
        assignment?.program?.name || "program",
        week ? `w${week}` : null,
        day ? `d${day.day}` : null,
      ]
        .filter(Boolean)
        .join("-");
      downloadBlob(blob, `${name}.${format}`);
    } catch (err) {
      setError(err);
    } finally {
      setExporting(false);
    }
  };

  const changeStatus = async (status) => {
    setAssignment(await training.setStatus(assignmentId, status));
  };

  if (error) return <div className="sf-page"><ErrorMessage error={error} /></div>;
  if (!assignment) return <Spinner />;

  // Only the member running the block may log sets; a coach opening the same
  // page is reading it.
  const isOwner = assignment.userId === user?.id;
  const missing = assignment.missingOneRms || [];

  return (
    <div className="sf-page">
      <div className="sf-page-header">
        <div>
          <h1>{assignment.program?.name}</h1>
          <p className="sf-subtitle">
            {assignment.clubName}
            {assignment.program?.weeks ? ` · ${t("programs.weeks", { count: assignment.program.weeks })}` : ""}
            {isOwner ? "" : ` · ${assignment.memberName}`}
          </p>
        </div>
        <div className="sf-row" style={{ gap: "0.5rem" }}>
          {/* Offered to whoever may open the block: the member sends it to
              their coach, and the coach exports their athlete's block to read
              it away from the app. */}
          <ExportMenu onExport={(format) => exportAs(format)} busy={exporting} label={t("session.exportBlock")} />
          {isOwner ? (
            <select className="sf-select" style={{ width: "auto" }} value={assignment.status || "active"} onChange={(event) => changeStatus(event.target.value)}>
              <option value="active">{t("session.statusActive")}</option>
              <option value="done">{t("session.statusDone")}</option>
              <option value="archived">{t("session.statusArchived")}</option>
            </select>
          ) : null}
        </div>
      </div>

      {assignment.note ? <div className="sf-notice">{assignment.note}</div> : null}

      {missing.length > 0 ? (
        <div className="sf-notice sf-notice-warning">
          <div className="sf-row-between">
            <span>
              {t("session.missingOneRmsList", {
                list: missing.map((exercise) => exercise.labels?.[locale] || exercise.labels?.en || exercise.slug).join(", "),
              })}
            </span>
            {isOwner ? (
              <Link className="sf-button sf-button-sm" to="/dashboard/one-rms">
                {t("session.setMyOneRms")}
              </Link>
            ) : null}
          </div>
        </div>
      ) : null}

      {/* Grouped by week, because that is the unit a week's work is discussed
          in: one export per week, and the sessions of that week under it. */}
      {weeksOf(assignment.days || []).map((week, weekIndex) => (
        <section key={week.number}>
          <div className="sf-row-between" style={{ alignItems: "center", margin: "1.2rem 0 0.4rem" }}>
            <h2 style={{ margin: 0 }}>{t("programs.week", { week: week.number })}</h2>
            <ExportMenu
              onExport={(format) => exportAs(format, week.number)}
              busy={exporting}
              label={t("session.exportWeek")}
              size="sf-button-sm"
            />
          </div>
          {week.days.map((day, index) => (
            <SessionDay
              key={day.id}
              day={day}
              locale={locale}
              defaultOpen={weekIndex === 0 && index === 0}
              editable={isOwner}
              onLog={logSet}
              onDayDone={setDayDone}
              onExport={(format) => exportAs(format, week.number, day)}
              exporting={exporting}
            />
          ))}
        </section>
      ))}
    </div>
  );
}

export { exerciseLabel };
