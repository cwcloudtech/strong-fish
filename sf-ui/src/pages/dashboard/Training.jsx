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
          {assignments.map((assignment) => (
            <Link key={assignment.id} to={`/dashboard/training/${assignment.id}`} className="sf-card sf-card-clickable" style={{ display: "block", color: "inherit" }}>
              <div className="sf-row-between">
                <h3 style={{ margin: 0 }}>{assignment.programName}</h3>
                <span className="sf-badge sf-badge-muted">{t(STATUS_LABELS[assignment.status] || STATUS_LABELS.active)}</span>
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
