import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { toast } from "react-toastify";
import { FiPlus, FiTrash2, FiUpload } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import { auth, clubs as clubsApi, programs as programsApi } from "../../api/services";
import { ClubFormModal } from "./Clubs";
import ImportProgramModal from "../../components/programs/ImportProgramModal";
import Avatar from "../../components/common/Avatar";
import Modal, { ConfirmModal } from "../../components/common/Modal";
import PostCard from "../../components/feed/PostCard";
import PostComposer from "../../components/feed/PostComposer";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";

/** A club: its roster, its programs, its feed, and the coach's feedback inbox. */
export default function ClubDetail() {
  const { t, locale } = useI18n();
  const { user } = useAuth();
  const { clubId } = useParams();
  const navigate = useNavigate();

  const [club, setClub] = useState(null);
  const [tab, setTab] = useState("programs");
  const [error, setError] = useState(null);
  const [editing, setEditing] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [confirmLeave, setConfirmLeave] = useState(false);

  const load = useCallback(() => {
    clubsApi.get(clubId).then(setClub).catch(setError);
  }, [clubId]);

  useEffect(load, [load]);

  if (error) return <div className="sf-page"><ErrorMessage error={error} /></div>;
  if (!club) return <Spinner />;

  const canManage = club.role === "owner" || club.role === "admin";
  const isOwner = club.role === "owner" || user?.role === "superadmin";

  const remove = async () => {
    setConfirmDelete(false);
    try {
      await clubsApi.remove(clubId);
      toast.success(t("clubs.deleted"), toastOptions);
      navigate("/dashboard/clubs");
    } catch (err) {
      setError(err);
    }
  };

  const leave = async () => {
    setConfirmLeave(false);
    try {
      await clubsApi.leave(clubId);
      navigate("/dashboard/clubs");
    } catch (err) {
      setError(err);
    }
  };

  const tabs = [
    { id: "programs", label: t("programs.title") },
    { id: "members", label: t("clubs.members") },
    { id: "feed", label: t("clubs.feed") },
  ];
  if (canManage) tabs.push({ id: "feedback", label: t("clubs.feedback") });

  return (
    <div className="sf-page">
      <div className="sf-page-header">
        <div>
          <h1>{club.name}</h1>
          <p className="sf-subtitle">
            {club.description || ""}
            {club.city ? ` · ${club.city}` : ""} · {t("clubs.memberCount", { count: club.memberCount })}
          </p>
        </div>
        <div className="sf-row">
          {canManage ? (
            <button className="sf-button sf-button-secondary" onClick={() => setEditing(true)}>
              {t("common.edit")}
            </button>
          ) : null}
          {isOwner ? (
            <button className="sf-button sf-button-danger" onClick={() => setConfirmDelete(true)}>
              {t("common.delete")}
            </button>
          ) : (
            <button className="sf-button sf-button-secondary" onClick={() => setConfirmLeave(true)}>
              {t("clubs.leave")}
            </button>
          )}
        </div>
      </div>

      <div className="sf-tabs">
        {tabs.map((item) => (
          <button key={item.id} type="button" className={`sf-tab ${tab === item.id ? "active" : ""}`} onClick={() => setTab(item.id)}>
            {item.label}
          </button>
        ))}
      </div>

      {tab === "programs" ? <ProgramsTab club={club} canManage={canManage} /> : null}
      {tab === "members" ? <MembersTab club={club} canManage={canManage} isOwner={isOwner} onChanged={load} /> : null}
      {tab === "feed" ? <ClubFeedTab club={club} /> : null}
      {tab === "feedback" ? <FeedbackTab club={club} locale={locale} /> : null}

      {editing ? (
        <ClubFormModal
          club={club}
          onClose={() => setEditing(false)}
          onSaved={(updated) => {
            setClub(updated);
            setEditing(false);
            toast.success(t("clubs.saved"), toastOptions);
          }}
        />
      ) : null}

      {confirmDelete ? (
        <ConfirmModal
          title={t("common.delete")}
          message={t("clubs.confirmDelete", { name: club.name })}
          onConfirm={remove}
          onClose={() => setConfirmDelete(false)}
        />
      ) : null}

      {confirmLeave ? (
        <ConfirmModal
          title={t("clubs.leave")}
          message={t("clubs.confirmLeave", { name: club.name })}
          onConfirm={leave}
          onClose={() => setConfirmLeave(false)}
        />
      ) : null}
    </div>
  );
}

function ProgramsTab({ club, canManage }) {
  const { t } = useI18n();
  const [programs, setPrograms] = useState(null);
  const [importing, setImporting] = useState(false);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState(null);
  const navigate = useNavigate();

  const load = useCallback(() => {
    programsApi.list(club.id).then(setPrograms).catch(setError);
  }, [club.id]);

  useEffect(load, [load]);

  if (!programs) return <Spinner />;

  return (
    <>
      <ErrorMessage error={error} />
      {canManage ? (
        <>
          <div className="sf-row" style={{ marginBottom: "0.4rem" }}>
            <button className="sf-button" onClick={() => setCreating(true)}>
              <FiPlus /> {t("programs.create")}
            </button>
            <button className="sf-button sf-button-secondary" onClick={() => setImporting(true)}>
              <FiUpload /> {t("programs.import")}
            </button>
          </div>
          <p className="sf-muted" style={{ marginBottom: "1rem" }}>
            {t("programs.createHelp")}
          </p>
        </>
      ) : null}

      {programs.length === 0 ? (
        <EmptyState message={t("programs.empty")} />
      ) : (
        <div className="sf-grid">
          {programs.map((program) => (
            <Link
              key={program.id}
              to={`/dashboard/clubs/${club.id}/programs/${program.id}`}
              className="sf-card sf-card-clickable"
              style={{ display: "block", color: "inherit" }}
            >
              <h3 style={{ margin: 0 }}>{program.name}</h3>
              {program.description ? <p className="sf-muted">{program.description}</p> : null}
              <div className="sf-muted">
                {t("programs.weeks", { count: program.weeks })} · {t("programs.sessions", { count: program.dayCount })} ·{" "}
                {t("programs.setCount", { count: program.setCount })}
              </div>
              <div className="sf-muted">{t("programs.author", { name: program.authorName })}</div>
            </Link>
          ))}
        </div>
      )}

      {importing ? (
        <ImportProgramModal
          club={club}
          onClose={() => setImporting(false)}
          onImported={() => {
            setImporting(false);
            load();
          }}
        />
      ) : null}

      {creating ? (
        <CreateProgramModal
          club={club}
          onClose={() => setCreating(false)}
          onCreated={(program) => {
            setCreating(false);
            toast.success(t("programs.created"), toastOptions);
            // An empty program is only useful once it has sessions, so the
            // coach lands straight on it rather than back in the list.
            navigate(`/dashboard/clubs/${club.id}/programs/${program.id}`);
          }}
        />
      ) : null}
    </>
  );
}

function MembersTab({ club, canManage, isOwner, onChanged }) {
  const { t } = useI18n();
  const [members, setMembers] = useState(null);
  const [adding, setAdding] = useState(false);
  const [confirming, setConfirming] = useState(null);
  const [error, setError] = useState(null);

  const load = useCallback(() => {
    clubsApi.members(club.id).then(setMembers).catch(setError);
  }, [club.id]);

  useEffect(load, [load]);

  if (!members) return <Spinner />;

  const setRole = async (member, role) => {
    try {
      await clubsApi.setMemberRole(club.id, member.userId, role);
      load();
    } catch (err) {
      setError(err);
    }
  };

  const remove = async (member) => {
    setConfirming(null);
    try {
      await clubsApi.removeMember(club.id, member.userId);
      toast.success(t("clubs.memberRemoved"), toastOptions);
      load();
      onChanged();
    } catch (err) {
      setError(err);
    }
  };

  const transfer = async (member) => {
    try {
      await clubsApi.transfer(club.id, member.userId);
      load();
      onChanged();
    } catch (err) {
      setError(err);
    }
  };

  return (
    <>
      <ErrorMessage error={error} />
      {canManage ? (
        <div className="sf-row" style={{ marginBottom: "1rem" }}>
          <button className="sf-button" onClick={() => setAdding(true)}>
            {t("clubs.addMember")}
          </button>
        </div>
      ) : null}

      <div className="sf-card">
        <div className="sf-table-wrapper">
          <table className="sf-table">
            <tbody>
              {members.map((member) => (
                <tr key={member.id}>
                  <td style={{ width: 44 }}>
                    <Avatar user={member} size="sf-avatar-sm" />
                  </td>
                  <td>
                    {member.handle ? (
                      <Link to={`/profile/${member.handle}`}>
                        {member.name} {member.surname}
                      </Link>
                    ) : (
                      `${member.name} ${member.surname}`
                    )}
                    <div className="sf-muted">{member.email}</div>
                  </td>
                  <td>
                    <span className="sf-badge">{t(`clubs.${member.role}`)}</span>
                  </td>
                  <td className="sf-table-num">
                    {canManage && member.role !== "owner" ? (
                      <div className="sf-row" style={{ justifyContent: "flex-end", gap: "0.3rem" }}>
                        <button
                          className="sf-button sf-button-secondary sf-button-sm"
                          onClick={() => setRole(member, member.role === "admin" ? "member" : "admin")}
                        >
                          {member.role === "admin" ? t("clubs.demote") : t("clubs.promote")}
                        </button>
                        {isOwner ? (
                          <button className="sf-button sf-button-secondary sf-button-sm" onClick={() => transfer(member)}>
                            {t("clubs.transfer")}
                          </button>
                        ) : null}
                        <button className="sf-button-ghost sf-button-sm" onClick={() => setConfirming(member)} aria-label={t("clubs.remove")}>
                          <FiTrash2 />
                        </button>
                      </div>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {adding ? (
        <AddMemberModal
          club={club}
          onClose={() => setAdding(false)}
          onAdded={() => {
            setAdding(false);
            toast.success(t("clubs.memberAdded"), toastOptions);
            load();
            onChanged();
          }}
        />
      ) : null}

      {confirming ? (
        <ConfirmModal
          title={t("clubs.remove")}
          message={t("clubs.confirmRemove", { name: `${confirming.name} ${confirming.surname}` })}
          onConfirm={() => remove(confirming)}
          onClose={() => setConfirming(null)}
        />
      ) : null}
    </>
  );
}

/** Adds an existing account to the club, found by email/handle/name. */
function AddMemberModal({ club, onClose, onAdded }) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [results, setResults] = useState([]);
  const [role, setRole] = useState("member");
  const [error, setError] = useState(null);

  useEffect(() => {
    if (query.trim().length < 2) {
      setResults([]);
      return;
    }
    // Debounced so typing an address doesn't fire a request per keystroke.
    const timer = setTimeout(() => {
      auth.search(query).then(setResults).catch(() => setResults([]));
    }, 250);
    return () => clearTimeout(timer);
  }, [query]);

  const add = async (userId) => {
    try {
      await clubsApi.addMember(club.id, { userId, role });
      onAdded();
    } catch (err) {
      setError(err);
    }
  };

  return (
    <Modal title={t("clubs.addMember")} onClose={onClose}>
      <p className="sf-muted">{t("clubs.addMemberHelp")}</p>
      <div className="sf-field">
        <label className="sf-label">{t("common.search")}</label>
        <input className="sf-input" value={query} onChange={(event) => setQuery(event.target.value)} autoFocus />
      </div>
      <div className="sf-field">
        <label className="sf-label">{t("clubs.role")}</label>
        <select className="sf-select" value={role} onChange={(event) => setRole(event.target.value)}>
          <option value="member">{t("clubs.member")}</option>
          <option value="admin">{t("clubs.admin")}</option>
        </select>
      </div>
      <ErrorMessage error={error} />
      {results.map((result) => (
        <div key={result.id} className="sf-row-between" style={{ padding: "0.4rem 0", borderTop: "1px solid var(--sf-border)" }}>
          <div className="sf-row">
            <Avatar user={result} size="sf-avatar-sm" />
            <div>
              <div>
                {result.name} {result.surname}
              </div>
              <div className="sf-muted">{result.email}</div>
            </div>
          </div>
          <button className="sf-button sf-button-sm" onClick={() => add(result.id)}>
            {t("common.add")}
          </button>
        </div>
      ))}
    </Modal>
  );
}

function ClubFeedTab({ club }) {
  const { t } = useI18n();
  const [posts, setPosts] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    clubsApi
      .feed(club.id)
      .then((page) => setPosts(page.results))
      .catch(setError);
  }, [club.id]);

  if (error) return <ErrorMessage error={error} />;
  if (!posts) return <Spinner />;

  return (
    <div className="sf-feed">
      <PostComposer clubs={[club]} defaultClubId={club.id} onPosted={(post) => setPosts((current) => [post, ...current])} />
      {posts.length === 0 ? (
        <EmptyState message={t("feed.empty")} />
      ) : (
        posts.map((post) => (
          <PostCard
            key={post.id}
            post={post}
            onChanged={(updated) => setPosts((current) => current.map((item) => (item.id === updated.id ? updated : item)))}
            onDeleted={(postId) => setPosts((current) => current.filter((item) => item.id !== postId))}
          />
        ))
      )}
    </div>
  );
}

/** The coach's inbox: every set a member left a perceived RPE or a comment on. */
function FeedbackTab({ club, locale }) {
  const { t } = useI18n();
  const [feedback, setFeedback] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    clubsApi
      .feedback(club.id)
      .then((page) => setFeedback(page.results))
      .catch(setError);
  }, [club.id]);

  if (error) return <ErrorMessage error={error} />;
  if (!feedback) return <Spinner />;
  if (feedback.length === 0) return <EmptyState message={t("clubs.feedback")} />;

  return (
    <div className="sf-card">
      <div className="sf-table-wrapper">
        <table className="sf-table">
          <thead>
            <tr>
              <th>{t("clubs.member")}</th>
              <th>{t("session.exercise")}</th>
              <th className="sf-table-num">{t("session.rpe")}</th>
              <th className="sf-table-num">{t("session.e1rm")}</th>
              <th>{t("session.comment")}</th>
            </tr>
          </thead>
          <tbody>
            {feedback.map((item) => (
              <tr key={item.id}>
                <td>{item.memberName}</td>
                <td>
                  {item.exerciseLabels?.[locale] || item.exerciseLabels?.en || item.exerciseSlug}
                  <div className="sf-muted">
                    {t("programs.week", { week: item.week })} · {t("programs.day", { day: item.day })}
                  </div>
                </td>
                <td className="sf-table-num">
                  {item.prescribedRpe ?? "—"} → <strong>{item.actualRpe ?? "—"}</strong>
                </td>
                <td className="sf-table-num">{item.actualLoad ? `${item.actualLoad} ${t("common.kg")}` : "—"}</td>
                <td>{item.comment || "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/** Opens an empty program, to be filled session by session. */
function CreateProgramModal({ club, onClose, onCreated }) {
  const { t } = useI18n();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      onCreated(await programsApi.create(club.id, { name, description }));
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  return (
    <Modal
      title={t("programs.create")}
      onClose={onClose}
      actions={
        <>
          <button className="sf-button sf-button-secondary" onClick={onClose} disabled={busy}>
            {t("common.cancel")}
          </button>
          <button className="sf-button" onClick={submit} disabled={busy || !name.trim()}>
            {t("common.save")}
          </button>
        </>
      }
    >
      <div className="sf-field">
        <label className="sf-label">{t("programs.programName")}</label>
        <input className="sf-input" value={name} onChange={(event) => setName(event.target.value)} autoFocus />
      </div>
      <div className="sf-field">
        <label className="sf-label">
          {t("clubs.description")} <span className="sf-muted">({t("common.optional")})</span>
        </label>
        <textarea
          className="sf-textarea"
          value={description}
          onChange={(event) => setDescription(event.target.value)}
        />
      </div>
      <ErrorMessage error={error} />
    </Modal>
  );
}
