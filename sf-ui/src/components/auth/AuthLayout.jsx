import { Link } from "react-router-dom";

import { FiCalendar, FiMenu, FiSearch, FiTrendingUp } from "react-icons/fi";

import Logo from "../common/Logo";
import MenuDropdown, { MenuItem } from "../common/MenuDropdown";
import LanguageDropdown from "../common/LanguageDropdown";
import { DownloadAppIcon } from "../common/DownloadApp";
import { aboutUrl } from "../../utils/aboutUrl";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";

// Where the sources live. Set at build time so a fork points at its own
// repository rather than at this one.
const REPO_URL = process.env.REACT_APP_GIT_REPO_URL || "https://gitlab.cwcloud.tech/oss/strong-fish";

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
  const { config } = useAuth();

  return (
    <div className="sf-auth">
      <aside className="sf-auth-art">
        <img src="/auth-background.jpg" alt="" loading="eager" />
        <div className="sf-auth-art-content">
          <h2>{t("auth.heroTitle")}</h2>
          <p>{t("auth.heroSubtitle")}</p>
        </div>
        <div className="sf-auth-art-footer">
        {/* The photograph is CC BY-SA, which requires naming the author and
            linking the licence - so the credit is a real attribution with
            reachable links, not a line of grey text. */}
        <p className="sf-auth-art-credit">
          {t("auth.photoCreditBy")}{" "}
          <a
            href="https://commons.wikimedia.org/wiki/File:Alessio_Pavone_deadlift_European_Championship2022.jpg"
            target="_blank"
            rel="noopener noreferrer"
          >
            FactNoter
          </a>{" "}
          &middot;{" "}
          <a href="https://creativecommons.org/licenses/by-sa/4.0/" target="_blank" rel="noopener noreferrer">
            CC BY-SA 4.0
          </a>
        </p>

        {/* Under the photo credit, and in the same register: who made this and
            where to read it. */}
        <p className="sf-auth-art-credit sf-auth-art-oss">
          {t("about.openSource")}{" "}
          <a href={REPO_URL} target="_blank" rel="noopener noreferrer">
            {t("about.openSourceLink")}
          </a>
        </p>
        </div>
      </aside>

      <main className="sf-auth-panel">
        <div className="sf-auth-card">
          <Logo className="sf-auth-logo" />
          {children}

          <div className="sf-auth-footer">
            <a href={aboutUrl(config)} target="_blank" rel="noopener noreferrer">
              {t("nav.about")}
            </a>
            {/* Hidden when no CWCLOUD_CONTACT_FORM_ID is set: the page would
                only be able to report that it isn't configured. */}
            {config?.contactEnabled ? <Link to="/contact">{t("nav.contact")}</Link> : null}
            {/* The pages worth opening before signing up - an open calendar, a
                calculator that answers the question people arrive with, and
                the members who chose to be visible to everyone - collapsed
                into one control, the way uprodit's own overflow menu does it.
                Spelled out, five links crowded the two that matter on a page
                whose job is a login form. */}
            <MenuDropdown
              trigger={<FiMenu />}
              align="start"
              triggerClassName="sf-auth-menu-trigger"
              ariaLabel={t("nav.more")}
            >
              {(close) => (
                <>
                  <MenuItem as={Link} to="/calendar" onClick={close}>
                    <FiCalendar /> {t("nav.events")}
                  </MenuItem>
                  <MenuItem as={Link} to="/strength" onClick={close}>
                    <FiTrendingUp /> {t("nav.strength")}
                  </MenuItem>
                  <MenuItem as={Link} to="/find" onClick={close}>
                    <FiSearch /> {t("nav.find")}
                  </MenuItem>
                </>
              )}
            </MenuDropdown>
            <DownloadAppIcon />
            <LanguageDropdown variant="light" align="left" />
          </div>
        </div>
      </main>
    </div>
  );
}
