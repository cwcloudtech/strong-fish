import { useCallback, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { toast } from "react-toastify";
import { FiCopy, FiEdit2, FiGlobe, FiLock, FiPlus, FiTrash2 } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import { clubs as clubsApi, programs as programsApi } from "../../api/services";
import Modal, { ConfirmModal } from "../../components/common/Modal";
import SessionEditor, { SessionFormModal } from "../../components/programs/SessionEditor";
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
  // Authoring is a distinct mode: the same sessions render either as a coach's
  // editable prescription or as the loads an athlete would lift, and showing
  // both at once would put two sets of controls on every row.
  const [building, setBuilding] = useState(false);
  const [addingSession, setAddingSession] = useState(false);
  const [publishing, setPublishing] = useState(false);

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
      toast.success(t("programs.deleted"), toastOptions);
      navigate(`/dashboard/clubs/${clubId}`);
    } catch (err) {
      setError(err);
    }
  };

  // Publishing is a two-state switch, not a form: a program is either scoped
  // to the club's members or readable by anybody holding its link. The link
  // itself is just the program's id - unguessable, but the visibility flag is
  // what actually authorises the read (see the API's FindPublicByID), so an
  // old link stops working the moment this is turned off.
  const setVisibility = async (visibility) => {
    setPublishing(true);
    try {
      await programsApi.update(clubId, programId, {
        name: program.name,
        description: program.description || "",
        visibility,
      });
      toast.success(visibility === "public" ? t("programs.published") : t("programs.unpublished"), toastOptions);
      await load();
    } catch (err) {
      toast.error(t("errors.generic"), toastOptions);
      setError(err);
    } finally {
      setPublishing(false);
    }
  };

  const copyShareLink = () => {
    const link = `${window.location.origin}/programs/${programId}`;
    navigator.clipboard
      ?.writeText(link)
      .then(() => toast.success(t("common.copied"), toastOptions))
      .catch(() => toast.error(t("errors.copyFailed"), toastOptions));
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
            <button
              className={`sf-button ${building ? "" : "sf-button-secondary"}`}
              onClick={() => setBuilding((current) => !current)}
            >
              <FiEdit2 /> {building ? t("programs.done") : t("programs.buildMode")}
            </button>
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
          <div className="sf-row-between" style={{ alignItems: "center", marginBottom: "0.8rem" }}>
            <div>
              <strong>
                {program.visibility === "public" ? <FiGlobe /> : <FiLock />}{" "}
                {program.visibility === "public" ? t("programs.public") : t("programs.private")}
              </strong>
              <div className="sf-muted" style={{ fontSize: "0.85rem" }}>
                {program.visibility === "public" ? t("programs.publicHelp") : t("programs.privateHelp")}
              </div>
            </div>
            <div className="sf-row" style={{ gap: "0.4rem" }}>
              {program.visibility === "public" ? (
                <button className="sf-button sf-button-secondary sf-button-sm" onClick={copyShareLink}>
                  <FiCopy /> {t("programs.copyLink")}
                </button>
              ) : null}
              {/* The icon says which way the button moves you: a globe to open
                  it up, a padlock to close it again - the same pair the state
                  above is labelled with. */}
              <button
                className="sf-button sf-button-secondary sf-button-sm"
                onClick={() => setVisibility(program.visibility === "public" ? "club" : "public")}
                disabled={publishing}
              >
                {program.visibility === "public" ? <FiLock /> : <FiGlobe />}{" "}
                {program.visibility === "public" ? t("programs.unpublish") : t("programs.publish")}
              </button>
            </div>
          </div>

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

      {building ? (
        <>
          {(program.days || []).map((day) => (
            <SessionEditor
              key={day.id}
              clubId={clubId}
              programId={programId}
              day={day}
              locale={locale}
              onChanged={load}
            />
          ))}
          {(program.days || []).length === 0 ? <EmptyState message={t("programs.noSessions")} /> : null}
          <button className="sf-button" onClick={() => setAddingSession(true)}>
            <FiPlus /> {t("programs.addSession")}
          </button>
        </>
      ) : (program.days || []).length === 0 ? (
        <EmptyState message={t("programs.noSessions")}>
          {canManage ? (
            <button className="sf-button" onClick={() => setBuilding(true)}>
              <FiPlus /> {t("programs.addSession")}
            </button>
          ) : null}
        </EmptyState>
      ) : (
        (program.days || []).map((day, index) => (
          <SessionDay key={day.id} day={day} locale={locale} defaultOpen={index === 0} />
        ))
      )}

      {addingSession ? (
        <SessionFormModal
          clubId={clubId}
          programId={programId}
          onClose={() => setAddingSession(false)}
          onSaved={() => {
            setAddingSession(false);
            load();
          }}
        />
      ) : null}

      {assigning ? (
        <AssignModal
          clubId={clubId}
          programId={programId}
          members={members}
          assigned={assignments.map((assignment) => assignment.userId)}
          onClose={() => setAssigning(false)}
          onAssigned={() => {
            setAssigning(false);
            toast.success(t("programs.assigned"), toastOptions);
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
