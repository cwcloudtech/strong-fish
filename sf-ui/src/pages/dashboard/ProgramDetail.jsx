import { useCallback, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { toast } from "react-toastify";
import { FiTrash2 } from "react-icons/fi";

import { clubs as clubsApi, programs as programsApi } from "../../api/services";
import Modal, { ConfirmModal } from "../../components/common/Modal";
import SessionDay from "../../components/training/SessionDay";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";

/**
 * A program as its coach sees it: the sessions, and who is running it.
 *
 * The loads shown are resolved against whoever is selected in "viewing as" -
 * the coach's own maxes by default, or a member's, which is how a coach checks
 * what an athlete will actually be asked to lift.
 */
export default function ProgramDetail() {
  const { t, locale } = useI18n();
  const { user } = useAuth();
  const { clubId, programId } = useParams();
  const navigate = useNavigate();

  const [program, setProgram] = useState(null);
  const [members, setMembers] = useState([]);
  const [assignments, setAssignments] = useState([]);
  const [viewAs, setViewAs] = useState("");
  const [canManage, setCanManage] = useState(false);
  const [assigning, setAssigning] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [error, setError] = useState(null);

  const load = useCallback(async () => {
    try {
      const [programResult, club] = await Promise.all([
        programsApi.get(clubId, programId, viewAs || undefined),
        clubsApi.get(clubId),
      ]);
      setProgram(programResult);
      const manage = club.role === "owner" || club.role === "admin" || user?.role === "superadmin";
      setCanManage(manage);
      if (manage) {
        const [memberList, assignmentList] = await Promise.all([
          clubsApi.members(clubId),
          programsApi.assignments(clubId, programId),
        ]);
        setMembers(memberList);
        setAssignments(assignmentList);
      }
    } catch (err) {
      setError(err);
    }
  }, [clubId, programId, viewAs, user?.role]);

  useEffect(() => {
    load();
  }, [load]);

  const remove = async () => {
    setConfirmDelete(false);
    try {
      await programsApi.remove(clubId, programId);
      toast.success(t("programs.deleted"));
      navigate(`/dashboard/clubs/${clubId}`);
    } catch (err) {
      setError(err);
    }
  };

  const unassign = async (assignment) => {
    try {
      await programsApi.unassign(clubId, programId, assignment.id);
      load();
    } catch (err) {
      setError(err);
    }
  };

  if (error) return <div className="sf-page"><ErrorMessage error={error} /></div>;
  if (!program) return <Spinner />;

  const missing = program.missingOneRms || [];

  return (
    <div className="sf-page">
      <div className="sf-page-header">
        <div>
          <h1>{program.name}</h1>
          <p className="sf-subtitle">
            {t("programs.weeks", { count: program.weeks })} · {t("programs.sessions", { count: program.dayCount })} ·{" "}
            {t("programs.setCount", { count: program.setCount })}
            {program.sourceFileName ? ` · ${program.sourceFileName}` : ""}
          </p>
        </div>
        {canManage ? (
          <div className="sf-row">
            <button className="sf-button" onClick={() => setAssigning(true)}>
              {t("programs.assign")}
            </button>
            <button className="sf-button sf-button-danger" onClick={() => setConfirmDelete(true)}>
              {t("common.delete")}
            </button>
          </div>
        ) : null}
      </div>

      {canManage ? (
        <div className="sf-card">
          <div className="sf-row-between">
            <div className="sf-field" style={{ margin: 0, minWidth: 220 }}>
              <label className="sf-label">{t("programs.viewAs")}</label>
              <select className="sf-select" value={viewAs} onChange={(event) => setViewAs(event.target.value)}>
                <option value="">{t("programs.myself")}</option>
                {members
                  .filter((member) => member.userId !== user?.id)
                  .map((member) => (
                    <option key={member.userId} value={member.userId}>
                      {member.name} {member.surname}
                    </option>
                  ))}
              </select>
            </div>
          </div>

          <h3 style={{ marginTop: "1rem" }}>{t("programs.assignments")}</h3>
          {assignments.length === 0 ? (
            <p className="sf-muted">{t("programs.noAssignments")}</p>
          ) : (
            <div className="sf-table-wrapper">
              <table className="sf-table">
                <tbody>
                  {assignments.map((assignment) => (
                    <tr key={assignment.id}>
                      <td>
                        {assignment.memberName}
                        <div className="sf-muted">{assignment.memberEmail}</div>
                      </td>
                      <td className="sf-muted">
                        {t("session.progress", { done: assignment.completedSets, total: assignment.totalSets })}
                      </td>
                      <td className="sf-table-num">
                        <button className="sf-button-ghost sf-button-sm" onClick={() => unassign(assignment)} aria-label={t("programs.unassign")}>
                          <FiTrash2 />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      ) : null}

      {missing.length > 0 ? (
        <div className="sf-notice sf-notice-warning">
          {t("session.missingOneRmsList", {
            list: missing.map((exercise) => exercise.labels?.[locale] || exercise.labels?.en || exercise.slug).join(", "),
          })}
        </div>
      ) : null}

      {(program.days || []).length === 0 ? (
        <EmptyState message={t("programs.empty")} />
      ) : (
        (program.days || []).map((day, index) => (
          <SessionDay key={day.id} day={day} locale={locale} defaultOpen={index === 0} />
        ))
      )}

      {assigning ? (
        <AssignModal
          clubId={clubId}
          programId={programId}
          members={members}
          assigned={assignments.map((assignment) => assignment.userId)}
          onClose={() => setAssigning(false)}
          onAssigned={() => {
            setAssigning(false);
            toast.success(t("programs.assigned"));
            load();
          }}
        />
      ) : null}

      {confirmDelete ? (
        <ConfirmModal
          title={t("common.delete")}
          message={t("programs.confirmDelete", { name: program.name })}
          onConfirm={remove}
          onClose={() => setConfirmDelete(false)}
        />
      ) : null}
    </div>
  );
}

function AssignModal({ clubId, programId, members, assigned, onClose, onAssigned }) {
  const { t } = useI18n();
  const [userId, setUserId] = useState("");
  const [startDate, setStartDate] = useState("");
  const [note, setNote] = useState("");
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      await programsApi.assign(clubId, programId, { userId, startDate, note });
      onAssigned();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  const available = members.filter((member) => !assigned.includes(member.userId));

  return (
    <Modal
      title={t("programs.assignTo")}
      onClose={onClose}
      actions={
        <>
          <button className="sf-button sf-button-secondary" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className="sf-button" onClick={submit} disabled={busy || !userId}>
            {t("programs.assign")}
          </button>
        </>
      }
    >
      <div className="sf-field">
        <label className="sf-label">{t("clubs.member")}</label>
        <select className="sf-select" value={userId} onChange={(event) => setUserId(event.target.value)}>
          <option value="">{t("common.none")}</option>
          {available.map((member) => (
            <option key={member.userId} value={member.userId}>
              {member.name} {member.surname} ({member.email})
            </option>
          ))}
        </select>
      </div>
      <div className="sf-field">
        <label className="sf-label">
          {t("programs.startDate")} <span className="sf-muted">({t("common.optional")})</span>
        </label>
        <input className="sf-input" type="date" value={startDate} onChange={(event) => setStartDate(event.target.value)} />
      </div>
      <div className="sf-field">
        <label className="sf-label">
          {t("programs.note")} <span className="sf-muted">({t("common.optional")})</span>
        </label>
        <textarea className="sf-textarea" value={note} onChange={(event) => setNote(event.target.value)} />
      </div>
      <ErrorMessage error={error} />
    </Modal>
  );
}
