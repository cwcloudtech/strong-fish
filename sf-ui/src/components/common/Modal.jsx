import { useEffect } from "react";

import { useI18n } from "../../i18n/I18nContext";

/**
 * A dialog with cwclock's structure: a header carrying the title and a close
 * button, a scrolling body, and a footer holding the actions. Escape closes it,
 * and so does a click on the backdrop.
 */
export default function Modal({ title, onClose, children, actions, wide = false }) {
  const { t } = useI18n();

  useEffect(() => {
    const onKeyDown = (event) => event.key === "Escape" && onClose?.();
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  return (
    <div className="sf-modal-backdrop" onMouseDown={onClose}>
      <div
        className="sf-modal"
        style={wide ? { maxWidth: 760 } : undefined}
        role="dialog"
        aria-modal="true"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="sf-modal-header">
          <h3 className="sf-modal-title">{title}</h3>
          <button type="button" className="sf-modal-close" onClick={onClose} aria-label={t("common.close")}>
            &times;
          </button>
        </div>
        <div className="sf-modal-body">{children}</div>
        {actions ? <div className="sf-modal-footer">{actions}</div> : null}
      </div>
    </div>
  );
}

/** A yes/no dialog, for the destructive actions that deserve one. */
export function ConfirmModal({ title, message, confirmLabel, onConfirm, onClose, danger = true, busy = false }) {
  const { t } = useI18n();

  return (
    <Modal
      title={title}
      onClose={onClose}
      actions={
        <>
          <button type="button" className="sf-button sf-button-secondary" onClick={onClose} disabled={busy}>
            {t("common.cancel")}
          </button>
          <button
            type="button"
            className={`sf-button ${danger ? "sf-button-danger" : ""}`}
            onClick={onConfirm}
            disabled={busy}
          >
            {confirmLabel || t("common.confirm")}
          </button>
        </>
      }
    >
      {typeof message === "string" ? <p style={{ margin: 0 }}>{message}</p> : message}
    </Modal>
  );
}
