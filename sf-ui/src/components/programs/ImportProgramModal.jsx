import { useState } from "react";
import { toast } from "react-toastify";

import { oneRms as oneRmsApi, programs as programsApi } from "../../api/services";
import Modal from "../common/Modal";
import { ErrorMessage } from "../common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

/**
 * Uploads a coach's program spreadsheet.
 *
 * The result screen is deliberately chatty: the importer has to infer things the
 * file doesn't state outright (which competition max each movement is programmed
 * off, what a clobbered day title should have said), so the coach is shown what
 * was created and what was guessed rather than being left to discover it later.
 */
export default function ImportProgramModal({ club, onClose, onImported }) {
  const { t, locale } = useI18n();
  const [file, setFile] = useState(null);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [result, setResult] = useState(null);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const submit = async () => {
    if (!file) return;
    setBusy(true);
    setError(null);
    try {
      setResult(await programsApi.importFile(club.id, file, name, description));
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  // The reference maxes the file was written against aren't anyone's 1RM - they
  // are the author's. Applying them is an explicit choice, so the coach can
  // start from them and adjust rather than having them silently become theirs.
  const applyReferenceOneRms = async () => {
    const entries = Object.entries(result.referenceOneRms || {});
    await Promise.all(entries.map(([exerciseId, value]) => oneRmsApi.set(exerciseId, value)));
    toast.success(t("oneRms.saved"));
  };

  if (result) {
    return (
      <Modal
        title={t("programs.import")}
        onClose={onImported}
        wide
        actions={
          <button className="sf-button" onClick={onImported}>
            {t("common.close")}
          </button>
        }
      >
        <div className="sf-notice">
          {t("programs.imported", {
            weeks: result.program.weeks,
            days: result.program.dayCount,
            sets: result.program.setCount,
          })}
        </div>

        {result.createdExercises?.length > 0 ? (
          <p className="sf-muted">
            {t("programs.createdExercises", {
              list: result.createdExercises
                .map((exercise) => exercise.labels?.[locale] || exercise.labels?.en || exercise.slug)
                .join(", "),
            })}
          </p>
        ) : null}

        {Object.keys(result.referenceOneRms || {}).length > 0 ? (
          <div className="sf-row-between" style={{ marginTop: "0.5rem" }}>
            <span className="sf-muted">{t("programs.referenceOneRms")}</span>
            <button className="sf-button sf-button-secondary sf-button-sm" onClick={applyReferenceOneRms}>
              {t("programs.useAsMyOneRms")}
            </button>
          </div>
        ) : null}

        {result.warnings?.length > 0 ? (
          <div className="sf-notice sf-notice-warning" style={{ marginTop: "0.8rem" }}>
            <strong>{t("programs.warnings")}</strong>
            <ul style={{ margin: "0.4rem 0 0", paddingLeft: "1.1rem" }}>
              {result.warnings.map((warning) => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
          </div>
        ) : null}
      </Modal>
    );
  }

  return (
    <Modal
      title={t("programs.import")}
      onClose={onClose}
      actions={
        <>
          <button className="sf-button sf-button-secondary" onClick={onClose} disabled={busy}>
            {t("common.cancel")}
          </button>
          <button className="sf-button" onClick={submit} disabled={busy || !file}>
            {busy ? t("programs.importing") : t("programs.import")}
          </button>
        </>
      }
    >
      <p className="sf-muted">{t("programs.importHelp")}</p>

      <div className="sf-field">
        <input
          className="sf-input"
          type="file"
          accept=".xlsx,.xlsm,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
          onChange={(event) => setFile(event.target.files?.[0] || null)}
        />
      </div>
      <div className="sf-field">
        <label className="sf-label">
          {t("programs.programName")} <span className="sf-muted">({t("common.optional")})</span>
        </label>
        <input className="sf-input" value={name} onChange={(event) => setName(event.target.value)} />
      </div>
      <div className="sf-field">
        <label className="sf-label">
          {t("clubs.description")} <span className="sf-muted">({t("common.optional")})</span>
        </label>
        <textarea className="sf-textarea" value={description} onChange={(event) => setDescription(event.target.value)} />
      </div>

      <ErrorMessage error={error} />
    </Modal>
  );
}
