import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";

import { detectLocale, isRtlLocale, supportedLocales, translate, translateError } from "./translate";

/** Puts the document in the picked language, and the right way round. */
function applyLocale(locale) {
  document.documentElement.lang = locale;
  document.documentElement.dir = isRtlLocale(locale) ? "rtl" : "ltr";
}

const I18nContext = createContext(null);

export function I18nProvider({ children }) {
  const [locale, setLocaleState] = useState(detectLocale);

  const setLocale = useCallback((next) => {
    localStorage.setItem("sf.locale", next);
    setLocaleState(next);
    applyLocale(next);
  }, []);

  // Also on the first render: the locale can come from storage or from the
  // browser, and the document would otherwise stay left-to-right until
  // somebody picked a language by hand.
  useEffect(() => applyLocale(locale), [locale]);

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
