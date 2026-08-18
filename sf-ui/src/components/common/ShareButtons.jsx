import { toast } from "react-toastify";
import { FiLink } from "react-icons/fi";

import Tooltip from "./Tooltip";
import toastOptions from "../../utils/toastOptions";
import { SOCIAL_NETWORKS } from "../../utils/socialNetworks";
import { useI18n } from "../../i18n/I18nContext";

/**
 * A row of share buttons, driven entirely by SOCIAL_NETWORKS - this component
 * has no knowledge of which networks exist.
 *
 * Networks with no web share intent (Instagram, TikTok) copy the link and say
 * so. A button that opened a composer which cannot be prefilled would look like
 * it worked and quietly drop the link.
 */
export default function ShareButtons({ url, text, label }) {
  const { t } = useI18n();

  const copy = (network) =>
    navigator.clipboard
      ?.writeText(url)
      .then(() => toast.info(t("share.copiedFor", { network: network.label }), toastOptions))
      .catch(() => toast.error(t("errors.copyFailed"), toastOptions));

  const open = (network) => {
    const target = network.share?.(url, text);
    if (!target) {
      copy(network);
      return;
    }
    // noopener matters here: the share window is somebody else's origin, and
    // without it that page can navigate this one through window.opener.
    window.open(target, "_blank", "noopener,noreferrer,width=600,height=520");
  };

  return (
    <div className="sf-share">
      {label ? <span className="sf-share-label">{label}</span> : null}

      {SOCIAL_NETWORKS.map((network) => (
        <Tooltip
          key={network.id}
          label={network.share ? t("share.on", { network: network.label }) : t("share.copyFor", { network: network.label })}
        >
          <button
            type="button"
            className="sf-share-button"
            onClick={() => open(network)}
            aria-label={t("share.on", { network: network.label })}
          >
            <network.Icon />
          </button>
        </Tooltip>
      ))}

      <Tooltip label={t("share.copyLink")}>
        <button
          type="button"
          className="sf-share-button"
          onClick={() =>
            navigator.clipboard
              ?.writeText(url)
              .then(() => toast.success(t("common.copied"), toastOptions))
              .catch(() => toast.error(t("errors.copyFailed"), toastOptions))
          }
          aria-label={t("share.copyLink")}
        >
          <FiLink />
        </button>
      </Tooltip>
    </div>
  );
}
