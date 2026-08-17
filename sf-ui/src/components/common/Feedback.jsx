import { useI18n } from "../../i18n/I18nContext";

/** The loading indicator used while a screen's first fetch is in flight. */
export function Spinner() {
  return <div className="sf-spinner" aria-label="loading" />;
}

/** The "nothing here" state, with an optional call to action. */
export function EmptyState({ title, message, children }) {
  return (
    <div className="sf-empty">
      {title ? <h3>{title}</h3> : null}
      {message ? <p>{message}</p> : null}
      {children}
    </div>
  );
}

/** Renders an API failure as a translated message. */
export function ErrorMessage({ error }) {
  const { tError } = useI18n();
  if (!error) return null;
  return <p className="sf-error">{typeof error === "string" ? error : tError(error)}</p>;
}
