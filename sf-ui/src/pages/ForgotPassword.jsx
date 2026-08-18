import { useState } from "react";
import { Link } from "react-router-dom";
import { toast } from "react-toastify";

import AuthLayout from "../components/auth/AuthLayout";
import toastOptions from "../utils/toastOptions";
import { auth } from "../api/services";
import { useI18n } from "../i18n/I18nContext";

export default function ForgotPassword() {
  const { t, tError } = useI18n();
  const [email, setEmail] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (event) => {
    event.preventDefault();
    setBusy(true);
    try {
      await auth.forgotPassword(email);
      toast.success(t("auth.resetLinkSent"), toastOptions);
    } catch (err) {
      toast.error(tError(err), toastOptions);
    } finally {
      setBusy(false);
    }
  };

  return (
    <AuthLayout>
        <h1 style={{ textAlign: "center" }}>{t("auth.resetPassword")}</h1>

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
          <button className="sf-button" type="submit" style={{ width: "100%" }} disabled={busy}>
            {busy ? t("common.loading") : t("auth.sendResetLink")}
          </button>
        </form>

        <div className="sf-auth-links">
          <Link to="/login">{t("auth.login")}</Link>
        </div>
    </AuthLayout>
  );
}
