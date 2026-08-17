import { useState } from "react";
import { Link } from "react-router-dom";

import { auth } from "../api/services";
import { ErrorMessage } from "../components/common/Feedback";
import { useI18n } from "../i18n/I18nContext";

export default function ForgotPassword() {
  const { t } = useI18n();
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const submit = async (event) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await auth.forgotPassword(email);
      setSent(true);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="sf-auth">
      <div className="sf-auth-card">
        <img className="sf-auth-logo" src="/logo512.png" alt={t("app.name")} />
        <h1 style={{ textAlign: "center" }}>{t("auth.resetPassword")}</h1>

        {sent ? (
          <div className="sf-notice">{t("auth.resetLinkSent")}</div>
        ) : (
          <form onSubmit={submit}>
            <div className="sf-field">
              <label className="sf-label" htmlFor="email">
                {t("auth.email")}
              </label>
              <input
                id="email"
                className="sf-input"
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                required
              />
            </div>
            <ErrorMessage error={error} />
            <button className="sf-button" type="submit" style={{ width: "100%" }} disabled={busy}>
              {busy ? t("common.loading") : t("auth.sendResetLink")}
            </button>
          </form>
        )}

        <div className="sf-auth-links">
          <Link to="/login">{t("auth.login")}</Link>
        </div>
      </div>
    </div>
  );
}
