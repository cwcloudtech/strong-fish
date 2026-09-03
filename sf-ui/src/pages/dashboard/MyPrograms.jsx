import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { toast } from "react-toastify";
import { FiPlus, FiUpload } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import Modal from "../../components/common/Modal";
import Select from "../../components/common/Select";
import { programs as programsApi } from "../../api/services";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

/**
 * The member's own programs: the blocks they wrote for themselves.
 *
 * Separate from a club's library because they belong to nobody else - an
 * athlete who trains alone, or one adapting what their coach gave them. The
 * same editor opens them, since a program is a program whoever wrote it.
 */
export default function MyPrograms() {
  const { t } = useI18n();
  const [programs, setPrograms] = useState(null);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState(null);

  const load = useCallback(async () => {
    try {
      // No club id: that is what asks for the caller's own programs.
      setPrograms(await programsApi.list(""));
    } catch (err) {
      setError(err);
      setPrograms([]);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  if (programs === null) return <Spinner />;

  return (
    <div className="sf-page">
      <div className="sf-page-header">
        <div>
          <h1 className="sf-title">{t("programs.mine")}</h1>
          <p className="sf-subtitle">{t("programs.mineSubtitle")}</p>
        </div>
        <button className="sf-button" onClick={() => setCreating(true)}>
          <FiPlus /> {t("programs.new")}
        </button>
      </div>

      <ErrorMessage error={error} />

      {programs.length === 0 ? (
        <EmptyState title={t("programs.mineEmptyTitle")} message={t("programs.mineEmptyBody")} />
      ) : (
        <div className="sf-grid">
          {programs.map((program) => (
            <Link
              key={program.id}
              to={`/dashboard/programs/${program.id}`}
              className="sf-card sf-card-clickable"
              style={{ display: "block", color: "inherit" }}
            >
              <h3 style={{ margin: 0 }}>{program.name}</h3>
              {program.description ? <p className="sf-muted">{program.description}</p> : null}
              <div className="sf-muted">
                {t("programs.weeks", { count: program.weeks })} ·{" "}
                {t("programs.sessions", { count: program.dayCount })} ·{" "}
                {t("programs.setCount", { count: program.setCount })}
              </div>
            </Link>
          ))}
        </div>
      )}

      {creating ? (
        <NewProgramModal
          onClose={() => setCreating(false)}
          onCreated={async () => {
            setCreating(false);
            toast.success(t("programs.created"), toastOptions);
            await load();
          }}
        />
      ) : null}
    </div>
  );
}

/**
 * Writes a new personal program, empty or from a spreadsheet.
 *
 * The same importer a coach uses: an athlete who keeps their block in a
 * spreadsheet should not have to retype it to get the loads worked out.
 */
function NewProgramModal({ onClose, onCreated }) {
  const { t } = useI18n();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  // Private by default: a block somebody writes for themselves is theirs until
  // they say otherwise.
  const [visibility, setVisibility] = useState("private");
  const [file, setFile] = useState(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      if (file) {
        await programsApi.importFile("", file, name, description);
      } else {
        await programsApi.create("", { name, description, visibility });
      }
      await onCreated();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  return (
    <Modal
      title={t("programs.new")}
      onClose={onClose}
      actions={
        <>
          <button className="sf-button sf-button-secondary" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className="sf-button" onClick={submit} disabled={busy || !name.trim()}>
            {t("common.save")}
          </button>
        </>
      }
    >
      <div className="sf-field">
        <label className="sf-label" htmlFor="programName">
          {t("programs.name")}
        </label>
        <input
          id="programName"
          className="sf-input"
          value={name}
          onChange={(event) => setName(event.target.value)}
          autoFocus
        />
      </div>

      <div className="sf-field">
        <label className="sf-label" htmlFor="programDescription">
          {t("programs.description")}
        </label>
        <textarea
          id="programDescription"
          className="sf-textarea"
          rows={2}
          value={description}
          onChange={(event) => setDescription(event.target.value)}
        />
      </div>

      <div className="sf-field">
        <label className="sf-label">{t("programs.visibility")}</label>
        <Select
          options={[
            { value: "private", label: t("programs.visibilityPrivate") },
            { value: "club", label: t("programs.visibilityClubs") },
            { value: "public", label: t("programs.visibilityPublic") },
          ]}
          value={visibility}
          onChange={setVisibility}
        />
        <p className="sf-muted" style={{ margin: "0.3rem 0 0", fontSize: "0.85rem" }}>
          {t(`programs.visibility${visibility === "private" ? "PrivateHelp" : visibility === "club" ? "ClubsHelp" : "PublicHelp"}`)}
        </p>
      </div>

      <div className="sf-field">
        <label className="sf-label" htmlFor="programFile">
          <FiUpload /> {t("programs.importOptional")}
        </label>
        <input
          id="programFile"
          className="sf-input"
          type="file"
          accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
          onChange={(event) => setFile(event.target.files?.[0] || null)}
        />
      </div>

      <ErrorMessage error={error} />
    </Modal>
  );
}
