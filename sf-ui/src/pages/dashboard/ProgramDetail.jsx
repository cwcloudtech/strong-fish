import { useCallback, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { toast } from "react-toastify";
import { FiCopy, FiDownload, FiEdit2, FiGlobe, FiLock, FiPlus, FiTrash2 } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import { clubs as clubsApi, programs as programsApi } from "../../api/services";
import Modal, { ConfirmModal } from "../../components/common/Modal";
import SessionEditor, { SessionFormModal } from "../../components/programs/SessionEditor";
import SessionDay from "../../components/training/SessionDay";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";
import Select from "../../components/common/Select";
import Switch from "../../components/common/Switch";
import MultiSelect from "../../components/common/MultiSelect";

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
  const [exporting, setExporting] = useState(false);
  const [copying, setCopying] = useState(false);

  const load = useCallback(async () => {
    try {
      const programResult = await programsApi.get(clubId, programId, viewAs || undefined);
      setProgram(programResult);

      // A program of somebody's own has no club to ask about: it is theirs to
      // manage because they wrote it. Asking a club anyway would 404 on a
      // club id that is not there.
      const manage = clubId
        ? await (async () => {
            const club = await clubsApi.get(clubId);
            return club.role === "owner" || club.role === "admin" || user?.role === "superadmin";
          })()
        : programResult.authorId === user?.id || user?.role === "superadmin";
      setCanManage(manage);

      if (manage) {
        const [memberList, assignmentList] = await Promise.all([
          // Only a club has members to assign to. A personal program is
          // assigned to its author and nobody else.
          clubId ? clubsApi.members(clubId) : Promise.resolve([]),
          programsApi.assignments(clubId, programId),
        ]);
        setMembers(memberList);
        setAssignments(assignmentList);
      }
    } catch (err) {
      setError(err);
    }
  }, [clubId, programId, viewAs, user?.role, user?.id]);

  useEffect(() => {
    load();
  }, [load]);

  /**
   * Downloads the printable sheet.
   *
   * Fetched and handed to the browser as a blob rather than linked directly:
   * the request needs the session's Authorization header, which a plain
   * <a href> would not send - the download would arrive as a 401.
   */
  const exportPdf = async () => {
    setExporting(true);
    try {
      const blob = await programsApi.exportPdf(clubId, programId, {
        memberId: viewAs || undefined,
        locale,
      });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `${program?.name || "program"}.pdf`;
      link.click();
      // Revoked once the click has been handled, or the blob is held for the
      // life of the tab.
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err);
    } finally {
      setExporting(false);
    }
  };

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
        <div className="sf-row">
          {/* Offered to anybody who can open the program: an athlete printing
              the block they were given is not a coaching action. */}
          <button className="sf-button sf-button-secondary" onClick={exportPdf} disabled={exporting}>
            <FiDownload /> {exporting ? t("programs.exporting") : t("programs.exportPdf")}
          </button>
          <button className="sf-button sf-button-secondary" onClick={() => setCopying(true)}>
            <FiCopy /> {t("programs.copy")}
          </button>
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
              <Select
                options={[
                  { value: "", label: t("programs.myself") },
                  ...members
                    .filter((member) => member.userId !== user?.id)
                    .map((member) => ({
                      value: member.userId,
                      label: `${member.name} ${member.surname}`.trim() || member.email,
                    })),
                ]}
                value={viewAs}
                onChange={setViewAs}
              />
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

      {copying ? (
        <CopyProgramModal
          program={program}
          clubId={clubId}
          canManage={canManage}
          onClose={() => setCopying(false)}
          onCopied={(copy) => {
            setCopying(false);
            toast.success(t("programs.copied"), toastOptions);
            // Straight to the copy: what somebody wants after making one is
            // to edit it, and it is not in the list they are looking at.
            navigate(copy.clubId ? `/dashboard/clubs/${copy.clubId}/programs/${copy.id}` : `/dashboard/programs/${copy.id}`);
          }}
          onMoved={(moved) => {
            setCopying(false);
            toast.success(t("programs.moved"), toastOptions);
            navigate(moved.clubId ? `/dashboard/clubs/${moved.clubId}/programs/${moved.id}` : `/dashboard/programs/${moved.id}`);
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
  // Several at once: a coach starting a block runs it with a group, and
  // reopening this modal once per athlete is the kind of repetition that stops
  // people assigning it at all.
  const [userIds, setUserIds] = useState([]);
  const [startDate, setStartDate] = useState("");
  const [note, setNote] = useState("");
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      await programsApi.assign(clubId, programId, { userIds, startDate, note });
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
          <button className="sf-button" onClick={submit} disabled={busy || userIds.length === 0}>
            {userIds.length > 1 ? t("programs.assignCount", { count: userIds.length }) : t("programs.assign")}
          </button>
        </>
      }
    >
      <div className="sf-field">
        <label className="sf-label">{t("clubs.members")}</label>
        <MultiSelect
          options={available.map((member) => ({
            value: member.userId,
            // The email is in the label so the search finds people by it, and
            // so two members with the same name are still told apart.
            label: `${member.name} ${member.surname} (${member.email})`.trim(),
          }))}
          selected={userIds}
          onChange={setUserIds}
          placeholder={t("programs.pickMembers")}
        />
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

/**
 * Copies a program somewhere else, or moves it.
 *
 * Two destinations that behave the same way: another club, for a coach running
 * the block with a second group, or the caller's own library, for a member who
 * wants to adapt what their club gave them. Only the clubs the caller may write
 * in are offered, and the API checks again.
 */
function CopyProgramModal({ program, clubId, canManage, onClose, onCopied, onMoved }) {
  const { t } = useI18n();
  const [clubs, setClubs] = useState([]);
  const [destination, setDestination] = useState("");
  const [move, setMove] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    clubsApi
      .list()
      .then((list) => setClubs(list.filter((club) => club.role === "owner" || club.role === "admin")))
      .catch(() => setClubs([]));
  }, []);

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      const result = await programsApi.copy(clubId, program.id, { clubId: destination, move });
      (move ? onMoved : onCopied)(result);
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  // The club it is already in is not a destination, and a personal program's
  // "own library" is not one either when that is where it already is.
  const options = [
    ...(clubId ? [{ value: "", label: t("programs.myLibrary") }] : []),
    ...clubs.filter((club) => club.id !== clubId).map((club) => ({ value: club.id, label: club.name })),
  ];

  return (
    <Modal
      title={t("programs.copy")}
      onClose={onClose}
      footer={
        <>
          <button className="sf-button sf-button-secondary" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className="sf-button" onClick={submit} disabled={busy || (!destination && !clubId)}>
            {move ? t("programs.move") : t("programs.copy")}
          </button>
        </>
      }
    >
      <p className="sf-muted" style={{ marginTop: 0 }}>
        {t("programs.copyHelp")}
      </p>

      <div className="sf-field">
        <label className="sf-label">{t("programs.destination")}</label>
        <Select
          options={options}
          value={destination}
          onChange={setDestination}
          placeholder={t("programs.pickDestination")}
        />
      </div>

      {/* Moving takes the program away from everybody reading it where it is
          now, so it is offered only to somebody who may manage it there. */}
      {canManage ? (
        <label className="sf-row" style={{ gap: "0.5rem", alignItems: "center" }}>
          <Switch checked={move} onChange={setMove} />
          <span>{t("programs.moveInstead")}</span>
        </label>
      ) : null}

      <ErrorMessage error={error} />
    </Modal>
  );
}
