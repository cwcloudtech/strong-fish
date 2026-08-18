import { useState } from "react";
import { toast } from "react-toastify";

import toastOptions from "../../utils/toastOptions";
import { useI18n } from "../../i18n/I18nContext";
import { getAssertion, isWebAuthnSupported } from "../../utils/webauthn";

/**
 * The second-factor step of a login. It's shared by the password flow and the
 * OIDC one, which both end up with the same short-lived challenge token.
 */
export default function MfaChallenge({ challenge, onVerified, onCancel, verifyTotp, beginWebauthn, finishWebauthn }) {
  const { t, tError } = useI18n();
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);

  const submitTotp = async (event) => {
    event.preventDefault();
    setBusy(true);
    try {
      await onVerified(await verifyTotp(code));
    } catch (err) {
      toast.error(tError(err), toastOptions);
    } finally {
      setBusy(false);
    }
  };

  const submitWebauthn = async () => {
    if (!isWebAuthnSupported()) {
      toast.error(t("mfa.unsupported"), toastOptions);
      return;
    }
    setBusy(true);
    try {
      const { ceremonyToken, options } = await beginWebauthn();
      const credential = await getAssertion(options);
      await onVerified(await finishWebauthn({ ceremonyToken, credential }));
    } catch (err) {
      toast.error(tError(err), toastOptions);
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <h1 style={{ textAlign: "center" }}>{t("auth.mfaTitle")}</h1>

      {challenge.hasTotp ? (
        <form onSubmit={submitTotp}>
          <div className="sf-field">
            <label className="sf-label" htmlFor="mfa-code">
              {t("auth.mfaCode")}
            </label>
            <input
              id="mfa-code"
              className="sf-input"
              inputMode="numeric"
              autoComplete="one-time-code"
              maxLength={6}
              autoFocus
              value={code}
              onChange={(event) => setCode(event.target.value.replace(/\D/g, ""))}
              required
            />
            <p className="sf-muted" style={{ marginTop: "0.3rem" }}>
              {t("auth.mfaCodeHelp")}
            </p>
          </div>
          <button className="sf-button" type="submit" style={{ width: "100%" }} disabled={busy || code.length < 6}>
            {busy ? t("common.loading") : t("auth.mfaVerify")}
          </button>
        </form>
      ) : null}

      {challenge.hasWebAuthn ? (
        <>
          {challenge.hasTotp ? <div className="sf-divider">{t("auth.orContinueWith")}</div> : null}
          <button
            type="button"
            className="sf-button sf-button-secondary"
            style={{ width: "100%" }}
            onClick={submitWebauthn}
            disabled={busy}
          >
            {t("auth.mfaUseKey")}
          </button>
        </>
      ) : null}

      <div className="sf-auth-links">
        <button type="button" className="sf-button-ghost" onClick={onCancel}>
          {t("common.back")}
        </button>
      </div>
    </>
  );
}
