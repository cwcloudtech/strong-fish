import { Navigate, useLocation } from "react-router-dom";

import Logo from "../../components/common/Logo";
import { Spinner } from "../common/Feedback";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";

/**
 * Gates the dashboard. A disabled or banned account is shown why rather than
 * bounced to the login screen it just came from - it has a valid session, it
 * just isn't allowed to use it yet.
 */
export default function RequireAuth({ children, superadmin = false }) {
  const { t } = useI18n();
  const { user, loading, isSuperadmin, isActive, logout } = useAuth();
  const location = useLocation();

  if (loading) return <Spinner />;
  if (!user) return <Navigate to="/login" replace state={{ from: location.pathname }} />;

  if (!isActive) {
    return (
      <div className="sf-auth">
        <div className="sf-auth-card">
          <Logo className="sf-auth-logo" />
          <div className="sf-notice sf-notice-warning">
            {t(user.i18nCode || "errors.accountDisabledAdmin")}
          </div>
          <button className="sf-button" style={{ width: "100%" }} onClick={logout}>
            {t("common.logout")}
          </button>
        </div>
      </div>
    );
  }

  if (superadmin && !isSuperadmin) return <Navigate to="/dashboard/feed" replace />;

  return children;
}
