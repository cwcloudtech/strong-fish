import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { toast } from "react-toastify";
import { FiGlobe, FiShield, FiTrash2 } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import { admin as adminApi, clubs as clubsApi } from "../../api/services";
import Avatar from "../../components/common/Avatar";
import Modal, { ConfirmModal } from "../../components/common/Modal";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";

const ROLES = ["superadmin", "coach", "confirmed", "disabled", "ban"];
const roleLabel = (t, role) => t(`admin.role${role[0].toUpperCase()}${role.slice(1)}`);

/** The superadmin's console: accounts, clubs, and the moderation queue. */
export default function Admin() {
  const { t } = useI18n();
  const [tab, setTab] = useState("users");
  const [stats, setStats] = useState(null);

  useEffect(() => {
    adminApi.stats().then(setStats).catch(() => setStats(null));
  }, [tab]);

  return (
    <div className="sf-page">
      <div className="sf-page-header">
        <h1>{t("admin.title")}</h1>
        {stats ? (
          <div className="sf-row" style={{ gap: "1.5rem" }}>
            <div>
              <span className="sf-stat-value">{stats.users}</span>
              <span className="sf-stat-label">{t("admin.totalUsers")}</span>
            </div>
            <div>
              <span className="sf-stat-value">{stats.openReports}</span>
              <span className="sf-stat-label">{t("admin.openReports")}</span>
            </div>
            <div>
              <span className="sf-stat-value">{stats.coachRequests}</span>
              <span className="sf-stat-label">{t("admin.coachRequests")}</span>
            </div>
          </div>
        ) : null}
      </div>

      <div className="sf-tabs">
        {[
          { id: "users", label: t("admin.users") },
          { id: "clubs", label: t("admin.clubs") },
          { id: "reports", label: t("admin.reports") },
          { id: "coaches", label: t("admin.coachRequests") },
        ].map((item) => (
          <button key={item.id} type="button" className={`sf-tab ${tab === item.id ? "active" : ""}`} onClick={() => setTab(item.id)}>
            {item.label}
          </button>
        ))}
      </div>

      {tab === "users" ? <UsersTab /> : null}
      {tab === "clubs" ? <ClubsTab /> : null}
      {tab === "reports" ? <ReportsTab /> : null}
      {tab === "coaches" ? <CoachRequestsTab /> : null}
    </div>
  );
}

function UsersTab() {
  const { t } = useI18n();
  const { user: me } = useAuth();
  const [users, setUsers] = useState(null);
  const [editing, setEditing] = useState(null);
  const [confirming, setConfirming] = useState(null);
  const [showingIps, setShowingIps] = useState(null);
  const [error, setError] = useState(null);

  const load = useCallback(() => {
    adminApi.users().then(setUsers).catch(setError);
  }, []);

  useEffect(load, [load]);

  const remove = async (user) => {
    setConfirming(null);
    try {
      await adminApi.removeUser(user.id);
      toast.success(t("admin.userDeleted"), toastOptions);
      load();
    } catch (err) {
      setError(err);
    }
  };

  const clearMfa = async (user) => {
    try {
      await adminApi.clearMfa(user.id);
      toast.success(t("admin.mfaCleared"), toastOptions);
      load();
    } catch (err) {
      setError(err);
    }
  };

  if (!users) return <Spinner />;

  return (
    <>
      <ErrorMessage error={error} />
      <div className="sf-card">
        <div className="sf-table-wrapper">
          <table className="sf-table">
            <thead>
              <tr>
                <th>{t("auth.email")}</th>
                <th>{t("auth.name")}</th>
                <th>{t("admin.role")}</th>
                <th>{t("mfa.title")}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {users.map((user) => (
                <tr key={user.id}>
                  <td>
                    {user.email}
                    {user.handle ? (
                      <div className="sf-muted">
                        <Link to={`/profile/${user.handle}`}>@{user.handle}</Link>
                      </div>
                    ) : null}
                  </td>
                  <td>
                    {user.name} {user.surname}
                  </td>
                  <td>
                    <span className={`sf-badge ${user.role === "ban" ? "sf-badge-danger" : user.role === "disabled" ? "sf-badge-warning" : ""}`}>
                      {roleLabel(t, user.role)}
                    </span>
                  </td>
                  <td>{user.mfaEnabled ? <FiShield title={t("mfa.title")} /> : <span className="sf-muted">—</span>}</td>
                  <td className="sf-table-num">
                    <div className="sf-row" style={{ justifyContent: "flex-end", gap: "0.25rem" }}>
                      <button className="sf-button sf-button-secondary sf-button-sm" onClick={() => setEditing(user)}>
                        {t("common.edit")}
                      </button>
                      <button
                        className="sf-button sf-button-secondary sf-button-sm"
                        onClick={() => setShowingIps(user)}
                        title={t("admin.ips")}
                      >
                        <FiGlobe />
                      </button>
                      {user.mfaEnabled ? (
                        <button className="sf-button sf-button-secondary sf-button-sm" onClick={() => clearMfa(user)} title={t("admin.clearMfaHelp")}>
                          {t("admin.clearMfa")}
                        </button>
                      ) : null}
                      {user.id !== me?.id ? (
                        <button className="sf-button-ghost sf-button-sm" onClick={() => setConfirming(user)} aria-label={t("common.delete")}>
                          <FiTrash2 />
                        </button>
                      ) : null}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {editing ? (
        <UserFormModal
          user={editing}
          isSelf={editing.id === me?.id}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            toast.success(t("admin.userSaved"), toastOptions);
            load();
          }}
        />
      ) : null}

      {showingIps ? <UserIpsModal user={showingIps} onClose={() => setShowingIps(null)} /> : null}

      {confirming ? (
        <ConfirmModal
          title={t("common.delete")}
          message={t("admin.confirmDeleteUser", { email: confirming.email })}
          onConfirm={() => remove(confirming)}
          onClose={() => setConfirming(null)}
        />
      ) : null}
    </>
  );
}

function UserFormModal({ user, isSelf, onClose, onSaved }) {
  const { t } = useI18n();
  const [form, setForm] = useState({
    email: user.email,
    name: user.name || "",
    surname: user.surname || "",
    role: user.role,
    password: "",
  });
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const set = (field) => (event) => setForm((current) => ({ ...current, [field]: event.target.value }));

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      await adminApi.updateUser(user.id, form);
      onSaved();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  return (
    <Modal
      title={t("admin.editUser")}
      onClose={onClose}
      actions={
        <>
          <button className="sf-button sf-button-secondary" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className="sf-button" onClick={submit} disabled={busy}>
            {t("common.save")}
          </button>
        </>
      }
    >
      <div className="sf-field">
        <label className="sf-label">{t("auth.email")}</label>
        <input className="sf-input" type="email" value={form.email} onChange={set("email")} />
      </div>
      <div className="sf-row" style={{ gap: "0.6rem" }}>
        <div className="sf-field" style={{ flex: 1, minWidth: 150 }}>
          <label className="sf-label">{t("auth.name")}</label>
          <input className="sf-input" value={form.name} onChange={set("name")} />
        </div>
        <div className="sf-field" style={{ flex: 1, minWidth: 150 }}>
          <label className="sf-label">{t("auth.surname")}</label>
          <input className="sf-input" value={form.surname} onChange={set("surname")} />
        </div>
      </div>
      <div className="sf-field">
        <label className="sf-label">{t("admin.role")}</label>
        <select className="sf-select" value={form.role} onChange={set("role")} disabled={isSelf}>
          {ROLES.map((role) => (
            <option key={role} value={role}>
              {roleLabel(t, role)}
            </option>
          ))}
        </select>
        {isSelf ? <p className="sf-muted">{t("errors.cantEditOwnRole")}</p> : null}
      </div>
      <div className="sf-field">
        <label className="sf-label">{t("admin.newPassword")}</label>
        <input className="sf-input" type="password" autoComplete="new-password" value={form.password} onChange={set("password")} />
      </div>
      <ErrorMessage error={error} />
    </Modal>
  );
}

/**
 * The queue of accounts that said "I'm a coach" at signup.
 *
 * Confirming is what actually grants the role - asking never did - and turning
 * one down requires a motive, because the applicant is emailed it and "no" on
 * its own tells them nothing about whether to try again.
 */
/**
 * The addresses one account has connected from, with a hit count each - the
 * same view ~/uprodit offers.
 *
 * There is deliberately no ban button: blocking an address is the firewall's
 * job, and a second, weaker version of that rule living in the application
 * would only be a place for the two to disagree.
 */
function UserIpsModal({ user, onClose }) {
  const { t } = useI18n();
  const [ips, setIps] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    adminApi
      .userIps(user.id)
      .then(setIps)
      .catch(setError);
  }, [user.id]);

  return (
    <Modal title={t("admin.ipsTitle", { name: `${user.name} ${user.surname}`.trim() })} onClose={onClose} wide>
      <p className="sf-muted" style={{ marginTop: 0 }}>
        {t("admin.ipsHelp")}
      </p>
      <ErrorMessage error={error} />

      {ips === null ? (
        <Spinner />
      ) : ips.length === 0 ? (
        <EmptyState title={t("admin.noIpsTitle")} message={t("admin.noIpsBody")} />
      ) : (
        <div className="sf-table-wrapper">
          <table className="sf-table">
            <thead>
              <tr>
                <th>{t("admin.ip")}</th>
                <th className="sf-table-num">{t("admin.ipCount")}</th>
                <th>{t("admin.ipFirstSeen")}</th>
                <th>{t("admin.ipLastSeen")}</th>
              </tr>
            </thead>
            <tbody>
              {ips.map((entry) => (
                <tr key={entry.ip}>
                  <td>
                    <code>{entry.ip}</code>
                  </td>
                  <td className="sf-table-num">{entry.count}</td>
                  <td className="sf-muted">{new Date(entry.firstSeen).toLocaleString()}</td>
                  <td className="sf-muted">{new Date(entry.lastSeen).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Modal>
  );
}

function CoachRequestsTab() {
  const { t } = useI18n();
  const [requests, setRequests] = useState(null);
  const [error, setError] = useState(null);
  const [rejecting, setRejecting] = useState(null);
  const [motive, setMotive] = useState("");
  const [busy, setBusy] = useState(null);

  const load = useCallback(async () => {
    try {
      setRequests(await adminApi.coachRequests());
    } catch (err) {
      setError(err);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const decide = async (applicant, status, why) => {
    setBusy(applicant.id);
    try {
      await adminApi.decideCoachRequest(applicant.id, status, why || "");
      toast.success(t(status === "approved" ? "admin.coachApproved" : "admin.coachRejected"), toastOptions);
      setRejecting(null);
      setMotive("");
      await load();
    } catch (err) {
      setError(err);
    } finally {
      setBusy(null);
    }
  };

  if (requests === null) return <Spinner />;

  return (
    <>
      <ErrorMessage error={error} />

      {requests.length === 0 ? (
        <EmptyState title={t("admin.noCoachRequestsTitle")} message={t("admin.noCoachRequestsBody")} />
      ) : (
        <ul className="sf-list">
          {requests.map((applicant) => (
            <li className="sf-list-item" key={applicant.id}>
              <Avatar user={applicant} size="sf-avatar-sm" />
              <div style={{ flex: 1, minWidth: 0 }}>
                <strong>
                  {applicant.name} {applicant.surname}
                </strong>
                <div className="sf-muted" style={{ fontSize: "0.85rem" }}>
                  {applicant.email}
                  {applicant.request?.requestedAt
                    ? ` · ${new Date(applicant.request.requestedAt).toLocaleDateString()}`
                    : ""}
                </div>
              </div>
              <div className="sf-row" style={{ gap: "0.35rem" }}>
                <button
                  className="sf-button sf-button-sm"
                  onClick={() => decide(applicant, "approved")}
                  disabled={busy === applicant.id}
                >
                  {t("admin.confirmCoach")}
                </button>
                <button
                  className="sf-button sf-button-secondary sf-button-sm"
                  onClick={() => setRejecting(applicant)}
                  disabled={busy === applicant.id}
                >
                  {t("admin.rejectCoach")}
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}

      {rejecting ? (
        <Modal
          title={t("admin.rejectCoachTitle")}
          onClose={() => setRejecting(null)}
          actions={
            <>
              <button className="sf-button sf-button-secondary" onClick={() => setRejecting(null)}>
                {t("common.cancel")}
              </button>
              <button
                className="sf-button sf-button-danger"
                onClick={() => decide(rejecting, "rejected", motive)}
                disabled={!motive.trim() || busy === rejecting.id}
              >
                {t("admin.rejectCoach")}
              </button>
            </>
          }
        >
          <p className="sf-muted" style={{ marginTop: 0 }}>
            {t("admin.rejectCoachHelp", { name: `${rejecting.name} ${rejecting.surname}`.trim() })}
          </p>
          <textarea
            className="sf-textarea"
            rows={4}
            autoFocus
            value={motive}
            onChange={(event) => setMotive(event.target.value)}
            placeholder={t("admin.rejectCoachPlaceholder")}
          />
        </Modal>
      ) : null}
    </>
  );
}

function ClubsTab() {
  const { t } = useI18n();
  const [clubs, setClubs] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    clubsApi.listAll().then(setClubs).catch(setError);
  }, []);

  if (error) return <ErrorMessage error={error} />;
  if (!clubs) return <Spinner />;

  return (
    <div className="sf-grid">
      {clubs.map((club) => (
        <Link key={club.id} to={`/dashboard/clubs/${club.id}`} className="sf-card sf-card-clickable" style={{ display: "block", color: "inherit" }}>
          <h3 style={{ margin: 0 }}>{club.name}</h3>
          <div className="sf-muted">
            {t("clubs.owner")}: {club.ownerName} · {t("clubs.memberCount", { count: club.memberCount })}
          </div>
        </Link>
      ))}
    </div>
  );
}

function ReportsTab() {
  const { t, locale } = useI18n();
  const [status, setStatus] = useState("open");
  const [reports, setReports] = useState(null);
  const [error, setError] = useState(null);

  const load = useCallback(() => {
    adminApi
      .reports(status)
      .then((page) => setReports(page?.results || []))
      .catch(setError);
  }, [status]);

  useEffect(load, [load]);

  const resolve = async (report, newStatus, deleteTarget) => {
    try {
      await adminApi.resolveReport(report.id, newStatus, deleteTarget);
      toast.success(t("admin.resolved"), toastOptions);
      load();
    } catch (err) {
      setError(err);
    }
  };

  return (
    <>
      <div className="sf-field" style={{ maxWidth: 220 }}>
        <label className="sf-label">{t("admin.reportStatus")}</label>
        <select className="sf-select" value={status} onChange={(event) => setStatus(event.target.value)}>
          <option value="open">{t("admin.reportOpen")}</option>
          <option value="resolved">{t("admin.reportResolved")}</option>
          <option value="dismissed">{t("admin.reportDismissed")}</option>
          <option value="">{t("common.all")}</option>
        </select>
      </div>

      <ErrorMessage error={error} />

      {!reports ? (
        <Spinner />
      ) : reports.length === 0 ? (
        <EmptyState message={t("admin.noReports")} />
      ) : (
        reports.map((report) => (
          <div key={report.id} className="sf-card">
            <div className="sf-row-between">
              <div>
                <span className="sf-badge sf-badge-warning">{report.reason}</span>{" "}
                <span className="sf-muted">
                  {t("admin.reportedBy")} {report.reporter?.name} {report.reporter?.surname} ·{" "}
                  {new Date(report.createdAt).toLocaleString(locale)}
                </span>
              </div>
              <span className="sf-badge sf-badge-muted">{t(`admin.report${report.status[0].toUpperCase()}${report.status.slice(1)}`)}</span>
            </div>

            {report.comment ? <p>{report.comment}</p> : null}

            <div className="sf-notice" style={{ marginTop: "0.6rem" }}>
              <strong>{t("admin.reportTarget")}</strong> ({report.targetType})
              <div style={{ whiteSpace: "pre-wrap" }}>{report.snapshot || "—"}</div>
            </div>

            {report.status === "open" ? (
              <div className="sf-row" style={{ justifyContent: "flex-end", gap: "0.4rem" }}>
                <button className="sf-button sf-button-secondary sf-button-sm" onClick={() => resolve(report, "dismissed", false)}>
                  {t("admin.dismiss")}
                </button>
                <button className="sf-button sf-button-sm" onClick={() => resolve(report, "resolved", false)}>
                  {t("admin.resolve")}
                </button>
                {report.targetType !== "user" ? (
                  <button className="sf-button sf-button-danger sf-button-sm" onClick={() => resolve(report, "resolved", true)}>
                    {t("admin.resolveAndDelete")}
                  </button>
                ) : null}
              </div>
            ) : null}
          </div>
        ))
      )}
    </>
  );
}
