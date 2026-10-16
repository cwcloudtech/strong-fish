import DeviceLinks from "../../components/devices/DeviceLinks";
import { useI18n } from "../../i18n/I18nContext";

/**
 * Steps, and the account they come from.
 *
 * A screen of its own rather than a panel inside Settings. Connecting a watch
 * is a setting, but reading what it counted is something a member comes back
 * to - and a chart buried three cards down a settings page is one nobody
 * finds. The connection lives here too, next to the thing it produces.
 */
export default function Steps() {
  const { t } = useI18n();

  return (
    <div className="sf-page">
      <div className="sf-page-header">
        <h1 className="sf-title">{t("nav.healthData")}</h1>
      </div>
      <DeviceLinks />
    </div>
  );
}
