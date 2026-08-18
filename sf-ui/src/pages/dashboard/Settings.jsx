import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { toast } from "react-toastify";
import { FiTrash2 } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import { auth, media as mediaApi, mfa as mfaApi } from "../../api/services";
import Avatar from "../../components/common/Avatar";
import Modal from "../../components/common/Modal";
import { ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";
import { readImageAsDataUrl } from "../../utils/image";
import { createCredential, isWebAuthnSupported } from "../../utils/webauthn";

/** The connected user's own settings: profile, avatar, and MFA enrollment. */
export default function Settings() {
  const { t, locale, setLocale } = useI18n();
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
      handle: user.handle || "",
      bio: user.bio || "",
      bodyweight: user.bodyweight || "",
      publicProfile: Boolean(user.publicProfile),
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

  const save = async (event) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const updated = await auth.updateProfile({
        ...form,
        bodyweight: form.bodyweight === "" ? 0 : Number(form.bodyweight),
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
          <label className="sf-label">{t("profile.handle")}</label>
          <input className="sf-input" value={form.handle} onChange={set("handle")} />
          <p className="sf-muted" style={{ marginTop: "0.25rem" }}>
            {t("profile.handleHelp", { handle: form.handle || "..." })}
          </p>
        </div>

        <div className="sf-field">
          <label className="sf-label">{t("profile.bio")}</label>
          <textarea className="sf-textarea" value={form.bio} onChange={set("bio")} />
        </div>

        <div className="sf-row" style={{ gap: "0.6rem" }}>
          <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
            <label className="sf-label">
              {t("profile.bodyweight")} ({t("common.kg")})
            </label>
            <input className="sf-input" type="number" min="0" step="0.1" value={form.bodyweight} onChange={set("bodyweight")} />
          </div>
          <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
            <label className="sf-label">{t("common.language")}</label>
            <select className="sf-select" value={locale} onChange={(event) => setLocale(event.target.value)}>
              <option value="en">English</option>
              <option value="fr">Français</option>
            </select>
          </div>
        </div>

        <label className="sf-checkbox" style={{ marginBottom: "1rem" }}>
          <input type="checkbox" checked={form.publicProfile} onChange={set("publicProfile")} />
          <span>
            {t("profile.public")}
            <div className="sf-muted">{t("profile.publicHelp")}</div>
          </span>
        </label>

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
 * Where this member's uploaded videos go.
 *
 * strong-fish hosts no video of its own, so posting one means bringing a
 * bucket - S3-compatible or a Google Drive folder. The credentials are
 * write-only: the API sends back a marker rather than the key, and echoing
 * that marker back on save is what lets somebody change their bucket name
 * without retyping a secret they can no longer read.
 */
function StorageSettings() {
  const { t } = useI18n();
  const [state, setState] = useState(null);
  const [form, setForm] = useState(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      const result = await mediaApi.storage();
      setState(result);
      setForm({
        type: result.connection?.type || "s3",
        endpoint: result.connection?.endpoint || "",
        bucketName: result.connection?.bucketName || "",
        region: result.connection?.region || "",
        accessKey: result.connection?.accessKey || "",
        secretKey: result.connection?.secretKey || "",
        serviceAccountBase64: result.connection?.serviceAccountBase64 || "",
        folderId: result.connection?.folderId || "",
        path: result.connection?.path || "",
        publicBaseUrl: result.connection?.publicBaseUrl || "",
      });
    } catch {
      setState({ configured: false });
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  if (!state || !form) return null;

  const set = (field) => (event) => setForm((current) => ({ ...current, [field]: event.target.value }));

  const save = async (event) => {
    event.preventDefault();
    setBusy(true);
    try {
      setState(await mediaApi.setStorage(form));
      toast.success(t("storage.saved"), toastOptions);
      await load();
    } catch (err) {
      toast.error(err?.response?.data?.message || t("errors.generic"), toastOptions);
    } finally {
      setBusy(false);
    }
  };

  const clear = async () => {
    setBusy(true);
    try {
      setState(await mediaApi.clearStorage());
      toast.success(t("storage.cleared"), toastOptions);
      await load();
    } catch {
      toast.error(t("errors.generic"), toastOptions);
    } finally {
      setBusy(false);
    }
  };

  const megabytes = state.maxSize ? Math.round(state.maxSize / (1024 * 1024)) : 20;

  return (
    <form className="sf-card" onSubmit={save}>
      <h2>{t("storage.title")}</h2>
      <p className="sf-subtitle">{t("storage.subtitle", { size: megabytes })}</p>

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
            <textarea
              id="serviceAccountBase64"
              className="sf-textarea"
              rows={3}
              autoComplete="off"
              value={form.serviceAccountBase64}
              onChange={set("serviceAccountBase64")}
            />
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
      </div>

      <div className="sf-row" style={{ gap: "0.4rem" }}>
        <button className="sf-button" type="submit" disabled={busy}>
          {t("common.save")}
        </button>
        {state.configured ? (
          <button type="button" className="sf-button sf-button-secondary" onClick={clear} disabled={busy}>
            {t("storage.clear")}
          </button>
        ) : null}
      </div>
    </form>
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
