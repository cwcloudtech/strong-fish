import { useEffect, useState } from "react";
import { Link, NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import {
  FiActivity,
  FiAward,
  FiBarChart2,
  FiInfo,
  FiLogOut,
  FiMenu,
  FiMoon,
  FiShield,
  FiSun,
  FiUser,
  FiUsers,
} from "react-icons/fi";

import { admin } from "../../api/services";
import Avatar from "../common/Avatar";
import Dropdown from "../common/Dropdown";
import LanguageDropdown from "../common/LanguageDropdown";
import Logo from "../common/Logo";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";
import { useTheme } from "../../context/ThemeContext";

export default function DashboardLayout() {
  const { t } = useI18n();
  const { theme, setTheme } = useTheme();
  const { user, logout, isCoach, isSuperadmin } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [menuOpen, setMenuOpen] = useState(false);
  const [openReports, setOpenReports] = useState(0);

  // Close the mobile drawer whenever the route changes, so tapping a link
  // doesn't leave it covering the page it just opened.
  useEffect(() => setMenuOpen(false), [location.pathname]);

  useEffect(() => {
    if (!isSuperadmin) return;
    admin
      .stats()
      .then((stats) => setOpenReports(stats.openReports || 0))
      .catch(() => setOpenReports(0));
  }, [isSuperadmin, location.pathname]);

  const signOut = () => {
    logout();
    navigate("/login");
  };

  const links = [
    { to: "/dashboard/feed", label: t("nav.feed"), icon: <FiActivity /> },
    { to: "/dashboard/training", label: t("nav.training"), icon: <FiBarChart2 /> },
    { to: "/dashboard/one-rms", label: t("nav.oneRms"), icon: <FiAward /> },
    { to: "/dashboard/clubs", label: t("nav.clubs"), icon: <FiUsers /> },
  ];
  if (isCoach) links.push({ to: "/dashboard/exercises", label: t("nav.exercises"), icon: <FiActivity /> });
  links.push({ to: "/dashboard/settings", label: t("nav.settings"), icon: <FiUser /> });
  links.push({ to: "/about", label: t("nav.about"), icon: <FiInfo /> });
  if (isSuperadmin) {
    links.push({ to: "/dashboard/admin", label: t("nav.admin"), icon: <FiShield />, count: openReports });
  }

  return (
    <div className="sf-shell">
      <aside className={`sf-sidebar ${menuOpen ? "open" : ""}`}>
        {/* The mark alone: the logo carries the name, so repeating it as text
            beside it only crowds the rail. The sidebar is navy in both themes,
            so the mark is pinned to the light-inked variant rather than
            following the app's theme. */}
        <Link className="sf-sidebar-brand" to="/dashboard/feed" aria-label={t("app.name")}>
          <Logo mark on="dark" alt={t("app.name")} />
        </Link>

        <nav>
          {links.map((link) => (
            <NavLink key={link.to} to={link.to} className="sf-nav-link">
              {link.icon}
              {link.label}
              {link.count ? <span className="sf-nav-count">{link.count}</span> : null}
            </NavLink>
          ))}
        </nav>

        <div className="sf-sidebar-footer">
          {user?.handle ? (
            <Link className="sf-nav-link" to={`/profile/${user.handle}`}>
              <Avatar user={user} size="sf-avatar-sm" />
              {user.name}
            </Link>
          ) : null}

          <div className="sf-sidebar-actions">
            <Dropdown
              icon={theme === "dark" ? <FiMoon /> : <FiSun />}
              value={theme}
              onChange={setTheme}
              variant="dark"
              align="left"
              ariaLabel={t("common.theme")}
              options={[
                { value: "light", code: <FiSun />, label: t("common.light") },
                { value: "dark", code: <FiMoon />, label: t("common.dark") },
              ]}
            />
            <LanguageDropdown variant="dark" />
          </div>

          <button
            type="button"
            className="sf-nav-link"
            onClick={signOut}
            style={{ background: "none", border: 0, cursor: "pointer", width: "100%" }}
          >
            <FiLogOut />
            {t("common.logout")}
          </button>
        </div>
      </aside>

      {/* Tapping outside the drawer closes it, which is what a drawer is
          expected to do on a phone. */}
      <div className={`sf-scrim ${menuOpen ? "open" : ""}`} onClick={() => setMenuOpen(false)} />

      <div className="sf-content">
        <div className="sf-topbar">
          <button
            type="button"
            className="sf-icon-button"
            style={{ flex: "0 0 auto" }}
            onClick={() => setMenuOpen((open) => !open)}
            aria-label="menu"
          >
            <FiMenu size={18} />
          </button>
          <strong>{t("app.name")}</strong>
        </div>
        <Outlet />
      </div>
    </div>
  );
}
