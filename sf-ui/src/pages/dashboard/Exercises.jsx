import { useCallback, useEffect, useState } from "react";
import { toast } from "react-toastify";
import { FiAward, FiEdit2, FiTrash2 } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import { exercises as exercisesApi } from "../../api/services";
import Modal, { ConfirmModal } from "../../components/common/Modal";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";

const CATEGORIES = ["squat", "bench", "deadlift", "accessory"];
const categoryLabel = (t, category) => t(`exercises.category${category[0].toUpperCase()}${category.slice(1)}`);
const label = (exercise, locale) => exercise.labels?.[locale] || exercise.labels?.en || exercise.slug;

/**
 * The shared exercise catalog. It's global rather than per-club on purpose: a
 * coach adds "larsen press" once and it shows up in every other coach's
 * autocomplete, and in every importer that meets that spelling.
 *
 * That sharing is also why editing and deleting are the superadmin's: a rename
 * ripples through everyone's programs, and a delete cascades into their sets.
 */
export default function Exercises() {
  const { t, locale } = useI18n();
  const { isSuperadmin } = useAuth();
  const [list, setList] = useState(null);
  const [query, setQuery] = useState("");
  const [editing, setEditing] = useState(null);
  const [deleting, setDeleting] = useState(null);
  const [error, setError] = useState(null);

  const load = useCallback(() => {
    exercisesApi.list(query).then(setList).catch(setError);
  }, [query]);

  useEffect(() => {
    const timer = setTimeout(load, query ? 250 : 0);
    return () => clearTimeout(timer);
  }, [load, query]);

  return (
    <div className="sf-page">
      <div className="sf-page-header">
        <div>
          <h1 className="sf-title">{t("exercises.title")}</h1>
          <p className="sf-subtitle">{t("exercises.subtitle")}</p>
        </div>
        <button className="sf-button" onClick={() => setEditing({})}>
          {t("exercises.create")}
        </button>
      </div>

      {!isSuperadmin ? <div className="sf-notice">{t("exercises.adminOnly")}</div> : null}

      <div className="sf-field" style={{ maxWidth: 320 }}>
        <input
          className="sf-input"
          placeholder={t("common.search")}
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
      </div>

      <ErrorMessage error={error} />

      {!list ? (
        <Spinner />
      ) : list.length === 0 ? (
        <EmptyState message={t("exercises.empty")} />
      ) : (
        <div className="sf-card">
          <div className="sf-table-wrapper">
            <table className="sf-table">
              <thead>
                <tr>
                  <th>{t("exercises.name")}</th>
                  <th>{t("exercises.category")}</th>
                  <th>{t("exercises.oneRmRef")}</th>
                  {isSuperadmin ? <th /> : null}
                </tr>
              </thead>
              <tbody>
                {list.map((exercise) => (
                  <tr key={exercise.id}>
                    <td>
                      <div className="sf-row" style={{ gap: "0.4rem" }}>
                        {label(exercise, locale)}
                        {exercise.main ? (
                          <span className="sf-badge" title={t("exercises.mainHelp")}>
                            <FiAward style={{ verticalAlign: "-2px" }} /> {t("exercises.main")}
                          </span>
                        ) : null}
                      </div>
                      <div className="sf-muted">{exercise.slug}</div>
                    </td>
                    <td>
                      <span className="sf-badge sf-badge-muted">{categoryLabel(t, exercise.category)}</span>
                      {exercise.bodyweight ? (
                        <span className="sf-badge sf-badge-muted" style={{ marginLeft: "0.3rem" }}>
                          {t("session.bodyweight")}
                        </span>
                      ) : null}
                    </td>
                    <td className="sf-muted">
                      {exercise.oneRmRef ? categoryLabel(t, exercise.oneRmRef) : t("exercises.oneRmRefNone")}
                    </td>
                    {isSuperadmin ? (
                      <td className="sf-table-num">
                        <div className="sf-row" style={{ justifyContent: "flex-end", gap: "0.25rem" }}>
                          <button
                            className="sf-button sf-button-ghost sf-button-sm"
                            onClick={() => setEditing(exercise)}
                            aria-label={t("common.edit")}
                          >
                            <FiEdit2 />
                          </button>
                          <button
                            className="sf-button sf-button-ghost sf-button-sm"
                            onClick={() => setDeleting(exercise)}
                            aria-label={t("common.delete")}
                          >
                            <FiTrash2 />
                          </button>
                        </div>
                      </td>
                    ) : null}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {editing ? (
        <ExerciseFormModal
          exercise={editing.id ? editing : null}
          canFlagMain={isSuperadmin}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            toast.success(t("exercises.saved"), toastOptions);
            load();
          }}
        />
      ) : null}

      {deleting ? (
        <DeleteExerciseModal
          exercise={deleting}
          onClose={() => setDeleting(null)}
          onDeleted={() => {
            setDeleting(null);
            toast.success(t("exercises.deleted"), toastOptions);
            load();
          }}
        />
      ) : null}
    </div>
  );
}

/**
 * Deleting a catalog entry cascades into every program prescribing it, so the
 * impact is fetched and shown before the confirmation rather than after the
 * fact - which is what the instruction asks for.
 */
function DeleteExerciseModal({ exercise, onClose, onDeleted }) {
  const { t, locale } = useI18n();
  const [usage, setUsage] = useState(null);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    exercisesApi.usage(exercise.id).then(setUsage).catch(setError);
  }, [exercise.id]);

  const confirm = async () => {
    setBusy(true);
    try {
      await exercisesApi.remove(exercise.id);
      onDeleted();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  const name = label(exercise, locale);
  const affects = usage && (usage.sets > 0 || usage.oneRms > 0);

  return (
    <ConfirmModal
      title={t("common.delete")}
      confirmLabel={t("common.delete")}
      onConfirm={confirm}
      onClose={onClose}
      busy={busy || !usage}
      message={
        <>
          {!usage ? (
            <p style={{ margin: 0 }}>{t("exercises.checkingUsage")}</p>
          ) : affects ? (
            <div className="sf-notice sf-notice-danger" style={{ marginBottom: 0 }}>
              {t("exercises.usageWarning", {
                name,
                sets: usage.sets,
                programs: usage.programs,
                oneRms: usage.oneRms,
              })}
            </div>
          ) : (
            <p style={{ margin: 0 }}>{t("exercises.usageNone", { name })}</p>
          )}
          <ErrorMessage error={error} />
        </>
      }
    />
  );
}

function ExerciseFormModal({ exercise, canFlagMain, onClose, onSaved }) {
  const { t } = useI18n();
  const [form, setForm] = useState({
    labelEn: exercise?.labels?.en || "",
    labelFr: exercise?.labels?.fr || "",
    aliases: (exercise?.aliases || []).join(", "),
    category: exercise?.category || "accessory",
    oneRmRef: exercise?.oneRmRef || "",
    bodyweight: exercise?.bodyweight || false,
    main: exercise?.main || false,
  });
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const set = (field) => (event) =>
    setForm((current) => ({
      ...current,
      [field]: event.target.type === "checkbox" ? event.target.checked : event.target.value,
    }));

  const submit = async () => {
    setBusy(true);
    setError(null);
    const payload = {
      name: form.labelEn,
      labels: { en: form.labelEn, fr: form.labelFr },
      aliases: form.aliases.split(",").map((alias) => alias.trim()).filter(Boolean),
      category: form.category,
      oneRmRef: form.oneRmRef,
      bodyweight: form.bodyweight,
      main: form.main,
    };
    try {
      if (exercise) await exercisesApi.update(exercise.id, payload);
      else await exercisesApi.create(payload);
      onSaved();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  return (
    <Modal
      title={exercise ? t("exercises.edit") : t("exercises.create")}
      onClose={onClose}
      actions={
        <>
          <button className="sf-button sf-button-secondary" onClick={onClose} disabled={busy}>
            {t("common.cancel")}
          </button>
          <button className="sf-button" onClick={submit} disabled={busy || !form.labelEn.trim()}>
            {t("common.save")}
          </button>
        </>
      }
    >
      <div className="sf-row" style={{ gap: "0.6rem", alignItems: "flex-start" }}>
        <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
          <label className="sf-label">{t("exercises.labelEn")}</label>
          <input className="sf-input" value={form.labelEn} onChange={set("labelEn")} autoFocus />
        </div>
        <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
          <label className="sf-label">{t("exercises.labelFr")}</label>
          <input className="sf-input" value={form.labelFr} onChange={set("labelFr")} />
        </div>
      </div>

      <div className="sf-field">
        <label className="sf-label">{t("exercises.aliases")}</label>
        <input className="sf-input" value={form.aliases} onChange={set("aliases")} />
        <p className="sf-muted">{t("exercises.aliasesHelp")}</p>
      </div>

      <div className="sf-row" style={{ gap: "0.6rem", alignItems: "flex-start" }}>
        <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
          <label className="sf-label">{t("exercises.category")}</label>
          <select className="sf-select" value={form.category} onChange={set("category")}>
            {CATEGORIES.map((category) => (
              <option key={category} value={category}>
                {categoryLabel(t, category)}
              </option>
            ))}
          </select>
        </div>
        <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
          <label className="sf-label">{t("exercises.oneRmRef")}</label>
          <select className="sf-select" value={form.oneRmRef} onChange={set("oneRmRef")}>
            <option value="">{t("exercises.oneRmRefNone")}</option>
            {CATEGORIES.filter((category) => category !== "accessory").map((category) => (
              <option key={category} value={category}>
                {categoryLabel(t, category)}
              </option>
            ))}
          </select>
        </div>
      </div>
      <p className="sf-muted">{t("exercises.oneRmRefHelp")}</p>

      <label className="sf-checkbox" style={{ marginBottom: "0.6rem" }}>
        <input type="checkbox" checked={form.bodyweight} onChange={set("bodyweight")} />
        {t("exercises.bodyweight")}
      </label>

      {/* The competition flag is instance-wide - it decides which 1RMs every
          member is prompted for - so only a superadmin may set it. */}
      {canFlagMain ? (
        <label className="sf-checkbox">
          <input type="checkbox" checked={form.main} onChange={set("main")} />
          <span>
            {t("exercises.main")}
            <div className="sf-muted">{t("exercises.mainHelp")}</div>
          </span>
        </label>
      ) : null}

      <ErrorMessage error={error} />
    </Modal>
  );
}
