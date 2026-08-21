import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { training } from "../../api/services";
import { ErrorMessage, Spinner } from "../../components/common/Feedback";
import SessionDay, { exerciseLabel } from "../../components/training/SessionDay";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";

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
        {isOwner ? (
          <select className="sf-select" style={{ width: "auto" }} value={assignment.status || "active"} onChange={(event) => changeStatus(event.target.value)}>
            <option value="active">{t("session.statusActive")}</option>
            <option value="done">{t("session.statusDone")}</option>
            <option value="archived">{t("session.statusArchived")}</option>
          </select>
        ) : null}
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

      {(assignment.days || []).map((day, index) => (
        <SessionDay
          key={day.id}
          day={day}
          locale={locale}
          defaultOpen={index === 0}
          editable={isOwner}
          onLog={logSet}
          onDayDone={setDayDone}
        />
      ))}
    </div>
  );
}

export { exerciseLabel };
