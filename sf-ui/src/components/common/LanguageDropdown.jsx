import { FiGlobe } from "react-icons/fi";

import Dropdown from "./Dropdown";
import { useI18n } from "../../i18n/I18nContext";

/** The language picker, in the shared dropdown's shape. */
export default function LanguageDropdown({ variant = "light", align = "right" }) {
  const { locale, setLocale, locales, t } = useI18n();

  return (
    <Dropdown
      icon={<FiGlobe />}
      value={locale}
      onChange={setLocale}
      variant={variant}
      align={align}
      ariaLabel={t("common.language")}
      options={locales.map((option) => ({
        value: option.code,
        code: option.code.toUpperCase(),
        label: option.label,
      }))}
    />
  );
}
