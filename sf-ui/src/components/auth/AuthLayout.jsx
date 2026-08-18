import { Link } from "react-router-dom";

import Logo from "../common/Logo";
import LanguageDropdown from "../common/LanguageDropdown";
import { useI18n } from "../../i18n/I18nContext";

/**
 * The frame every signed-out screen sits in: a competition photograph filling
 * the left half, the form on the right.
 *
 * The photograph is decorative, so it carries an empty alt and the headline
 * over it is real text rather than baked into the image - it has to translate,
 * and it has to stay legible when the crop moves at other viewport ratios.
 * Below the layout's breakpoint the photo is dropped entirely (see index.css):
 * a letterboxed strip above a login form earns nothing on a phone.
 */
export default function AuthLayout({ children }) {
  const { t } = useI18n();

  return (
    <div className="sf-auth">
      <aside className="sf-auth-art">
        <img src="/auth-background.jpg" alt="" loading="eager" />
        <div className="sf-auth-art-content">
          <h2>{t("auth.heroTitle")}</h2>
          <p>{t("auth.heroSubtitle")}</p>
        </div>
        <span className="sf-auth-art-credit">{t("auth.photoCredit")}</span>
      </aside>

      <main className="sf-auth-panel">
        <div className="sf-auth-card">
          <Logo className="sf-auth-logo" />
          {children}

          <div className="sf-auth-footer">
            <Link to="/about">{t("nav.about")}</Link>
            <LanguageDropdown variant="light" align="left" />
          </div>
        </div>
      </main>
    </div>
  );
}
