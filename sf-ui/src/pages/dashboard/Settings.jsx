import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { toast } from "react-toastify";
import { FiTrash2 } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import { auth, mfa as mfaApi } from "../../api/services";
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
