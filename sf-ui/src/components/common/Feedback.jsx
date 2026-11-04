import { FiCopy } from "react-icons/fi";

import Logo from "./Logo";
import { useI18n } from "../../i18n/I18nContext";

/** The loading indicator used while a screen's first fetch is in flight. */
export function Spinner() {
  return <div className="sf-spinner" aria-label="loading" />;
}

/**
 * The "nothing here" state. Like cwclock's, it leads with the app's mark rather
 * than a generic icon, so an empty screen still looks like part of the product.
 */
export function EmptyState({ title, message, children }) {
  return (
    <div className="sf-empty">
      <Logo mark alt="" />
      {title ? <h3>{title}</h3> : null}
      {message ? <p>{message}</p> : null}
      {children}
    </div>
  );
}

/** Renders an API failure as a translated message, with the raw upstream
 * error below it in a console when the server sent one (a provider that could
 * not be reached), so an operator can see the status and body. */
export function ErrorMessage({ error }) {
  const { tError } = useI18n();
  if (!error) return null;
  return (
    <>
      <p className="sf-error">{typeof error === "string" ? error : tError(error)}</p>
      {typeof error === "string" ? null : <DebugConsole error={error} />}
    </>
  );
}

/** The status and body of an upstream failure, collapsed by default. Shown
 * only when the API attached a `detail` (the "could not be reached" case). */
export function DebugConsole({ error }) {
  const { t } = useI18n();
  const detail = error?.response?.data?.detail;
  if (!detail) return null;
  const status = error?.response?.status;
  const provider = error?.response?.data?.provider;
  const text = [
    provider ? `provider: ${provider}` : null,
    status ? `HTTP ${status}` : null,
    detail,
  ]
    .filter(Boolean)
    .join("\n");
  const copy = () => {
    try {
      navigator.clipboard?.writeText(text);
    } catch (_) {
      // Clipboard blocked (insecure context, denied): nothing to do but leave
      // the member to select the text themselves.
    }
  };
  return (
    <details className="sf-console">
      <summary>{t("common.troubleshoot")}{provider ? ` \u2014 ${provider}` : ""}</summary>
      <div className="sf-console-bar">
        <button type="button" className="sf-console-copy" onClick={copy} title={t("common.copy")}>
          <FiCopy /> {t("common.copy")}
        </button>
      </div>
      <pre className="sf-console-body">{text}</pre>
    </details>
  );
}
