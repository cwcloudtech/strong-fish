import { useEffect, useState } from "react";
import { Link, NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import {
  FiActivity,
  FiAward,
  FiBarChart2,
  FiChevronLeft,
  FiChevronRight,
  FiCalendar,
  FiInfo,
  FiKey,
  FiMail as FiMailIcon,
  FiMessageSquare,
  FiSearch,
  FiLogOut,
  FiMail,
  FiMenu,
  FiMoon,
  FiShield,
  FiSun,
  FiUser,
  FiUsers,
} from "react-icons/fi";

import { admin, invitations as invitationsApi, messages as messagesApi } from "../../api/services";
import Avatar from "../common/Avatar";
import DownloadAppButton from "../common/DownloadApp";
import Dropdown from "../common/Dropdown";
import LanguageDropdown from "../common/LanguageDropdown";
import Logo from "../common/Logo";
import Tooltip from "../common/Tooltip";
import useAppVersion from "../../utils/useAppVersion";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";
import { useTheme } from "../../context/ThemeContext";

const COLLAPSED_KEY = "sf.sidebarCollapsed";

export default function DashboardLayout() {
  const { t } = useI18n();
  const { theme, setTheme } = useTheme();
  const { user, config, logout, isCoach, isSuperadmin } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [menuOpen, setMenuOpen] = useState(false);
  // Collapsing the rail is a lasting preference, not a per-visit one: someone
  // who wants the room back wants it on every screen and every session.
  const [collapsed, setCollapsed] = useState(() => localStorage.getItem(COLLAPSED_KEY) === "1");
  const [openReports, setOpenReports] = useState(0);
  const [pendingInvitations, setPendingInvitations] = useState(0);
  const [unreadMessages, setUnreadMessages] = useState(0);
  const version = useAppVersion(config?.version);

  useEffect(() => localStorage.setItem(COLLAPSED_KEY, collapsed ? "1" : "0"), [collapsed]);

  // Close the mobile drawer whenever the route changes, so tapping a link
  // doesn't leave it covering the page it just opened.
  useEffect(() => setMenuOpen(false), [location.pathname]);

  useEffect(() => {
    if (!isSuperadmin) return;
    admin
      .stats()
      // Reports and coach requests are both "somebody is waiting on you", so
      // one badge counts them together rather than two competing for the eye.
      .then((stats) => setOpenReports((stats.openReports || 0) + (stats.coachRequests || 0)))
      .catch(() => setOpenReports(0));
  }, [isSuperadmin, location.pathname]);

  useEffect(() => {
    invitationsApi
      .mine()
      .then((list) => setPendingInvitations(list.length))
      .catch(() => setPendingInvitations(0));
    messagesApi
      .unread()
      .then(({ unread }) => setUnreadMessages(unread || 0))
      .catch(() => setUnreadMessages(0));
  }, [location.pathname]);

  const signOut = () => {
    logout();
    navigate("/login");
  };

  const links = [
    { to: "/dashboard/feed", label: t("nav.feed"), icon: <FiActivity /> },
    { to: "/dashboard/training", label: t("nav.training"), icon: <FiBarChart2 /> },
    { to: "/dashboard/one-rms", label: t("nav.oneRms"), icon: <FiAward /> },
    { to: "/dashboard/clubs", label: t("nav.clubs"), icon: <FiUsers /> },
    { to: "/dashboard/events", label: t("nav.events"), icon: <FiCalendar /> },
    {
      to: "/dashboard/messages",
      label: t("nav.messages"),
      icon: <FiMessageSquare />,
      count: unreadMessages,
    },
    { to: "/dashboard/search", label: t("nav.search"), icon: <FiSearch /> },
    {
      to: "/dashboard/invitations",
      label: t("nav.invitations"),
      icon: <FiMailIcon />,
      count: pendingInvitations,
    },
  ];
  if (isCoach) links.push({ to: "/dashboard/exercises", label: t("nav.exercises"), icon: <FiActivity /> });
  links.push({ to: "/dashboard/settings", label: t("nav.settings"), icon: <FiUser /> });
  links.push({ to: "/dashboard/api-keys", label: t("nav.apiKeys"), icon: <FiKey /> });
  links.push({ to: "/about", label: t("nav.about"), icon: <FiInfo /> });
  if (config?.contactEnabled) {
    links.push({ to: "/contact", label: t("nav.contact"), icon: <FiMail /> });
  }
  if (isSuperadmin) {
    links.push({ to: "/dashboard/admin", label: t("nav.admin"), icon: <FiShield />, count: openReports });
  }

  return (
    <div className="sf-shell">
      <aside className={`sf-sidebar ${menuOpen ? "open" : ""} ${collapsed ? "collapsed" : ""}`}>
        {/* The mark alone: the logo carries the name, so repeating it as text
            beside it only crowds the rail. The rail's ground is the brand navy
            in light mode and the app's own background in dark, so the mark is
            pinned to the light-inked variant in both rather than following the
            theme. */}
        <Link className="sf-sidebar-brand" to="/dashboard/feed" aria-label={t("app.name")}>
          <Logo mark on="dark" alt={t("app.name")} />
        </Link>

        {/* Directly under the mark: the version belongs to the product, not to
            the account, so it sits with the branding rather than down with the
            sign-out controls. */}
        {version ? (
          <div className="sf-version-wrap">
            <span className="sf-version-badge">v{version}</span>
          </div>
        ) : null}

        {/* Immediately above the first destination, so the control that changes
            the nav's shape sits with the nav it changes. */}
        <Tooltip label={collapsed ? t("nav.expandSidebar") : t("nav.collapseSidebar")} position="right">
          <button
            type="button"
            className="sf-sidebar-toggle"
            onClick={() => setCollapsed((current) => !current)}
            aria-label={collapsed ? t("nav.expandSidebar") : t("nav.collapseSidebar")}
            aria-expanded={!collapsed}
          >
            {collapsed ? <FiChevronRight /> : <FiChevronLeft />}
          </button>
        </Tooltip>

        <nav>
          {links.map((link) => (
            /* Collapsed, the icon is all that's left of a link, so the label
               has to come back as a tooltip. Expanded, it's already on screen
               and a bubble repeating it would just be noise. */
            <Tooltip key={link.to} label={collapsed ? link.label : null} position="right">
              <NavLink to={link.to} className="sf-nav-link">
                {link.icon}
                <span className="sf-nav-label">{link.label}</span>
                {link.count ? <span className="sf-nav-count">{link.count}</span> : null}
              </NavLink>
            </Tooltip>
          ))}
          <DownloadAppButton collapsed={collapsed} />
        </nav>

        <div className="sf-sidebar-footer">
          {user?.handle ? (
            <Tooltip label={collapsed ? user.name : null} position="right">
              <Link className="sf-nav-link" to={`/profile/${user.handle}`}>
                <Avatar user={user} size="sf-avatar-sm" />
                <span className="sf-nav-label">{user.name}</span>
              </Link>
            </Tooltip>
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

          <Tooltip label={collapsed ? t("common.logout") : null} position="right">
            <button
              type="button"
              className="sf-nav-link"
              onClick={signOut}
              style={{ background: "none", border: 0, cursor: "pointer", width: "100%" }}
            >
              <FiLogOut />
              <span className="sf-nav-label">{t("common.logout")}</span>
            </button>
          </Tooltip>
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
