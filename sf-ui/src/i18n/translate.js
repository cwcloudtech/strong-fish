import ar from "./translations/ar";
import en from "./translations/en";
import fr from "./translations/fr";

export const dictionaries = { en, fr, ar };

export const supportedLocales = [
  { code: "en", label: "English" },
  { code: "fr", label: "Français" },
  { code: "ar", label: "العربية" },
];

/**
 * Locales written right to left.
 *
 * The document's `dir` follows this (see I18nContext): Arabic text renders
 * right-to-left on its own by Unicode's rules, but the *layout* does not - the
 * sidebar, the table columns and the form labels stay where they were unless
 * the direction is set. The phone app makes the same flip through a
 * Directionality wrapper.
 */
export const RTL_LOCALES = new Set(["ar"]);

export function isRtlLocale(locale) {
  return RTL_LOCALES.has(locale);
}

export const DEFAULT_LOCALE = "en";

/** Reads a dotted key ("errors.notFound") out of a nested dictionary. */
function resolve(dictionary, key) {
  return key.split(".").reduce((acc, part) => (acc && acc[part] !== undefined ? acc[part] : undefined), dictionary);
}

/**
 * Translates key into locale, interpolating {{name}} placeholders. Falls back
 * to English, then to the key itself - a missing translation shows the key
 * rather than an empty string, which makes it obvious in review.
 */
export function translate(locale, key, vars) {
  const value = resolve(dictionaries[locale], key) ?? resolve(dictionaries[DEFAULT_LOCALE], key) ?? key;
  if (!vars) return String(value);
  return Object.entries(vars).reduce(
    (acc, [name, replacement]) => acc.replaceAll(`{{${name}}}`, String(replacement)),
    String(value)
  );
}

/**
 * Turns an API failure into a translated message. The backend sends an
 * `i18n_code` the frontend looks up here, falling back to the server's English
 * message when the code is unknown (an older client, or a newer server).
 */
export function translateError(locale, error) {
  const body = error?.response?.data;
  if (body?.i18n_code) {
    const translated = resolve(dictionaries[locale], body.i18n_code) ?? resolve(dictionaries[DEFAULT_LOCALE], body.i18n_code);
    if (translated) return translated;
  }
  if (body?.message) return body.message;
  if (error?.response) return translate(locale, "errors.internal");
  return translate(locale, "errors.network");
}

/** Picks the initial locale from what the browser advertises. */
export function detectLocale() {
  const stored = localStorage.getItem("sf.locale");
  if (stored && dictionaries[stored]) return stored;
  const browser = (navigator.language || DEFAULT_LOCALE).slice(0, 2);
  return dictionaries[browser] ? browser : DEFAULT_LOCALE;
}
