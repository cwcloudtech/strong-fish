import { useEffect, useState } from "react";
import { Link, NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import {
  FiActivity,
  FiAward,
  FiBarChart2,
  FiLogOut,
  FiMenu,
  FiShield,
  FiUser,
  FiUsers,
} from "react-icons/fi";

import { admin } from "../../api/services";
import Avatar from "../common/Avatar";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";
import { useTheme } from "../../context/ThemeContext";

export default function DashboardLayout() {
  const { t, locale, setLocale, locales } = useI18n();
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
  if (isSuperadmin) {
    links.push({ to: "/dashboard/admin", label: t("nav.admin"), icon: <FiShield />, count: openReports });
  }

  return (
    <div className="sf-shell">
      <aside className={`sf-sidebar ${menuOpen ? "open" : ""}`}>
        <Link className="sf-sidebar-brand" to="/dashboard/feed">
          <img src="/logo192.png" alt="" />
          {t("app.name")}
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

          <select
            className="sf-sidebar-select"
            value={locale}
            onChange={(event) => setLocale(event.target.value)}
            aria-label={t("common.language")}
          >
            {locales.map((option) => (
              <option key={option.code} value={option.code}>
                {option.label}
              </option>
            ))}
          </select>

          <select
            className="sf-sidebar-select"
            value={theme}
            onChange={(event) => setTheme(event.target.value)}
            aria-label={t("common.theme")}
          >
            <option value="light">{t("common.light")}</option>
            <option value="dark">{t("common.dark")}</option>
          </select>

          <button type="button" className="sf-nav-link" onClick={signOut} style={{ background: "none", border: 0, cursor: "pointer" }}>
            <FiLogOut />
            {t("common.logout")}
          </button>
        </div>
      </aside>

      <div className="sf-content">
        <div className="sf-topbar">
          <button
            type="button"
            className="sf-button-ghost"
            style={{ color: "#fff" }}
            onClick={() => setMenuOpen((open) => !open)}
            aria-label="menu"
          >
            <FiMenu size={20} />
          </button>
          <strong>{t("app.name")}</strong>
        </div>
        <Outlet />
      </div>
    </div>
  );
}
