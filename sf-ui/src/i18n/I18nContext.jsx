import { createContext, useCallback, useContext, useMemo, useState } from "react";

import { detectLocale, supportedLocales, translate, translateError } from "./translate";

const I18nContext = createContext(null);

export function I18nProvider({ children }) {
  const [locale, setLocaleState] = useState(detectLocale);

  const setLocale = useCallback((next) => {
    localStorage.setItem("sf.locale", next);
    setLocaleState(next);
    document.documentElement.lang = next;
  }, []);

  const value = useMemo(
    () => ({
      locale,
      setLocale,
      locales: supportedLocales,
      t: (key, vars) => translate(locale, key, vars),
      tError: (error) => translateError(locale, error),
    }),
    [locale, setLocale]
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const context = useContext(I18nContext);
  if (!context) throw new Error("useI18n must be used inside an I18nProvider");
  return context;
}
