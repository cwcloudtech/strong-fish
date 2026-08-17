import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";

const PROVIDER_LABELS = { google: "Google", github: "GitHub", keycloak: "Keycloak" };

/**
 * One button per OIDC provider the deployment has configured. The list comes
 * from GET /v1/config, so a single build works against any configuration -
 * nothing is shown when no provider is set up.
 */
export default function OidcButtons() {
  const { t } = useI18n();
  const { config } = useAuth();
  const providers = config?.oidcProviders || [];

  if (providers.length === 0) return null;

  // The API redirects the browser to the provider and handles the callback
  // itself, handing the session back to /oidc/callback - so this is a plain
  // navigation, not an XHR.
  const apiURL = (process.env.REACT_APP_APIURL || "").replace(/\/+$/, "");

  return (
    <>
      <div className="sf-divider">{t("auth.orContinueWith")}</div>
      <div className="sf-oidc-buttons">
        {providers.map((provider) => (
          <a
            key={provider}
            className="sf-button sf-button-secondary"
            href={`${apiURL}/v1/oidc/${provider}/login`}
          >
            {PROVIDER_LABELS[provider] || provider}
          </a>
        ))}
      </div>
    </>
  );
}
