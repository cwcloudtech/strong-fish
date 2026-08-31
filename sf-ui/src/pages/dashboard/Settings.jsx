import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { toast } from "react-toastify";
import { FiArrowDown, FiArrowUp, FiEdit2, FiPlus, FiSave, FiTrash2, FiUsers } from "react-icons/fi";

import Switch from "../../components/common/Switch";

import toastOptions from "../../utils/toastOptions";
import { auth, media as mediaApi, mfa as mfaApi } from "../../api/services";
import Avatar from "../../components/common/Avatar";
import Modal from "../../components/common/Modal";
import Select from "../../components/common/Select";
import { search as searchApi } from "../../api/services";
import MultiSelect from "../../components/common/MultiSelect";
import SOCIAL_PROFILES from "../../utils/socialProfiles";
import ProfileBadges from "../../components/common/ProfileBadges";
import { ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";
import { readImageAsDataUrl } from "../../utils/image";
import { createCredential, isWebAuthnSupported } from "../../utils/webauthn";

/**
 * The badges a member may pick, in the order the API offers them (see the
 * API's models/specialty.go). The three lifts in competition order, then the
 * totaler as the "no single lift is mine" answer.
 */
const SPECIALTIES = ["squat", "bench", "deadlift", "total"];

/** The connected user's own settings: profile, avatar, and MFA enrollment. */
/**
 * The same normalization the API applies to a username, so the handle preview
 * shows what will actually be saved rather than what was typed.
 */
function slugify(value) {
  return (value || "")
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export default function Settings() {
  const { t, locale, setLocale, locales } = useI18n();
  const { user, setUser, config } = useAuth();
  const fileInput = useRef(null);

  const [form, setForm] = useState(null);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!user) return;
    setForm({
      name: user.name || "",
      surname: user.surname || "",
      username: user.username || "",
      anonymous: Boolean(user.anonymous),
      bio: user.bio || "",
      bodyweight: user.bodyweight || "",
      gender: user.gender || "male",
      specialty: user.specialty || "",
      // One key per entry in the table, plus the rank the one entry that has
      // it carries. Built from the table so a network added there needs
      // nothing here.
      socials: Object.fromEntries(
        SOCIAL_PROFILES.flatMap((network) => [
          [network.key, user.socials?.[network.key] || ""],
          ...(network.rankKey ? [[network.rankKey, user.socials?.[network.rankKey] || ""]] : []),
        ])
      ),
      profileVisibility: user.profileVisibility || "private",
      birthdate: user.birthdate || "",
      password: "",
      confirmPassword: "",
    });
  }, [user]);

  if (!form) return <Spinner />;

  const set = (field) => (event) =>
    setForm((current) => ({
      ...current,
      [field]: event.target.type === "checkbox" ? event.target.checked : event.target.value,
    }));

  const setSocial = (field) => (event) =>
    setForm((current) => ({ ...current, socials: { ...current.socials, [field]: event.target.value } }));

  const save = async (event) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const updated = await auth.updateProfile({
        ...form,
        bodyweight: form.bodyweight === "" ? 0 : Number(form.bodyweight),
        gender: form.gender,
        // The language is part of the profile so transactional emails go out in
        // the one the user is actually reading the app in.
        locale,
      });
      setUser(updated);
      setForm((current) => ({ ...current, password: "", confirmPassword: "" }));
      toast.success(t("profile.saved"), toastOptions);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const changeAvatar = async (event) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    try {
      const dataUrl = await readImageAsDataUrl(file, config?.maxImageSize);
      setUser(await auth.updatePicture(dataUrl, 50, 50));
      toast.success(t("profile.saved"), toastOptions);
    } catch (err) {
      setError(err.message === "too-large" ? t("errors.imageTooLarge") : err);
    }
  };

  return (
    <div className="sf-page">
      <div className="sf-page-header">
        <h1>{t("nav.settings")}</h1>
        {user.handle ? (
          <Link className="sf-button sf-button-secondary" to={`/profile/${user.handle}`}>
            {t("profile.title")}
          </Link>
        ) : null}
      </div>

      <form className="sf-card" onSubmit={save}>
        <div className="sf-row" style={{ marginBottom: "1rem" }}>
          <Avatar user={user} size="sf-avatar-lg" />
          <div>
            <button type="button" className="sf-button sf-button-secondary sf-button-sm" onClick={() => fileInput.current?.click()}>
              {t("profile.changeAvatar")}
            </button>
            <input ref={fileInput} type="file" accept="image/*" hidden onChange={changeAvatar} />
            <div className="sf-muted">{user.email}</div>
          </div>
        </div>

        <div className="sf-row" style={{ gap: "0.6rem" }}>
          <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
            <label className="sf-label">{t("auth.name")}</label>
            <input className="sf-input" value={form.name} onChange={set("name")} required />
          </div>
          <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
            <label className="sf-label">{t("auth.surname")}</label>
            <input className="sf-input" value={form.surname} onChange={set("surname")} required />
          </div>
        </div>

        <div className="sf-field">
          <label className="sf-label" htmlFor="username">
            {t("profile.username")} <span className="sf-muted">({t("common.optional")})</span>
          </label>
          <input id="username" className="sf-input" value={form.username} onChange={set("username")} />
          {/* The handle is not an editable field any more: it follows the
              username, or the name when there is none. Showing what it will
              become beats a second box that can disagree with this one. */}
          <p className="sf-muted" style={{ marginTop: "0.25rem" }}>
            {t("profile.handleHelp", {
              handle: slugify(form.username) || slugify(`${form.name} ${form.surname}`) || "...",
            })}
          </p>
        </div>

        {/* A switch rather than a checkbox: this turns a mode on, it is not
            one of a set of things being ticked. */}
        <label className={`sf-switch-row ${form.username.trim() ? "" : "disabled"}`}>
          <span>
            {t("profile.anonymous")}
            <div className="sf-muted">
              {form.username.trim() ? t("profile.anonymousHelp") : t("profile.anonymousNeedsUsername")}
            </div>
          </span>
          <Switch
            checked={form.anonymous}
            onChange={(anonymous) => setForm((current) => ({ ...current, anonymous }))}
            // Nothing to be known by without a username, and the API refuses
            // it - so the switch is unavailable rather than silently failing.
            disabled={!form.username.trim()}
            aria-label={t("profile.anonymous")}
          />
        </label>

        <div className="sf-field">
          <label className="sf-label">{t("profile.bio")}</label>
          <textarea className="sf-textarea" value={form.bio} onChange={set("bio")} />
        </div>

        <div className="sf-row" style={{ gap: "0.6rem" }}>
          {/* Beside the bodyweight because the two are asked the same
              question: powerlifting's coefficients are fitted separately for
              men and women, so a DOTS score cannot be computed without both.
              Male by default, which is what an account that has never been
              asked reads as. */}
          <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
            <label className="sf-label">{t("profile.gender")}</label>
            <Select
              options={[
                { value: "male", label: t("profile.genderMale") },
                { value: "female", label: t("profile.genderFemale") },
              ]}
              value={form.gender}
              onChange={(gender) => setForm((current) => ({ ...current, gender }))}
            />
          </div>
          <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
            <label className="sf-label">
              {t("profile.bodyweight")} ({t("common.kg")})
            </label>
            <input className="sf-input" type="number" min="0" step="0.1" value={form.bodyweight} onChange={set("bodyweight")} />
          </div>
          <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
            <label className="sf-label">{t("common.language")}</label>
            {/* The app's own list rather than a copy: Arabic was added to the
                header's picker and this one was left behind, so the language
                could be chosen from the top bar and not from the profile. */}
            <Select
              options={locales.map((option) => ({ value: option.code, label: option.label }))}
              value={locale}
              onChange={setLocale}
            />
          </div>
        </div>

        {/* What the member calls themselves as a lifter. It is a claim, not a
            computation: nobody's badge should change because they had a bad
            squat day, and somebody who has entered no maxes at all is still
            entitled to say what they are. Clearing it is a first-class choice,
            so the field is clearable rather than carrying a "none" option. */}
        <div className="sf-row" style={{ gap: "0.6rem", alignItems: "flex-start" }}>
          <div className="sf-field" style={{ flex: 1, minWidth: 220 }}>
            <label className="sf-label" htmlFor="specialty">
              {t("profile.specialty")} <span className="sf-muted">({t("common.optional")})</span>
            </label>
            <Select
              id="specialty"
              options={SPECIALTIES.map((value) => ({ value, label: t(`profile.specialties.${value}`) }))}
              value={form.specialty}
              onChange={(value) => setForm((current) => ({ ...current, specialty: value }))}
              placeholder={t("profile.specialtyNone")}
              clearable
            />
            <p className="sf-muted" style={{ fontSize: "0.82rem", marginBottom: 0 }}>
              {t("profile.specialtyHelp")}
            </p>
          </div>
          <div className="sf-field" style={{ flex: 1, minWidth: 220 }}>
            <label className="sf-label">{t("profile.badgesPreview")}</label>
            {/* The badges as somebody else will see them, so the picker is not
                a guess at what the profile ends up looking like. */}
            <ProfileBadges role={user.role} specialty={form.specialty} />
          </div>
        </div>

        {/* Generated from the table in utils/socialProfiles.js: the label, the
            icon and the address an account lives at all come from the entry,
            so this block does not grow when a network is added. */}
        <div className="sf-field">
          <label className="sf-label">{t("profile.socials")}</label>
          <p className="sf-muted" style={{ fontSize: "0.82rem", marginTop: 0 }}>
            {t("profile.socialsHelp")}
          </p>
          <div className="sf-socials-form">
            {SOCIAL_PROFILES.map(({ key, label, Icon, placeholder, rankKey }) => (
              <div className="sf-social-field" key={key}>
                <span className="sf-social-icon" aria-hidden="true">
                  <Icon />
                </span>
                <input
                  className="sf-input"
                  aria-label={label}
                  placeholder={`${label} · ${placeholder}`}
                  value={form.socials[key]}
                  onChange={setSocial(key)}
                />
                {rankKey ? (
                  <input
                    className="sf-input sf-social-rank"
                    aria-label={`${label} · ${t("profile.rank")}`}
                    placeholder={t("profile.rank")}
                    value={form.socials[rankKey]}
                    onChange={setSocial(rankKey)}
                  />
                ) : null}
              </div>
            ))}
          </div>
        </div>

        <div className="sf-row" style={{ gap: "0.6rem" }}>
          <div className="sf-field" style={{ flex: 1, minWidth: 200 }}>
            <label className="sf-label" htmlFor="profileVisibility">
              {t("profile.visibility")}
            </label>
            <Select
              id="profileVisibility"
              options={[
                { value: "public", label: t("profile.visibilityPublic") },
                { value: "clubs", label: t("profile.visibilityClubs") },
                { value: "private", label: t("profile.visibilityPrivate") },
              ]}
              value={form.profileVisibility}
              onChange={(value) => setForm((current) => ({ ...current, profileVisibility: value }))}
            />
            <p className="sf-muted" style={{ fontSize: "0.82rem", marginBottom: 0 }}>
              {t(`profile.visibilityHelp.${form.profileVisibility}`)}
            </p>
          </div>

          <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
            <label className="sf-label" htmlFor="birthdate">
              {t("profile.birthdate")} <span className="sf-muted">({t("common.optional")})</span>
            </label>
            <input
              id="birthdate"
              className="sf-input"
              type="date"
              value={form.birthdate}
              onChange={set("birthdate")}
            />
            <p className="sf-muted" style={{ fontSize: "0.82rem", marginBottom: 0 }}>
              {t("profile.birthdateHelp")}
            </p>
          </div>
        </div>

        <div className="sf-row" style={{ gap: "0.6rem" }}>
          <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
            <label className="sf-label">{t("auth.newPassword")}</label>
            <input className="sf-input" type="password" autoComplete="new-password" value={form.password} onChange={set("password")} />
          </div>
          <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
            <label className="sf-label">{t("auth.confirmPassword")}</label>
            <input
              className="sf-input"
              type="password"
              autoComplete="new-password"
              value={form.confirmPassword}
              onChange={set("confirmPassword")}
            />
          </div>
        </div>

        <ErrorMessage error={error} />
        <button className="sf-button" type="submit" disabled={busy}>
          {t("common.save")}
        </button>
      </form>

      <MfaSettings />
      <StorageSettings />
    </div>
  );
}

/**
 * The member's storage targets, in the order that decides where a posted link
 * points.
 *
 * StrongFish keeps no video of its own, so posting one means bringing your own
 * storage. More than one is allowed and useful: an upload is written to *every*
 * target, which is how a club keeps a second copy of its athletes' videos, and
 * the link in the post comes from the FIRST - so the order is a setting, not a
 * display preference.
 */
function StorageSettings() {
  const { t } = useI18n();
  const [state, setState] = useState(null);
  const [busy, setBusy] = useState(false);
  // Which target's form is open: a storage id, "new" while adding one, or
  // null. One at a time - two open credential forms is two ways to lose track
  // of which bucket you are editing.
  const [editing, setEditing] = useState(null);
  const [sharing, setSharing] = useState(null);

  const load = useCallback(async () => {
    try {
      setState(await mediaApi.storages());
    } catch {
      setState({ storages: [] });
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  if (!state) return null;

  const storages = state.storages || [];
  const megabytes = state.maxSize ? Math.round(state.maxSize / (1024 * 1024)) : 20;

  const act = async (run, message) => {
    setBusy(true);
    try {
      setState(await run());
      if (message) toast.success(message, toastOptions);
      setEditing(null);
    } catch (err) {
      toast.error(err?.response?.data?.message || t("errors.generic"), toastOptions);
    } finally {
      setBusy(false);
    }
  };

  // Moving a target is a swap with its neighbour, sent as the whole order:
  // the API takes a list, so there is one way for the order to be wrong rather
  // than two.
  const move = (storageId, by) => {
    const ids = storages.map((storage) => storage.id);
    const from = ids.indexOf(storageId);
    const to = from + by;
    if (from < 0 || to < 0 || to >= ids.length) return;
    [ids[from], ids[to]] = [ids[to], ids[from]];
    act(() => mediaApi.reorderStorages(ids));
  };

  return (
    <div className="sf-card">
      <h2>{t("storage.title")}</h2>
      <p className="sf-subtitle">{t("storage.subtitle", { size: megabytes })}</p>

      {storages.length === 0 ? (
        <p className="sf-muted">{t("storage.noTargets")}</p>
      ) : (
        <>
          <p className="sf-muted" style={{ fontSize: "0.85rem" }}>{t("storage.orderHelp")}</p>
          <ul className="sf-list">
            {storages.map((storage, index) => (
              <li className="sf-list-item" key={storage.id} style={{ flexWrap: "wrap" }}>
                <span className="sf-badge">{index + 1}</span>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <strong>
                    {storage.connection?.type === "google_drive" ? t("storage.typeDrive") : t("storage.typeS3")}
                  </strong>
                  <div className="sf-muted" style={{ fontSize: "0.85rem" }}>
                    {storageSummary(storage.connection)}
                    {index === 0 ? ` · ${t("storage.first")}` : ""}
                  </div>
                </div>

                <div className="sf-row" style={{ gap: "0.25rem" }}>
                  <button
                    type="button"
                    className="sf-button-ghost sf-button-sm"
                    onClick={() => move(storage.id, -1)}
                    disabled={busy || index === 0}
                    aria-label={t("storage.moveUp")}
                    title={t("storage.moveUp")}
                  >
                    <FiArrowUp />
                  </button>
                  <button
                    type="button"
                    className="sf-button-ghost sf-button-sm"
                    onClick={() => move(storage.id, 1)}
                    disabled={busy || index === storages.length - 1}
                    aria-label={t("storage.moveDown")}
                    title={t("storage.moveDown")}
                  >
                    <FiArrowDown />
                  </button>
                  <button
                    type="button"
                    className="sf-button-ghost sf-button-sm"
                    onClick={() => setSharing(sharing === storage.id ? null : storage.id)}
                    aria-label={t("storage.shares")}
                    title={t("storage.shares")}
                  >
                    <FiUsers />
                  </button>
                  <button
                    type="button"
                    className="sf-button-ghost sf-button-sm"
                    onClick={() => setEditing(editing === storage.id ? null : storage.id)}
                    aria-label={t("common.edit")}
                    title={t("common.edit")}
                  >
                    <FiEdit2 />
                  </button>
                  <button
                    type="button"
                    className="sf-button-ghost sf-button-sm"
                    onClick={() => act(() => mediaApi.clearStorage(storage.id), t("storage.cleared"))}
                    disabled={busy}
                    aria-label={t("storage.clear")}
                    title={t("storage.clear")}
                  >
                    <FiTrash2 />
                  </button>
                </div>

                {editing === storage.id ? (
                  <div style={{ flexBasis: "100%" }}>
                    <StorageForm
                      connection={storage.connection}
                      busy={busy}
                      onCancel={() => setEditing(null)}
                      onSubmit={(payload) =>
                        act(() => mediaApi.setStorage(storage.id, payload), t("storage.saved"))
                      }
                    />
                  </div>
                ) : null}

                {sharing === storage.id ? (
                  <div style={{ flexBasis: "100%" }}>
                    <StorageShares storageId={storage.id} />
                  </div>
                ) : null}
              </li>
            ))}
          </ul>
        </>
      )}

      {editing === "new" ? (
        <StorageForm
          connection={null}
          busy={busy}
          onCancel={() => setEditing(null)}
          onSubmit={(payload) => act(() => mediaApi.addStorage(payload), t("storage.saved"))}
        />
      ) : (
        <button type="button" className="sf-button" onClick={() => setEditing("new")}>
          <FiPlus /> {t("storage.addTarget")}
        </button>
      )}
    </div>
  );
}

/** One target in a line: enough to tell two buckets apart. */
function storageSummary(connection) {
  if (!connection) return "";
  if (connection.type === "google_drive") {
    return [connection.folderId, connection.path].filter(Boolean).join(" · ");
  }
  return [connection.bucketName, connection.endpoint, connection.path].filter(Boolean).join(" · ");
}

/**
 * The connection form, for a new target or one being edited.
 *
 * The same fields either way: what differs between adding and editing is only
 * which request the submit makes, and that belongs to the caller.
 */
function StorageForm({ connection, busy, onSubmit, onCancel }) {
  const { t } = useI18n();
  const [form, setForm] = useState(() => ({
    type: connection?.type || "s3",
    endpoint: connection?.endpoint || "",
    bucketName: connection?.bucketName || "",
    region: connection?.region || "",
    accessKey: connection?.accessKey || "",
    secretKey: connection?.secretKey || "",
    serviceAccountBase64: connection?.serviceAccountBase64 || "",
    folderId: connection?.folderId || "",
    path: connection?.path || "",
    publicBaseUrl: connection?.publicBaseUrl || "",
    private: Boolean(connection?.private),
  }));
  // The name of the key file that was picked, purely so the field can say
  // which one is loaded - the key itself is write-only and never comes back.
  const [serviceAccountFile, setServiceAccountFile] = useState("");

  const set = (field) => (event) => setForm((current) => ({ ...current, [field]: event.target.value }));

  /**
   * Reads a Google service-account key as a file rather than asking for base64.
   *
   * Google hands out a JSON file; making somebody base64 it by hand first is a
   * step that exists only because of how the value happens to be stored. The
   * data model is unchanged - the API still receives and stores base64 - the
   * encoding just happens here instead of in a terminal.
   */
  const readServiceAccount = (event) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;

    const reader = new FileReader();
    reader.onload = () => {
      // FileReader gives a data: URL; the payload is everything after the
      // comma, which is exactly the base64 the API wants.
      const base64 = String(reader.result || "").split(",")[1] || "";
      if (!base64) {
        toast.error(t("storage.serviceAccountReadError"), toastOptions);
        return;
      }
      setForm((current) => ({ ...current, serviceAccountBase64: base64 }));
      setServiceAccountFile(file.name);
    };
    reader.onerror = () => toast.error(t("storage.serviceAccountReadError"), toastOptions);
    reader.readAsDataURL(file);
  };

  return (
    <form
      style={{ marginTop: "0.8rem" }}
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit(form);
      }}
    >

      <div className="sf-field">
        <label className="sf-label" htmlFor="storageType">
          {t("storage.type")}
        </label>
        <select id="storageType" className="sf-select" value={form.type} onChange={set("type")}>
          <option value="s3">{t("storage.typeS3")}</option>
          <option value="google_drive">{t("storage.typeDrive")}</option>
        </select>
      </div>

      {form.type === "s3" ? (
        <>
          <div className="sf-row" style={{ gap: "0.6rem" }}>
            <div className="sf-field" style={{ flex: 2, minWidth: 200 }}>
              <label className="sf-label" htmlFor="endpoint">
                {t("storage.endpoint")}
              </label>
              <input
                id="endpoint"
                className="sf-input"
                placeholder="https://s3.eu-west-3.amazonaws.com"
                value={form.endpoint}
                onChange={set("endpoint")}
              />
            </div>
            <div className="sf-field" style={{ flex: 1, minWidth: 140 }}>
              <label className="sf-label" htmlFor="region">
                {t("storage.region")}
              </label>
              <input id="region" className="sf-input" value={form.region} onChange={set("region")} />
            </div>
          </div>
          <div className="sf-field">
            <label className="sf-label" htmlFor="bucketName">
              {t("storage.bucket")}
            </label>
            <input id="bucketName" className="sf-input" value={form.bucketName} onChange={set("bucketName")} />
          </div>
          <div className="sf-row" style={{ gap: "0.6rem" }}>
            <div className="sf-field" style={{ flex: 1, minWidth: 170 }}>
              <label className="sf-label" htmlFor="accessKey">
                {t("storage.accessKey")}
              </label>
              <input id="accessKey" className="sf-input" value={form.accessKey} onChange={set("accessKey")} />
            </div>
            <div className="sf-field" style={{ flex: 1, minWidth: 170 }}>
              <label className="sf-label" htmlFor="secretKey">
                {t("storage.secretKey")}
              </label>
              <input
                id="secretKey"
                className="sf-input"
                type="password"
                autoComplete="off"
                value={form.secretKey}
                onChange={set("secretKey")}
              />
            </div>
          </div>
          <div className="sf-field">
            <label className="sf-label" htmlFor="publicBaseUrl">
              {t("storage.publicBaseUrl")} <span className="sf-muted">({t("common.optional")})</span>
            </label>
            <input id="publicBaseUrl" className="sf-input" value={form.publicBaseUrl} onChange={set("publicBaseUrl")} />
            <p className="sf-muted" style={{ fontSize: "0.82rem", marginBottom: 0 }}>
              {t("storage.publicBaseUrlHelp")}
            </p>
          </div>
        </>
      ) : (
        <>
          <div className="sf-field">
            <label className="sf-label" htmlFor="folderId">
              {t("storage.folderId")}
            </label>
            <input id="folderId" className="sf-input" value={form.folderId} onChange={set("folderId")} />
          </div>
          <div className="sf-field">
            <label className="sf-label" htmlFor="serviceAccountBase64">
              {t("storage.serviceAccount")}
            </label>
            <input
              id="serviceAccountBase64"
              className="sf-input"
              type="file"
              accept="application/json,.json"
              onChange={readServiceAccount}
            />
            {serviceAccountFile ? (
              <p className="sf-muted" style={{ fontSize: "0.82rem", margin: "0.3rem 0 0" }}>
                {serviceAccountFile}
              </p>
            ) : form.serviceAccountBase64 ? (
              <p className="sf-muted" style={{ fontSize: "0.82rem", margin: "0.3rem 0 0" }}>
                {t("storage.serviceAccountSet")}
              </p>
            ) : null}
            <p className="sf-muted" style={{ fontSize: "0.82rem", marginBottom: 0 }}>
              {t("storage.serviceAccountHelp")}
            </p>
          </div>
        </>
      )}

      <div className="sf-field">
        <label className="sf-label" htmlFor="path">
          {t("storage.path")} <span className="sf-muted">({t("common.optional")})</span>
        </label>
        <input id="path" className="sf-input" value={form.path} onChange={set("path")} />
        <p className="sf-muted" style={{ fontSize: "0.82rem", margin: "0.3rem 0 0" }}>
          {form.type === "google_drive" ? t("storage.pathHelpDrive") : t("storage.pathHelpS3")}
        </p>
      </div>

      {/* A bucket that refuses public objects used to make posting a video
          impossible: the upload granted public access as it wrote, and the
          post carried the bucket's own address. With this on, neither happens
          - StrongFish serves the file itself, to readers it has checked
          against your profile's visibility. */}
      <div className="sf-field">
        <Switch
          checked={form.private}
          onChange={(checked) => setForm((current) => ({ ...current, private: checked }))}
          label={t("storage.private")}
        />
        <p className="sf-muted" style={{ fontSize: "0.82rem", margin: "0.3rem 0 0" }}>
          {t("storage.privateHelp")}
        </p>
      </div>
      <div className="sf-row" style={{ gap: "0.4rem" }}>
        <button className="sf-button" type="submit" disabled={busy}>
          <FiSave /> {t("common.save")}
        </button>
        <button type="button" className="sf-button sf-button-secondary" onClick={onCancel} disabled={busy}>
          {t("common.cancel")}
        </button>
      </div>
    </form>
  );
}

/**
 * Lending one of your targets to other members.
 *
 * A coach pays for one bucket and their athletes upload their form videos to
 * it; an athlete opens theirs to the coach so the coach can post demonstrations
 * from it. Two roles, because those are two different requests: a *writer* may
 * upload, a *reader* may play what is already there even when your profile
 * would not otherwise let them.
 *
 * Sharing is the owner's alone - a writer who could hand out further access
 * would be widening a bucket somebody else is paying for.
 */
function StorageShares({ storageId }) {
  const { t } = useI18n();
  const [grants, setGrants] = useState(null);
  const [role, setRole] = useState("reader");
  const [userIds, setUserIds] = useState([]);
  const [members, setMembers] = useState([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    mediaApi.shares(storageId).then(setGrants).catch(setError);
    // The people this member can already see: the same list every other
    // "pick somebody" control in this app is built from.
    searchApi
      .members({ size: 100 })
      .then((page) => setMembers(page?.results || []))
      .catch(() => setMembers([]));
  }, [storageId]);

  const share = async () => {
    setBusy(true);
    setError(null);
    try {
      let latest = grants;
      // One request per person rather than a batch endpoint: granting is rare,
      // and a partial failure then names who it failed for.
      for (const userId of userIds) {
        latest = await mediaApi.share(storageId, userId, role);
      }
      setGrants(latest);
      setUserIds([]);
      toast.success(t("storage.shared"), toastOptions);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const revoke = async (userId) => {
    setBusy(true);
    try {
      setGrants(await mediaApi.unshare(storageId, userId));
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const shared = (grants || []).filter((grant) => grant.role !== "owner");
  const available = members.filter((member) => !(grants || []).some((grant) => grant.userId === member.id));

  return (
    <div className="sf-card" style={{ marginTop: "0.8rem" }}>
      <h3 style={{ marginTop: 0 }}>{t("storage.shares")}</h3>
      <p className="sf-muted" style={{ marginTop: 0 }}>{t("storage.sharesHelp")}</p>

      {shared.length === 0 ? (
        <p className="sf-muted">{t("storage.noShares")}</p>
      ) : (
        <ul className="sf-list">
          {shared.map((grant) => (
            <li className="sf-list-item" key={grant.userId}>
              <div style={{ flex: 1, minWidth: 0 }}>
                <strong>{grant.name}</strong>
                {grant.handle ? <div className="sf-muted">@{grant.handle}</div> : null}
              </div>
              <span className="sf-badge sf-badge-muted">{t(`storage.role.${grant.role}`)}</span>
              <button
                type="button"
                className="sf-button-ghost sf-button-sm"
                onClick={() => revoke(grant.userId)}
                disabled={busy}
                aria-label={t("storage.unshare")}
                title={t("storage.unshare")}
              >
                <FiTrash2 />
              </button>
            </li>
          ))}
        </ul>
      )}

      <div className="sf-row" style={{ gap: "0.5rem", alignItems: "flex-end", flexWrap: "wrap" }}>
        <div className="sf-field" style={{ flex: 2, minWidth: 220, margin: 0 }}>
          <label className="sf-label">{t("storage.shareWith")}</label>
          <MultiSelect
            options={available.map((member) => ({
              value: member.id,
              label: `${member.name} ${member.surname}`.trim() || member.handle,
            }))}
            selected={userIds}
            onChange={setUserIds}
            placeholder={t("storage.pickMembers")}
          />
        </div>
        <div className="sf-field" style={{ flex: 1, minWidth: 150, margin: 0 }}>
          <label className="sf-label">{t("storage.roleLabel")}</label>
          <Select
            options={[
              { value: "reader", label: t("storage.role.reader") },
              { value: "writer", label: t("storage.role.writer") },
            ]}
            value={role}
            onChange={setRole}
          />
        </div>
        <button
          type="button"
          className="sf-button"
          onClick={share}
          disabled={busy || userIds.length === 0}
        >
          {t("storage.shareAction")}
        </button>
      </div>

      <ErrorMessage error={error} />
    </div>
  );
}

/** TOTP and security-key enrollment. */
function MfaSettings() {
  const { t } = useI18n();
  const { refresh } = useAuth();
  const [status, setStatus] = useState(null);
  const [setup, setSetup] = useState(null);
  const [code, setCode] = useState("");
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const load = () => mfaApi.status().then(setStatus).catch(setError);

  useEffect(() => {
    load();
  }, []);

  if (!status) return <Spinner />;

  const startTotp = async () => {
    setError(null);
    try {
      setSetup(await mfaApi.totpSetup());
    } catch (err) {
      setError(err);
    }
  };

  const confirmTotp = async () => {
    setBusy(true);
    setError(null);
    try {
      await mfaApi.totpConfirm(code);
      setSetup(null);
      setCode("");
      toast.success(t("mfa.totpConfirmed"), toastOptions);
      await load();
      await refresh();
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const disableTotp = async () => {
    try {
      await mfaApi.totpDisable();
      toast.success(t("mfa.totpRemoved"), toastOptions);
      await load();
      await refresh();
    } catch (err) {
      setError(err);
    }
  };

  const addKey = async () => {
    if (!isWebAuthnSupported()) {
      setError(t("mfa.unsupported"));
      return;
    }
    setError(null);
    try {
      const { ceremonyToken, options } = await mfaApi.webauthnRegisterBegin();
      const credential = await createCredential(options);
      await mfaApi.webauthnRegisterFinish({
        ceremonyToken,
        credential,
        name: credential.transports?.includes("internal") ? "Platform authenticator" : "Security key",
      });
      toast.success(t("mfa.keyAdded"), toastOptions);
      await load();
      await refresh();
    } catch (err) {
      setError(err);
    }
  };

  const removeKey = async (credential) => {
    try {
      await mfaApi.webauthnDelete(credential.id);
      toast.success(t("mfa.keyRemoved"), toastOptions);
      await load();
      await refresh();
    } catch (err) {
      setError(err);
    }
  };

  return (
    <div className="sf-card">
      <h2>{t("mfa.title")}</h2>
      <p className="sf-subtitle">{t("mfa.subtitle")}</p>

      <div className="sf-row-between" style={{ marginTop: "1rem", paddingTop: "0.8rem", borderTop: "1px solid var(--sf-border)" }}>
        <div>
          <strong>{t("mfa.totp")}</strong>
          <div className="sf-muted">{status.totpEnabled ? t("mfa.totpEnabled") : t("mfa.totpDisabled")}</div>
        </div>
        {status.totpEnabled ? (
          <button className="sf-button sf-button-secondary sf-button-sm" onClick={disableTotp}>
            {t("mfa.disableTotp")}
          </button>
        ) : (
          <button className="sf-button sf-button-sm" onClick={startTotp}>
            {t("mfa.enableTotp")}
          </button>
        )}
      </div>

      <div className="sf-row-between" style={{ marginTop: "0.8rem", paddingTop: "0.8rem", borderTop: "1px solid var(--sf-border)" }}>
        <div>
          <strong>{t("mfa.securityKeys")}</strong>
          <div className="sf-muted">{t("mfa.securityKeysHelp")}</div>
        </div>
        <button className="sf-button sf-button-sm" onClick={addKey}>
          {t("mfa.addKey")}
        </button>
      </div>

      {(status.webauthnCredentials || []).length === 0 ? (
        <p className="sf-muted">{t("mfa.noKeys")}</p>
      ) : (
        <div className="sf-table-wrapper">
          <table className="sf-table">
            <tbody>
              {status.webauthnCredentials.map((credential) => (
                <tr key={credential.id}>
                  <td>{credential.name}</td>
                  <td className="sf-muted">{new Date(credential.createdAt).toLocaleDateString()}</td>
                  <td className="sf-table-num">
                    <button className="sf-button-ghost sf-button-sm" onClick={() => removeKey(credential)} aria-label={t("common.delete")}>
                      <FiTrash2 />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ErrorMessage error={error} />

      {setup ? (
        <Modal
          title={t("mfa.totp")}
          onClose={() => setSetup(null)}
          actions={
            <>
              <button className="sf-button sf-button-secondary" onClick={() => setSetup(null)}>
                {t("common.cancel")}
              </button>
              <button className="sf-button" onClick={confirmTotp} disabled={busy || code.length < 6}>
                {t("common.confirm")}
              </button>
            </>
          }
        >
          <p>{t("mfa.scanQr")}</p>
          <img src={setup.qrCodePng} alt="" style={{ display: "block", margin: "0 auto 1rem" }} />
          <p className="sf-muted">{t("mfa.manualSecret")}</p>
          <code style={{ display: "block", wordBreak: "break-all", marginBottom: "1rem" }}>{setup.secret}</code>
          <div className="sf-field">
            <label className="sf-label">{t("auth.mfaCode")}</label>
            <input
              className="sf-input"
              inputMode="numeric"
              maxLength={6}
              value={code}
              onChange={(event) => setCode(event.target.value.replace(/\D/g, ""))}
              autoFocus
            />
          </div>
          <ErrorMessage error={error} />
        </Modal>
      ) : null}
    </div>
  );
}
