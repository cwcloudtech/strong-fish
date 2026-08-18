import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";

const STORAGE_KEY = "sf.theme";
const ThemeContext = createContext(null);

const getSystemTheme = () =>
  window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";

/**
 * Applies the OS colour scheme by default; once the user toggles the switch,
 * that explicit choice is persisted and wins over the OS setting in both
 * directions (light OS + dark choice, or the reverse) via the [data-theme]
 * attribute on <html> - see index.css. Ported from cwclock's ThemeContext, so
 * both apps behave identically.
 */
export function ThemeProvider({ children }) {
  const [explicitTheme, setExplicitTheme] = useState(() => localStorage.getItem(STORAGE_KEY));
  const [systemTheme, setSystemTheme] = useState(getSystemTheme);

  useEffect(() => {
    const query = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = (event) => setSystemTheme(event.matches ? "dark" : "light");
    query.addEventListener("change", onChange);
    return () => query.removeEventListener("change", onChange);
  }, []);

  useEffect(() => {
    // No attribute while the user hasn't chosen: the media query in index.css
    // is then what decides, so the app follows the OS live.
    if (explicitTheme) {
      document.documentElement.setAttribute("data-theme", explicitTheme);
    } else {
      document.documentElement.removeAttribute("data-theme");
    }
  }, [explicitTheme]);

  const theme = explicitTheme || systemTheme;

  // setTheme picks a theme outright (the sidebar dropdown lists both), while
  // toggleTheme flips the current one - the same persisted explicit choice
  // either way.
  const setTheme = useCallback((next) => {
    localStorage.setItem(STORAGE_KEY, next);
    setExplicitTheme(next);
  }, []);

  const toggleTheme = useCallback(() => {
    setExplicitTheme((current) => {
      const next = (current || getSystemTheme()) === "dark" ? "light" : "dark";
      localStorage.setItem(STORAGE_KEY, next);
      return next;
    });
  }, []);

  const value = useMemo(() => ({ theme, setTheme, toggleTheme }), [theme, setTheme, toggleTheme]);

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  const context = useContext(ThemeContext);
  if (!context) throw new Error("useTheme must be used inside a ThemeProvider");
  return context;
}
