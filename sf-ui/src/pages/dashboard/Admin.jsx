import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { toast } from "react-toastify";
import { FiEye, FiMapPin, FiShield, FiSlash, FiTrash2 } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import useToastError from "../../utils/useToast";
import { admin as adminApi, clubs as clubsApi } from "../../api/services";
import Avatar from "../../components/common/Avatar";
import Modal, { ConfirmModal } from "../../components/common/Modal";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";

const ROLES = ["superadmin", "coach", "nutritionist", "confirmed", "disabled", "ban"];
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
          { id: "ips", label: t("admin.ips") },
          { id: "audit", label: t("admin.audit.tab") },
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
      {tab === "ips" ? <IpsTab /> : null}
      {tab === "audit" ? <AuditTab /> : null}
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
                <th>{t("admin.ips")}</th>
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
                  <td>
                    {/* The connection popin: every address this account has
                        connected from, how often, where, and whether banned. */}
                    <button
                      className="sf-button sf-button-secondary sf-button-sm"
                      onClick={() => setShowingIps(user)}
                      title={t("admin.ipsView")}
                      aria-label={t("admin.ipsView")}
                    >
                      <FiEye />
                    </button>
                  </td>
                  <td className="sf-table-num">
                    <div className="sf-row" style={{ justifyContent: "flex-end", gap: "0.25rem" }}>
                      <button className="sf-button sf-button-secondary sf-button-sm" onClick={() => setEditing(user)}>
                        {t("common.edit")}
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
 * The connection popin: every address one account has connected from, with how
 * often, where it is, and whether it is banned - plus a ban/unban toggle on
 * each. The same per-member view ~/uprodit offers, opened from the eye button
 * on the users table.
 */
function UserIpsModal({ user, onClose }) {
  const { t } = useI18n();
  const [ips, setIps] = useState(null);
  const [error, setError] = useState(null);
  const [busyIp, setBusyIp] = useState(null);

  const load = useCallback(() => {
    adminApi.userIps(user.id).then(setIps).catch(setError);
  }, [user.id]);

  useEffect(() => {
    load();
  }, [load]);

  const toggleBan = async (entry) => {
    const next = !entry.banned;
    setBusyIp(entry.ip);
    // Optimistic: flip the row now, and roll it back if the call fails. Banning
    // or unbanning from here is global - the same ban the IP screen manages -
    // so an address banned on one member is banned for everyone.
    setIps((list) => list.map((x) => (x.ip === entry.ip ? { ...x, banned: next } : x)));
    try {
      if (next) await adminApi.banIp(entry.ip, "");
      else await adminApi.unbanIp(entry.ip);
      toast.success(t(next ? "admin.ipsScreen.bannedToast" : "admin.ipsScreen.unbannedToast"), toastOptions);
    } catch (err) {
      setIps((list) => list.map((x) => (x.ip === entry.ip ? { ...x, banned: entry.banned } : x)));
      setError(err);
    } finally {
      setBusyIp(null);
    }
  };

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
                <th>{t("admin.ipsScreen.location")}</th>
                <th className="sf-table-num">{t("admin.ipCount")}</th>
                <th>{t("admin.ipFirstSeen")}</th>
                <th>{t("admin.ipLastSeen")}</th>
                <th>{t("admin.ipsScreen.status")}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {ips.map((entry) => (
                <tr key={entry.ip}>
                  <td>
                    <code>{entry.ip}</code>
                  </td>
                  <td className="sf-muted">{geoLabel(entry.geo) || "—"}</td>
                  <td className="sf-table-num">{entry.count}</td>
                  <td className="sf-muted">{new Date(entry.firstSeen).toLocaleString()}</td>
                  <td className="sf-muted">{new Date(entry.lastSeen).toLocaleString()}</td>
                  <td>
                    {entry.banned ? (
                      <span className="sf-badge sf-badge-danger">{t("admin.ipsScreen.banned")}</span>
                    ) : (
                      <span className="sf-muted">—</span>
                    )}
                  </td>
                  <td className="sf-table-num">
                    <button
                      className={`sf-button sf-button-sm ${entry.banned ? "sf-button-secondary" : "sf-button-danger"}`}
                      onClick={() => toggleBan(entry)}
                      disabled={busyIp === entry.ip}
                    >
                      {entry.banned ? t("admin.ipsScreen.unban") : t("admin.ipsScreen.ban")}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Modal>
  );
}

// geoLabel renders what the enrichment knows of where an address is - "City,
// Region, Country (CC)" - using only the parts that resolved.
function geoLabel(geo) {
  if (!geo) return "";
  const place = [geo.city, geo.region, geo.country].filter(Boolean).join(", ");
  const code = geo.countryCode ? ` (${geo.countryCode})` : "";
  return place ? place + code : "";
}

// networkLabel names the operator behind an address - its ISP, else the
// organisation or AS number.
function networkLabel(geo) {
  if (!geo) return "";
  return geo.isp || geo.org || geo.asn || "";
}

/**
 * The IP administration screen: every address seen (plus any banned before it
 * was seen), with who has been behind it, where it is, and whether it is
 * banned. The same roster ~/uprodit offers - search, ban/unban, geolocation -
 * backed here by the sessions table.
 */
function IpsTab() {
  const { t } = useI18n();
  const toastError = useToastError();
  const [input, setInput] = useState("");
  const [term, setTerm] = useState("");
  const [page, setPage] = useState(0);
  const [data, setData] = useState(null);
  const [stats, setStats] = useState(null);
  const [error, setError] = useState(null);
  const [banOpen, setBanOpen] = useState(false);
  const [banIpValue, setBanIpValue] = useState("");
  const [banMotif, setBanMotif] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirmUnban, setConfirmUnban] = useState(null);

  const loadStats = useCallback(() => {
    adminApi.ipStats().then(setStats).catch(() => setStats(null));
  }, []);

  const load = useCallback(() => {
    setData(null);
    setError(null);
    adminApi.ips(term, page).then(setData).catch(setError);
  }, [term, page]);

  useEffect(() => {
    loadStats();
  }, [loadStats]);

  useEffect(() => {
    load();
  }, [load]);

  // Debounce the filter box, and go back to the first page when it changes.
  useEffect(() => {
    const id = setTimeout(() => {
      setTerm(input.trim());
      setPage(0);
    }, 350);
    return () => clearTimeout(id);
  }, [input]);

  const openBan = (ip = "") => {
    setBanIpValue(ip);
    setBanMotif("");
    setBanOpen(true);
  };

  const submitBan = async () => {
    const ip = banIpValue.trim();
    if (!ip) return;
    setBusy(true);
    try {
      await adminApi.banIp(ip, banMotif.trim());
      toast.success(t("admin.ipsScreen.bannedToast"), toastOptions);
      setBanOpen(false);
      loadStats();
      load();
    } catch (err) {
      // Toast the API's error (translated from its i18n code) rather than a line
      // behind the modal; the modal stays open so the address can be corrected.
      toastError(err);
    } finally {
      setBusy(false);
    }
  };

  const unban = async (ip) => {
    try {
      await adminApi.unbanIp(ip);
      toast.success(t("admin.ipsScreen.unbannedToast"), toastOptions);
      loadStats();
      load();
    } catch (err) {
      toastError(err);
    }
    setConfirmUnban(null);
  };

  const rows = data?.results || null;

  return (
    <>
      <div className="sf-row-between" style={{ marginBottom: "0.8rem", flexWrap: "wrap", gap: "0.6rem" }}>
        <div className="sf-row" style={{ gap: "1.5rem" }}>
          <div>
            <span className="sf-stat-value">{stats ? stats.total : "—"}</span>
            <span className="sf-stat-label">{t("admin.ipsScreen.totalIps")}</span>
          </div>
          <div>
            <span className="sf-stat-value">{stats ? stats.banned : "—"}</span>
            <span className="sf-stat-label">{t("admin.ipsScreen.bannedIps")}</span>
          </div>
        </div>
        <div className="sf-row" style={{ gap: "0.5rem", flexWrap: "wrap" }}>
          <input
            className="sf-input"
            style={{ maxWidth: 240 }}
            placeholder={t("admin.ipsScreen.searchPlaceholder")}
            value={input}
            onChange={(e) => setInput(e.target.value)}
          />
          <button className="sf-button sf-button-danger sf-button-sm" onClick={() => openBan("")}>
            <FiSlash /> {t("admin.ipsScreen.banNew")}
          </button>
        </div>
      </div>

      <p className="sf-muted" style={{ marginTop: 0 }}>
        {t("admin.ipsScreen.help")}
      </p>
      <ErrorMessage error={error} />

      {rows === null ? (
        <Spinner />
      ) : rows.length === 0 ? (
        <EmptyState title={t("admin.ipsScreen.emptyTitle")} message={t("admin.ipsScreen.emptyBody")} />
      ) : (
        <div className="sf-card">
          <div className="sf-table-wrapper">
            <table className="sf-table">
              <thead>
                <tr>
                  <th>{t("admin.ip")}</th>
                  <th>{t("admin.ipsScreen.members")}</th>
                  <th>{t("admin.ipsScreen.location")}</th>
                  <th>{t("admin.ipsScreen.network")}</th>
                  <th className="sf-table-num">{t("admin.ipCount")}</th>
                  <th>{t("admin.ipLastSeen")}</th>
                  <th>{t("admin.ipsScreen.status")}</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {rows.map((row) => (
                  <tr key={row.ip}>
                    <td>
                      <code>{row.ip}</code>
                    </td>
                    <td>
                      {row.members.length === 0 ? (
                        <span className="sf-muted">{t("admin.ipsScreen.anonymous")}</span>
                      ) : (
                        row.members.map((m, i) => (
                          <span key={m.id}>
                            {i > 0 ? ", " : ""}
                            {m.handle ? <Link to={`/profile/${m.handle}`}>{m.email}</Link> : m.email}
                          </span>
                        ))
                      )}
                    </td>
                    <td className="sf-muted">
                      {geoLabel(row.geo) ? (
                        <>
                          <FiMapPin style={{ verticalAlign: "-2px" }} /> {geoLabel(row.geo)}
                        </>
                      ) : (
                        "—"
                      )}
                    </td>
                    <td className="sf-muted">{networkLabel(row.geo) || "—"}</td>
                    <td className="sf-table-num">{row.count}</td>
                    <td className="sf-muted">{row.lastSeen ? new Date(row.lastSeen).toLocaleString() : "—"}</td>
                    <td>
                      {row.banned ? (
                        <span className="sf-badge sf-badge-danger" title={row.reason || ""}>
                          {t("admin.ipsScreen.banned")}
                        </span>
                      ) : (
                        <span className="sf-muted">—</span>
                      )}
                    </td>
                    <td className="sf-table-num">
                      {row.banned ? (
                        <button className="sf-button sf-button-secondary sf-button-sm" onClick={() => setConfirmUnban(row)}>
                          {t("admin.ipsScreen.unban")}
                        </button>
                      ) : (
                        <button className="sf-button sf-button-danger sf-button-sm" onClick={() => openBan(row.ip)}>
                          {t("admin.ipsScreen.ban")}
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="sf-row" style={{ justifyContent: "flex-end", gap: "0.5rem", marginTop: "0.6rem" }}>
            <button
              className="sf-button sf-button-secondary sf-button-sm"
              disabled={page === 0}
              onClick={() => setPage((p) => Math.max(0, p - 1))}
            >
              {t("admin.ipsScreen.prev")}
            </button>
            <span className="sf-muted" style={{ alignSelf: "center" }}>
              {page + 1}
            </span>
            <button
              className="sf-button sf-button-secondary sf-button-sm"
              disabled={!data?.hasNext}
              onClick={() => setPage((p) => p + 1)}
            >
              {t("admin.ipsScreen.next")}
            </button>
          </div>
        </div>
      )}

      {banOpen ? (
        <Modal
          title={t("admin.ipsScreen.banTitle")}
          onClose={() => setBanOpen(false)}
          actions={
            <>
              <button className="sf-button sf-button-secondary" onClick={() => setBanOpen(false)}>
                {t("common.cancel")}
              </button>
              <button className="sf-button sf-button-danger" onClick={submitBan} disabled={busy || !banIpValue.trim()}>
                {t("admin.ipsScreen.ban")}
              </button>
            </>
          }
        >
          <div className="sf-field">
            <label className="sf-label">{t("admin.ip")}</label>
            <input className="sf-input" value={banIpValue} onChange={(e) => setBanIpValue(e.target.value)} placeholder="203.0.113.7" autoFocus />
          </div>
          <div className="sf-field">
            <label className="sf-label">{t("admin.ipsScreen.reason")}</label>
            <input
              className="sf-input"
              value={banMotif}
              onChange={(e) => setBanMotif(e.target.value)}
              placeholder={t("admin.ipsScreen.reasonPlaceholder")}
            />
          </div>
        </Modal>
      ) : null}

      {confirmUnban ? (
        <ConfirmModal
          title={t("admin.ipsScreen.unban")}
          message={t("admin.ipsScreen.confirmUnban", { ip: confirmUnban.ip })}
          onConfirm={() => unban(confirmUnban.ip)}
          onClose={() => setConfirmUnban(null)}
        />
      ) : null}
    </>
  );
}

// The audit actions and subject types the filter dropdowns offer, mirroring the
// backend's models.AuditActions / models.AuditSubjectTypes. The stored values
// carry dots ("ai.call"), so each maps to a dot-free i18n leaf the screen shows.
const AUDIT_ACTIONS = [
  "auth.signedIn",
  "account.registered",
  "ai.call",
  "user.roleChanged",
  "user.deleted",
  "ip.banned",
  "ip.unbanned",
];
const AUDIT_SUBJECTS = ["user", "aiConnection", "ip"];

const AUDIT_ACTION_LABEL = {
  "auth.signedIn": "actionSignedIn",
  "account.registered": "actionRegistered",
  "ai.call": "actionAiCall",
  "user.roleChanged": "actionRoleChanged",
  "user.deleted": "actionUserDeleted",
  "ip.banned": "actionIpBanned",
  "ip.unbanned": "actionIpUnbanned",
};
const AUDIT_SUBJECT_LABEL = { user: "subjectUser", aiConnection: "subjectAiConnection", ip: "subjectIp" };

const auditActionLabel = (t, action) =>
  AUDIT_ACTION_LABEL[action] ? t(`admin.audit.${AUDIT_ACTION_LABEL[action]}`) : action;
const auditSubjectLabel = (t, subject) =>
  AUDIT_SUBJECT_LABEL[subject] ? t(`admin.audit.${AUDIT_SUBJECT_LABEL[subject]}`) : subject;

// auditDetails flattens an event's detail map to "key=value · key=value", the
// one compact cell that says what the event carried (tokens, a ban reason, a
// role change's from/to) without a column per possible field.
function auditDetails(detail) {
  if (!detail || typeof detail !== "object") return "";
  return Object.entries(detail)
    .filter(([, value]) => value !== null && value !== undefined && value !== "")
    .map(([key, value]) => `${key}=${typeof value === "object" ? JSON.stringify(value) : value}`)
    .join(" · ");
}

/**
 * The audit trail: every auditable event newest-first - AI calls, sign-ins,
 * role changes, bans, deletions. Pages load on scroll (like the feed) rather
 * than with prev/next, since the trail only grows and reads newest-first.
 */
function AuditTab() {
  const { t } = useI18n();
  const [rows, setRows] = useState([]);
  const [page, setPage] = useState(0);
  const [hasNext, setHasNext] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadedOnce, setLoadedOnce] = useState(false);
  const [error, setError] = useState(null);
  const [action, setAction] = useState("");
  const [subject, setSubject] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const sentinel = useRef(null);

  // Changing a filter restarts paging from scratch.
  useEffect(() => {
    setRows([]);
    setPage(0);
    setHasNext(false);
    setLoadedOnce(false);
    setLoading(true);
  }, [action, subject, from, to]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      try {
        const result = await adminApi.audit({ page, action, subject, from, to });
        if (cancelled) return;
        setRows((current) => (page === 0 ? result.results : [...current, ...result.results]));
        setHasNext(result.hasNext);
        setError(null);
      } catch (err) {
        if (!cancelled) setError(err);
      } finally {
        if (!cancelled) {
          setLoading(false);
          setLoadedOnce(true);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [page, action, subject, from, to]);

  useEffect(() => {
    const node = sentinel.current;
    if (!node || !loadedOnce || loading || !hasNext) return;
    const observer = new IntersectionObserver(
      (entries) => entries[0].isIntersecting && setPage((current) => current + 1),
      { rootMargin: "200px" }
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, [loadedOnce, loading, hasNext]);

  return (
    <>
      <div className="sf-row-between" style={{ marginBottom: "0.8rem", flexWrap: "wrap", gap: "0.6rem" }}>
        <p className="sf-muted" style={{ margin: 0 }}>
          {t("admin.audit.help")}
        </p>
        <div className="sf-row" style={{ gap: "0.5rem", flexWrap: "wrap" }}>
          <label className="sf-row" style={{ gap: "0.35rem", alignItems: "center" }}>
            <span className="sf-muted" style={{ fontSize: "0.85rem" }}>
              {t("admin.audit.from")}
            </span>
            <input
              type="date"
              className="sf-input"
              style={{ maxWidth: 170 }}
              value={from}
              max={to || undefined}
              onChange={(e) => setFrom(e.target.value)}
            />
          </label>
          <label className="sf-row" style={{ gap: "0.35rem", alignItems: "center" }}>
            <span className="sf-muted" style={{ fontSize: "0.85rem" }}>
              {t("admin.audit.to")}
            </span>
            <input
              type="date"
              className="sf-input"
              style={{ maxWidth: 170 }}
              value={to}
              min={from || undefined}
              onChange={(e) => setTo(e.target.value)}
            />
          </label>
          <select className="sf-select" value={action} onChange={(e) => setAction(e.target.value)}>
            <option value="">{t("admin.audit.allActions")}</option>
            {AUDIT_ACTIONS.map((a) => (
              <option key={a} value={a}>
                {auditActionLabel(t, a)}
              </option>
            ))}
          </select>
          <select className="sf-select" value={subject} onChange={(e) => setSubject(e.target.value)}>
            <option value="">{t("admin.audit.allSubjects")}</option>
            {AUDIT_SUBJECTS.map((s) => (
              <option key={s} value={s}>
                {auditSubjectLabel(t, s)}
              </option>
            ))}
          </select>
        </div>
      </div>

      <ErrorMessage error={error} />

      {loadedOnce && rows.length === 0 && !loading ? (
        <EmptyState title={t("admin.audit.emptyTitle")} message={t("admin.audit.emptyBody")} />
      ) : rows.length > 0 ? (
        <div className="sf-card">
          <div className="sf-table-wrapper">
            <table className="sf-table">
              <thead>
                <tr>
                  <th>{t("admin.audit.when")}</th>
                  <th>{t("admin.audit.who")}</th>
                  <th>{t("admin.audit.action")}</th>
                  <th>{t("admin.audit.subject")}</th>
                  <th>{t("admin.audit.details")}</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((row) => (
                  <tr key={row.id}>
                    <td className="sf-muted">{new Date(row.createdAt).toLocaleString()}</td>
                    <td>{row.userEmail || <span className="sf-muted">{t("admin.audit.anonymous")}</span>}</td>
                    <td>{auditActionLabel(t, row.action)}</td>
                    <td className="sf-muted">
                      {row.subjectType ? auditSubjectLabel(t, row.subjectType) : "—"}
                      {row.subjectId ? (
                        <>
                          {" "}
                          <code>{row.subjectId}</code>
                        </>
                      ) : null}
                    </td>
                    <td className="sf-muted">{auditDetails(row.detail) || "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      ) : null}

      {loading ? <Spinner /> : null}
      {loadedOnce && hasNext ? <div ref={sentinel} style={{ height: 1 }} /> : null}
    </>
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
              {/* The picture opens the person behind the request: deciding on
                  a coach means looking at who they are, and the profile route
                  takes an id as readily as a handle, so an applicant who never
                  picked a profile name opens just the same. */}
              <Link to={`/profile/${applicant.handle || applicant.id}`} title={t("search.openProfile")}>
                <Avatar user={applicant} size="sf-avatar-sm" />
              </Link>
              <div style={{ flex: 1, minWidth: 0 }}>
                <strong>
                  {applicant.name} {applicant.surname}
                </strong>
                {/* Which role they applied for - coach or nutritionist - so the
                    superadmin confirms the right one. */}
                <span className="sf-badge" style={{ marginLeft: "0.4rem" }}>
                  {t(`profile.${applicant.request?.role === "nutritionist" ? "nutritionist" : "coach"}`)}
                </span>
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
