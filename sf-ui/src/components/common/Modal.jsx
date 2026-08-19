import { useEffect } from "react";
import { createPortal } from "react-dom";

import { useI18n } from "../../i18n/I18nContext";

/**
 * A dialog with cwclock's structure: a header carrying the title and a close
 * button, a scrolling body, and a footer holding the actions. Escape closes it,
 * and so does a click on the backdrop.
 *
 * It is portaled to <body>, and that is load-bearing rather than tidy. A dialog
 * opened from the sidebar used to render inside it, and `position: sticky`
 * creates a stacking context whether or not it carries a z-index - so the whole
 * dialog was trapped at the sidebar's level in the paint order, underneath the
 * content column that follows it. A post's video embed drew straight over the
 * top of it.
 *
 * No z-index could have fixed that: a value only orders siblings within their
 * own stacking context, and the dialog's problem was which context it was in.
 * Rendering at the document root is what lets its z-index mean what it says.
 */
export default function Modal({ title, onClose, children, actions, wide = false }) {
  const { t } = useI18n();

  useEffect(() => {
    const onKeyDown = (event) => event.key === "Escape" && onClose?.();
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  return createPortal(
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
    </div>,
    document.body
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
