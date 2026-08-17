import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";

import { clearToken, getToken, setToken } from "../api/client";
import { auth } from "../api/services";
import { useI18n } from "../i18n/I18nContext";

const AuthContext = createContext(null);

/**
 * Holds the session. `user` is re-read from the server on mount rather than
 * trusted from storage: an account's role can change after its token was issued
 * (a superadmin confirms it, promotes it to coach, or bans it), and every screen
 * keys off the role.
 */
export function AuthProvider({ children }) {
  const { locale, setLocale } = useI18n();
  const [user, setUser] = useState(null);
  const [config, setConfig] = useState(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    if (!getToken()) {
      setUser(null);
      return null;
    }
    try {
      const me = await auth.me();
      setUser(me);
      // The account's stored language wins over the browser's, so a member who
      // picked French keeps it on a new device.
      if (me.locale && me.locale !== locale) setLocale(me.locale);
      return me;
    } catch {
      // A failure here means the token is unusable; the interceptor has already
      // cleared it.
      setUser(null);
      return null;
    }
  }, [locale, setLocale]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      // The deployment's config (enabled OIDC providers, activation mode) is
      // public and needed by the login screen, so it loads regardless of session.
      try {
        const loaded = await auth.config();
        if (!cancelled) setConfig(loaded);
      } catch {
        if (!cancelled) setConfig({ oidcProviders: [], activationMode: "email", plateIncrement: 2.5 });
      }
      await refresh();
      if (!cancelled) setLoading(false);
    })();
    return () => {
      cancelled = true;
    };
    // Deliberately once on mount: refresh's identity changes with the locale,
    // and re-running this on every language switch would reload the session.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const login = useCallback(
    async (response) => {
      setToken(response.token);
      const me = await auth.me();
      setUser(me);
      if (me.locale) setLocale(me.locale);
      return me;
    },
    [setLocale]
  );

  const logout = useCallback(() => {
    clearToken();
    setUser(null);
  }, []);

  const value = useMemo(
    () => ({
      user,
      setUser,
      config,
      loading,
      login,
      logout,
      refresh,
      isCoach: user?.role === "coach" || user?.role === "superadmin",
      isSuperadmin: user?.role === "superadmin",
      isActive: Boolean(user) && user.role !== "disabled" && user.role !== "ban",
    }),
    [user, config, loading, login, logout, refresh]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) throw new Error("useAuth must be used inside an AuthProvider");
  return context;
}
