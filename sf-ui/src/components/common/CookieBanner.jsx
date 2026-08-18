import { useState } from "react";

import { useI18n } from "../../i18n/I18nContext";

// strong-fish only ever stores functional data in localStorage (the session
// token, the theme and the locale) - there are no third-party tracking or
// analytics cookies to consent to, so this is an informational, one-time
// notice rather than an accept/reject consent flow. Ported from cwclock's
// CookieBanner, with the same wording.
const STORAGE_KEY = "sf.cookie_notice_ack";

export default function CookieBanner() {
  const { t } = useI18n();
  const [dismissed, setDismissed] = useState(() => localStorage.getItem(STORAGE_KEY) === "1");

  if (dismissed) return null;

  const acknowledge = () => {
    localStorage.setItem(STORAGE_KEY, "1");
    setDismissed(true);
  };

  return (
    <div className="sf-cookie-banner" role="region" aria-label={t("cookieBanner.title")}>
      <p>{t("cookieBanner.message")}</p>
      <button type="button" className="sf-button sf-button-sm" onClick={acknowledge}>
        {t("cookieBanner.understand")}
      </button>
    </div>
  );
}
