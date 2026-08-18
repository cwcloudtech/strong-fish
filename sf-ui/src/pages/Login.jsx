import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { toast } from "react-toastify";

import AuthLayout from "../components/auth/AuthLayout";
import toastOptions from "../utils/toastOptions";
import { auth, mfa } from "../api/services";
import MfaChallenge from "../components/auth/MfaChallenge";
import OidcButtons from "../components/auth/OidcButtons";
import { useAuth } from "../context/AuthContext";
import { useI18n } from "../i18n/I18nContext";

export default function Login() {
  const { t, tError } = useI18n();
  const { login } = useAuth();
  const navigate = useNavigate();
  const [params] = useSearchParams();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  // Set when the password was right but the account has a second factor: the
  // form is replaced by the challenge rather than navigating, so a failed code
  // doesn't lose the session in progress.
  const [challenge, setChallenge] = useState(null);

  useEffect(() => {
    if (params.get("confirmed") === "1") toast.success(t("auth.confirmed"), toastOptions);
    if (params.get("confirmed") === "0") {
      toast.error(params.get("reason") === "banned" ? t("auth.confirmBanned") : t("auth.confirmFailed"), toastOptions);
    }
    // Only on the first render: re-running would re-toast on every keystroke.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const submit = async (event) => {
    event.preventDefault();
    setBusy(true);
    try {
      const response = await auth.login(email, password);
      if (response.mfaRequired) {
        setChallenge(response);
        return;
      }
      await login(response);
      toast.success(t("auth.welcomeBack"), toastOptions);
      navigate("/dashboard/feed");
    } catch (err) {
      toast.error(tError(err), toastOptions);
    } finally {
      setBusy(false);
    }
  };

  const finishMfa = async (response) => {
    await login(response);
    navigate("/dashboard/feed");
  };

  return (
    <AuthLayout>

        {challenge ? (
          <MfaChallenge
            challenge={challenge}
            onVerified={finishMfa}
            onCancel={() => setChallenge(null)}
            verifyTotp={(code) => mfa.loginTotp(challenge.challengeToken, code)}
            beginWebauthn={() => mfa.loginWebauthnBegin(challenge.challengeToken)}
            finishWebauthn={(payload) => mfa.loginWebauthnFinish(payload)}
          />
        ) : (
          <>
            <h1 style={{ textAlign: "center" }}>{t("auth.login")}</h1>
            <form onSubmit={submit}>
              <div className="sf-field">
                <label className="sf-label" htmlFor="email">
                  {t("auth.email")}
                </label>
                <input
                  id="email"
                  className="sf-input"
                  type="email"
                  autoComplete="email"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  required
                />
              </div>
              <div className="sf-field">
                <label className="sf-label" htmlFor="password">
                  {t("auth.password")}
                </label>
                <input
                  id="password"
                  className="sf-input"
                  type="password"
                  autoComplete="current-password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  required
                />
              </div>
              <button className="sf-button" type="submit" style={{ width: "100%" }} disabled={busy}>
                {busy ? t("common.loading") : t("auth.login")}
              </button>
            </form>

            <OidcButtons />

            <div className="sf-auth-links">
              <Link to="/forgot-password">{t("auth.forgotPassword")}</Link>
              <Link to="/signup">{t("auth.noAccount")}</Link>
            </div>
          </>
        )}
    </AuthLayout>
  );
}
