import { useEffect, useState } from "react";
import { FiSmartphone } from "react-icons/fi";
// The auth footer names the platform rather than the act: "download" could be
// anything, and what is on offer is specifically the Android build.
import { FaAndroid } from "react-icons/fa";

import Modal from "./Modal";
import Tooltip from "./Tooltip";
import isAndroidDevice from "../../utils/isAndroidDevice";
import { mobileApp } from "../../api/services";
import { useI18n } from "../../i18n/I18nContext";

/**
 * Where this deployment's Android build lives, and a QR code of that link.
 *
 * Both come from the API rather than being assembled here, because the link
 * has to be absolute to survive being scanned on a phone, and the API is what
 * knows its own public URLs. A deployment that publishes no mobile build
 * answers 404, which leaves this null and hides the control entirely rather
 * than offering a link that goes nowhere.
 */
export function useMobileApp() {
  const [app, setApp] = useState(null);

  useEffect(() => {
    let cancelled = false;
    mobileApp
      .get()
      .then((data) => !cancelled && setApp(data))
      .catch(() => !cancelled && setApp(null));
    return () => {
      cancelled = true;
    };
  }, []);

  return app;
}

/** The QR code, for a screen that can't install the app itself. */
export function DownloadAppModal({ app, onClose }) {
  const { t } = useI18n();

  return (
    <Modal title={t("nav.downloadApp")} onClose={onClose}>
      <p className="sf-muted" style={{ marginTop: 0, textAlign: "center" }}>
        {t("nav.downloadAppScan")}
      </p>
      {app?.qrCodePng ? (
        <div style={{ display: "flex", justifyContent: "center", padding: "var(--sf-spacing) 0" }}>
          <img src={app.qrCodePng} alt="" width={220} height={220} />
        </div>
      ) : null}
      <p className="sf-muted" style={{ textAlign: "center", margin: 0, wordBreak: "break-all" }}>
        <a href={app?.url} target="_blank" rel="noopener noreferrer">
          {app?.url}
        </a>
      </p>
    </Modal>
  );
}

/**
 * The "get the app" control.
 *
 * On Android it is a plain link, because tapping it installs the app. Anywhere
 * else that link would be useless, so it opens the QR code instead - which is
 * the same decision cwclock's navbar makes.
 */
export default function DownloadAppButton({ className = "sf-nav-link", collapsed = false }) {
  const { t } = useI18n();
  const app = useMobileApp();
  const [showQr, setShowQr] = useState(false);

  if (!app?.url) return null;

  const label = t("nav.downloadApp");
  const icon = <FiSmartphone />;

  return (
    <>
      <Tooltip label={collapsed ? label : null} position="right">
        {isAndroidDevice() ? (
          <a className={className} href={app.url}>
            {icon}
            <span className="sf-nav-label">{label}</span>
          </a>
        ) : (
          <button
            type="button"
            className={className}
            onClick={() => setShowQr(true)}
            style={{ background: "none", border: 0, cursor: "pointer", width: "100%", textAlign: "left" }}
          >
            {icon}
            <span className="sf-nav-label">{label}</span>
          </button>
        )}
      </Tooltip>

      {showQr ? <DownloadAppModal app={app} onClose={() => setShowQr(false)} /> : null}
    </>
  );
}

/** The same control as a standalone icon, for the logged-out auth screens. */
export function DownloadAppIcon() {
  const { t } = useI18n();
  const app = useMobileApp();
  const [showQr, setShowQr] = useState(false);

  if (!app?.url) return null;

  return (
    <>
      <Tooltip label={t("nav.downloadApp")} position="top">
        {isAndroidDevice() ? (
          <a className="sf-auth-footer-icon" href={app.url} aria-label={t("nav.downloadApp")}>
            <FaAndroid />
          </a>
        ) : (
          <button
            type="button"
            className="sf-auth-footer-icon"
            onClick={() => setShowQr(true)}
            aria-label={t("nav.downloadApp")}
          >
            <FaAndroid />
          </button>
        )}
      </Tooltip>

      {showQr ? <DownloadAppModal app={app} onClose={() => setShowQr(false)} /> : null}
    </>
  );
}
