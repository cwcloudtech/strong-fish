import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { toast } from "react-toastify";

import { clubs as clubsApi } from "../../api/services";
import Modal from "../../components/common/Modal";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";

export default function Clubs() {
  const { t } = useI18n();
  const { isCoach } = useAuth();
  const [clubs, setClubs] = useState(null);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    clubsApi.list().then(setClubs).catch(setError);
  }, []);

  if (error) return <div className="sf-page"><ErrorMessage error={error} /></div>;
  if (!clubs) return <Spinner />;

  return (
    <div className="sf-page">
      <div className="sf-page-header">
        <h1>{t("clubs.title")}</h1>
        {isCoach ? (
          <button className="sf-button" onClick={() => setCreating(true)}>
            {t("clubs.create")}
          </button>
        ) : null}
      </div>

      {clubs.length === 0 ? (
        <EmptyState message={isCoach ? t("clubs.emptyCoach") : t("clubs.empty")} />
      ) : (
        <div className="sf-grid">
          {clubs.map((club) => (
            <Link key={club.id} to={`/dashboard/clubs/${club.id}`} className="sf-card sf-card-clickable" style={{ display: "block", color: "inherit" }}>
              <div className="sf-row-between">
                <h3 style={{ margin: 0 }}>{club.name}</h3>
                <span className="sf-badge">{t(`clubs.${club.role}`)}</span>
              </div>
              {club.description ? <p className="sf-muted">{club.description}</p> : null}
              <div className="sf-muted">
                {t("clubs.memberCount", { count: club.memberCount })}
                {club.city ? ` · ${club.city}` : ""}
              </div>
            </Link>
          ))}
        </div>
      )}

      {creating ? (
        <ClubFormModal
          onClose={() => setCreating(false)}
          onSaved={(club) => {
            setClubs((current) => [club, ...current]);
            setCreating(false);
            toast.success(t("clubs.created"));
          }}
        />
      ) : null}
    </div>
  );
}

/** The create/edit club dialog, shared by this page and the club detail page. */
export function ClubFormModal({ club, onClose, onSaved }) {
  const { t } = useI18n();
  const [form, setForm] = useState({
    name: club?.name || "",
    description: club?.description || "",
    city: club?.city || "",
    country: club?.country || "",
  });
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const set = (field) => (event) => setForm((current) => ({ ...current, [field]: event.target.value }));

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      onSaved(club ? await clubsApi.update(club.id, form) : await clubsApi.create(form));
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  return (
    <Modal
      title={club ? t("clubs.edit") : t("clubs.create")}
      onClose={onClose}
      actions={
        <>
          <button className="sf-button sf-button-secondary" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className="sf-button" onClick={submit} disabled={busy || !form.name.trim()}>
            {t("common.save")}
          </button>
        </>
      }
    >
      <div className="sf-field">
        <label className="sf-label">{t("clubs.name")}</label>
        <input className="sf-input" value={form.name} onChange={set("name")} autoFocus />
      </div>
      <div className="sf-field">
        <label className="sf-label">{t("clubs.description")}</label>
        <textarea className="sf-textarea" value={form.description} onChange={set("description")} />
      </div>
      <div className="sf-row" style={{ gap: "0.6rem" }}>
        <div className="sf-field" style={{ flex: 1 }}>
          <label className="sf-label">{t("clubs.city")}</label>
          <input className="sf-input" value={form.city} onChange={set("city")} />
        </div>
        <div className="sf-field" style={{ flex: 1 }}>
          <label className="sf-label">{t("clubs.country")}</label>
          <input className="sf-input" value={form.country} onChange={set("country")} />
        </div>
      </div>
      <ErrorMessage error={error} />
    </Modal>
  );
}
