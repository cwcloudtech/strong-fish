import { useEffect, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import AuthLayout from "../components/auth/AuthLayout";
import { mfa } from "../api/services";
import MfaChallenge from "../components/auth/MfaChallenge";
import { ErrorMessage, Spinner } from "../components/common/Feedback";
import { useAuth } from "../context/AuthContext";
import { useI18n } from "../i18n/I18nContext";

/**
 * Where the API sends the browser back after an OIDC login. It arrives with
 * either a session token, an MFA challenge (the provider proved the first factor
 * only), or an error code.
 */
export default function OidcCallback() {
  const { t } = useI18n();
  const { login } = useAuth();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const [error, setError] = useState(null);
  const [challenge, setChallenge] = useState(null);
  // React 18's StrictMode mounts effects twice in development; the token is
  // single-use on the server side, so guard against consuming it twice.
  const handled = useRef(false);

  useEffect(() => {
    if (handled.current) return;
    handled.current = true;

    const errorCode = params.get("error");
    if (errorCode) {
      setError(t("errors.invalidCredentials"));
      return;
    }

    const mfaChallenge = params.get("mfaChallenge");
    if (mfaChallenge) {
      setChallenge({
        challengeToken: mfaChallenge,
        hasTotp: params.get("hasTotp") === "1",
        hasWebAuthn: params.get("hasWebAuthn") === "1",
      });
      return;
    }

    const token = params.get("token");
    if (!token) {
      setError(t("errors.invalidToken"));
      return;
    }

    login({ token })
      .then(() => navigate("/dashboard/feed"))
      .catch(() => setError(t("errors.internal")));
  }, [params, login, navigate, t]);

  return (
    <AuthLayout>

        {challenge ? (
          <MfaChallenge
            challenge={challenge}
            onVerified={async (response) => {
              await login(response);
              navigate("/dashboard/feed");
            }}
            onCancel={() => navigate("/login")}
            verifyTotp={(code) => mfa.loginTotp(challenge.challengeToken, code)}
            beginWebauthn={() => mfa.loginWebauthnBegin(challenge.challengeToken)}
            finishWebauthn={(payload) => mfa.loginWebauthnFinish(payload)}
          />
        ) : error ? (
          <>
            <ErrorMessage error={error} />
            <button className="sf-button" style={{ width: "100%" }} onClick={() => navigate("/login")}>
              {t("auth.login")}
            </button>
          </>
        ) : (
          <Spinner />
        )}
    </AuthLayout>
  );
}
