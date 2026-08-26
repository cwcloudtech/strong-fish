import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { training } from "../../api/services";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

const STATUS_LABELS = {
  active: "session.statusActive",
  done: "session.statusDone",
  archived: "session.statusArchived",
};

// What is still being run comes first; what is finished stays below it rather
// than disappearing, because the comments left on a block are the record of how
// it went. Archived goes last - it is history somebody chose to file away.
const STATUS_ORDER = { active: 0, done: 1, archived: 2 };

const STATUS_BADGES = {
  active: "sf-badge-primary",
  done: "sf-badge-success",
  archived: "sf-badge-muted",
};

/** Newest first within each status, which is the order the API already sends. */
function byStatus(assignments) {
  return [...assignments].sort((a, b) => (STATUS_ORDER[a.status] ?? 0) - (STATUS_ORDER[b.status] ?? 0));
}

/** The programs a coach has assigned to the connected member. */
export default function Training() {
  const { t } = useI18n();
  const [assignments, setAssignments] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    training.list().then(setAssignments).catch(setError);
  }, []);

  if (error) return <div className="sf-page"><ErrorMessage error={error} /></div>;
  if (!assignments) return <Spinner />;

  return (
    <div className="sf-page">
      <div className="sf-page-header">
        <h1>{t("nav.training")}</h1>
      </div>

      {assignments.length === 0 ? (
        <EmptyState title={t("session.noAssignments")} message={t("session.noAssignmentsHelp")} />
      ) : (
        <div className="sf-grid">
          {byStatus(assignments).map((assignment) => (
            <Link
              key={assignment.id}
              to={`/dashboard/training/${assignment.id}`}
              // A finished block is kept for its history and coloured as such:
              // still readable, visibly not the one being run.
              className={`sf-card sf-card-clickable${assignment.status && assignment.status !== "active" ? " sf-card-history" : ""}`}
              style={{ display: "block", color: "inherit" }}
            >
              <div className="sf-row-between">
                <h3 style={{ margin: 0 }}>{assignment.programName}</h3>
                <span className={`sf-badge ${STATUS_BADGES[assignment.status] || STATUS_BADGES.active}`}>
                  {t(STATUS_LABELS[assignment.status] || STATUS_LABELS.active)}
                </span>
              </div>
              <p className="sf-muted" style={{ margin: "0.2rem 0 0.6rem" }}>{assignment.clubName}</p>
              <div className="sf-row-between">
                <span className="sf-muted">
                  {t("session.progress", { done: assignment.completedSets, total: assignment.totalSets })}
                </span>
                {assignment.startDate ? <span className="sf-muted">{assignment.startDate}</span> : null}
              </div>
              {assignment.note ? <p className="sf-muted" style={{ marginBottom: 0 }}>{assignment.note}</p> : null}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
