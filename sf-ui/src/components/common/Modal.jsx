import { useEffect } from "react";

import { useI18n } from "../../i18n/I18nContext";

/** A dialog. Escape closes it, and a click on the backdrop does too. */
export default function Modal({ title, onClose, children, actions, wide = false }) {
  useEffect(() => {
    const onKeyDown = (event) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  return (
    <div className="sf-modal-backdrop" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <div className="sf-modal" style={wide ? { maxWidth: 760 } : undefined} role="dialog" aria-modal="true">
        <div className="sf-row-between" style={{ marginBottom: "0.9rem" }}>
          <h2 style={{ margin: 0 }}>{title}</h2>
        </div>
        {children}
        {actions ? <div className="sf-modal-actions">{actions}</div> : null}
      </div>
    </div>
  );
}

/** A yes/no dialog, for the destructive actions that deserve one. */
export function ConfirmModal({ title, message, confirmLabel, onConfirm, onClose, danger = true }) {
  const { t } = useI18n();
  return (
    <Modal
      title={title}
      onClose={onClose}
      actions={
        <>
          <button type="button" className="sf-button sf-button-secondary" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button type="button" className={`sf-button ${danger ? "sf-button-danger" : ""}`} onClick={onConfirm}>
            {confirmLabel || t("common.confirm")}
          </button>
        </>
      }
    >
      <p style={{ margin: 0 }}>{message}</p>
    </Modal>
  );
}
