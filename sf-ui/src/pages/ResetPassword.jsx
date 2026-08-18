import { useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { toast } from "react-toastify";

import AuthLayout from "../components/auth/AuthLayout";
import toastOptions from "../utils/toastOptions";
import { auth } from "../api/services";
import { useI18n } from "../i18n/I18nContext";

export default function ResetPassword() {
  const { t, tError } = useI18n();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const token = params.get("token") || "";

  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (event) => {
    event.preventDefault();
    if (password !== confirmPassword) {
      toast.error(t("errors.passwordsMismatch"), toastOptions);
      return;
    }
    setBusy(true);
    try {
      await auth.resetPassword({ token, password, confirmPassword });
      toast.success(t("auth.passwordUpdated"), toastOptions);
      navigate("/login");
    } catch (err) {
      toast.error(tError(err), toastOptions);
    } finally {
      setBusy(false);
    }
  };

  return (
    <AuthLayout>
        <h1 style={{ textAlign: "center" }}>{t("auth.resetPassword")}</h1>

        {token ? (
          <form onSubmit={submit}>
            <div className="sf-field">
              <label className="sf-label" htmlFor="password">
                {t("auth.newPassword")}
              </label>
              <input
                id="password"
                className="sf-input"
                type="password"
                autoComplete="new-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                required
              />
            </div>
            <div className="sf-field">
              <label className="sf-label" htmlFor="confirmPassword">
                {t("auth.confirmPassword")}
              </label>
              <input
                id="confirmPassword"
                className="sf-input"
                type="password"
                autoComplete="new-password"
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                required
              />
            </div>
            <button className="sf-button" type="submit" style={{ width: "100%" }} disabled={busy}>
              {busy ? t("common.loading") : t("common.save")}
            </button>
          </form>
        ) : (
          <div className="sf-notice sf-notice-danger">{t("errors.invalidToken")}</div>
        )}

        <div className="sf-auth-links">
          <Link to="/login">{t("auth.login")}</Link>
        </div>
    </AuthLayout>
  );
}
