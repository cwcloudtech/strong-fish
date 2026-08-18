import { useCallback, useEffect, useState } from "react";
import { toast } from "react-toastify";
import { FiCopy, FiDownload, FiEye, FiEyeOff, FiTrash2 } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import Tooltip from "../../components/common/Tooltip";
import Modal, { ConfirmModal } from "../../components/common/Modal";
import { apiKeys as apiKeysApi } from "../../api/services";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

const EMPTY_FORM = { description: "", expiresAt: "" };

/**
 * A member's API keys: credentials that authenticate a script - or the mobile
 * app, enrolled by scanning a QR code - with an `X-Api-Key` header instead of
 * a session.
 *
 * The plaintext token exists exactly once, in the response to the call that
 * created it: the API keeps only its sha256. That is why creating one opens a
 * modal that will not come back, and why the QR code and the config file are
 * built from the token held in this component's state rather than fetched by
 * key id - there is nothing on the server left to fetch.
 */
export default function ApiKeys() {
  const { t } = useI18n();
  const [keys, setKeys] = useState(null);
  const [error, setError] = useState(null);
  const [form, setForm] = useState(EMPTY_FORM);
  const [busy, setBusy] = useState(false);
  const [created, setCreated] = useState(null);
  const [qrCode, setQrCode] = useState(null);
  const [qrBusy, setQrBusy] = useState(false);
  const [deleting, setDeleting] = useState(null);

  const load = useCallback(async () => {
    try {
      setKeys(await apiKeysApi.list());
    } catch (err) {
      setError(err);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const set = (field) => (event) => setForm((current) => ({ ...current, [field]: event.target.value }));

  const submit = async (event) => {
    event.preventDefault();
    setBusy(true);
    try {
      const key = await apiKeysApi.create({
        description: form.description,
        // A date alone means "valid through that whole day", so the expiry is
        // the end of it rather than its midnight.
        expiresAt: form.expiresAt ? `${form.expiresAt}T23:59:59.000Z` : null,
      });
      setCreated(key);
      setForm(EMPTY_FORM);
      await load();
    } catch (err) {
      toast.error(typeof err === "string" ? err : t("errors.generic"), toastOptions);
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    try {
      await apiKeysApi.remove(deleting.id);
      toast.success(t("apiKeys.revoked"), toastOptions);
      await load();
    } catch (err) {
      setError(err);
    } finally {
      setDeleting(null);
    }
  };

  const copy = () => {
    navigator.clipboard
      ?.writeText(created.token)
      .then(() => toast.success(t("common.copied"), toastOptions))
      .catch(() => toast.error(t("errors.copyFailed"), toastOptions));
  };

  const toggleQr = async () => {
    if (qrCode) {
      setQrCode(null);
      return;
    }
    setQrBusy(true);
    try {
      const { qrCodePng } = await apiKeysApi.configQr(created.token);
      setQrCode(qrCodePng);
    } catch (err) {
      setError(err);
    } finally {
      setQrBusy(false);
    }
  };

  const downloadConfig = async () => {
    try {
      const blob = await apiKeysApi.configFile(created.token);
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = "strong-fish.conf";
      link.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err);
    }
  };

  const closeCreated = () => {
    setCreated(null);
    setQrCode(null);
  };

  return (
    <div className="sf-page" style={{ maxWidth: 820 }}>
      <h1 className="sf-title">{t("apiKeys.title")}</h1>
      <p className="sf-muted">{t("apiKeys.intro")}</p>

      <div className="sf-card">
        <h3 style={{ marginTop: 0 }}>{t("apiKeys.create")}</h3>
        <form onSubmit={submit}>
          <div className="sf-row" style={{ gap: "0.6rem", alignItems: "flex-end" }}>
            <div className="sf-field" style={{ flex: 2, minWidth: 200 }}>
              <label className="sf-label" htmlFor="description">
                {t("apiKeys.description")}
              </label>
              <input
                id="description"
                className="sf-input"
                value={form.description}
                onChange={set("description")}
                placeholder={t("apiKeys.descriptionPlaceholder")}
                required
              />
            </div>
            <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
              <label className="sf-label" htmlFor="expiresAt">
                {t("apiKeys.expiresAt")} <span className="sf-muted">({t("common.optional")})</span>
              </label>
              <input
                id="expiresAt"
                className="sf-input"
                type="date"
                value={form.expiresAt}
                onChange={set("expiresAt")}
              />
            </div>
          </div>
          <button className="sf-button" type="submit" disabled={busy || !form.description}>
            {busy ? t("common.loading") : t("common.create")}
          </button>
        </form>
      </div>

      <ErrorMessage error={error} />

      {keys === null ? (
        <Spinner />
      ) : keys.length === 0 ? (
        <EmptyState title={t("apiKeys.emptyTitle")} message={t("apiKeys.emptyBody")} />
      ) : (
        <ul className="sf-list">
          {keys.map((key) => (
            <li className="sf-list-item" key={key.id}>
              <div style={{ flex: 1, minWidth: 0 }}>
                <strong>{key.description}</strong>
                <div className="sf-muted" style={{ fontSize: "0.85rem" }}>
                  {key.expiresAt
                    ? t("apiKeys.expiresOn", { date: new Date(key.expiresAt).toLocaleDateString() })
                    : t("apiKeys.neverExpires")}
                </div>
              </div>
              <Tooltip label={t("apiKeys.revoke")}>
                <button
                  type="button"
                  className="sf-icon-button sf-icon-button-plain"
                  onClick={() => setDeleting(key)}
                  aria-label={t("apiKeys.revoke")}
                >
                  <FiTrash2 />
                </button>
              </Tooltip>
            </li>
          ))}
        </ul>
      )}

      {created ? (
        <Modal title={t("apiKeys.createdTitle")} onClose={closeCreated}>
          <p className="sf-notice sf-notice-warning">{t("apiKeys.createdWarning")}</p>

          <div className="sf-token-box">
            <code className="sf-token">{created.token}</code>
            <Tooltip label={t("common.copy")}>
              <button type="button" className="sf-icon-button sf-icon-button-plain" onClick={copy}>
                <FiCopy />
              </button>
            </Tooltip>
            <Tooltip label={t("apiKeys.downloadConfig")}>
              <button type="button" className="sf-icon-button sf-icon-button-plain" onClick={downloadConfig}>
                <FiDownload />
              </button>
            </Tooltip>
            <Tooltip label={qrCode ? t("apiKeys.hideQr") : t("apiKeys.showQr")}>
              <button
                type="button"
                className="sf-icon-button sf-icon-button-plain"
                onClick={toggleQr}
                disabled={qrBusy}
              >
                {qrCode ? <FiEyeOff /> : <FiEye />}
              </button>
            </Tooltip>
          </div>

          {qrCode ? (
            <>
              <p className="sf-muted" style={{ textAlign: "center", marginBottom: 0 }}>
                {t("apiKeys.qrHelp")}
              </p>
              <div style={{ display: "flex", justifyContent: "center", padding: "var(--sf-spacing) 0" }}>
                <img src={qrCode} alt="" width={220} height={220} />
              </div>
            </>
          ) : null}
        </Modal>
      ) : null}

      {deleting ? (
        <ConfirmModal
          title={t("apiKeys.revokeTitle")}
          message={t("apiKeys.revokeBody", { description: deleting.description })}
          confirmLabel={t("apiKeys.revoke")}
          onConfirm={remove}
          onClose={() => setDeleting(null)}
        />
      ) : null}
    </div>
  );
}
